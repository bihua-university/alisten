package main

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/bihua-university/alisten/cmd/alisten/internal/musicqueue"
	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/music"
	pb "github.com/bihua-university/alisten/internal/proto/musicd"
	"github.com/bihua-university/alisten/protocol"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Provider calls stay outside the room lock. Per-room dependencies also allow
// the complete websocket contract to be exercised without external services.
type musicBackend struct {
	Load      func(context.Context, *pb.SongRequest) (*pb.Song, error)
	Search    func(context.Context, *pb.SearchRequest) (pb.MusicResult, error)
	Playlists func(context.Context, *pb.SearchRequest) (pb.PlaylistResult, error)
	SongList  func(context.Context, *pb.SearchRequest) (pb.MusicResult, error)
	Loudness  func(context.Context, string, string) (float64, error)
}

type recommender interface {
	AddHistory(string)
	Mark(string)
	Recommend(context.Context, []string) []string
}

type House struct {
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	Name       string
	Desc       string
	Password   string
	Mode       protocol.PlayMode
	Connection []*Connection

	ultimate       bool
	lastActiveTime time.Time
	close          chan struct{}
	stopped        chan struct{}
	closed         bool
	lastOrderTime  time.Time
	recommander    recommender
	backend        musicBackend
	queue          *musicqueue.Queue
	timer          *time.Timer

	searchLimiter *rate.Limiter
	orderLimiter  *rate.Limiter
	likeLimiter   *rate.Limiter
}

var housesMu sync.Mutex
var houses = make(map[string]*House)

func newHouse(name, desc, password string, persist bool) *House {
	ctx, cancel := context.WithCancel(context.Background())
	h := &House{ctx: ctx, cancel: cancel,
		Name:           name,
		Desc:           desc,
		Password:       password,
		Mode:           protocol.PlayModeSequential,
		ultimate:       persist,
		lastActiveTime: time.Now(),
		close:          make(chan struct{}),
		stopped:        make(chan struct{}),
		recommander:    music.NewRecommender(musicClient.Similar),
		backend: musicBackend{
			Load: musicClient.Load, Search: musicClient.Search,
			Playlists: musicClient.Playlists, SongList: musicClient.SongList,
			Loudness: musicClient.Loudness,
		},
	}
	h.queue = musicqueue.New()
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
	type room struct {
		id    string
		house *House
	}
	housesMu.Lock()
	rooms := make([]room, 0, len(houses))
	for id, h := range houses {
		rooms = append(rooms, room{id: id, house: h})
	}
	housesMu.Unlock()

	response := make([]base.H, 0, len(rooms))
	for _, room := range rooms {
		h := room.house
		h.mu.Lock()
		response = append(response, base.H{
			"id":         room.id,
			"name":       h.Name,
			"desc":       h.Desc,
			"population": len(h.Connection),
			"createTime": time.Now().UnixMilli(),
			"needPwd":    h.Password != "",
			"ultimate":   h.ultimate,
		})
		h.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *House) Start() {
	go func() {
		ticker := time.NewTicker(time.Minute)
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
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcastLocked(message)
}

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

// broadcastQueueLocked 广播完整队列快照，只用于增删、排序、成员变化等结构性变化。
func (h *House) broadcastQueueLocked() {
	h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queue.Snapshot(int64(len(h.Connection)))))
}

// broadcastQueueRemovedLocked 只广播离开待播队列的那一首歌，
// 客户端按 source/id 从本地列表删除，随机播放时也无需知道下标。
func (h *House) broadcastQueueRemovedLocked(track musicqueue.Track) {
	h.broadcastLocked(protocol.Event(protocol.EventQueueRemoved, protocol.SongRef{
		ID: track.ID, Source: protocol.Source(track.Source),
	}))
}

func (h *House) broadcastPlaybackLocked() {
	playback, _ := h.queue.Playback()
	h.broadcastLocked(protocol.Event(protocol.EventPlaybackUpdated, playback))
}

// broadcastTransitionLocked 只下发变化的部分：取走一首歌时广播该歌曲引用，
// 没有下一首时广播空的播放状态；两种情况都不再重复整份等待队列。
func (h *House) broadcastTransitionLocked(transition musicqueue.Transition) {
	if !transition.Changed {
		return
	}
	if transition.Track.ID == "" {
		h.broadcastPlaybackLocked()
		return
	}
	h.broadcastQueueRemovedLocked(transition.Track)
}

func (h *House) Update() {
	h.mu.Lock()
	expired := !h.closed && len(h.Connection) == 0 && !h.ultimate && time.Since(h.lastActiveTime) > 5*time.Minute
	h.mu.Unlock()
	if expired {
		h.closeHouse()
	}
}

func (h *House) stopTimerLocked() {
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
}

func (h *House) resetTimerLocked() {
	h.stopTimerLocked()
	deadline := h.queue.Deadline()
	if deadline.IsZero() {
		return
	}
	h.timer = time.AfterFunc(time.Until(deadline), func() { h.Skip(false) })
}

func (h *House) Skip(force bool) bool {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return false
	}
	transition := h.queue.Next(force, h.Mode == protocol.PlayModeRandom)
	if transition.Changed {
		h.stopTimerLocked()
	}
	h.broadcastTransitionLocked(transition)
	h.mu.Unlock()
	if transition.Changed && transition.Track.ID != "" {
		return h.push(transition.Track, transition.Generation)
	}
	return transition.Changed
}

