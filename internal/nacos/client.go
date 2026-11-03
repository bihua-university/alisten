// Package nacos implements the r-nacos APIs used by alisten.
package nacos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	pb "github.com/bihua-university/alisten/internal/proto/nacos"
)

const (
	defaultGroup     = "DEFAULT_GROUP"
	maxResponseBytes = 16 << 20
)

type client struct {
	baseURL     *url.URL
	namespace   string
	serviceName string
	username    string
	password    string
	httpClient  *http.Client

	mu           sync.Mutex
	accessToken  string
	tokenExpires time.Time
}

var shared *client

// Init initializes the process-wide client from NACOS_SERVER_ADDR.
func Init(serviceName string) error {
	if strings.TrimSpace(serviceName) == "" {
		return fmt.Errorf("nacos service name is required")
	}
	client, err := newClient(os.Getenv("NACOS_SERVER_ADDR"))
	if err != nil {
		return err
	}
	client.serviceName = serviceName
	shared = client
	return nil
}

func getClient() *client {
	if shared == nil {
		panic("nacos.Init must be called before using Nacos")
	}
	return shared
}

func newClient(serverURL string) (*client, error) {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		return nil, fmt.Errorf("NACOS_SERVER_ADDR is required")
	}
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse nacos server address: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, fmt.Errorf("nacos server address must be an HTTP(S) URL")
	}
	if u.Path == "" {
		u.Path = "/nacos"
	} else {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	namespace := u.Query().Get("namespace")
	username := u.User.Username()
	password, _ := u.User.Password()
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return &client{
		baseURL:    u,
		namespace:  namespace,
		username:   username,
		password:   password,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func GetConfig(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigReply, error) {
	getClient()
	if req == nil || strings.TrimSpace(req.DataId) == "" {
		return nil, fmt.Errorf("nacos data ID is required")
	}
	query := url.Values{"dataId": {req.DataId}, "group": {configGroup(req.Group)}}
	body, err := do(ctx, http.MethodGet, "/v1/cs/configs", query, nil)
	if err != nil {
		return nil, err
	}
	return &pb.ConfigReply{Content: string(body)}, nil
}

func PublishConfig(ctx context.Context, req *pb.PublishConfigRequest) (*pb.Empty, error) {
	getClient()
	if req == nil || strings.TrimSpace(req.DataId) == "" {
		return nil, fmt.Errorf("nacos data ID is required")
	}
	configType := req.Type
	if configType == "" {
		configType = "json"
	}
	form := url.Values{
		"dataId":  {req.DataId},
		"group":   {configGroup(req.Group)},
		"content": {req.Content},
		"type":    {configType},
	}
	body, err := do(ctx, http.MethodPost, "/v1/cs/configs", nil, form)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(body)) != "true" {
		return nil, fmt.Errorf("r-nacos rejected config publish: %s", responseSummary(body))
	}
	return &pb.Empty{}, nil
}

func configGroup(group string) string {
	if group == "" {
		return defaultGroup
	}
	return group
}

func do(ctx context.Context, method, path string, query, form url.Values) ([]byte, error) {
	client := getClient()
	token, err := requestToken(ctx, false)
	if err != nil {
		return nil, err
	}
	body, status, err := request(ctx, method, path, query, form, token)
	if err == nil && client.username != "" && (status == http.StatusUnauthorized || status == http.StatusForbidden) {
		token, err = requestToken(ctx, true)
		if err == nil {
			body, status, err = request(ctx, method, path, query, form, token)
		}
	}
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("r-nacos %s %s: HTTP %d: %s", method, path, status, responseSummary(body))
	}
	return body, nil
}

func request(ctx context.Context, method, path string, query, form url.Values, token string) ([]byte, int, error) {
	client := getClient()
	u := *client.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + path
	q := u.Query()
	for key, values := range query {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	if client.namespace != "" && client.namespace != "public" {
		namespaceKey := "tenant"
		if strings.HasPrefix(path, "/v1/ns/") {
			namespaceKey = "namespaceId"
		}
		q.Set(namespaceKey, client.namespace)
	}
	if token != "" {
		q.Set("accessToken", token)
	}
	u.RawQuery = q.Encode()

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, 0, err
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("r-nacos request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read r-nacos response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return nil, resp.StatusCode, fmt.Errorf("r-nacos response exceeds %d bytes", maxResponseBytes)
	}
	return data, resp.StatusCode, nil
}

func requestToken(ctx context.Context, force bool) (string, error) {
	client := getClient()
	if client.username == "" {
		return "", nil
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if !force && client.accessToken != "" && time.Now().Before(client.tokenExpires) {
		return client.accessToken, nil
	}
	form := url.Values{"username": {client.username}, "password": {client.password}}
	body, status, err := request(ctx, http.MethodPost, "/v1/auth/login", nil, form, "")
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("r-nacos login: HTTP %d: %s", status, responseSummary(body))
	}
	var reply struct {
		AccessToken string `json:"accessToken"`
		TokenTTL    int64  `json:"tokenTtl"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return "", fmt.Errorf("decode r-nacos login response: %w", err)
	}
	if reply.AccessToken == "" {
		return "", fmt.Errorf("r-nacos login response has no access token")
	}
	if reply.TokenTTL <= 0 {
		reply.TokenTTL = 3600
	}
	lifetime := time.Duration(reply.TokenTTL) * time.Second
	refreshAfter := lifetime - time.Minute
	if refreshAfter <= 0 {
		refreshAfter = lifetime / 2
	}
	client.accessToken = reply.AccessToken
	client.tokenExpires = time.Now().Add(refreshAfter)
	return client.accessToken, nil
}

func responseSummary(body []byte) string {
	const limit = 512
	summary := strings.TrimSpace(string(body))
	if len(summary) > limit {
		return summary[:limit] + "..."
	}
	if summary == "" {
		return "empty response"
	}
	return summary
}
