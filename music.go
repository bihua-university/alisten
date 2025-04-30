package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wdvxdr1123/alisten/internal/music"
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

func deleteMusic(c *Context) {
	name := c.Get("id").String()

	deleted := false
	c.WithHouse(func(h *House) {
		for i, o := range h.Playlist {
			m, _ := music.GetMusic(o.source, o.id)["name"].(string)
			if m == name {
				deleted = true
				h.Playlist = append(h.Playlist[:i], h.Playlist[i+1:]...)
				return
			}
		}
	})

	if deleted {
		c.house.PushPlaylist()
		c.Chat("删除音乐 " + name)
	}
}

func pickMusic(c *Context) {
	id := c.Get("id").String()
	name := c.Get("name").String()
	source := c.Get("source").String()
	// 聊天点歌只有名字， 没有
	if id == "" {
		r := music.SearchMusic(music.SearchOption{
			Source:   source,
			Keyword:  c.Get("name").String(),
			Page:     1,
			PageSize: 10,
		})
		if len(r.Data) > 0 {
			id = r.Data[0].ID
		}
	}

	m := music.GetMusic(source, id)
	if m["url"] == nil || m["url"] == "" {
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
		house.Playlist = append(house.Playlist, Order{source: source, id: id, user: c.conn.user})
	})
	c.house.Update()
	if !same {
		// Push new playlist
		c.house.PushPlaylist()
		c.Chat("点歌 " + name)
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

func goodMusic(c *Context) {
	index := c.Get("index").Int()
	name := c.Get("name").String()
	if index == 0 {
		return
	}
	index -= 1 // 跳过正在播放的
	change := false
	likes := 0
	c.WithHouse(func(house *House) {
		// 点赞
		if int(index) < len(house.Playlist) {
			house.Playlist[index].likes += 1
			likes = house.Playlist[index].likes
			sort.SliceStable(house.Playlist, func(i, j int) bool {
				return house.Playlist[i].likes > house.Playlist[j].likes
			})
			change = true
		}
	})
	if change {
		c.Chat(fmt.Sprintf("%s 点赞%d", name, likes))
		c.house.PushPlaylist()
	}
}

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