func (h *House) push(order Order, generation uint64) bool {
	var song *pb.Song
	for {
		var err error
		song, err = h.backend.Load(h.ctx, &pb.SongRequest{Source: order.Source, Id: order.ID})
		if err == nil && song != nil && song.Url != "" {
			break
		}
		h.mu.Lock()
		if h.closed {
			h.mu.Unlock()
			return false
		}
		transition, valid := h.queue.LoadFailed(generation, h.Mode == protocol.PlayModeRandom)
		if valid {
			order, generation = transition.Track, transition.Generation
			h.stopTimerLocked()
			h.broadcastLocked(protocol.Event(protocol.EventNotice, protocol.NoticeData{
				Code: protocol.ErrorCodeProviderError, Message: "播放加载失败，无法获取音乐信息",
			}))
			h.broadcastTransitionLocked(transition)
		}
		h.mu.Unlock()
		if !valid {
			return false
		}
		if !transition.Changed || order.ID == "" {
			return transition.Changed
		}
	}
	order.Song = *song
	order.Song.Id, order.Song.Source = order.ID, order.Source
	order.Song.Duration = max(order.Song.Duration, 0)
	coefficient, err := h.backend.Loudness(h.ctx, order.Source, order.ID)
	if err != nil {
		logx.Error("music loudness RPC:", err)
		coefficient = 1
	}
	h.mu.Lock()
	applied := false
	if !h.closed {
		applied = h.queue.Apply(order, generation, coefficient)
		if applied {
			h.resetTimerLocked()
			h.broadcastPlaybackLocked()
		}
	}
	h.mu.Unlock()
	if !applied {
		return false
	}
	if music.ParseSource(order.Source) != music.NetEase {
		return true
	}
	if order.User == (auth.User{Name: "系统推荐"}) {
		h.recommander.Mark(order.ID)
	} else {
		h.recommander.AddHistory(order.ID)
	}
	h.recommendNext(generation)
	return true
}

func (h *House) recommendNext(generation uint64) {
	h.mu.Lock()
	eligible := !h.closed && h.queue.CanRecommend(generation) && len(h.Connection) > 0 &&
		h.lastOrderTime.Add(10*time.Second).Before(time.Now())
	h.mu.Unlock()
	if !eligible {
		return
	}
	ids := h.recommander.Recommend(h.ctx, nil)
	if len(ids) == 0 {
		return
	}
	id := ids[rand.IntN(len(ids))]
	song, err := h.backend.Load(h.ctx, &pb.SongRequest{Source: "netease", Id: id, UseCache: true})
	if err != nil || song == nil || song.Url == "" {
		return
	}
	order := Order{Source: "netease", ID: id, User: auth.User{Name: "系统推荐"}, Song: *song}
	order.Song.Id, order.Song.Source = id, order.Source
	h.mu.Lock()
	added := false
	if !h.closed && len(h.Connection) > 0 && h.lastOrderTime.Add(10*time.Second).Before(time.Now()) {
		added = h.queue.AddRecommendation(order, generation)
		if added {
			h.broadcastQueueLocked()
		}
	}
	h.mu.Unlock()
	if added {
		h.prewarmLoudness(h.ctx, order.Source, order.ID)
	}
}

// prewarmLoudness 在后台触发响度分析；Loudness 本身同步返回结果，异步化由调用方负责。
func (h *House) prewarmLoudness(ctx context.Context, source, id string) {
	go func() {
		if _, err := h.backend.Loudness(context.WithoutCancel(ctx), source, id); err != nil {
			logx.Error("music loudness prewarm RPC:", err)
		}
	}()
}

func (h *House) membersLocked() []auth.User {
	users := make([]auth.User, 0, len(h.Connection))
	for _, conn := range h.Connection {
		users = append(users, conn.GetUser())
	}
	return users
}

func (h *House) enter(conn *Connection) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	h.Connection = append(h.Connection, conn)
	h.lastActiveTime = time.Now()
	playback, _ := h.queue.Playback()
	mode := h.Mode
	queueEvent := protocol.Event(protocol.EventQueueUpdated, h.queue.Snapshot(int64(len(h.Connection))))
	membersEvent := protocol.Event(protocol.EventMembersUpdated, protocol.Members(h.membersLocked()))
	conn.Send(protocol.Event(protocol.EventPlaybackUpdated, playback))
	conn.Send(protocol.Event(protocol.EventSettingsUpdated, protocol.SettingsData{PlayMode: mode}))
	h.broadcastLocked(queueEvent)
	h.broadcastLocked(membersEvent)
	return true
}

func (h *House) PushMembers() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcastLocked(protocol.Event(protocol.EventMembersUpdated, protocol.Members(h.membersLocked())))
}

func (h *House) Leave(conn *Connection) {
	conn.Close()
	h.mu.Lock()
	defer h.mu.Unlock()
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
	h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queue.Snapshot(int64(len(h.Connection)))))
}

func (h *House) membersSnapshot() []auth.User {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.membersLocked()
}

func (h *House) settingsSnapshot() protocol.SettingsData {
	h.mu.Lock()
	defer h.mu.Unlock()
	return protocol.SettingsData{PlayMode: h.Mode}
}

func houseuser(c *Context) {
	users := c.house.membersSnapshot()
	if c.IsHTTP() {
		c.Send(users)
	} else {
		c.Send(protocol.Members(users))
	}
}

func settingSync(c *Context) {
	c.Send(c.house.settingsSnapshot())
}

func (h *House) closeHouse() {
	housesMu.Lock()
	for id, house := range houses {
		if house == h {
			delete(houses, id)
		}
	}
	housesMu.Unlock()

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	h.cancel()
	h.stopTimerLocked()
	close(h.close)
	connections := append([]*Connection(nil), h.Connection...)
	h.mu.Unlock()
	for _, conn := range connections {
		conn.Close()
	}
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
