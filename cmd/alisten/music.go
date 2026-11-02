package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bihua-university/alisten/cmd/alisten/internal/musicqueue"
	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/music"
	pb "github.com/bihua-university/alisten/internal/proto/musicd"
	"github.com/bihua-university/alisten/protocol"
)

type Order = musicqueue.Track

func (h *House) addOrder(ctx context.Context, order Order, limit int) *protocol.Error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ctx.Err() != nil || h.closed {
		return &protocol.Error{Code: protocol.ErrorCodeUnavailable, Message: "请求已取消或房间已关闭"}
	}
	switch h.queue.Add(order, limit) {
	case musicqueue.Full:
		return &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "已超过最大点歌数量10首，请稍后再试"}
	case musicqueue.Duplicate:
		return &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "重复点歌"}
	}
	h.lastOrderTime = time.Now()
	h.broadcastQueueLocked()
	return nil
}

func (h *House) removeOrder(id, source, legacyName string, legacy bool) (Order, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return Order{}, false
	}
	removed, found := h.queue.Remove(id, source, legacyName, legacy)
	if found {
		h.broadcastQueueLocked()
	}
	return removed, found
}

func (h *House) addLike(id, source string, legacyIndex int64, legacy bool) (Order, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return Order{}, false
	}
	liked, found := h.queue.AddLike(id, source, legacyIndex, legacy)
	if found {
		h.broadcastQueueLocked()
	}
	return liked, found
}

func (h *House) vote(user auth.User) (protocol.VoteResult, musicqueue.Transition, *protocol.Error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return protocol.VoteResult{}, musicqueue.Transition{}, &protocol.Error{Code: protocol.ErrorCodeUnavailable, Message: "房间已关闭"}
	}
	result, transition, voteResult := h.queue.Vote(user, len(h.Connection), h.Mode == protocol.PlayModeRandom)
	h.broadcastTransitionLocked(transition)
	var err *protocol.Error
	switch voteResult {
	case musicqueue.VotePreparing:
		err = &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "正在切歌，请稍后再试"}
	case musicqueue.VoteNoCurrent:
		err = &protocol.Error{Code: protocol.ErrorCodeNotFound, Message: "当前没有播放音乐"}
	case musicqueue.VoteDuplicate:
		err = &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "您已经投过票了"}
	}
	return result, transition, err
}

func (h *House) setPlayMode(mode protocol.PlayMode) protocol.SettingsData {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch mode {
	case protocol.PlayModeSequential, protocol.PlayModeRandom:
		h.Mode = mode
		h.broadcastLocked(protocol.Event(protocol.EventSettingsUpdated, protocol.SettingsData{PlayMode: h.Mode}))
	}
	return protocol.SettingsData{PlayMode: h.Mode}
}

func (h *House) playbackSnapshot() (protocol.PlaybackData, Order) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.queue.Playback()
}

func (h *House) queueSnapshot() protocol.QueueData {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.queue.Snapshot(int64(len(h.Connection)))
}

func (h *House) queueItems() []Order {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.queue.Items()
}

func doPickMusic(ctx context.Context, h *House, request protocol.PickRequest, user auth.User) (protocol.PickResult, *protocol.Error) {
	fail := func(code protocol.ErrorCode, message string) (protocol.PickResult, *protocol.Error) {
		return protocol.PickResult{}, &protocol.Error{Code: code, Message: message}
	}
	source := music.ParseSource(string(request.Source))
	id, name := strings.TrimSpace(request.ID), strings.TrimSpace(request.Name)
	if id == "" {
		result, err := h.backend.Search(ctx, &pb.SearchRequest{Source: int32(source), Keyword: name, Page: 1, PageSize: 10})
		if err != nil || len(result.Data) == 0 || result.Data[0] == nil || result.Data[0].Id == "" {
			return fail(protocol.ErrorCodeProviderError, "点歌失败，无法搜索到音乐")
		}
		id = result.Data[0].Id
	}
	song, err := h.backend.Load(ctx, &pb.SongRequest{Source: source.String(), Id: id, UseCache: true})
	if err != nil || song == nil || song.Url == "" {
		return fail(protocol.ErrorCodeProviderError, "点歌失败，无法获取音乐信息")
	}
	order := Order{Source: source.String(), ID: id, User: user, Song: *song}
	order.Song.Id, order.Song.Source = id, order.Source
	order.Song.Duration = max(order.Song.Duration, 0)

	limit := 0
	if !h.ultimate {
		limit = 10
	}
	if enqueueErr := h.addOrder(ctx, order, limit); enqueueErr != nil {
		return protocol.PickResult{}, enqueueErr
	}
	h.prewarmLoudness(ctx, order.Source, order.ID)
	h.Skip(false)
	artist := song.Artist
	if artist == "" {
		artist = "unknown"
	}
	return protocol.PickResult{ID: id, Source: protocol.Source(order.Source), Name: song.Name, Artist: artist}, nil
}

