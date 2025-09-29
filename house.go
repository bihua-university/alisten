package main

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/music"
	"github.com/bihua-university/alisten/internal/syncx"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type Mode int

const (
	NormalMode Mode = iota
	RandomMode
)

func (m Mode) String() string {
	switch m {
	case NormalMode:
		return "sequential"
	case RandomMode:
		return "random"
	default:
		return "unknown"
	}
}

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
	VoteSkip   []auth.User
	Connection []*Connection

	// private
	ultimate       bool
	lastActiveTime time.Time
	queue          syncx.UnboundedChan[[]byte]
	close          chan struct{}
	lastOrderTime  time.Time
	recommander    *music.NeteaseMusicRecommander

	// limiters
	searchLimiter *rate.Limiter
	orderLimiter  *rate.Limiter
	likeLimiter   *rate.Limiter
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
		writeJSON(w, http.StatusBadRequest, base.H{"error": err.Error()})
		return
	}

	houseID := uuid.New().String()
	createHouse(houseID, requestBody.Name, requestBody.Desc, requestBody.Password, false)
	writeJSON(w, http.StatusOK, base.H{"houseId": houseID})
}

func createHouse(houseID string, name, desc, password string, persist bool) {
	house := &House{
		Name:     name,
		Desc:     desc,
		Password: password,
		Mode:     NormalMode,
		Playlist: make([]Order, 0),
		VoteSkip: make([]auth.User, 0),

		ultimate:       persist,
		lastActiveTime: time.Now(),
		queue:          syncx.NewUnboundedChan[[]byte](8),
		close:          make(chan struct{}),
		recommander:    music.NewNeteaseMusicRecommander(),
	}
	if !house.ultimate {
		house.searchLimiter = rate.NewLimiter(rate.Every(time.Minute), 10)
		house.orderLimiter = rate.NewLimiter(rate.Every(time.Minute), 5)
		house.likeLimiter = rate.NewLimiter(rate.Every(time.Minute), 5)
	} else {
		house.orderLimiter = rate.NewLimiter(rate.Every(time.Minute), 30)
		house.likeLimiter = rate.NewLimiter(rate.Every(time.Minute), 30)
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
		writeJSON(w, http.StatusBadRequest, base.H{"error": err.Error()})
		return
	}

	house, exists := houses[request.HouseID]
	if !exists {
		writeJSON(w, http.StatusNotFound, base.H{"error": "房间不存在"})
		return
	}

	if house.Password != request.Password {
		writeJSON(w, http.StatusUnauthorized, base.H{"error": "密码错误"})
		return
	}
	writeJSON(w, http.StatusOK, base.H{"houseId": request.HouseID})
}

func searchHousesHTTP(w http.ResponseWriter, r *http.Request) {
	var response []map[string]interface{}
	housesMu.Lock()
	for houseId, house := range houses {
		house.Mu.Lock()
		response = append(response, map[string]interface{}{
			"id":         houseId,
			"name":       house.Name,
			"desc":       house.Desc,
			"population": len(house.Connection),
			"createTime": time.Now().UnixMilli(),
			"needPwd":    house.Password != "",
			"ultimate":   house.ultimate,
		})
		house.Mu.Unlock()
	}
	housesMu.Unlock()

	writeJSON(w, http.StatusOK, response)
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
	h.lock(func() {
		// no song to play
		if h.Current.id == "" || h.End.Before(time.Now()) {
			skip = true
		}
		// 检查是否需要清理房间
		if len(h.Connection) == 0 && !h.ultimate && time.Since(h.lastActiveTime) > 5*time.Minute {
			h.closeHouse()
		}
	})

	if skip {
		h.Skip(false) // 切歌
	}
}

func (h *House) Push(o Order) {
	m := music.GetMusic(o.source, o.id, false)
	duration, ok := m["duration"].(int64)
	if !ok {
		return
	}

	var r base.H
	h.lock(func() {
		now := time.Now()
		h.PushTime = now.Add(200 * time.Millisecond).UnixMilli() // 200ms delay
		h.End = now.Add(time.Duration(duration) * time.Millisecond)
		r = merge(m, base.H{
			"pushTime": h.PushTime,
		})
	})

	h.Broadcast(r)
}

