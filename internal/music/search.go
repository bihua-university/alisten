package music

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/bihua-university/alisten/internal/task"
)

func SearchMusic(o SearchOption) SearchResult[Music] {
	switch o.Source {
	case "wy":
		r := NeteasePost("/cloudsearch", H{
			"keywords": o.Keyword,
			"type":     NeteaseSong,
		}, "keywords")
		return GetNeteaseMusicResult(r.Get("result.songs"), o)
	case "qq":
		r := QQGet("/search", url.Values{
			"key": []string{o.Keyword},
		})
		return GetQQMusicResult(r.Get("data.list"), o)
	case "db":
		t := task.Scheduler.NewTask("bilibili:search_music", map[string]string{
			"keyword":  o.Keyword,
			"page":     fmt.Sprintf("%d", o.Page),
			"pageSize": fmt.Sprintf("%d", o.PageSize),
		})
		r := task.Scheduler.Call(t, 1*time.Minute)
		if r == nil || r.Result == nil {
			return SearchResult[Music]{}
		}

		var res struct {
			Data  []*Music `json:"data"`
			Total int      `json:"total"`
		}
		json.Unmarshal([]byte(r.Result), &res)
		return SearchResult[Music]{
			Total: int64(res.Total),
			Data:  res.Data,
		}
	}
	return SearchResult[Music]{}
}

func SearchPlaylist(o SearchOption) SearchResult[Playlist] {
	switch o.Source {
	case "wy":
		return SearchNeteasePlaylist(o)
	case "wy_user":
		return SearchNeteaseUserPlaylist(o)
	case "qq":
		return SearchQQPlaylist(o)
	case "qq_user":
		return SearchQQUserPlaylist(o)
	}
	return SearchResult[Playlist]{}
}

func GetSongList(o SearchOption) SearchResult[Music] {
	switch o.Source {
	case "wy":
		r := NeteasePost("/playlist/track/all", H{
			"id": o.ID,
		}, "id")
		return GetNeteaseMusicResult(r.Get("songs"), o)
	case "qq":
		r := QQGet("/songlist", url.Values{
			"id": []string{o.ID},
		})
		return GetQQMusicResult(r.Get("data.songlist"), o)
	}
	return SearchResult[Music]{}
}
