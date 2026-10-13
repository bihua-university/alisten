package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/music"
	"github.com/bihua-university/alisten/protocol"
)

type Order struct {
	source string
	id     string
	user   auth.User
	likes  int
	song   music.Song
}

func doPickMusic(ctx context.Context, h *House, request protocol.PickRequest, user auth.User) (protocol.PickResult, *protocol.Error) {
	fail := func(code protocol.ErrorCode, message string) (protocol.PickResult, *protocol.Error) {
		return protocol.PickResult{}, &protocol.Error{Code: code, Message: message}
	}
	source := music.ParseSource(string(request.Source))
	id, name := strings.TrimSpace(request.ID), strings.TrimSpace(request.Name)
	if id == "" {
		result, err := h.backend.Search(music.SearchOption{Source: source, Keyword: name, Page: 1, PageSize: 10})
		if err != nil || len(result.Data) == 0 || result.Data[0] == nil || result.Data[0].ID == "" {
			return fail(protocol.ErrorCodeProviderError, "点歌失败，无法搜索到音乐")
		}
		id = result.Data[0].ID
	}
	song, err := h.backend.Load(source.String(), id, true)
	if err != nil || song == nil || song.URL == "" {
		return fail(protocol.ErrorCodeProviderError, "点歌失败，无法获取音乐信息")
	}
	order := Order{source: source.String(), id: id, user: user, song: *song}
	order.song.ID, order.song.Source = id, order.source
	order.song.Duration = max(order.song.Duration, 0)

	var enqueueErr *protocol.Error
	h.lock(func() {
		switch {
		case ctx.Err() != nil || h.closed:
			enqueueErr = &protocol.Error{Code: protocol.ErrorCodeUnavailable, Message: "请求已取消或房间已关闭"}
		case !h.ultimate && len(h.Playlist) >= 10:
			enqueueErr = &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "已超过最大点歌数量10首，请稍后再试"}
		case h.preparing.id == id && h.preparing.source == order.source:
			enqueueErr = &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "重复点歌"}
		default:
			for _, queued := range h.Playlist {
				if queued.id == id && queued.source == order.source {
					enqueueErr = &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "重复点歌"}
					return
				}
			}
			h.Playlist = append(h.Playlist, order)
			h.lastOrderTime = time.Now()
		}
	})
	if enqueueErr != nil {
		return protocol.PickResult{}, enqueueErr
	}
	h.backend.Analyze(order.source, order.id)
	h.PushPlaylist()
	h.Update()
	artist := song.Artist
	if artist == "" {
		artist = "unknown"
	}
	return protocol.PickResult{ID: id, Source: protocol.Source(order.source), Name: song.Name, Artist: artist}, nil
}

func searchMusic(c *Context) {
	var request protocol.SearchRequest
	if !c.Decode(&request) || !c.Wait(WaitSearch) {
		return
	}
	option := music.SearchOption{
		Source: music.ParseSource(string(request.Source)), Keyword: request.Keyword,
		Page: request.PageIndex, PageSize: request.PageSize,
	}
	var result music.Result[music.Music]
	if strings.HasPrefix(request.Keyword, "*") {
		option.ID = strings.TrimPrefix(request.Keyword, "*")
		result, _ = c.house.backend.SongList(option)
	} else {
		result, _ = c.house.backend.Search(option)
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
	var removed Order
	c.WithHouse(func(h *House) {
		for i, order := range h.Playlist {
			matches := order.id == request.ID && order.source == music.ParseSource(string(request.Source)).String()
			if legacy {
				matches = order.song.Name == c.Get("id").String()
			}
			if matches {
				removed = order
				h.Playlist = append(h.Playlist[:i], h.Playlist[i+1:]...)
				h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queueLocked()))
				return
			}
		}
	})
	if removed.id == "" {
		c.Fail(protocol.ErrorCodeNotFound, "未找到要删除的音乐")
		return
	}
	if c.IsHTTP() {
		c.Send(base.H{"name": removed.song.Name})
	} else {
		c.Send(protocol.SongRef{ID: removed.id, Source: protocol.Source(removed.source)})
		c.Chat("删除音乐 " + removed.song.Name)
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
	user := c.User()
	var result protocol.VoteResult
	var err *protocol.Error
	var play Order
	var generation uint64
	c.WithHouse(func(h *House) {
		if h.closed {
			err = &protocol.Error{Code: protocol.ErrorCodeUnavailable, Message: "房间已关闭"}
			return
		}
		if h.preparing.id != "" {
			err = &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "正在切歌，请稍后再试"}
			return
		}
		if h.Current.id == "" {
			err = &protocol.Error{Code: protocol.ErrorCodeNotFound, Message: "当前没有播放音乐"}
			return
		}
		if slices.Contains(h.VoteSkip, user) {
			err = &protocol.Error{Code: protocol.ErrorCodeConflict, Message: "您已经投过票了"}
			return
		}
		h.VoteSkip = append(h.VoteSkip, user)
		result.CurrentVotes = int64(len(h.VoteSkip))
		result.RequiredVotes = int64(max((len(h.Connection)+2)/3, 1))
		if result.CurrentVotes >= result.RequiredVotes {
			play, generation, result.Skipped = h.nextLocked(true)
		}
	})
	if err != nil {
		c.Fail(err.Code, err.Message)
		return
	}
	if result.Skipped && play.id != "" {
		result.Skipped = c.house.push(play, generation)
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
	var liked Order
	c.WithHouse(func(h *House) {
		index := -1
		if legacy {
			// The HTTP index includes the current song at index zero.
			value := c.Get("index").Int()
			if value >= 1 && value <= int64(len(h.Playlist)) {
				index = int(value - 1)
			}
		} else {
			for i, order := range h.Playlist {
				if order.id == request.ID && order.source == music.ParseSource(string(request.Source)).String() {
					index = i
					break
				}
			}
		}
		if index < 0 {
			return
		}
		h.Playlist[index].likes++
		liked = h.Playlist[index]
		sort.SliceStable(h.Playlist, func(i, j int) bool { return h.Playlist[i].likes > h.Playlist[j].likes })
		h.broadcastLocked(protocol.Event(protocol.EventQueueUpdated, h.queueLocked()))
	})
	if liked.id == "" {
		c.Fail(protocol.ErrorCodeNotFound, "未找到对应音乐")
		return
	}
	if c.IsHTTP() {
		c.Send(base.H{"name": liked.song.Name, "likes": liked.likes})
	} else {
		c.Send(protocol.LikeResult{ID: liked.id, Source: protocol.Source(liked.source), Likes: int64(liked.likes)})
		c.Chat(fmt.Sprintf("%s 点赞%d", liked.song.Name, liked.likes))
	}
}

