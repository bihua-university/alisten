package music

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/wdvxdr1123/alisten/internal/base"
	"github.com/wdvxdr1123/alisten/internal/music/kuwo"
)

var kuwoClient = kuwo.NewClient()

func QQPost(u string, k gin.H) gjson.Result {
	marshal, err := json.Marshal(k)
	if err != nil {
		return gjson.Result{}
	}

	dest := fmt.Sprintf("%s%s", base.Config.QQAPI, u)
	req, err := http.NewRequest("POST", dest, bytes.NewReader(marshal))
	if err != nil {
		return gjson.Result{}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0")
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

func Get(u string, k url.Values) gjson.Result {
	dest := fmt.Sprintf("%s?%s", u, k.Encode())
	req, err := http.NewRequest("GET", dest, nil)
	if err != nil {
		return gjson.Result{}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0")
	response, err := kuwo.HttpClient.Do(req)
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

func QQGet(u string, k url.Values) gjson.Result {
	// k["cookie"] = "MUSIC_U=00201A0BE484D09BD4344F9784163FAE1F0E044C291E03C48547A0F0E3FE1E3AC7EE8CB8DF835C2134F4F67EBFF29A8D39C6F8B11B32A501D329E2DC88E35FF9C1F5B8C15B6110F9A7F379E8F7627106082330503BD7D7D7D1F99313EA6EB65AD7D5C66F9D86188483E5E9E74310F2A682E34565E3532A4018874100AFD12DD265602BF305B20B5EB49CD1528270287E4112C4E920F7092AAE60AE66A8CA4A52084D607F9F242E3313E5D2E9409FF793E417EAA424A49A5A3153AB9DA66BFF1327CB657E5FBBEAD5242AB9CA972AAC6ABD52619E6BE8AB2399335133FD9DFC9F174D6F73AF2FA1FC4E892795234E6FA61CA56EB91DBD079E3147C28142738EB41262B2F528F7313F3D5244E9DC5FA120E7BEBB6F609B1E5FA9AC92F9878C5597872ED4FB406784954ADD36581FFCE8B7B4BA7BA08973CEFFA098AF3FDEBBC0113BD653100E4EAFA86DD0078EB03D66DF2AB22FBCB16BF28A1423F70E1F32AB1408"
	return Get(base.Config.QQAPI+u, k)
}

func GetQQMusicResult(r gjson.Result, o SearchOption) SearchResult[Music] {
	var total int64
	var res []*Music
	r.ForEach(func(_, item gjson.Result) bool {
		total++
		if total < (o.Page-1)*o.PageSize || int64(len(res)) > o.PageSize {
			return true
		}

		artist := ""
		item.Get("singer").ForEach(func(_, value gjson.Result) bool {
			if artist != "" {
				artist += ", "
			}
			artist += value.Get("name").String()
			return true
		})

		m := &Music{
			ID:       item.Get("songmid").String(),
			Name:     item.Get("songname").String(),
			Artist:   artist,
			Album:    Album{Name: item.Get("albumname").String()},
			Duration: item.Get("interval").Int() * 1000,
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

func getQQMusic(id string) gin.H {
	if g, ok := qq.Get(id); ok {
		return g
	}

	detail := QQGet("/song", url.Values{
		"songmid": []string{id},
	}).Get("data.track_info")
	lyric := QQGet("/lyric", url.Values{
		"songmid": []string{id},
	})

	name := detail.Get("name").String()
	artist := ""
	detail.Get("singer").ForEach(func(_, value gjson.Result) bool {
		if artist != "" {
			artist += ", "
		}
		artist += value.Get("name").String()
		return true
	})

	resp := kuwoClient.SearchMusic(0, 10, name+" "+artist)
	var rid string
	if len(resp.Abslist) > 0 {
		rid = strings.TrimPrefix(resp.Abslist[0].MUSICRID, "MUSIC_")
	}

	download := Get("https://mobi.kuwo.cn/mobi.s", url.Values{
		"f":      []string{"web"},
		"source": []string{"kwplayer_ar_4.4.2.7_B_nuoweida_vh.apk"},
		"format": []string{"mp3"},
		"br":     []string{"2000kflac"},
		"type":   []string{"convert_url_with_sign"},
		"rid":    []string{rid},
	})

	ablumMid := detail.Get("album.mid").String()
	picture := fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", ablumMid)

	h := gin.H{
		"type":       "music",
		"url":        download.Get("data.url").String(),
		"pictureUrl": picture,
		"duration":   download.Get("data.duration").Int() * 1000,
		"source":     "netease",
		"lyric":      lyric.Get("data.lyric").String(),
		"artist":     artist,
		"name":       name,
		"album": gin.H{
			"name": detail.Get("album.name").String(),
		},
	}

	qq.Add(id, h)
	return h
}

func SearchQQPlaylist(o SearchOption) SearchResult[Playlist] {
	r := QQGet("/search", url.Values{
		"key": []string{o.Keyword},
		"t":   []string{"2"},
	})

	var total int64
	var res []*Playlist
	r.Get("data.list").ForEach(func(_, item gjson.Result) bool {
		total++
		if total < (o.Page-1)*o.PageSize || int64(len(res)) > o.PageSize {
			return true
		}
		creator := item.Get("creator")
		m := &Playlist{
			ID:         item.Get("dissid").String(),
			Name:       item.Get("dissname").String(),
			PictureURL: item.Get("imgurl").String(),
			Desc:       item.Get("introduction").String(),
			Creator:    creator.Get("name").String(),
			CreatorUid: creator.Get("creator_uin").String(),
			PlayCount:  item.Get("listennum").Int(),
			SongCount:  item.Get("song_count").Int(),
		}
		res = append(res, m)
		return true
	})
	return SearchResult[Playlist]{Total: total, Data: res}
}

func SearchQQUserPlaylist(o SearchOption) SearchResult[Playlist] {
	r := QQGet("/user/songlist", url.Values{
		"id": []string{o.Keyword},
	})

	var total int64
	var res []*Playlist
	r.Get("data.list").ForEach(func(_, item gjson.Result) bool {
		total++
		if total < (o.Page-1)*o.PageSize || int64(len(res)) > o.PageSize {
			return true
		}
		creator := item.Get("creator")

		m := &Playlist{
			ID:         item.Get("tid").String(),
			Name:       item.Get("diss_name").String(),
			PictureURL: item.Get("diss_cover").String(),
			Desc:       item.Get("introduction").String(),
			Creator:    creator.Get("name").String(),
			CreatorUid: o.Keyword,
			PlayCount:  item.Get("listen_num").Int(),
			SongCount:  item.Get("song_cnt").Int(),
		}
		res = append(res, m)
		return true
	})
	return SearchResult[Playlist]{Total: total, Data: res}
}
