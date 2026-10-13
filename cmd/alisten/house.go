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
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/music"
	"github.com/bihua-university/alisten/protocol"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Provider calls stay outside the room lock. Per-room dependencies also allow
// the complete websocket contract to be exercised without external services.
type musicBackend struct {
	Load      func(string, string, bool) (*music.Song, error)
	Search    func(music.SearchOption) (music.Result[music.Music], error)
	Playlists func(music.SearchOption) (music.Result[music.Playlist], error)
	SongList  func(music.SearchOption) (music.Result[music.Music], error)
	Loudness  func(string, string) float64
	Analyze   func(string, string)
}

type recommender interface {
	AddHistory(string)
	Mark(string)
	Recommend([]string) []string
}

type House struct {
	Mu         sync.Mutex
	Name       string
	Desc       string
	Password   string
	Mode       protocol.PlayMode
	Current    Order
	End        time.Time
	PushTime   int64
	Playlist   []Order
	VoteSkip   []auth.User
	Connection []*Connection

	ultimate       bool
	lastActiveTime time.Time
	close          chan struct{}
	stopped        chan struct{}
	closed         bool
	lastOrderTime  time.Time
	recommander    recommender
	backend        musicBackend
	playback       protocol.PlaybackData
	preparing      Order
	generation     uint64

	searchLimiter *rate.Limiter
	orderLimiter  *rate.Limiter
	likeLimiter   *rate.Limiter
}

var housesMu sync.Mutex
var houses = make(map[string]*House)

func newHouse(name, desc, password string, persist bool) *House {
	h := &House{
		Name:           name,
		Desc:           desc,
		Password:       password,
		Mode:           protocol.PlayModeSequential,
		Playlist:       make([]Order, 0),
		VoteSkip:       make([]auth.User, 0),
		ultimate:       persist,
		lastActiveTime: time.Now(),
		close:          make(chan struct{}),
		stopped:        make(chan struct{}),
		recommander:    music.NewRecommender(),
		playback:       protocol.PlaybackData{VolumeCoefficient: 1},
		backend: musicBackend{
			Load: music.GetMusic, Search: music.SearchMusic, Playlists: music.SearchPlaylist,
			SongList: music.GetSongList, Loudness: music.GetLoudnessCoefficient, Analyze: music.AnalyzeLoudnessAsync,
		},
	}
	burst := 5
	if persist {
		burst = 30
	} else {
		h.searchLimiter = rate.NewLimiter(rate.Every(time.Minute), 10)
	}
	h.orderLimiter = rate.NewLimiter(rate.Every(time.Minute), burst)
	h.likeLimiter = rate.NewLimiter(rate.Every(time.Minute), burst)
	return h
}

