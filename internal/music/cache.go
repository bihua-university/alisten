package music

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/task"
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
		t := task.Scheduler.NewTask("bilibili:get_music", map[string]string{"bvid": id})
		r := task.Scheduler.Call(t, 1*time.Minute)
		if r != nil && r.Result != nil {
			rg := gjson.ParseBytes(r.Result)
			h = H{
				"type":       rg.Get("type").String(),
				"url":        rg.Get("url").String(),
				"id":         id,
				"webUrl":     rg.Get("webUrl").String(),
				"pictureUrl": rg.Get("pictureUrl").String(),
				"duration":   rg.Get("duration").Int(),
				"source":     "db",
				"artist":     rg.Get("artist").String(),
				"name":       rg.Get("name").String(),
				"album":      rg.Get("al.name").String(),
			}
		}
	}

	cache.Add(key, h)
	return h
}
