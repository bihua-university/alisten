package music

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/qq"
)

var qqClient = qq.New()

func post(u string, k url.Values) gjson.Result {
	response, err := http.PostForm(u, k)
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

func crc(id string) string {
	data := "music.gdstudio.org|20251104|" + strconv.FormatInt(time.Now().UnixMilli(), 10)[:9] + "|" + url.PathEscape(id)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%X", hash[12:])
}

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
	search := post("https://music.gdstudio.org/api.php", url.Values{
		"types":  []string{"search"},
		"source": []string{"kuwo"},
		"name":   []string{key},
		"pages":  []string{"1"},
		"count":  []string{"20"},
		"s":      []string{crc(key)},
	})

	rid := search.Get("0.id").String()
	download := post("https://music.gdstudio.org/api.php", url.Values{
		"types":  []string{"url"},
		"source": []string{"kuwo"},
		"id":     []string{rid},
		"br":     []string{"320"},
		"s":      []string{crc(rid)},
	})

	ablumMid := detail.Get("album.mid").String()
	picture := fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", ablumMid)

	return H{
		"type":       "music",
		"url":        download.Get("url").String(),
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
