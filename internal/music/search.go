package music

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/task"
)

func SearchMusic(o SearchOption) SearchResult[Music] {
	o.normalize()
	switch o.Source {
	case "wy":
		client := neteaseClient()
		result, _ := client.Search(o.Keyword)
		start := (o.Page - 1) * o.PageSize
		end := start + o.PageSize
		var data []*Music
		var idx int64
		result.Get("songs").ForEach(func(_, item gjson.Result) bool {
			if idx >= start && idx < end {
				data = append(data, &Music{
					ID:       item.Get("id").String(),
					Name:     item.Get("name").String(),
					Artist:   parseArtists(item),
					Album:    item.Get("al.name").String(),
					Duration: item.Get("dt").Int(),
					Cover:    item.Get("al.picUrl").String(),
					Source:   NetEase,
				})
			}
			idx++
			return idx < end
		})
		return SearchResult[Music]{Total: min(result.Get("songCount").Int(), 100), Data: data}
	case "qq":
		result, _ := qqClient.Search(o.Keyword)
		return GetQQMusicResult(result.Get("list"), o)
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
	o.normalize()
	switch o.Source {
	case "wy":
		return searchNeteasePlaylist(o)
	case "qq":
		return searchQQPlaylist(o)
	}
	return SearchResult[Playlist]{}
}

func searchNeteasePlaylist(o SearchOption) SearchResult[Playlist] {
	client := neteaseClient()
	result, _ := client.SearchPlaylist(o.Keyword)
	start := (o.Page - 1) * o.PageSize
	end := start + o.PageSize
	var data []*Playlist
	var idx int64
	result.Get("playlists").ForEach(func(_, item gjson.Result) bool {
		if idx >= start && idx < end {
			data = append(data, &Playlist{
				ID:         item.Get("id").String(),
				Name:       item.Get("name").String(),
				PictureURL: item.Get("coverImgUrl").String(),
				Desc:       item.Get("description").String(),
				Creator:    item.Get("creator.nickname").String(),
				PlayCount:  item.Get("playCount").Int(),
				SongCount:  item.Get("trackCount").Int(),
			})
		}
		idx++
		return idx < end
	})
	return SearchResult[Playlist]{Total: min(result.Get("playlistCount").Int(), 100), Data: data}
}

func GetSongList(o SearchOption) SearchResult[Music] {
	switch o.Source {
	case "wy":
		client := neteaseClient()
		detail, _ := client.GetPlaylistDetail(o.ID)

		var songIDs []string
		detail.Get("playlist.trackIds").ForEach(func(_, tid gjson.Result) bool {
			songIDs = append(songIDs, tid.Get("id").String())
			return true
		})

		var data []*Music
		const batchSize = 500
		for i := 0; i < len(songIDs); i += batchSize {
			end := i + batchSize
			if end > len(songIDs) {
				end = len(songIDs)
			}
			batch, _ := client.GetSongDetail(songIDs[i:end])
			batch.Get("songs").ForEach(func(_, item gjson.Result) bool {
				data = append(data, &Music{
					ID:       item.Get("id").String(),
					Name:     item.Get("name").String(),
					Artist:   parseArtists(item),
					Album:    item.Get("al.name").String(),
					Duration: item.Get("dt").Int(),
					Cover:    item.Get("al.picUrl").String(),
					Source:   NetEase,
				})
				return true
			})
		}
		return SearchResult[Music]{Total: int64(len(data)), Data: data}
	}
	return SearchResult[Music]{}
}
