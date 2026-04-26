package music

import (
	"strings"
	"sync"

	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/music/netease"
	"github.com/tidwall/gjson"
)

var neteaseClient = sync.OnceValue(func() *netease.Netease { return netease.New(base.Config.Cookie) })

func getNeteaseMusic(id string) H {
	client := neteaseClient()
	result, err := client.GetSongDetail([]string{id})
	if err != nil {
		return nil
	}

	song := result.Get("songs.0")
	if !song.Exists() {
		return nil
	}

	url, _ := client.GetDownloadURL(id)
	lyric, _ := client.GetLyrics(id)

	return H{
		"type":       "music",
		"url":        url,
		"webUrl":     GenerateWebURL("wy", id),
		"pictureUrl": song.Get("al.picUrl").String(),
		"duration":   song.Get("dt").Int(),
		"source":     "netease",
		"lyric":      lyric,
		"artist":     parseArtists(song),
		"name":       song.Get("name").String(),
		"album":      song.Get("al.name").String(),
		"id":         id,
	}
}

// parseArtists 从 gjson 中提取艺术家名称
func parseArtists(item gjson.Result) string {
	var names []string
	item.Get("ar").ForEach(func(_, ar gjson.Result) bool {
		if name := ar.Get("name").String(); name != "" {
			names = append(names, name)
		}
		return true
	})
	return strings.Join(names, ", ")
}
