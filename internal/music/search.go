package music

import (
	"fmt"
	"net/url"

	"github.com/bihua-university/alisten/internal/music/bihua"
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
		ms, i, err := bihua.SearchMusicByDB(o.Keyword, o.Page, o.PageSize)
		if err != nil {
			fmt.Println(err)
			return SearchResult[Music]{}
		}
		var data []*Music
		for _, m := range ms {
			data = append(data, &Music{
				ID:       m.MusicID,
				Name:     m.Name,
				Artist:   m.Artist,
				Album:    m.AlbumName,
				Duration: m.Duration,
				Source:   NeteaseSong,
			})
		}
		return SearchResult[Music]{
			Total: i,
			Data:  data,
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
