package music

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/base"
)

func WyPostTimestamp(u string, k gin.H, timestamp bool) gjson.Result {
	k["cookie"] = base.Config.Cookie
	marshal, err := json.Marshal(k)
	if err != nil {
		return gjson.Result{}
	}

	var dest string
	if timestamp {
		dest = fmt.Sprintf("%s%s?timestamp=%d", base.Config.NeteaseAPI, u, time.Now().UnixMilli())
	} else {
		dest = base.Config.NeteaseAPI + u
	}
	req, err := http.NewRequest("POST", dest, bytes.NewReader(marshal))
	if err != nil {
		return gjson.Result{}
	}
	req.Header.Set("content-type", "application/json;charset=UTF-8")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return gjson.Result{}
	}
	defer response.Body.Close()

	all, err := io.ReadAll(response.Body)
	if err != nil {
		return gjson.Result{}
	}

	return gjson.ParseBytes(all)
}

func NeteasePost(u string, k gin.H) gjson.Result {
	return WyPostTimestamp(u, k, true)
}

const (
	NeteaseSong     = 1
	NeteaseAlbum    = 10
	NeteasePlaylist = 1000
)

type SearchOption struct {
	ID       string
	Source   string
	Keyword  string
	Page     int64
	PageSize int64
}

type SearchResult[T any] struct {
	Total int64
	Data  []*T
}

func GetNeteaseMusicResult(r gjson.Result, o SearchOption) SearchResult[Music] {
	var total int64
	var res []*Music
	r.ForEach(func(_, item gjson.Result) bool {
		total++
		if total < (o.Page-1)*o.PageSize || int64(len(res)) > o.PageSize {
			return true
		}

		artist := ""
		item.Get("ar").ForEach(func(_, value gjson.Result) bool {
			if artist != "" {
				artist += ", "
			}
			artist += value.Get("name").String()
			return true
		})

		m := &Music{
			ID:       item.Get("id").String(),
			Name:     item.Get("name").String(),
			Artist:   artist,
			Album:    Album{Name: item.Get("al.name").String()},
			Duration: item.Get("dt").Int(),
			Privilege: Privilege{
				St: 1,
				Fl: 1,
			},
			Cover: item.Get("al.picUrl").String(),
		}
		res = append(res, m)
		return true
	})
	return SearchResult[Music]{Total: total, Data: res}
}

func SearchNeteasePlaylist(o SearchOption) SearchResult[Playlist] {
	r := WyPostTimestamp("/cloudsearch?keywords="+o.Keyword, gin.H{
		"keywords": o.Keyword,
		"type":     NeteasePlaylist,
	}, false)

	var total int64
	var res []*Playlist
	r.Get("result.playlists").ForEach(func(_, item gjson.Result) bool {
		total++
		if total < (o.Page-1)*o.PageSize || int64(len(res)) > o.PageSize {
			return true
		}
		creator := item.Get("creator")
		m := &Playlist{
			ID:         item.Get("id").String(),
			Name:       item.Get("name").String(),
			PictureURL: item.Get("coverImgUrl").String(),
			Desc:       item.Get("description").String(),
			Creator:    creator.Get("nickname").String(),
			CreatorUid: creator.Get("userId").String(),
			PlayCount:  item.Get("playCount").Int(),
			SongCount:  item.Get("trackCount").Int(),
		}
		res = append(res, m)
		return true
	})
	return SearchResult[Playlist]{Total: total, Data: res}
}

func getNeteaseMusic(id string) gin.H {
	// 从试听链接中下载
	try := NeteasePost("/song/url/v1", gin.H{
		"level": "exhigh", // 320kps
		"id":    id,
	})
	url := try.Get("data.0.url").String()
	if url == "" {
		download := NeteasePost("/song/download/url/v1", gin.H{
			"level": "exhigh", // 320kps
			"id":    id,
		})
		url = download.Get("data.url").String()
	}

	detail := NeteasePost("/song/detail", gin.H{
		"ids": id,
	}).Get("songs.0")
	lyric := NeteasePost("/lyric", gin.H{
		"id": id,
	})

	artist := ""
	detail.Get("ar").ForEach(func(_, value gjson.Result) bool {
		if artist != "" {
			artist += ", "
		}
		artist += value.Get("name").String()
		return true
	})

	h := gin.H{
		"type":       "music",
		"url":        url,
		"pictureUrl": detail.Get("al.picUrl").String(),
		"duration":   detail.Get("dt").Int(),
		"source":     "netease",
		"lyric":      lyric.Get("lrc.lyric").String(),
		"artist":     artist,
		"name":       detail.Get("name").String(),
		"album": gin.H{
			"name": detail.Get("al.name").String(),
		},
		"id": id,
	}

	return h
}
