package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bihua-university/alisten/cmd/musichub/internal/utils"
	"github.com/tidwall/gjson"
)

// DownloadInfo 包含同一音频文件的播放地址与可选的 ReplayGain (dB)。
type DownloadInfo struct {
	URL  string
	Gain *float64
}

var errEAPITemporaryRejection = errors.New("netease EAPI temporarily rejected the request (-460)")

// GetDownloadURL 获取歌曲下载链接。
func (c *Client) GetDownloadURL(songID string) (string, error) {
	info, err := c.GetDownloadInfo(songID)
	return info.URL, err
}

// GetDownloadInfo 优先使用 EAPI，并保留 WeAPI 回退。
func (c *Client) GetDownloadInfo(songID string) (DownloadInfo, error) {
	if info, err := c.tryEAPIQualities(songID, "exhigh"); err == nil && info.URL != "" {
		return info, nil
	}
	return c.getWeapiDownloadURL(songID)
}

func (c *Client) tryEAPIQualities(songID string, qualities ...string) (DownloadInfo, error) {
	for _, q := range qualities {
		info, err := c.getEAPIDownloadURL(songID, q)
		if err == nil && info.URL != "" {
			return info, nil
		}
	}
	return DownloadInfo{}, errors.New("no eapi quality available")
}

func (c *Client) getWeapiDownloadURL(songID string) (DownloadInfo, error) {
	body, err := c.request(downloadAPI, map[string]any{
		"ids": []string{songID},
		"br":  320000,
	}, false)
	if err != nil {
		return DownloadInfo{}, err
	}
	return parseDownloadInfo(body)
}

func (c *Client) getEAPIDownloadURL(songID, quality string) (DownloadInfo, error) {
	for attempt := 0; ; attempt++ {
		info, err := c.requestEAPIDownloadURL(songID, quality)
		if !errors.Is(err, errEAPITemporaryRejection) || attempt >= 2 {
			return info, err
		}
		// 暂时拒绝时先重试 EAPI，避免直接回退到不提供有效增益的 WeAPI。
		time.Sleep(250 * time.Millisecond)
	}
}

func (c *Client) requestEAPIDownloadURL(songID, quality string) (DownloadInfo, error) {
	idNum, err := strconv.Atoi(songID)
	if err != nil {
		return DownloadInfo{}, fmt.Errorf("invalid song id: %w", err)
	}

	// EAPI 需要一致的桌面客户端身份和会话；旧的空 appver / 浏览器 Cookie
	// 虽然能取到 URL，但网易云可能只返回 gain: 0。
	header := c.eapiHeader()
	payload := map[string]any{
		"ids":        fmt.Sprintf("[%d]", idNum),
		"level":      quality,
		"encodeType": "mp3",
		"header":     header,
		"e_r":        false,
	}
	if quality == "lossless" || quality == "hires" {
		payload["encodeType"] = "flac"
	}
	payloadBytes, _ := json.Marshal(payload)
	params := encryptEApi(downloadEAPI, string(payloadBytes))

	form := url.Values{}
	form.Set("params", params)

	cookies := make([]string, 0, len(header))
	for key, value := range header {
		cookies = append(cookies, url.QueryEscape(key)+"="+url.QueryEscape(value))
	}
	// 使用独立的 EAPI 请求头，避免混入浏览器 UA 和随机转发 IP。
	client := utils.Client{}
	body, err := client.Post(downloadEAPI, strings.NewReader(form.Encode()), http.Header{
		"Content-Type": {"application/x-www-form-urlencoded"},
		"User-Agent":   {"NeteaseMusic 9.0.90/5038 (iPhone; iOS 16.2; zh_CN)"},
		"Cookie":       {strings.Join(cookies, "; ")},
	})
	if err != nil {
		return DownloadInfo{}, err
	}

	if gjson.GetBytes(body, "code").Int() == -460 {
		return DownloadInfo{}, errEAPITemporaryRejection
	}
	return parseDownloadInfo(body)
}

func (c *Client) eapiHeader() map[string]string {
	now := time.Now()
	header := map[string]string{
		"os":          "pc",
		"appver":      "3.1.17.204416",
		"osver":       "Microsoft-Windows-10-Professional-build-19045-64bit",
		"deviceId":    c.deviceID,
		"versioncode": "140",
		"mobilename":  "",
		"buildver":    strconv.FormatInt(now.Unix(), 10),
		"resolution":  "1920x1080",
		"channel":     "netease",
		"requestId":   fmt.Sprintf("%d_%04d", now.UnixMilli(), now.Nanosecond()%1000),
		"__csrf":      "",
	}
	request := &http.Request{Header: c.header()}
	for _, cookie := range request.Cookies() {
		switch cookie.Name {
		case "MUSIC_U", "NMTID", "__csrf":
			header[cookie.Name] = cookie.Value
		}
	}
	return header
}

func parseDownloadInfo(body []byte) (DownloadInfo, error) {
	data := gjson.ParseBytes(body).Get("data.0")
	info := DownloadInfo{URL: data.Get("url").String()}
	if strings.TrimSpace(info.URL) == "" {
		return DownloadInfo{}, errors.New("download url not found (might be vip or copyright restricted)")
	}
	if gain := data.Get("gain"); gain.Type == gjson.Number {
		value := gain.Float()
		if !math.IsNaN(value) && !math.IsInf(value, 0) {
			info.Gain = &value
		}
	}
	return info, nil
}
