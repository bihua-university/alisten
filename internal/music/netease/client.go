package netease

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/utils"
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
func (n *Netease) defaultHeaders() []utils.RequestOption {
	return []utils.RequestOption{
		utils.WithHeader("Referer", referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
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

	body, err := utils.Post(apiURL, strings.NewReader(form.Encode()), n.defaultHeaders()...)
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

	body, err := utils.Post(apiURL, strings.NewReader(form.Encode()), n.defaultHeaders()...)
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
