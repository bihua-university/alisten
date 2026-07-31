package music

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/music/kuwo"
	"github.com/bihua-university/alisten/internal/music/qq"
)

var qqClient = qq.New()

func GetQQMusicResult(r gjson.Result, o SearchOption) SearchResult[Music] {
	start := (o.Page - 1) * o.PageSize
	end := start + o.PageSize
	var data []*Music
	var idx int64
	r.ForEach(func(_, item gjson.Result) bool {
		if idx >= start && idx < end {
			artist := ""
			item.Get("singer").ForEach(func(_, value gjson.Result) bool {
				if artist != "" {
					artist += ", "
				}
				artist += value.Get("name").String()
				return true
			})

			picture := fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.Get("albummid").String())
			data = append(data, &Music{
				ID:       item.Get("songmid").String(),
				Name:     item.Get("songname").String(),
				Artist:   artist,
				Album:    item.Get("albumname").String(),
				Duration: item.Get("interval").Int() * 1000,
				Cover:    picture,
				Source:   QQ,
			})
		}
		idx++
		return true
	})
	return SearchResult[Music]{Total: idx, Data: data}
}

func getQQMusic(id string) H {
	detail, _ := qqClient.GetSongDetail(id)
	lyric, _ := qqClient.GetLyrics(id)

	artist := ""
	detail.Get("singer").ForEach(func(_, value gjson.Result) bool {
		if artist != "" {
			artist += ", "
		}
		artist += value.Get("name").String()
		return true
	})

	songName := detail.Get("name").String()
	key := artist + " " + songName
	kclient := kuwo.New()
	klist, err := kclient.Search(key)
	if len(klist) == 0 || err != nil {
		logx.Error("ksearch", key, err)
		return H{}
	}

	rid := klist[0].ID
	songName = klist[0].Name
	artist = klist[0].Artist
	url, err := kclient.GetDownloadURL(rid)
	if err != nil {
		logx.Error("kdownload", key, "->", rid, err)
		return H{}
	}

	ablumMid := detail.Get("album.mid").String()
	picture := fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", ablumMid)

	return H{
		"type":       "music",
		"url":        url,
		"webUrl":     GenerateWebURL("qq", id),
		"pictureUrl": picture,
		"duration":   detail.Get("interval").Int() * 1000,
		"source":     "qq",
		"lyric":      lyric,
		"artist":     artist,
		"name":       songName,
		"album":      detail.Get("album.name").String(),
		"id":         id,
	}
}

func searchQQPlaylist(o SearchOption) SearchResult[Playlist] {
	result, _ := qqClient.SearchPlaylist(o.Keyword)

	start := (o.Page - 1) * o.PageSize
	end := start + o.PageSize
	var data []*Playlist
	var idx int64
	result.Get("list").ForEach(func(_, item gjson.Result) bool {
		if idx >= start && idx < end {
			creator := item.Get("creator")
			cover := item.Get("imgurl").String()
			if cover != "" && strings.HasPrefix(cover, "http://") {
				cover = strings.Replace(cover, "http://", "https://", 1)
			}
			data = append(data, &Playlist{
				ID:         item.Get("dissid").String(),
				Name:       item.Get("dissname").String(),
				PictureURL: cover,
				Desc:       item.Get("introduction").String(),
				Creator:    creator.Get("name").String(),
				CreatorUid: creator.Get("creator_uin").String(),
				PlayCount:  item.Get("listennum").Int(),
				SongCount:  item.Get("song_count").Int(),
			})
		}
		idx++
		return true
	})
	return SearchResult[Playlist]{Total: idx, Data: data}
}
