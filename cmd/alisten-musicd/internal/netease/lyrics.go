package netease

import (
	"github.com/tidwall/gjson"
)

// GetLyrics 获取歌词
func (c *Client) GetLyrics(songID string) (string, error) {
	body, err := c.request("https://music.163.com/weapi/song/lyric", map[string]any{
		"csrf_token": "",
		"id":         songID,
		"lv":         -1,
		"tv":         -1,
	}, true)
	if err != nil {
		return "", err
	}

	result := gjson.ParseBytes(body)
	if err := checkCode(result); err != nil {
		return "", err
	}
	return result.Get("lrc.lyric").String(), nil
}
