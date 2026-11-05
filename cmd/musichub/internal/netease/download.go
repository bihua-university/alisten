package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// DownloadInfo 包含同一音频文件的播放地址与可选的 ReplayGain (dB)。
type DownloadInfo struct {
	URL  string
	Gain *float64
}

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
	idNum, err := strconv.Atoi(songID)
	if err != nil {
		return DownloadInfo{}, fmt.Errorf("invalid song id: %w", err)
	}

	headerJSON := `{"os":"pc","appver":"","osver":"","deviceId":"pyncm!","requestId":"12345678"}`
	payload := map[string]any{
		"ids":        []int{idNum},
		"level":      quality,
		"encodeType": "flac",
		"header":     headerJSON,
	}
	payloadBytes, _ := json.Marshal(payload)
	params := encryptEApi(downloadEAPI, string(payloadBytes))

	form := url.Values{}
	form.Set("params", params)

	body, err := c.hc.Post(downloadEAPI, strings.NewReader(form.Encode()), c.header())
	if err != nil {
		return DownloadInfo{}, err
	}

	return parseDownloadInfo(body)
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