func addHouseHTTP(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name     string `json:"name"`
		Desc     string `json:"desc"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, base.H{"error": err.Error()})
		return
	}
	id := uuid.New().String()
	createHouse(id, request.Name, request.Desc, request.Password, false)
	writeJSON(w, http.StatusOK, base.H{"houseId": id})
}

func createHouse(id, name, desc, password string, persist bool) {
	h := newHouse(name, desc, password, persist)
	housesMu.Lock()
	houses[id] = h
	housesMu.Unlock()
	h.Start()
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
	h := GetHouse(request.HouseID)
	if h == nil {
		writeJSON(w, http.StatusNotFound, base.H{"error": "房间不存在"})
		return
	}
	if h.Password != request.Password {
		writeJSON(w, http.StatusUnauthorized, base.H{"error": "密码错误"})
		return
	}
	writeJSON(w, http.StatusOK, base.H{"houseId": request.HouseID})
}

func searchHousesHTTP(w http.ResponseWriter, r *http.Request) {
	response := make([]base.H, 0)
	housesMu.Lock()
	for id, h := range houses {
		h.lock(func() {
			response = append(response, base.H{
				"id":         id,
				"name":       h.Name,
				"desc":       h.Desc,
				"population": len(h.Connection),
				"createTime": time.Now().UnixMilli(),
				"needPwd":    h.Password != "",
				"ultimate":   h.ultimate,
			})
		})
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
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		defer close(h.stopped)
		for {
			select {
			case <-h.close:
				return
			case <-ticker.C:
				h.Update()
			}
		}
	}()
}

func (h *House) Broadcast(message any) {
	h.lock(func() { h.broadcastLocked(message) })
}

// Keep state snapshots and their publication in the same critical section so
// a slower transition cannot publish an old song after a newer one.
func (h *House) broadcastLocked(message any) {
	if h.closed {
		return
	}
	raw, err := json.Marshal(message)
	if err != nil {
		logx.Error("encode room event:", err)
		return
	}
	for _, conn := range h.Connection {
		conn.SendRaw(raw)
	}
}

func (h *House) Update() {
	expired := false
	h.lock(func() {
		expired = !h.closed && len(h.Connection) == 0 && !h.ultimate && time.Since(h.lastActiveTime) > 5*time.Minute
	})
	if expired {
		h.closeHouse()
		return
	}
	h.Skip(false)
}

// nextLocked atomically reserves a song, preventing concurrent timer/vote
// transitions from consuming the same queue or publishing an obsolete load.
func (h *House) nextLocked(force bool) (Order, uint64, bool) {
	if h.closed || h.preparing.id != "" {
		return Order{}, 0, false
	}
	if !force && h.Current.id != "" && h.End.After(time.Now()) {
		return Order{}, 0, false
	}
	if len(h.Playlist) == 0 {
		if h.Current.id == "" {
			return Order{}, 0, false
		}
		h.Current = Order{}
		h.End = time.Time{}
		h.PushTime = 0
		h.VoteSkip = nil
		h.generation++
		h.playback = protocol.PlaybackData{VolumeCoefficient: 1}
		h.broadcastLocked(protocol.Event(protocol.EventPlaybackUpdated, h.playback))
		h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queueLocked()))
		return Order{}, h.generation, true
	}
	index := 0
	if h.Mode == protocol.PlayModeRandom {
		index = rand.IntN(len(h.Playlist))
	}
	play := h.Playlist[index]
	h.Playlist = append(h.Playlist[:index], h.Playlist[index+1:]...)
	h.preparing = play
	h.generation++
	h.VoteSkip = nil
	h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queueLocked()))
	return play, h.generation, true
}

func (h *House) Skip(force bool) bool {
	var play Order
	var generation uint64
	var changed bool
	h.lock(func() {
		play, generation, changed = h.nextLocked(force)
	})
	if changed && play.id != "" {
		return h.push(play, generation)
	}
	return changed
}

func (h *House) push(order Order, generation uint64) bool {
	var song *music.Song
	for {
		var err error
		song, err = h.backend.Load(order.source, order.id, false)
		if err == nil && song != nil && song.URL != "" {
			break
		}
		var changed bool
		h.lock(func() {
			if h.closed || h.generation != generation {
				return
			}
			h.preparing = Order{}
			h.broadcastLocked(protocol.Event(protocol.EventNotice, protocol.NoticeData{
				Code: protocol.ErrorCodeProviderError, Message: "播放加载失败，无法获取音乐信息",
			}))
			// Continue the same skip through unavailable songs, even when the
			// previous song has not reached its natural end yet.
			order, generation, changed = h.nextLocked(true)
		})
		if !changed || order.id == "" {
			return changed
		}
	}
	song = proxyMusicURL(song)
	order.song = *song
	order.song.ID, order.song.Source = order.id, order.source
	order.song.Duration = max(order.song.Duration, 0)
	coefficient := h.backend.Loudness(order.source, order.id)
	applied := false
	h.lock(func() {
		if h.closed || h.generation != generation {
			return
		}
		now := time.Now()
		h.Current = order
		h.preparing = Order{}
		h.VoteSkip = nil
		h.PushTime = now.Add(200 * time.Millisecond).UnixMilli()
		h.End = time.UnixMilli(h.PushTime).Add(time.Duration(order.song.Duration) * time.Millisecond)
		playable := protocol.PlayableData(order.song)
		user := protocol.UserData(order.user)
		h.playback = protocol.PlaybackData{
			Song:              &playable,
			User:              &user,
			StartedAtMS:       h.PushTime,
			VolumeCoefficient: coefficient,
		}
		h.broadcastLocked(protocol.Event(protocol.EventPlaybackUpdated, h.playback))
		h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queueLocked()))
		applied = true
	})
	if !applied {
		return false
	}
	if music.ParseSource(order.source) != music.NetEase {
		return true
	}
	if order.user == (auth.User{Name: "系统推荐"}) {
		h.recommander.Mark(order.id)
	} else {
		h.recommander.AddHistory(order.id)
	}
	h.recommendNext(generation)
	return true
}

func (h *House) recommendNext(generation uint64) {
	eligible := false
	h.lock(func() {
		eligible = !h.closed && h.generation == generation && len(h.Playlist) == 0 && len(h.Connection) > 0 &&
			h.lastOrderTime.Add(10*time.Second).Before(time.Now())
	})
	if !eligible {
		return
	}
	ids := h.recommander.Recommend(nil)
	if len(ids) == 0 {
		return
	}
	id := ids[rand.IntN(len(ids))]
	song, err := h.backend.Load("netease", id, true)
	if err != nil || song == nil || song.URL == "" {
		return
	}
	order := Order{source: "netease", id: id, user: auth.User{Name: "系统推荐"}, song: *song}
	order.song.ID, order.song.Source = id, order.source
	added := false
	h.lock(func() {
		if h.closed || h.generation != generation || len(h.Playlist) != 0 ||
			len(h.Connection) == 0 || h.lastOrderTime.Add(10*time.Second).After(time.Now()) ||
			h.Current.id == id || h.preparing.id == id {
			return
		}
		h.Playlist = append(h.Playlist, order)
		h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queueLocked()))
		added = true
	})
	if added {
		h.backend.Analyze(order.source, order.id)
	}
}

func (h *House) membersLocked() []auth.User {
	users := make([]auth.User, 0, len(h.Connection))
	for _, conn := range h.Connection {
		users = append(users, conn.GetUser())
	}
	return users
}

func (o Order) item() protocol.QueueItem {
	song := o.song
	song.ID, song.Source = o.id, o.source
	song.Duration = max(song.Duration, 0)
	return protocol.QueueItem{Song: protocol.SongData(song), User: protocol.UserData(o.user), Likes: int64(o.likes)}
}

func (h *House) queueLocked() protocol.QueueData {
	data := protocol.QueueData{Items: make([]protocol.QueueItem, 0, len(h.Playlist)), OnlineCount: int64(len(h.Connection))}
	if h.Current.id != "" {
		current := h.Current.item()
		data.Current = &current
	}
	for _, order := range h.Playlist {
		data.Items = append(data.Items, order.item())
	}
	return data
}

func (h *House) enter(conn *Connection) bool {
	h.Mu.Lock()
	if h.closed {
		h.Mu.Unlock()
		return false
	}
	h.Connection = append(h.Connection, conn)
	h.lastActiveTime = time.Now()
	playback := h.playback
	mode := h.Mode
	queueEvent := protocol.Event(protocol.EventQueueUpdated, h.queueLocked())
	membersEvent := protocol.Event(protocol.EventMembersUpdated, protocol.Members(h.membersLocked()))
	conn.Send(protocol.Event(protocol.EventPlaybackUpdated, playback))
	conn.Send(protocol.Event(protocol.EventSettingsUpdated, protocol.SettingsData{PlayMode: mode}))
	h.broadcastLocked(queueEvent)
	h.broadcastLocked(membersEvent)
	h.Mu.Unlock()
	return true
}

func (h *House) PushMembers() {
	var event any
	h.lock(func() {
		event = protocol.Event(protocol.EventMembersUpdated, protocol.Members(h.membersLocked()))
	})
	h.Broadcast(event)
}

func (h *House) PushPlaylist() {
	h.lock(func() {
		h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queueLocked()))
	})
}

func (h *House) Leave(conn *Connection) {
	conn.Close()
	h.lock(func() {
		for i, member := range h.Connection {
			if member == conn {
				h.Connection = append(h.Connection[:i], h.Connection[i+1:]...)
				break
			}
		}
		if len(h.Connection) == 0 {
			h.lastActiveTime = time.Now()
		}
		h.broadcastLocked(protocol.Event(protocol.EventMembersUpdated, protocol.Members(h.membersLocked())))
		h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queueLocked()))
	})
}

func houseuser(c *Context) {
	var users []auth.User
	c.WithHouse(func(h *House) { users = h.membersLocked() })
	if c.IsHTTP() {
		c.Send(users)
	} else {
		c.Send(protocol.Members(users))
	}
}

func settingSync(c *Context) {
	var data protocol.SettingsData
	c.WithHouse(func(h *House) { data.PlayMode = h.Mode })
	c.Send(data)
}

func (h *House) closeHouse() {
	// Match the registry -> room lock order used by room searches.
	housesMu.Lock()
	defer housesMu.Unlock()
	h.lock(func() {
		if h.closed {
			return
		}
		h.closed = true
		close(h.close)
		for _, conn := range h.Connection {
			conn.Close()
		}
		for id, house := range houses {
			if house == h {
				delete(houses, id)
			}
		}
	})
}

const (
	WaitSearch = 1 << iota
	WaitOrder
	WaitLike
)

func (h *House) Wait(ctx context.Context, kind uint8) bool {
	if ctx.Err() != nil {
		return false
	}
	if kind&WaitSearch != 0 && h.searchLimiter != nil {
		if err := h.searchLimiter.Wait(ctx); err != nil {
			return false
		}
	}
	if kind&WaitOrder != 0 && h.orderLimiter != nil && !h.orderLimiter.Allow() {
		return false
	}
	if kind&WaitLike != 0 && h.likeLimiter != nil && !h.likeLimiter.Allow() {
		return false
	}
	return true
}
