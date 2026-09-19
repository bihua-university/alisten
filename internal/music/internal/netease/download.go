package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// GetDownloadURL 获取歌曲下载链接
func (c *Client) GetDownloadURL(songID string) (string, error) {
	if url, err := c.tryEAPIQualities(songID, "exhigh"); err == nil && url != "" {
		return url, nil
	}
	return c.getWeapiDownloadURL(songID)
}

func (c *Client) tryEAPIQualities(songID string, qualities ...string) (string, error) {
	for _, q := range qualities {
		url, err := c.getEAPIDownloadURL(songID, q)
		if err == nil && url != "" {
			return url, nil
		}
	}
	return "", errors.New("no eapi quality available")
}

func (c *Client) getWeapiDownloadURL(songID string) (string, error) {
	body, err := c.request(downloadAPI, map[string]any{
		"ids": []string{songID},
		"br":  320000,
	}, false)
	if err != nil {
		return "", err
	}
	url := gjson.ParseBytes(body).Get("data.0.url").String()
	if url == "" {
		return "", errors.New("download url not found (might be vip or copyright restricted)")
	}
	return url, nil
}

func (c *Client) getEAPIDownloadURL(songID, quality string) (string, error) {
	idNum, err := strconv.Atoi(songID)
	if err != nil {
		return "", fmt.Errorf("invalid song id: %w", err)
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
		return "", err
	}

	url := gjson.ParseBytes(body).Get("data.0.url").String()
	if url == "" {
		return "", errors.New("eapi download url not found")
	}
	return url, nil
}
