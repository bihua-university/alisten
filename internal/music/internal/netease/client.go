// Package netease 是网易云音乐 API 的低层客户端。
package netease

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/internal/types"
	"github.com/bihua-university/alisten/internal/music/internal/utils"
)

const (
	referer        = "http://music.163.com/"
	searchAPI      = "http://music.163.com/api/linux/forward"
	downloadAPI    = "http://music.163.com/weapi/song/enhance/player/url"
	downloadEAPI   = "https://interface3.music.163.com/eapi/song/enhance/player/url/v1"
	detailAPI      = "https://music.163.com/weapi/v3/song/detail"
	playlistAPI    = "https://music.163.com/weapi/v3/playlist/detail"
	similarSongAPI = "https://music.163.com/weapi/discovery/simiSong"
)

// Client 是网易云音乐 API 客户端
type Client struct {
	hc    utils.Client
	cache *expirable.LRU[string, []byte]
}

// New 创建一个新的网易云客户端
func New(cookie string) *Client {
	return &Client{
		hc: utils.Client{
			Header: http.Header{
				"Referer":      {referer},
				"Content-Type": {"application/x-www-form-urlencoded"},
				"User-Agent":   {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"},
				"Cookie":       {cookie},
			},
			RandomIP: true,
		},
		cache: expirable.NewLRU[string, []byte](512, nil, 30*time.Minute),
	}
}

func cacheKey(apiURL string, reqData map[string]any) string {
	b, _ := json.Marshal(reqData)
	return apiURL + "|" + string(b)
}

// post 发送表单 POST 请求并返回响应体，可选择是否使用缓存
func (c *Client) post(apiURL string, form url.Values, useCache bool, cacheID string) ([]byte, error) {
	if useCache {
		if v, ok := c.cache.Get(cacheID); ok {
			return v, nil
		}
	}
	body, err := c.hc.Post(apiURL, strings.NewReader(form.Encode()), nil)
	if err != nil {
		return nil, err
	}
	if useCache {
		c.cache.Add(cacheID, body)
	}
	return body, nil
}

// request 发送 WeAPI 加密请求
func (c *Client) request(apiURL string, reqData map[string]any, useCache bool) ([]byte, error) {
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))

	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	return c.post(apiURL, form, useCache, cacheKey(apiURL, reqData))
}

// requestLinux 发送 Linux API 加密请求
func (c *Client) requestLinux(apiURL string, eparams map[string]any, useCache bool) ([]byte, error) {
	eparamsJSON, _ := json.Marshal(eparams)

	form := url.Values{}
	form.Set("eparams", EncryptLinux(string(eparamsJSON)))

	return c.post(apiURL, form, useCache, cacheKey(apiURL, eparams))
}

// checkCode 校验响应中的业务错误码
func checkCode(result gjson.Result) error {
	if code := result.Get("code").Int(); code != 200 {
		return fmt.Errorf("netease api error code: %d", code)
	}
	return nil
}

// parseMusic 从单曲数据解析 Music
func parseMusic(item gjson.Result) *types.Music {
	return &types.Music{
		ID:       item.Get("id").String(),
		Name:     item.Get("name").String(),
		Artist:   parseArtists(item),
		Album:    item.Get("al.name").String(),
		Duration: item.Get("dt").Int(),
		Cover:    item.Get("al.picUrl").String(),
		Source:   types.NetEase,
	}
}

// parseArtists 拼接歌手名称
func parseArtists(item gjson.Result) string {
	var names []string
	item.Get("ar").ForEach(func(_, ar gjson.Result) bool {
		if name := ar.Get("name").String(); name != "" {
			names = append(names, name)
		}
		return true
	})
	return strings.Join(names, ", ")
}
