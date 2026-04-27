package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/utils"
)

// GetDownloadURL 获取歌曲下载链接
func (n *Netease) GetDownloadURL(songID string) (string, error) {
	if url, err := n.tryEAPIQualities(songID, "exhigh"); err == nil && url != "" {
		return url, nil
	}
	return n.getWeapiDownloadURL(songID)
}

func (n *Netease) tryEAPIQualities(songID string, qualities ...string) (string, error) {
	for _, q := range qualities {
		url, err := n.getEAPIDownloadURL(songID, q)
		if err == nil && url != "" {
			return url, nil
		}
	}
	return "", errors.New("no eapi quality available")
}

func (n *Netease) getWeapiDownloadURL(songID string) (string, error) {
	body, err := n.postWeapiNoCache(downloadAPI, map[string]interface{}{
		"ids": []string{songID},
		"br":  320000,
	})
	if err != nil {
		return "", err
	}
	url := gjson.ParseBytes(body).Get("data.0.url").String()
	if url == "" {
		return "", errors.New("download url not found (might be vip or copyright restricted)")
	}
	return url, nil
}

func (n *Netease) getEAPIDownloadURL(songID, quality string) (string, error) {
	idNum, err := strconv.Atoi(songID)
	if err != nil {
		return "", fmt.Errorf("invalid song id: %w", err)
	}

	headerJSON := `{"os":"pc","appver":"","osver":"","deviceId":"pyncm!","requestId":"12345678"}`
	payload := map[string]interface{}{
		"ids":        []int{idNum},
		"level":      quality,
		"encodeType": "flac",
		"header":     headerJSON,
	}
	payloadBytes, _ := json.Marshal(payload)
	params := encryptEApi(downloadEAPI, string(payloadBytes))

	form := url.Values{}
	form.Set("params", params)

	body, err := utils.Post(downloadEAPI, strings.NewReader(form.Encode()), n.defaultHeaders()...)
	if err != nil {
		return "", err
	}

	url := gjson.ParseBytes(body).Get("data.0.url").String()
	if url == "" {
		return "", errors.New("eapi download url not found")
	}
	return url, nil
}
