package task

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	ServerURL   string
	Token       string // 鉴权Token
	PollTimeout time.Duration
	HTTPClient  *http.Client
}

// NewClient 创建新的长轮询客户端
func NewClient(serverURL, token string) *Client {
	certPool := x509.NewCertPool()
	return &Client{
		ServerURL:   serverURL,
		Token:       token,
		PollTimeout: 30 * time.Second, // 30秒超时
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:            certPool,
					InsecureSkipVerify: true, // 信任所有证书
				},
			},
			Timeout: 35 * time.Second, // 稍微长于轮询超时
		},
	}
}

// GetTask 通过长轮询获取任务
func (c *Client) GetTask(ctx context.Context) (*Task, error) {
	url := fmt.Sprintf("%s/tasks/poll?timeout=%d", c.ServerURL, int(c.PollTimeout.Seconds()))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var task Task
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, fmt.Errorf("解析任务失败: %w", err)
	}

	return &task, nil
}

// SubmitResult 提交任务结果
func (c *Client) SubmitResult(result *Result) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("序列化结果失败: %w", err)
	}

	url := fmt.Sprintf("%s/tasks/result", c.ServerURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(resultJSON))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// 添加鉴权头
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("提交结果失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("服务器返回错误状态: %d", resp.StatusCode)
	}

	return nil
}
