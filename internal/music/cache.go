package music

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/bihua-university/alisten/internal/music/bihua"
)

var cache = expirable.NewLRU[string, gin.H](256, nil, 30*time.Minute)

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
