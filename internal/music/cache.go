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
	if useCache || source != "wy" { // 网易云链接有时效性，可以强制刷新
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
	// deprecated, 可以使用common_url平替
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
	case "url_common":
		// 目前`id`兼职`url`
		var url string = id
		t := task.Scheduler.NewTask("url_common:get_music", map[string]string{"url": url})
		r := task.Scheduler.Call(t, 1*time.Minute)
		if r != nil && r.Result != nil {
			rg := gjson.ParseBytes(r.Result)
			h = H{
				"type":       rg.Get("type").String(),
				"url":        rg.Get("url").String(),
				"id":         id,
				"webUrl":     url,
				"pictureUrl": rg.Get("pictureUrl").String(),
				"duration":   rg.Get("duration").Int(),
				"source":     "url_common",
				"artist":     rg.Get("artist").String(),
				"name":       rg.Get("name").String(),
				"album":      rg.Get("al.name").String(),
			}
		}
	}

	cache.Add(key, h)
	return h
}
