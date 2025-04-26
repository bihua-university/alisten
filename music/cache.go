package music

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

var netease = expirable.NewLRU[string, gin.H](256, nil, 30*time.Minute)
var qq = expirable.NewLRU[string, gin.H](256, nil, 30*time.Minute)

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
