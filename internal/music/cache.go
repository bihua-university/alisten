package music

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/wdvxdr1123/alisten/internal/music/bihua"
)

var cache = expirable.NewLRU[string, gin.H](32, nil, 30*time.Minute)

/*
   gin.H{
        "type":       "music",
        "url":        r.Get("data.url").String(),
        "pictureUrl": r2.Get("songs.0.al.picUrl").String(),
        "duration":   r2.Get("songs.0.dt").Int(),
        "source":     "netease",
        "lyric":      lyric.Get("lrc.lyric").String(),
        "pushTime":   time.Now().UnixMilli(),
        "name":       m.Name,
    }
*/

func GetMusic(source, id string, useCache bool) gin.H {
	key := source + "OvO" + id
	if useCache {
		if v, ok := cache.Get(key); ok {
			return v
		}
	}

	var h gin.H
	switch source {
	case "wy":
		h = getNeteaseMusic(id)
	case "qq":
		h = getQQMusic(id)
	case "db":
		m, err := bihua.GetMusicByID(id)
		if err != nil {
			return h
		}
		h = bihua.ConvertToGinH(m)
	}

	cache.Add(key, h)
	return h
}
