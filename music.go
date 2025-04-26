package main

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wdvxdr1123/alisten/music"
)

type Order struct {
	source string
	id     string
	user   string
	likes  int
}

func searchMusic(c *Context) {
	name := c.Get("name").String()
	o := music.SearchOption{
		Source:   c.Get("source").String(),
		Keyword:  c.Get("name").String(),
		Page:     c.Get("pageIndex").Int(),
		PageSize: c.Get("pageSize").Int(),
	}
	var r music.SearchResult[music.Music]
	if strings.HasPrefix(name, "*") {
		o.ID = strings.TrimPrefix(name, "*")
		r = music.GetSongList(o)
	} else {
		r = music.SearchMusic(o)
	}
	c.conn.Send(gin.H{
		"type":      "search",
		"data":      r.Data,
		"totalSize": r.Total,
	})
}

func pickMusic(c *Context) {
	id := c.Get("id").String()
	name := c.Get("name").String()
	source := c.Get("source").String()
	m := music.GetMusic(source, id)

	if m["name"] != name {
		// 点歌失败
		return
	}

	same := false
	c.WithHouse(func(house *House) {
		for _, o := range house.Playlist {
			if o.id == id {
				same = true
			}
		}
		if same {
			// 重复点歌
			return
		}
		c.Chat("点歌 " + name)
		house.Playlist = append(house.Playlist, Order{source: source, id: id, user: c.conn.user})
	})
	c.house.Update()
	if !same {
		// push new playlist
		c.house.PushPlaylist()
	}
}

func merge(h1, h2 gin.H) gin.H {
	r := make(gin.H, len(h1)+len(h2))
	for k, v := range h1 {
		r[k] = v
	}
	for k, v := range h2 {
		r[k] = v
	}
	return r
}

func voteSkip(c *Context) {
	c.WithHouse(func(house *House) {
		// 检查用户是否已投票
		for _, user := range house.VoteSkip {
			if user == c.conn.user {
				return
			}
		}
	})

	c.Chat("投票切歌")
	c.house.Skip()
	/*
	   house.VoteSkip = append(house.VoteSkip, request.User)

	   // 如果有超过3个不同用户投票就切歌
	   if len(house.VoteSkip) >= 3 {
	       if len(house.Playlist) > 0 {
	           house.Playlist = house.Playlist[1:]
	       }
	       house.VoteSkip = nil
	       house.Mu.Unlock()
	       c.JSON(http.StatusOK, gin.H{"message": "歌曲已切换"})
	       return
	   }

	   c.JSON(http.StatusOK, gin.H{"message": "投票已记录", "current_votes": len(house.VoteSkip)})
	*/
}
