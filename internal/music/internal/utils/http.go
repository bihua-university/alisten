// Package utils 提供简单的 HTTP 请求工具。
package utils

import (
	"fmt"
	"io"
	"maps"
	"math/rand"
	"net/http"
)

// Client 是带有默认请求头的 HTTP 客户端，零值即可使用。
type Client struct {
	// Header 是每个请求都会携带的默认请求头，可为 nil。
	Header http.Header
	// RandomIP 为每个请求设置随机的 X-Real-IP 与 X-Forwarded-For。
	RandomIP bool
}

// Get 发送 GET 请求并返回响应体。header 中非 nil 的字段会覆盖默认请求头。
func (c *Client) Get(apiURL string, header http.Header) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req, header)
}

// Post 发送 POST 请求并返回响应体。header 中非 nil 的字段会覆盖默认请求头。
func (c *Client) Post(apiURL string, body io.Reader, header http.Header) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return nil, err
	}
	return c.do(req, header)
}

func (c *Client) do(req *http.Request, header http.Header) ([]byte, error) {
	maps.Copy(req.Header, c.Header)
	maps.Copy(req.Header, header)
	if c.RandomIP {
		ip := randomIP()
		req.Header.Set("X-Real-IP", ip)
		req.Header.Set("X-Forwarded-For", ip)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

var prefixes = [...][2]int{
	{116, 255},
	{116, 228},
	{218, 192},
	{124, 0},
	{14, 132},
	{183, 14},
	{58, 14},
	{113, 116},
	{120, 230},
}

// randomIP 生成随机 IP 地址
func randomIP() string {
	p := prefixes[rand.Intn(len(prefixes))]
	return fmt.Sprintf("%d.%d.%d.%d", p[0], p[1], rand.Intn(256), rand.Intn(256))
}
