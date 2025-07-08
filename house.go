package main

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/bihua-university/alisten/internal/music"
	"github.com/bihua-university/alisten/internal/syncx"

	"github.com/google/uuid"
)

type Mode int

const (
	NormalMode Mode = iota
	RandomMode
)

type House struct {
	Mu         sync.Mutex
	Name       string
	Desc       string
	Password   string
	Mode       Mode
	Current    Order
	End        time.Time
	PushTime   int64
	Playlist   []Order
	VoteSkip   []string
	Connection []*Connection

	// private
	persist        bool
	lastActiveTime time.Time
	queue          syncx.UnboundedChan[[]byte]
	close          chan struct{}
}

var housesMu sync.Mutex
var houses = make(map[string]*House)

func addHouseHTTP(w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		Name     string `json:"name"`
		Desc     string `json:"desc"`
		NeedPwd  bool   `json:"needPwd"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		writeJSON(w, http.StatusBadRequest, H{"error": err.Error()})
		return
	}

	houseID := uuid.New().String()
	createHouse(houseID, requestBody.Name, requestBody.Desc, requestBody.Password, false)
	writeJSON(w, http.StatusOK, H{"code": "20000", "message": "房间创建成功", "data": houseID})
}

func createHouse(houseID string, name, desc, password string, persist bool) {
	house := &House{
		Name:     name,
		Desc:     desc,
		Password: password,
		Mode:     NormalMode,
		Playlist: make([]Order, 0),
		VoteSkip: make([]string, 0),

		persist:        persist,
		lastActiveTime: time.Now(),
		queue:          syncx.NewUnboundedChan[[]byte](8),
		close:          make(chan struct{}),
	}
	housesMu.Lock()
	houses[houseID] = house
	housesMu.Unlock()

	house.Start()
}

func GetHouse(id string) *House {
	housesMu.Lock()
	defer housesMu.Unlock()
	return houses[id]
}

func enterHouseHTTP(w http.ResponseWriter, r *http.Request) {
	var request struct {
		HouseID  string `json:"id"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, H{"error": err.Error()})
		return
	}

	house, exists := houses[request.HouseID]
	if !exists {
		writeJSON(w, http.StatusNotFound, H{"error": "房间不存在"})
		return
	}

	if house.Password != request.Password {
		writeJSON(w, http.StatusUnauthorized, H{"error": "密码错误"})
		return
	}
	writeJSON(w, http.StatusOK, H{"code": "20000", "message": "进入房间成功", "data": request.HouseID})
}

func searchHousesHTTP(w http.ResponseWriter, r *http.Request) {
	var response []map[string]interface{}
	housesMu.Lock()
	for houseId, house := range houses {
		house.Mu.Lock()
		response = append(response, map[string]interface{}{
			"id":           houseId,
			"name":         house.Name,
			"desc":         house.Desc,
			"population":   len(house.Connection),
			"createTime":   time.Now().UnixMilli(),
			"needPwd":      house.Password != "",
			"enableStatus": true,
		})
		house.Mu.Unlock()
	}
	housesMu.Unlock()

	writeJSON(w, http.StatusOK, H{"code": "20000", "message": "房间列表", "data": response})
}

func (h *House) lock(fn func()) {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	fn()
}

func (h *House) Start() {
	ticker := time.NewTicker(time.Millisecond * 500)
	go func() {
		for {
			select {
			case <-h.close:
				ticker.Stop()
				return
			case j := <-h.queue.Out():
				h.lock(func() {
					for _, conn := range h.Connection {
						conn.SendRaw(j)
					}
				})
			case <-ticker.C:
				h.Update()
			}
		}
	}()
}

func (h *House) Broadcast(msg any) {
	j := encJson(msg)
	h.queue.In() <- j
}

func (h *House) Update() {
	skip := false
	cleanup := false
	h.lock(func() {
		// no song to play
		if h.Current.id == "" || h.End.Before(time.Now()) {
			skip = true
		}
		// 检查是否需要清理房间
		if len(h.Connection) == 0 && !h.persist && time.Since(h.lastActiveTime) > 5*time.Minute {
			cleanup = true
		}
	})

	if cleanup {
		h.Close()
		return
	}

	if skip {
		h.Skip() // 切歌
	}
}

