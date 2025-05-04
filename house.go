package main

import (
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/wdvxdr1123/alisten/internal/music"
	"github.com/wdvxdr1123/alisten/internal/syncx"

	"github.com/gin-gonic/gin"
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
	queue syncx.UnboundedChan[[]byte]
	close chan struct{}
}

var housesMu sync.Mutex
var houses = make(map[string]*House)

func addHouse(c *gin.Context) {
	var requestBody struct {
		Name         string `json:"name"`
		Desc         string `json:"desc"`
		NeedPwd      bool   `json:"needPwd"`
		Password     string `json:"password"`
		EnableStatus bool   `json:"enableStatus"`
		RetainKey    string `json:"retainKey"`
	}

	err := c.ShouldBindBodyWithJSON(&requestBody)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	houseId := uuid.New().String()
	house := &House{
		Name:     requestBody.Name,
		Desc:     requestBody.Desc,
		Password: requestBody.Password,
		Mode:     NormalMode,
		Playlist: make([]Order, 0),
		VoteSkip: make([]string, 0),

		queue: syncx.NewUnboundedChan[[]byte](8),
		close: make(chan struct{}),
	}
	housesMu.Lock()
	houses[houseId] = house
	housesMu.Unlock()

	house.Start()
	c.JSON(http.StatusOK, gin.H{"code": "20000", "message": "房间创建成功", "data": houseId})
}

func GetHouse(id string) *House {
	housesMu.Lock()
	defer housesMu.Unlock()
	return houses[id]
}

func enterHouse(c *gin.Context) {
	var request struct {
		HouseID  string `json:"id"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	house, exists := houses[request.HouseID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
		return
	}

	if house.Password != request.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "20000", "message": "进入房间成功", "data": request.HouseID})
}

func searchHouses(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{"code": "20000", "message": "房间列表", "data": response})
}

/*
{
	"id": "FjOQWUKc",
	"sessionId": null,
	"name": "花样小学点歌",
	"desc": "花样小学的都来",
	"remoteAddress": null,
	"createTime": 1639756889039,
	"password": null,
	"enableStatus": null,
	"needPwd": true,
	"population": 0,
	"canDestroy": null,
	"retainKey": null,
	"announce": {
		"sessionId": "afgi3tpv",
		"content": "欢迎大家点歌",
		"nickName": "白宇(111.121.*.*)",
		"sendTime": 1643543706686,
		"pushTime": 1643543706686
	},
	"forbiddenModiPwd": null,
	"adminPwd": null
}
*/
// house/search

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
	})
	if skip {
		h.Skip()
	}
}

func (h *House) Push(o Order) {
	m := music.GetMusic(o.source, o.id)
	duration, ok := m["duration"].(int64)
	if !ok {
		return
	}

	var r gin.H
	h.lock(func() {
		now := time.Now()
		h.PushTime = now.Add(200 * time.Millisecond).UnixMilli() // 200ms delay
		h.End = now.Add(time.Duration(duration) * time.Millisecond)
		r = merge(m, gin.H{
			"pushTime": h.PushTime, // delay 500ms
		})
	})

	h.Broadcast(r)
}

func (h *House) enter(c *Connection) {
	var list []gin.H
	h.lock(func() {
		if h.Current.id != "" {
			// 发送播放单曲
			m := music.GetMusic(h.Current.source, h.Current.id)
			r := merge(m, gin.H{
				"pushTime": h.PushTime,
			})
			c.Send(r)
		}
		list = h.playlist()
	})
	// 推送播放列表
	c.Send(gin.H{
		"type": "pick",
		"data": list,
	})
}

func (h *House) playlist() []gin.H {
	var list []gin.H
	push := func(o Order) {
		if o.id == "" {
			return
		}
		m := music.GetMusic(o.source, o.id)
		list = append(list, merge(m, gin.H{
			"nickName": o.user,
		}))
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
	h.Broadcast(gin.H{
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

func houseuser(c *Context) {
	var u []string
	c.WithHouse(func(h *House) {
		for _, conn := range h.Connection {
			u = append(u, conn.user)
		}
	})
	c.conn.Send(gin.H{
		"type": "house_user",
		"data": u,
	})
}
