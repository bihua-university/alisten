// Package livehouse provides the internal client for livehouse-worker RPC.
package livehouse

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type Client struct {
	baseURL string
	http    *http.Client
}

type request struct {
	Type     string `json:"type"`
	RoomID   string `json:"roomId,omitempty"`
	Password string `json:"password,omitempty"`
	Data     any    `json:"data"`
}

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func New(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

func (c *Client) Call(ctx context.Context, operation, roomID, password string, data any, target any) error {
	body, err := json.Marshal(request{Type: operation, RoomID: roomID, Password: password, Data: data})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rpc", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(os.Getenv("INTERNAL_API_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return &Error{Status: http.StatusServiceUnavailable, Message: "livehouse-worker 暂不可用"}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var body struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&body)
		if body.Error == "" {
			body.Error = "livehouse-worker 请求失败"
		}
		return &Error{Status: response.StatusCode, Message: body.Error}
	}
	var reply struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		return &Error{Status: http.StatusBadGateway, Message: "livehouse-worker 响应格式错误"}
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(reply.Data, target); err != nil {
		return &Error{Status: http.StatusBadGateway, Message: "livehouse-worker 数据格式错误"}
	}
	return nil
}
