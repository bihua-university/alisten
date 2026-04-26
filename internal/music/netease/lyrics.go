package netease

import (
	"fmt"

	"github.com/tidwall/gjson"
)

// GetLyrics 获取歌词
func (n *Netease) GetLyrics(songID string) (string, error) {
	body, err := n.postWeapi("https://music.163.com/weapi/song/lyric", map[string]interface{}{
		"csrf_token": "",
		"id":         songID,
		"lv":         -1,
		"tv":         -1,
	})
	if err != nil {
		return "", err
	}

	result := gjson.ParseBytes(body)
	if result.Get("code").Int() != 200 {
		return "", fmt.Errorf("netease api error code: %d", result.Get("code").Int())
	}
	return result.Get("lrc.lyric").String(), nil
}
