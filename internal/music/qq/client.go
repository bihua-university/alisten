package qq

import (
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/bihua-university/alisten/internal/music/utils"
)

const (
	userAgent     = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	searchReferer = "http://m.y.qq.com"
	lyricReferer  = "https://y.qq.com/portal/player.html"
)

// QQ 是 QQ 音乐 API 客户端
type QQ struct {
	cache *expirable.LRU[string, []byte]
}

// New 创建一个新的 QQ 音乐客户端
func New() *QQ {
	return &QQ{
		cache: expirable.NewLRU[string, []byte](512, nil, 30*time.Minute),
	}
}

// getCached 发送 GET 请求并返回响应体，结果会缓存
func (q *QQ) getCached(apiURL string, opts ...utils.RequestOption) ([]byte, error) {
	if v, ok := q.cache.Get(apiURL); ok {
		return v, nil
	}
	body, err := utils.Get(apiURL, opts...)
	if err != nil {
		return nil, err
	}
	q.cache.Add(apiURL, body)
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
