package kuwo

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/tidwall/gjson"
)

// GetDownloadURL 获取歌曲下载链接
func (c *Client) GetDownloadURL(rid string) (string, error) {
	qualities := []string{"320kmp3", "128kmp3"}
	randomID := fmt.Sprintf("C_APK_guanwang_%d%d", time.Now().UnixNano(), rand.Intn(1000000))

	for _, br := range qualities {
		params := url.Values{}
		params.Set("f", "web")
		params.Set("source", "kwplayercar_ar_6.0.0.9_B_jiakong_vh.apk")
		params.Set("from", "PC")
		params.Set("type", "convert_url_with_sign")
		params.Set("br", br)
		params.Set("rid", rid)
		params.Set("user", randomID)

		apiURL := "https://mobi.kuwo.cn/mobi.s?" + params.Encode()

		body, err := httpClient.Get(apiURL, http.Header{
			"User-Agent": {userAgent},
		})
		if err != nil {
			continue
		}

		url := gjson.ParseBytes(body).Get("data.url").String()
		if url != "" {
			return url, nil
		}
	}

	// fallback
	fallbackURL := fmt.Sprintf("http://www.kuwo.cn/api/v1/www/music/playUrl?mid=%s&type=music&httpsStatus=1", rid)
	fallbackBody, err := httpClient.Get(fallbackURL, http.Header{
		"User-Agent": {userAgent},
		"Secret":     {"kuwo_web_secret"},
	})
	if err != nil {
		return "", errors.New("download url not found")

	}
	url := gjson.ParseBytes(fallbackBody).Get("data.url").String()
	return url, nil
}
