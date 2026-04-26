package netease

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
)

// GetPlaylistDetail 获取歌单详情，返回原始响应 gjson.Result（含 playlist 和 trackIds）
func (n *Netease) GetPlaylistDetail(playlistID string) (gjson.Result, error) {
	body, err := n.postWeapi(playlistAPI, map[string]interface{}{
		"id":         playlistID,
		"n":          0,
		"csrf_token": "",
	})
	if err != nil {
		return gjson.Result{}, err
	}
	result := gjson.ParseBytes(body)
	if result.Get("code").Int() != 200 {
		return gjson.Result{}, fmt.Errorf("netease api error code: %d", result.Get("code").Int())
	}
	return result, nil
}

// GetSongDetail 批量获取歌曲详情，返回原始响应 gjson.Result（含 songs）
func (n *Netease) GetSongDetail(songIDs []string) (gjson.Result, error) {
	if len(songIDs) == 0 {
		return gjson.Result{}, nil
	}

	cList := make([]map[string]interface{}, 0, len(songIDs))
	for _, id := range songIDs {
		cList = append(cList, map[string]interface{}{"id": id})
	}
	cJSON, _ := json.Marshal(cList)
	idsJSON, _ := json.Marshal(songIDs)

	body, err := n.postWeapi(detailAPI, map[string]interface{}{
		"c":   string(cJSON),
		"ids": string(idsJSON),
	})
	if err != nil {
		return gjson.Result{}, err
	}
	return gjson.ParseBytes(body), nil
}

