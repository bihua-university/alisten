package qq

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/utils"
)

// GetLyrics 获取歌词
func (q *QQ) GetLyrics(songMID string) (string, error) {
	params := url.Values{}
	params.Set("songmid", songMID)
	params.Set("loginUin", "0")
	params.Set("hostUin", "0")
	params.Set("format", "json")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	params.Set("notice", "0")
	params.Set("platform", "y.qq.json")
	params.Set("needNewCode", "0")

	apiURL := "https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?" + params.Encode()
	body, err := q.getCached(apiURL,
		utils.WithHeader("Referer", lyricReferer),
		utils.WithHeader("User-Agent", userAgent),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return "", err
	}

	result := gjson.ParseBytes(unwrapJSONP(body))
	lyric := result.Get("lyric").String()
	if lyric == "" {
		return "", errors.New("lyric is empty or not found")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(lyric)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}
	return string(decodedBytes), nil
}