func searchMusic(c *Context) {
	var request protocol.SearchRequest
	if !c.Decode(&request) || !c.Wait(WaitSearch) {
		return
	}
	option := &pb.SearchRequest{
		Source: int32(music.ParseSource(string(request.Source))), Keyword: request.Keyword,
		Page: request.PageIndex, PageSize: request.PageSize,
	}
	var result pb.MusicResult
	if strings.HasPrefix(request.Keyword, "*") {
		option.Id = strings.TrimPrefix(request.Keyword, "*")
		result, _ = c.house.backend.SongList(c.ctx, option)
	} else {
		result, _ = c.house.backend.Search(c.ctx, option)
	}
	if c.IsHTTP() {
		c.Send(base.H{"list": result.Data, "totalSize": result.Total})
	} else {
		c.Send(protocol.SearchData(result.Data, result.Total))
	}
}

func deleteMusic(c *Context) {
	if !c.Wait(WaitOrder) {
		return
	}
	var request protocol.SongRef
	legacy := c.IsHTTP() && c.Get("source").String() == ""
	if !legacy && !c.Decode(&request) {
		return
	}
	removed, found := c.house.removeOrder(
		request.ID, music.ParseSource(string(request.Source)).String(), c.Get("id").String(), legacy,
	)
	if !found {
		c.Fail(protocol.ErrorCodeNotFound, "未找到要删除的音乐")
		return
	}
	if c.IsHTTP() {
		c.Send(base.H{"name": removed.Song.Name})
	} else {
		c.Send(protocol.SongRef{ID: removed.ID, Source: protocol.Source(removed.Source)})
		c.Chat("删除音乐 " + removed.Song.Name)
	}
}

func pickMusic(c *Context) {
	var request protocol.PickRequest
	if !c.Decode(&request) || !c.Wait(WaitOrder) {
		return
	}
	result, err := doPickMusic(c.ctx, c.house, request, c.User())
	if err != nil {
		c.Fail(err.Code, err.Message)
		return
	}
	if c.IsHTTP() {
		c.Send(base.H{"name": result.Name, "artist": result.Artist, "id": result.ID, "source": string(request.Source)})
	} else {
		c.Send(result)
		c.Chat("点歌 " + result.Name)
	}
}

func voteSkip(c *Context) {
	result, transition, err := c.house.vote(c.User())
	if err != nil {
		c.Fail(err.Code, err.Message)
		return
	}
	if result.Skipped && transition.Track.ID != "" {
		result.Skipped = c.house.push(transition.Track, transition.Generation)
	}
	if c.IsHTTP() {
		c.Send(base.H{"current_votes": result.CurrentVotes, "required_votes": result.RequiredVotes})
	} else {
		c.Send(result)
		if result.Skipped {
			c.Chat("投票切歌成功")
		} else {
			c.Chat(fmt.Sprintf("投票切歌 (%d/%d)", result.CurrentVotes, result.RequiredVotes))
		}
	}
}

