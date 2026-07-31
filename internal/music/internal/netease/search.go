package netease

import (
	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/internal/types"
)

// 网易云搜索类型
const (
	searchTypeSong     = 1
	searchTypePlaylist = 1000
)

// 搜索结果最多返回条数
const maxSearchTotal = 100

// Search 搜索歌曲，返回全量结果（分页由上层处理）
func (c *Client) Search(keyword string) (*types.Result[types.Music], error) {
	result, err := c.cloudSearch(keyword, searchTypeSong)
	if err != nil {
		return nil, err
	}

	var data []*types.Music
	result.Get("songs").ForEach(func(_, item gjson.Result) bool {
		data = append(data, parseMusic(item))
		return true
	})
	return &types.Result[types.Music]{
		Total: min(result.Get("songCount").Int(), maxSearchTotal),
		Data:  data,
	}, nil
}

// SearchPlaylist 搜索歌单，返回全量结果（分页由上层处理）
func (c *Client) SearchPlaylist(keyword string) (*types.Result[types.Playlist], error) {
	result, err := c.cloudSearch(keyword, searchTypePlaylist)
	if err != nil {
		return nil, err
	}

	var data []*types.Playlist
	result.Get("playlists").ForEach(func(_, item gjson.Result) bool {
		data = append(data, &types.Playlist{
			ID:         item.Get("id").String(),
			Name:       item.Get("name").String(),
			PictureURL: item.Get("coverImgUrl").String(),
			Desc:       item.Get("description").String(),
			Creator:    item.Get("creator.nickname").String(),
			PlayCount:  item.Get("playCount").Int(),
			SongCount:  item.Get("trackCount").Int(),
		})
		return true
	})
	return &types.Result[types.Playlist]{
		Total: min(result.Get("playlistCount").Int(), maxSearchTotal),
		Data:  data,
	}, nil
}

// GetSimilarSongs 获取相似歌曲，返回歌曲 ID 列表
func (c *Client) GetSimilarSongs(songID string) ([]string, error) {
	body, err := c.request(similarSongAPI, map[string]any{
		"songid": songID,
		"limit":  50,
		"offset": 0,
	}, true)
	if err != nil {
		return nil, err
	}

	var ids []string
	gjson.ParseBytes(body).Get("songs").ForEach(func(_, v gjson.Result) bool {
		ids = append(ids, v.Get("id").String())
		return true
	})
	return ids, nil
}

// cloudSearch 调用网易云搜索接口，返回 result 节点
func (c *Client) cloudSearch(keyword string, searchType int) (gjson.Result, error) {
	body, err := c.requestLinux(searchAPI, map[string]any{
		"method": "POST",
		"url":    "http://music.163.com/api/cloudsearch/pc",
		"params": map[string]any{
			"s":      keyword,
			"type":   searchType,
			"offset": 0,
			"limit":  100,
		},
	}, true)
	if err != nil {
		return gjson.Result{}, err
	}

	result := gjson.ParseBytes(body)
	if err := checkCode(result); err != nil {
		return gjson.Result{}, err
	}
	return result.Get("result"), nil
}