func searchList(c *Context) {
	var request protocol.SearchRequest
	if !c.Decode(&request) || !c.Wait(WaitSearch) {
		return
	}
	result, _ := c.house.backend.Playlists(music.SearchOption{
		Source: music.ParseSource(string(request.Source)), Keyword: request.Keyword,
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
	var settings protocol.SettingsData
	var event any
	c.WithHouse(func(h *House) {
		switch request.PlayMode {
		case protocol.PlayModeSequential, protocol.PlayModeRandom:
			h.Mode = request.PlayMode
			event = protocol.Event(protocol.EventSettingsUpdated, protocol.SettingsData{PlayMode: h.Mode})
		}
		settings.PlayMode = h.Mode
	})
	if event != nil {
		c.house.Broadcast(event)
	}
	if c.IsHTTP() {
		c.Send(base.H{"mode": settings.PlayMode})
	} else {
		c.Send(settings)
	}
}

func getCurrentMusic(c *Context) {
	var playback protocol.PlaybackData
	var current Order
	c.WithHouse(func(h *House) {
		playback, current = h.playback, h.Current
	})
	if c.IsWebSocket() {
		c.Send(playback)
		return
	}
	if playback.Song == nil {
		c.Fail(protocol.ErrorCodeNotFound, "当前没有播放音乐")
		return
	}
	message := songToH(&current.song)
	message["pushTime"], message["volumeCoef"], message["user"] = playback.StartedAtMS, playback.VolumeCoefficient, current.user
	c.Send(message)
}

func getPlaylist(c *Context) {
	if c.IsWebSocket() {
		var data protocol.QueueData
		c.WithHouse(func(h *House) { data = h.queueLocked() })
		c.Send(data)
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
	list := make([]item, 0)
	c.WithHouse(func(h *House) {
		for _, order := range h.Playlist {
			artist := order.song.Artist
			if artist == "" {
				artist = "unknown"
			}
			list = append(list, item{
				Name: order.song.Name, Artist: artist, Source: order.source, ID: order.id, Likes: order.likes, User: order.user,
			})
		}
	})
	c.Send(base.H{"playlist": list})
}

func recommendMusic(c *Context) {
	if !c.Wait(WaitSearch) {
		return
	}
	ids := make([]string, 0)
	c.WithHouse(func(h *House) {
		for _, order := range h.Playlist {
			if music.ParseSource(order.source) == music.NetEase {
				ids = append(ids, order.id)
			}
		}
	})
	recommended := c.house.recommander.Recommend(ids)
	items := make([]*music.Music, 0, len(recommended))
	for _, id := range recommended {
		if c.ctx.Err() != nil {
			c.Fail(protocol.ErrorCodeUnavailable, "请求已取消或超时")
			return
		}
		song, err := c.house.backend.Load("netease", id, true)
		if err != nil || song == nil {
			continue
		}
		items = append(items, &music.Music{
			ID: id, Name: song.Name, Artist: song.Artist, Album: song.Album,
			Duration: max(song.Duration, 0), Cover: song.PictureURL, Source: music.NetEase,
		})
	}
	c.Send(protocol.SearchData(items, int64(len(items))))
}

func proxyMusicURL(song *music.Song) *music.Song {
	const useProxy = false
	if !useProxy || song == nil || music.ParseSource(song.Source) != music.NetEase {
		return song
	}
	copy := *song
	copy.URL = localhost() + "/music/file?source=netease&id=" + song.ID
	return &copy
}

// The flat legacy song representation is used only by the HTTP API.
func songToH(song *music.Song) base.H {
	if song == nil {
		return nil
	}
	raw, _ := json.Marshal(song)
	var message base.H
	_ = json.Unmarshal(raw, &message)
	return message
}
