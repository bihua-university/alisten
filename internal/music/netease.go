package music

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/base"
)

func NeteasePost(u string, k gin.H, key string) gjson.Result {
	k["cookie"] = base.Config.Cookie
	marshal, err := json.Marshal(k)
	if err != nil {
		return gjson.Result{}
	}

	dest := base.Config.NeteaseAPI + u
	if key != "" {
		dest += fmt.Sprintf("?%s=%s", key, url.QueryEscape(fmt.Sprint(k[key])))
	} else {
		dest += "?timestamp=" + strconv.FormatInt(time.Now().UnixMilli(), 10)
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
	r := NeteasePost("/cloudsearch", gin.H{
		"keywords": o.Keyword,
		"type":     NeteasePlaylist,
	}, "keywords")

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

func SearchNeteaseUserPlaylist(o SearchOption) SearchResult[Playlist] {
	list := NeteasePost("/user/playlist", gin.H{
		"uid": o.Keyword,
	}, "uid")

	var total int64
	var res []*Playlist
	list.Get("playlist").ForEach(func(_, item gjson.Result) bool {
		total++
		if total < (o.Page-1)*o.PageSize || int64(len(res)) > o.PageSize {
			return true
		}
		m := &Playlist{
			ID:         item.Get("id").String(),
			Name:       item.Get("name").String(),
			PictureURL: item.Get("coverImgUrl").String(),
			Desc:       item.Get("description").String(),
			Creator:    item.Get("creator.nickname").String(),
			CreatorUid: item.Get("creator.userId").String(),
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
	}, "id")
	url := try.Get("data.0.url").String()
	if url == "" {
		download := NeteasePost("/song/download/url/v1", gin.H{
			"level": "exhigh", // 320kps
			"id":    id,
		}, "id")
		url = download.Get("data.url").String()
	}

	detail := NeteasePost("/song/detail", gin.H{
		"ids": id,
	}, "ids").Get("songs.0")
	lyric := NeteasePost("/lyric", gin.H{
		"id": id,
	}, "id")

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
		"webUrl":     GenerateWebURL("wy", id),
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
