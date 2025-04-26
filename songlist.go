package main

import (
	"github.com/gin-gonic/gin"

	"github.com/wdvxdr1123/alisten/music"
)

func searchList(c *Context) {
	r := music.SearchPlaylist(music.SearchOption{
		Source:   c.Get("source").String(),
		Keyword:  c.Get("name").String(),
		Page:     c.Get("pageIndex").Int(),
		PageSize: c.Get("pageSize").Int(),
	})
	c.conn.Send(gin.H{
		"type":      "searchlist",
		"data":      r.Data,
		"totalSize": r.Total,
	})
}
