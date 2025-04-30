package music

import (
	"net/url"

	"github.com/gin-gonic/gin"
)

func SearchMusic(o SearchOption) SearchResult[Music] {
	switch o.Source {
	case "wy":
		r := NeteasePost("/cloudsearch", gin.H{
			"keywords": o.Keyword,
			"type":     NeteaseSong,
		})
		return GetNeteaseMusicResult(r.Get("result.songs"), o)
	case "qq":
		r := QQGet("/search", url.Values{
			"key": []string{o.Keyword},
		})
		return GetQQMusicResult(r.Get("data.list"), o)
	}
	return SearchResult[Music]{}
}

func SearchPlaylist(o SearchOption) SearchResult[Playlist] {
	switch o.Source {
	case "wy":
		return SearchNeteasePlaylist(o)
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
		r := NeteasePost("/playlist/track/all", gin.H{
			"id": o.ID,
		})
		return GetNeteaseMusicResult(r.Get("songs"), o)
	case "qq":
		r := QQGet("/songlist", url.Values{
			"id": []string{o.ID},
		})
		return GetQQMusicResult(r.Get("data.songlist"), o)
	}
	return SearchResult[Music]{}
}