func (h *House) enter(c *Connection) {
	var list []base.H
	var u []auth.User
	h.lock(func() {
		if h.Current.id != "" {
			// 发送播放单曲
			m := music.GetMusic(h.Current.source, h.Current.id, false)
			r := merge(m, base.H{
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
	c.Send(base.H{
		"type": "pick",
		"data": list,
	})
	h.Broadcast(base.H{
		"type": "house_user",
		"data": u,
	})
}

func (h *House) playlist() []base.H {
	var list []base.H

	// playlist don't need this information
	push := func(o Order) {
		if o.id == "" {
			return
		}
		m := music.GetMusic(o.source, o.id, true)
		keep := []string{"type", "source", "artist", "duration", "name", "album", "pictureUrl", "webUrl"}
		r := make(base.H, len(keep)+1)
		for _, k := range keep {
			if v, ok := m[k]; ok {
				r[k] = v
			}
		}
		r["user"] = o.user
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
	h.Broadcast(base.H{
		"type":         "pick",
		"data":         list,
		"online_count": online,
	})
}

func (h *House) Skip(force bool) {
	var play Order
	change := false
	h.lock(func() {
		// Outer check cannot guarantee that we need to skip
		// because the current song may be updated by another goroutine.
		// Double check is needed to avoid skipping twice.
		if !force && h.Current.id != "" && h.End.After(time.Now()) {
			return
		}
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
		h.VoteSkip = nil
		change = true
	})
	if change {
		h.Push(play)
		if play.source == "wy" {
			if play.user == (auth.User{Name: "系统推荐"}) {
				// system recommended music, also mark it to avoid recommend again
				h.recommander.Mark(play.id)
			} else {
				h.recommander.AddHistory(play.id)
			}
		}
		h.lock(func() {
			if len(h.Playlist) > 0 || h.lastOrderTime.Add(10*time.Second).After(time.Now()) || len(h.Connection) == 0 {
				return
			}
			// recommend a music
			list := h.recommander.Recommend(nil)
			choose := rand.IntN(len(list))
			h.Playlist = append(h.Playlist, Order{source: "wy", id: list[choose], user: auth.User{Name: "系统推荐"}})
		})
		h.PushPlaylist()
	}
}

func (h *House) Leave(c *Connection) {
	var u []auth.User
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

		// free
		close(c.send.In())
	})
	// 广播更新后的用户列表
	h.Broadcast(base.H{
		"type": "house_user",
		"data": u,
	})
}

func houseuser(c *Context) {
	// 初始化为非 nil 的空切片，确保通过 HTTP 返回时为 [] 而不是 null
	u := make([]auth.User, 0)
	c.WithHouse(func(h *House) {
		for _, conn := range h.Connection {
			u = append(u, conn.user)
		}
	})

	if c.IsWebSocket() {
		c.conn.Send(base.H{
			"type": "house_user",
			"data": u,
		})
	}
	if c.IsHTTP() {
		c.Send(u)
	}
}

func settingSync(c *Context) {
	c.conn.mu.Lock()
	defer c.conn.mu.Unlock()

	c.conn.Send(base.H{
		"type": "setting/push",
		"data": base.H{
			"playmode": c.house.Mode.String(),
		},
	})
}

// 关闭当前房间
func (h *House) closeHouse() {
	// 查找当前房间的ID
	housesMu.Lock()
	for id, house := range houses {
		if house == h {
			close(h.queue.In())
			close(h.close)
			delete(houses, id)
			break
		}
	}
	housesMu.Unlock()
}

const (
	WaitSearch = 1 << iota // 搜索
	WaitOrder              // 点歌
	WaitLike               // 点赞
)

func (h *House) Wait(t uint8) bool {
	if t&WaitSearch != 0 && h.searchLimiter != nil {
		h.searchLimiter.Wait(context.Background())
	}
	if t&WaitOrder != 0 && h.orderLimiter != nil {
		if !h.orderLimiter.Allow() {
			return false
		}
	}
	if t&WaitLike != 0 && h.likeLimiter != nil {
		if !h.likeLimiter.Allow() {
			return false
		}
	}
	return true
}
