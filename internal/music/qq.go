package music

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/music/kuwo"
)

var kuwoClient = kuwo.NewClient()

func QQPost(u string, k H) gjson.Result {
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

		picture := fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.Get("albummid").String())
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
			Cover: picture,
		}
		res = append(res, m)
		return true
	})
	return SearchResult[Music]{Total: total, Data: res}
}

func getQQMusic(id string) H {
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

	h := H{
		"type":       "music",
		"url":        download.Get("data.url").String(),
		"webUrl":     GenerateWebURL("qq", id),
		"pictureUrl": picture,
		"duration":   download.Get("data.duration").Int() * 1000,
		"source":     "qq",
		"lyric":      lyric.Get("data.lyric").String(),
		"artist":     artist,
		"name":       name,
		"album": H{
			"name": detail.Get("album.name").String(),
		},
		"id": id,
	}

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