func (h *House) Push(o Order) {
	m := music.GetMusic(o.source, o.id, false)
	duration, ok := m["duration"].(int64)
	if !ok {
		return
	}

	var r H
	h.lock(func() {
		now := time.Now()
		h.PushTime = now.Add(200 * time.Millisecond).UnixMilli() // 200ms delay
		h.End = now.Add(time.Duration(duration) * time.Millisecond)
		r = merge(m, H{
			"pushTime": h.PushTime,
		})
	})

	h.Broadcast(r)
}

func (h *House) enter(c *Connection) {
	var list []H
	var u []string
	h.lock(func() {
		if h.Current.id != "" {
			// 发送播放单曲
			m := music.GetMusic(h.Current.source, h.Current.id, false)
			r := merge(m, H{
				"pushTime": h.PushTime,
			})
			c.Send(r)
		}
		list = h.playlist()
		for _, conn := range h.Connection {
			u = append(u, conn.user)
		}
	})
	// 推送播放列表
	c.Send(H{
		"type": "pick",
		"data": list,
	})
	h.Broadcast(H{
		"type": "house_user",
		"data": u,
	})
}

func (h *House) playlist() []H {
	var list []H

	// playlist don't need this information
	push := func(o Order) {
		if o.id == "" {
			return
		}
		m := music.GetMusic(o.source, o.id, true)
		keep := []string{"type", "source", "artist", "duration", "name", "album", "pictureUrl", "webUrl"}
		r := make(H, len(keep)+1)
		for _, k := range keep {
			if v, ok := m[k]; ok {
				r[k] = v
			}
		}
		r["nickName"] = o.user
		list = append(list, r)
	}

	push(h.Current)
	for _, o := range h.Playlist {
		push(o)
	}
	return list
}

func (h *House) PushPlaylist() {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	list := h.playlist()
	online := len(h.Connection)
	h.Broadcast(H{
		"type":         "pick",
		"data":         list,
		"online_count": online,
	})
}

func (h *House) Skip() {
	var play Order
	change := false
	h.lock(func() {
		if len(h.Playlist) == 0 {
			return
		}
		switch h.Mode {
		case NormalMode:
			h.Current = h.Playlist[0]
			h.Playlist = h.Playlist[1:]
		case RandomMode:
			choose := rand.IntN(len(h.Playlist))
			h.Current = h.Playlist[choose]
			h.Playlist = append(h.Playlist[:choose], h.Playlist[choose+1:]...)
		default:
			// nothing
		}
		play = h.Current
		change = true
	})
	if change {
		h.Push(play)
		h.PushPlaylist()
	}
}

func (h *House) Leave(c *Connection) {
	var u []string
	h.lock(func() {
		// 移除连接
		for i, conn := range h.Connection {
			if conn.conn == c.conn {
				h.Connection = append(h.Connection[:i], h.Connection[i+1:]...)
				break
			}
		}
		// 如果房间为空，更新最后活跃时间
		if len(h.Connection) == 0 {
			h.lastActiveTime = time.Now()
		}
		// 获取更新后的用户列表
		for _, conn := range h.Connection {
			u = append(u, conn.user)
		}
	})
	// 广播更新后的用户列表
	h.Broadcast(H{
		"type": "house_user",
		"data": u,
	})
}

func houseuser(c *Context) {
	var u []string
	c.WithHouse(func(h *House) {
		for _, conn := range h.Connection {
			u = append(u, conn.user)
		}
	})
	c.conn.Send(H{
		"type": "house_user",
		"data": u,
	})
}

// 关闭当前房间
func (h *House) Close() {
	// 查找当前房间的ID
	var houseID string
	housesMu.Lock()
	for id, house := range houses {
		if house == h {
			houseID = id
			break
		}
	}

	if houseID != "" {
		close(h.close)
		delete(houses, houseID)
	}
	housesMu.Unlock()
}