func goodMusic(c *Context) {
	if !c.Wait(WaitLike) {
		return
	}
	var request protocol.SongRef
	legacy := c.IsHTTP() && c.Get("source").String() == ""
	if !legacy && !c.Decode(&request) {
		return
	}
	liked, found := c.house.addLike(
		request.ID, music.ParseSource(string(request.Source)).String(), c.Get("index").Int(), legacy,
	)
	if !found {
		c.Fail(protocol.ErrorCodeNotFound, "未找到对应音乐")
		return
	}
	if c.IsHTTP() {
		c.Send(base.H{"name": liked.Song.Name, "likes": liked.Likes})
	} else {
		c.Send(protocol.LikeResult{ID: liked.ID, Source: protocol.Source(liked.Source), Likes: int64(liked.Likes)})
		c.Chat(fmt.Sprintf("%s 点赞%d", liked.Song.Name, liked.Likes))
	}
}

func searchList(c *Context) {
	var request protocol.SearchRequest
	if !c.Decode(&request) || !c.Wait(WaitSearch) {
		return
	}
	result, _ := c.house.backend.Playlists(c.ctx, &pb.SearchRequest{
		Source: int32(music.ParseSource(string(request.Source))), Keyword: request.Keyword,
		Page: request.PageIndex, PageSize: request.PageSize,
	})
	if c.IsHTTP() {
		c.Send(base.H{"list": result.Data, "totalSize": result.Total})
	} else {
		c.Send(protocol.PlaylistData(result.Data, result.Total, string(request.Source)))
	}
}

func playMode(c *Context) {
	var request protocol.SettingsPatch
	if c.IsHTTP() {
		request.PlayMode = protocol.PlayMode(c.Get("mode").String())
	} else if !c.Decode(&request) {
		return
	}
	settings := c.house.setPlayMode(request.PlayMode)
	if c.IsHTTP() {
		c.Send(base.H{"mode": settings.PlayMode})
	} else {
		c.Send(settings)
	}
}

func getCurrentMusic(c *Context) {
	playback, current := c.house.playbackSnapshot()
	if c.IsWebSocket() {
		c.Send(playback)
		return
	}
	if playback.Song == nil {
		c.Fail(protocol.ErrorCodeNotFound, "当前没有播放音乐")
		return
	}
	message := songToH(&current.Song)
	message["pushTime"], message["volumeCoef"], message["user"] = playback.StartedAtMS, playback.VolumeCoefficient, current.User
	c.Send(message)
}

func getPlaylist(c *Context) {
	if c.IsWebSocket() {
		c.Send(c.house.queueSnapshot())
		return
	}
	type item struct {
		Name   string    `json:"name"`
		Artist string    `json:"artist"`
		Source string    `json:"source"`
		ID     string    `json:"id"`
		Likes  int       `json:"likes"`
		User   auth.User `json:"user"`
	}
	orders := c.house.queueItems()
	list := make([]item, 0, len(orders))
	for _, order := range orders {
		artist := order.Song.Artist
		if artist == "" {
			artist = "unknown"
		}
		list = append(list, item{
			Name: order.Song.Name, Artist: artist, Source: order.Source, ID: order.ID, Likes: order.Likes, User: order.User,
		})
	}
	c.Send(base.H{"playlist": list})
}

func recommendMusic(c *Context) {
	if !c.Wait(WaitSearch) {
		return
	}
	ids := make([]string, 0)
	for _, order := range c.house.queueItems() {
		if music.ParseSource(order.Source) == music.NetEase {
			ids = append(ids, order.ID)
		}
	}
	recommended := c.house.recommander.Recommend(c.ctx, ids)
	items := make([]*pb.Music, 0, len(recommended))
	for _, id := range recommended {
		if c.ctx.Err() != nil {
			c.Fail(protocol.ErrorCodeUnavailable, "请求已取消或超时")
			return
		}
		song, err := c.house.backend.Load(c.ctx, &pb.SongRequest{Source: "netease", Id: id, UseCache: true})
		if err != nil || song == nil {
			continue
		}
		items = append(items, &pb.Music{
			Id: id, Name: song.Name, Artist: song.Artist, Album: song.Album,
			Duration: max(song.Duration, 0), Cover: song.PictureUrl, Source: int32(music.NetEase),
		})
	}
	c.Send(protocol.SearchData(items, int64(len(items))))
}

// The flat legacy song representation is used only by the HTTP API.
func songToH(song *pb.Song) base.H {
	if song == nil {
		return nil
	}
	raw, _ := json.Marshal(song)
	var message base.H
	_ = json.Unmarshal(raw, &message)
	return message
}
