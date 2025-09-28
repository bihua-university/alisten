package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/music"
)

type Order struct {
	source string
	id     string
	user   auth.User
	likes  int
}

// PickMusicResult 点歌结果
type PickMusicResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Name    string `json:"name,omitempty"`
	Source  string `json:"source,omitempty"`
	ID      string `json:"id,omitempty"`
}

func doPickMusic(house *House, id, name, source string, user auth.User) PickMusicResult {
	// 聊天点歌只有名字，没有ID的情况
	if id == "" {
		r := music.SearchMusic(music.SearchOption{
			Source:   source,
			Keyword:  name,
			Page:     1,
			PageSize: 10,
		})
		if len(r.Data) > 0 {
			id = r.Data[0].ID
		}
	}

	m := music.GetMusic(source, id, true)
	url, ok := m["url"].(string)
	if !ok || url == "" {
		return PickMusicResult{
			Success: false,
			Message: "点歌失败，无法获取音乐信息",
		}
	}

	same := false
	house.Mu.Lock()
	for _, o := range house.Playlist {
		if o.id == id {
			same = true
			break
		}
	}
	if !same {
		house.Playlist = append(house.Playlist, Order{source: source, id: id, user: user})
		house.lastOrderTime = time.Now()
	}
	house.Mu.Unlock()

	if same {
		return PickMusicResult{
			Success: false,
			Message: "重复点歌",
		}
	}

	house.Update()
	house.PushPlaylist()

	// 获取实际的音乐名称
	if actualName, ok := m["name"].(string); ok && actualName != "" {
		name = actualName
	}

	return PickMusicResult{
		Success: true,
		Message: "点歌成功",
		Name:    name,
		Source:  source,
		ID:      id,
	}
}

func searchMusic(c *Context) {
	c.house.Wait(WaitSearch)
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

	if c.IsWebSocket() {
		c.conn.Send(base.H{
			"type":      "search",
			"data":      r.Data,
			"totalSize": r.Total,
		})
	}
	if c.IsHTTP() {
		c.Send(base.H{
			"list":      r.Data,
			"totalSize": r.Total,
		})
	}
}

func deleteMusic(c *Context) {
	if !c.house.Wait(WaitOrder) { // 与点歌共用
		c.Info("操作过于频繁，请稍后再试")
		return
	}
	name := c.Get("id").String()

	deleted := false
	c.WithHouse(func(h *House) {
		for i, o := range h.Playlist {
			m, _ := music.GetMusic(o.source, o.id, true)["name"].(string)
			if m == name {
				deleted = true
				h.Playlist = append(h.Playlist[:i], h.Playlist[i+1:]...)
				return
			}
		}
	})

	if deleted {
		c.house.PushPlaylist()
		if c.IsWebSocket() {
			c.Chat("删除音乐 " + name)
		}
		if c.IsHTTP() {
			c.Send(base.H{"name": name})
		}
	} else if c.IsHTTP() {
		writeJSON(c.hw, http.StatusNotFound, base.H{"error": "未找到要删除的音乐"})
	}
}

func pickMusic(c *Context) {
	if !c.house.Wait(WaitOrder) {
		c.Info("操作过于频繁，请稍后再试")
		return
	}

	// 限制点歌数量
	exceed := false
	c.WithHouse(func(h *House) {
		if h.ultimate {
			return
		}
		if len(h.Playlist) >= 10 {
			exceed = true
		}
	})
	if exceed {
		c.Info("已超过最大点歌数量10首,请稍后再试!")
		return
	}

	id := c.Get("id").String()
	name := c.Get("name").String()
	source := c.Get("source").String()

	// 调用核心点歌逻辑
	result := doPickMusic(c.house, id, name, source, c.User())

	if result.Success && c.IsWebSocket() {
		// Push new playlist
		c.Chat("点歌 " + result.Name)
	}
	if c.IsHTTP() {
		if result.Success {
			c.Send(base.H{
				"name":   result.Name,
				"source": result.Source,
				"id":     result.ID,
			})
		} else {
			writeJSON(c.hw, http.StatusBadRequest, base.H{"error": result.Message})
		}
	}
}

func merge(h1, h2 base.H) base.H {
	r := make(base.H, len(h1)+len(h2))
	for k, v := range h1 {
		r[k] = v
	}
	for k, v := range h2 {
		r[k] = v
	}
	return r
}

