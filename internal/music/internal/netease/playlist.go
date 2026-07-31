package netease

import (
	"encoding/json"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/internal/types"
)

// GetSongDetail 批量获取歌曲详情
func (c *Client) GetSongDetail(songIDs []string) ([]*types.Music, error) {
	if len(songIDs) == 0 {
		return nil, nil
	}

	cList := make([]map[string]any, 0, len(songIDs))
	for _, id := range songIDs {
		cList = append(cList, map[string]any{"id": id})
	}
	cJSON, _ := json.Marshal(cList)
	idsJSON, _ := json.Marshal(songIDs)

	body, err := c.request(detailAPI, map[string]any{
		"c":   string(cJSON),
		"ids": string(idsJSON),
	}, true)
	if err != nil {
		return nil, err
	}

	var data []*types.Music
	gjson.ParseBytes(body).Get("songs").ForEach(func(_, item gjson.Result) bool {
		data = append(data, parseMusic(item))
		return true
	})
	return data, nil
}

// PlaylistSongs 获取歌单的全部歌曲
func (c *Client) PlaylistSongs(playlistID string) ([]*types.Music, error) {
	body, err := c.request(playlistAPI, map[string]any{
		"id":         playlistID,
		"n":          0,
		"csrf_token": "",
	}, true)
	if err != nil {
		return nil, err
	}

	result := gjson.ParseBytes(body)
	if err := checkCode(result); err != nil {
		return nil, err
	}

	var songIDs []string
	result.Get("playlist.trackIds").ForEach(func(_, tid gjson.Result) bool {
		songIDs = append(songIDs, tid.Get("id").String())
		return true
	})

	var data []*types.Music
	const batchSize = 500
	for i := 0; i < len(songIDs); i += batchSize {
		end := min(i+batchSize, len(songIDs))
		batch, err := c.GetSongDetail(songIDs[i:end])
		if err != nil {
			continue // 跳过失败的批次，保持部分结果可用
		}
		data = append(data, batch...)
	}
	return data, nil
}
