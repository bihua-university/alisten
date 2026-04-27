package utils

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
)

// RequestOption 是 HTTP 请求的选项函数
type RequestOption func(*http.Request)

// WithHeader 设置请求头
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

// WithRandomIPHeader 添加随机 IP 请求头
func WithRandomIPHeader() RequestOption {
	return func(req *http.Request) {
		ip := RandomIP()
		req.Header.Set("X-Real-IP", ip)
		req.Header.Set("X-Forwarded-For", ip)
	}
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

// RandomIP 生成随机 IP 地址
func RandomIP() string {
	p := prefixes[rand.Intn(len(prefixes))]
	return fmt.Sprintf("%d.%d.%d.%d", p[0], p[1], rand.Intn(256), rand.Intn(256))
}

// Get 发送 GET 请求并返回响应体
func Get(apiURL string, opts ...RequestOption) ([]byte, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Post 发送 POST 请求并返回响应体
func Post(apiURL string, body io.Reader, opts ...RequestOption) ([]byte, error) {
	req, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
