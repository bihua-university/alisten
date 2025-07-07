package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bihua-university/alisten/internal/music"
	"github.com/bihua-university/alisten/internal/music/bihua"
)

type Order struct {
	source string
	id     string
	user   string
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

// doPickMusic 核心点歌逻辑，被pickMusic和pickMusicHTTP共同使用
func doPickMusic(house *House, id, name, source, user string) PickMusicResult {
	// 聊天点歌只有名字，没有ID的情况
	if id == "" {
		if strings.HasPrefix(name, "BV") {
			db := music.GetMusic("db", name, true)
			if db["id"] != name {
				t := scheduler.NewTask("bilibili_upload", map[string]string{"bvid": name})
				result := scheduler.Call(t, time.Minute*5)

				if result != nil && result.Result != nil {
					duration, _ := strconv.ParseInt(result.Result["duration"], 10, 64)
					bihua.InsertMusic(&bihua.MusicModel{
						MusicID:    name,
						Name:       result.Result["name"],
						Artist:     result.Result["artist"],
						AlbumName:  result.Result["album"],
						PictureURL: result.Result["picture"],
						Duration:   duration,
						URL:        result.Result["audio"],
					})
				}
			}
			source = "db"
			id = name
		} else {
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

// pickMusicHTTP 为HTTP请求提供点歌功能
func pickMusicHTTP(c *gin.Context) {
	var request struct {
		HouseID  string `json:"houseId"`
		Password string `json:"password"`
		ID       string `json:"id"`
		Name     string `json:"name"`
		Source   string `json:"source"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// 验证房间存在性和密码
	house := GetHouse(request.HouseID)
	if house == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
		return
	}
	if house.Password != request.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	// 调用核心点歌逻辑
	result := doPickMusic(house, request.ID, request.Name, request.Source, "HTTP User")

	if result.Success {
		c.JSON(http.StatusOK, gin.H{
			"code":    "20000",
			"message": result.Message,
			"data": gin.H{
				"name":   result.Name,
				"source": result.Source,
				"id":     result.ID,
			},
		})
	} else {
		statusCode := http.StatusBadRequest
		c.JSON(statusCode, gin.H{"error": result.Message})
	}
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
		c.Chat("删除音乐 " + name)
	}
}

func pickMusic(c *Context) {
	id := c.Get("id").String()
	name := c.Get("name").String()
	source := c.Get("source").String()

	// 调用核心点歌逻辑
	result := doPickMusic(c.house, id, name, source, c.conn.user)

	if result.Success {
		// Push new playlist
		c.Chat("点歌 " + result.Name)
	}
	// WebSocket版本不需要返回错误响应，静默失败即可
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

func playMode(c *Context) {
	mode := c.Get("mode").String()
	c.WithHouse(func(house *House) {
		switch mode {
		case "normal":
			house.Mode = NormalMode
		case "random":
			house.Mode = RandomMode
		}
	})
}

func getCurrentMusic(c *Context) {
	c.WithHouse(func(h *House) {
		if h.Current.id != "" {
			// 发送播放单曲
			m := music.GetMusic(h.Current.source, h.Current.id, false)
			r := merge(m, gin.H{
				"pushTime": h.PushTime,
			})
			c.conn.Send(r)
		}
	})
}