func voteSkip(c *Context) {
	voted := false
	requiredVotes := 0
	voteCount := 0
	c.WithHouse(func(house *House) {
		// 检查用户是否已投票
		user := c.User()
		for _, existingUser := range house.VoteSkip {
			if user == existingUser {
				voted = true
				return
			}
		}

		if !voted {
			house.VoteSkip = append(house.VoteSkip, user)
		}

		// 向上取整，至少需要三分之一的用户投票
		requiredVotes = max((len(house.Connection)+2)/3, 1)
		voteCount = len(c.house.VoteSkip)
	})

	if voted {
		if c.IsHTTP() {
			writeJSON(c.hw, http.StatusBadRequest, base.H{"error": "您已经投过票了"})
		}
		return
	}

	// 如果票数达到要求，直接切歌
	if voteCount >= requiredVotes {
		c.house.Skip(true)
		if c.IsWebSocket() {
			c.Chat("投票切歌成功")
		}
		if c.IsHTTP() {
			c.Send(base.H{"current_votes": voteCount, "required_votes": requiredVotes})
		}
	} else {
		if c.IsWebSocket() {
			c.Chat(fmt.Sprintf("投票切歌 (%d/%d)", voteCount, requiredVotes))
		}
		if c.IsHTTP() {
			c.Send(base.H{"current_votes": voteCount, "required_votes": requiredVotes})
		}
	}
}

func goodMusic(c *Context) {
	if !c.house.Wait(WaitLike) {
		c.Info("操作过于频繁，请稍后再试")
		return
	}
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
		c.house.PushPlaylist()
		if c.IsWebSocket() {
			c.Chat(fmt.Sprintf("%s 点赞%d", name, likes))
		}
		if c.IsHTTP() {
			c.Send(base.H{"name": name, "likes": likes})
		}
	} else if c.IsHTTP() {
		writeJSON(c.hw, http.StatusNotFound, base.H{"error": "未找到对应音乐"})
	}
}

func searchList(c *Context) {
	c.house.Wait(WaitSearch)
	r := music.SearchPlaylist(music.SearchOption{
		Source:   c.Get("source").String(),
		Keyword:  c.Get("name").String(),
		Page:     c.Get("pageIndex").Int(),
		PageSize: c.Get("pageSize").Int(),
	})

	if c.IsWebSocket() {
		c.conn.Send(base.H{
			"type":      "searchlist",
			"data":      r.Data,
			"totalSize": r.Total,
		})
	}
	if c.IsHTTP() {
		c.Send(base.H{
			"list":      r.Data,
			"totalSize": r.Total,
		})
	}
}

func playMode(c *Context) {
	mode := c.Get("mode").String()
	c.WithHouse(func(house *House) {
		switch mode {
		case "sequential":
			house.Mode = NormalMode
		case "random":
			house.Mode = RandomMode
		}
	})

	if c.IsHTTP() {
		c.Send(base.H{"mode": mode})
	}
}

func getCurrentMusic(c *Context) {
	c.WithHouse(func(h *House) {
		if h.Current.id != "" {
			// 发送播放单曲
			m := music.GetMusic(h.Current.source, h.Current.id, false)
			r := merge(m, base.H{
				"pushTime": h.PushTime,
			})

			if c.IsWebSocket() {
				c.conn.Send(r)
			}
			if c.IsHTTP() {
				// ensure user who picked the song is included in HTTP response
				// r already merged music fields and pushTime; add user if available
				r["user"] = h.Current.user
				c.Send(r)
			}
		} else if c.IsHTTP() {
			writeJSON(c.hw, http.StatusNotFound, base.H{"error": "当前没有播放音乐"})
		}
	})
}

func getPlaylist(c *Context) {
	// build playlist response
	type item struct {
		Name   string    `json:"name"`
		Source string    `json:"source"`
		ID     string    `json:"id"`
		Likes  int       `json:"likes"`
		User   auth.User `json:"user"`
	}

	var list []item
	c.WithHouse(func(house *House) {
		for _, o := range house.Playlist {
			m := music.GetMusic(o.source, o.id, true)
			name, _ := m["name"].(string)
			list = append(list, item{
				Name:   name,
				Source: o.source,
				ID:     o.id,
				Likes:  o.likes,
				User:   o.user,
			})
		}
	})

	if c.IsWebSocket() {
		c.conn.Send(base.H{
			"type": "playlist",
			"data": list,
		})
	}
	if c.IsHTTP() {
		c.Send(base.H{"playlist": list})
	}
}
