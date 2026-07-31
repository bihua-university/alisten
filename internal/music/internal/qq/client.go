// Package qq 是 QQ 音乐 API 的低层客户端。
package qq

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/internal/utils"
)

const (
	userAgent     = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	searchReferer = "http://m.y.qq.com"
	lyricReferer  = "https://y.qq.com/portal/player.html"
)

// Client 是 QQ 音乐 API 客户端
type Client struct {
	hc    utils.Client
	cache *expirable.LRU[string, []byte]
}

// New 创建一个新的 QQ 音乐客户端
func New() *Client {
	return &Client{
		hc:    utils.Client{RandomIP: true},
		cache: expirable.NewLRU[string, []byte](512, nil, 30*time.Minute),
	}
}

// getCached 发送 GET 请求并返回响应体，结果会缓存
func (c *Client) getCached(apiURL string, header http.Header) ([]byte, error) {
	if v, ok := c.cache.Get(apiURL); ok {
		return v, nil
	}
	body, err := c.hc.Get(apiURL, header)
	if err != nil {
		return nil, err
	}
	c.cache.Add(apiURL, body)
	return body, nil
}

// unwrapJSONP 去除 JSONP 包装
func unwrapJSONP(body []byte) []byte {
	s := string(body)
	if idx := strings.Index(s, "("); idx >= 0 {
		if idx2 := strings.LastIndex(s, ")"); idx2 >= 0 {
			return []byte(s[idx+1 : idx2])
		}
	}
	return body
}

// coverURL 按专辑 ID 拼接封面地址
func coverURL(albumMid string) string {
	if albumMid == "" {
		return ""
	}
	return fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", albumMid)
}

// joinSingers 拼接歌手名称
func joinSingers(singers gjson.Result) string {
	names := make([]string, 0, 2)
	singers.ForEach(func(_, v gjson.Result) bool {
		if name := v.Get("name").String(); name != "" {
			names = append(names, name)
		}
		return true
	})
	return strings.Join(names, ", ")
}
