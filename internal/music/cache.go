package music

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/bihua-university/alisten/internal/music/bihua"
)

type H = map[string]any

var cache = expirable.NewLRU[string, H](256, nil, 30*time.Minute)

func GetMusic(source, id string, useCache bool) H {
	key := source + "OvO" + id
	if useCache {
		if v, ok := cache.Get(key); ok {
			return v
		}
	}

	var h H
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
