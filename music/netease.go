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
)

var wyBaseUrl = "http://123.60.73.23:3000"

func WyPostTimestamp(u string, k gin.H, timestamp bool) gjson.Result {
	k["cookie"] = "MUSIC_U=00201A0BE484D09BD4344F9784163FAE1F0E044C291E03C48547A0F0E3FE1E3AC7EE8CB8DF835C2134F4F67EBFF29A8D39C6F8B11B32A501D329E2DC88E35FF9C1F5B8C15B6110F9A7F379E8F7627106082330503BD7D7D7D1F99313EA6EB65AD7D5C66F9D86188483E5E9E74310F2A682E34565E3532A4018874100AFD12DD265602BF305B20B5EB49CD1528270287E4112C4E920F7092AAE60AE66A8CA4A52084D607F9F242E3313E5D2E9409FF793E417EAA424A49A5A3153AB9DA66BFF1327CB657E5FBBEAD5242AB9CA972AAC6ABD52619E6BE8AB2399335133FD9DFC9F174D6F73AF2FA1FC4E892795234E6FA61CA56EB91DBD079E3147C28142738EB41262B2F528F7313F3D5244E9DC5FA120E7BEBB6F609B1E5FA9AC92F9878C5597872ED4FB406784954ADD36581FFCE8B7B4BA7BA08973CEFFA098AF3FDEBBC0113BD653100E4EAFA86DD0078EB03D66DF2AB22FBCB16BF28A1423F70E1F32AB1408"
	marshal, err := json.Marshal(k)
	if err != nil {
		return gjson.Result{}
	}

	var dest string
	if timestamp {
		dest = fmt.Sprintf("%s%s?timestamp=%d", wyBaseUrl, u, time.Now().UnixMilli())
	} else {
		dest = wyBaseUrl + u
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

func GetMusic(source, id string) gin.H {
	switch source {
	case "wy":
		return getNeteaseMusic(id)
	case "qq":
		return getQQMusic(id)
	}
	return gin.H{}
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
	})

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
	if g, ok := netease.Get(id); ok {
		return g
	}

	download := NeteasePost("/song/download/url", gin.H{
		"br": 320000, // 320kps
		"id": id,
	})
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
		"url":        download.Get("data.url").String(),
		"pictureUrl": detail.Get("al.picUrl").String(),
		"duration":   detail.Get("dt").Int(),
		"source":     "netease",
		"lyric":      lyric.Get("lrc.lyric").String(),
		"artist":     artist,
		"name":       detail.Get("name").String(),
		"album": gin.H{
			"name": detail.Get("al.name").String(),
		},
	}
	netease.Add(id, h)

	return h
}
