package netease

import (
	"fmt"

	"github.com/tidwall/gjson"
)

// Search 搜索歌曲，返回 gjson.Result（路径：result.songs）
func (n *Netease) Search(keyword string) (gjson.Result, error) {
	body, err := n.cloudSearch(keyword, 1)
	if err != nil {
		return gjson.Result{}, err
	}
	result := gjson.ParseBytes(body)
	if result.Get("code").Int() != 200 {
		return gjson.Result{}, fmt.Errorf("netease api error code: %d", result.Get("code").Int())
	}
	return result.Get("result"), nil
}

// SearchPlaylist 搜索歌单，返回 gjson.Result（路径：result.playlists）
func (n *Netease) SearchPlaylist(keyword string) (gjson.Result, error) {
	body, err := n.cloudSearch(keyword, 1000)
	if err != nil {
		return gjson.Result{}, err
	}
	result := gjson.ParseBytes(body)
	if result.Get("code").Int() != 200 {
		return gjson.Result{}, fmt.Errorf("netease api error code: %d", result.Get("code").Int())
	}
	return result.Get("result"), nil
}

// cloudSearch 调用网易云搜索接口
func (n *Netease) cloudSearch(keyword string, searchType int) ([]byte, error) {
	return n.postLinux(searchAPI, map[string]interface{}{
		"method": "POST",
		"url":    "http://music.163.com/api/cloudsearch/pc",
		"params": map[string]interface{}{
			"s":      keyword,
			"type":   searchType,
			"offset": 0,
			"limit":  100,
		},
	})
}
