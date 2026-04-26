package netease

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/tidwall/gjson"
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

// Netease 是网易云音乐 API 客户端
type Netease struct {
	cookie string
	cache  *expirable.LRU[string, []byte]
}

// New 创建一个新的网易云客户端
func New(cookie string) *Netease {
	return &Netease{
		cookie: cookie,
		cache:  expirable.NewLRU[string, []byte](512, nil, 30*time.Minute),
	}
}

// defaultHeaders 返回带有认证信息的默认请求头
func (n *Netease) defaultHeaders() []requestOption {
	return []requestOption{
		withHeader("Referer", referer),
		withHeader("Content-Type", "application/x-www-form-urlencoded"),
		withHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
		withHeader("Cookie", n.cookie),
		withRandomIPHeader(),
	}
}

func cacheKey(apiURL string, reqData map[string]interface{}) string {
	b, _ := json.Marshal(reqData)
	return apiURL + "|" + string(b)
}

// _postWeapi 发送 WeApi 加密请求，可选择是否使用缓存
func (n *Netease) _postWeapi(apiURL string, reqData map[string]interface{}, useCache bool) ([]byte, error) {
	var key string
	if useCache {
		key = cacheKey(apiURL, reqData)
		if v, ok := n.cache.Get(key); ok {
			return v, nil
		}
	}

	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))

	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	body, err := post(apiURL, strings.NewReader(form.Encode()), n.defaultHeaders()...)
	if err != nil {
		return nil, err
	}

	if useCache {
		n.cache.Add(key, body)
	}
	return body, nil
}

// postWeapi 发送 WeApi 加密请求（带缓存）
func (n *Netease) postWeapi(apiURL string, reqData map[string]interface{}) ([]byte, error) {
	return n._postWeapi(apiURL, reqData, true)
}

// postWeapiNoCache 发送 WeApi 加密请求（不带缓存）
func (n *Netease) postWeapiNoCache(apiURL string, reqData map[string]interface{}) ([]byte, error) {
	return n._postWeapi(apiURL, reqData, false)
}

// _postLinux 发送 Linux API 加密请求，可选择是否使用缓存
func (n *Netease) _postLinux(apiURL string, eparams map[string]interface{}, useCache bool) ([]byte, error) {
	var key string
	if useCache {
		key = cacheKey(apiURL, eparams)
		if v, ok := n.cache.Get(key); ok {
			return v, nil
		}
	}

	eparamsJSON, _ := json.Marshal(eparams)
	encryptedParam := EncryptLinux(string(eparamsJSON))

	form := url.Values{}
	form.Set("eparams", encryptedParam)

	body, err := post(apiURL, strings.NewReader(form.Encode()), n.defaultHeaders()...)
	if err != nil {
		return nil, err
	}

	if useCache {
		n.cache.Add(key, body)
	}
	return body, nil
}

// postLinux 发送 Linux API 加密请求（带缓存）
func (n *Netease) postLinux(apiURL string, eparams map[string]interface{}) ([]byte, error) {
	return n._postLinux(apiURL, eparams, true)
}

// postLinuxNoCache 发送 Linux API 加密请求（不带缓存）
func (n *Netease) postLinuxNoCache(apiURL string, eparams map[string]interface{}) ([]byte, error) {
	return n._postLinux(apiURL, eparams, false)
}

// GetSimilarSongs 获取相似歌曲
func (n *Netease) GetSimilarSongs(songID string) (gjson.Result, error) {
	body, err := n.postWeapi(similarSongAPI, map[string]interface{}{
		"songid": songID,
		"limit":  50,
		"offset": 0,
	})
	if err != nil {
		return gjson.Result{}, err
	}
	return gjson.ParseBytes(body), nil
}

// requestOption 是 HTTP 请求的选项函数
type requestOption func(*http.Request)

func withHeader(key, value string) requestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

func withRandomIPHeader() requestOption {
	return func(req *http.Request) {
		ip := randomIP()
		req.Header.Set("X-Real-IP", ip)
		req.Header.Set("X-Forwarded-For", ip)
	}
}

func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rand.Intn(255)+1,
		rand.Intn(256),
		rand.Intn(256),
		rand.Intn(256),
	)
}

// post 发送 POST 请求并返回响应体
func post(apiURL string, body io.Reader, opts ...requestOption) ([]byte, error) {
	req, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(req)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
