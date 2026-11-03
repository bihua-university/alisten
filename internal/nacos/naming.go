package nacos

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bihua-university/alisten/internal/logx"
)

const heartbeatInterval = 5 * time.Second

var discoverySequence atomic.Uint64

// Register registers an instance and keeps its ephemeral registration alive until ctx is canceled.
func Register(ctx context.Context, serviceName, address string) error {
	client := getClient()
	serviceName = namingService(client, serviceName)
	ip, port, err := instanceAddress(client, address)
	if err != nil {
		return err
	}
	form := instanceQuery(serviceName, ip, port)
	if _, err := do(ctx, http.MethodPost, "/v1/ns/instance", nil, form); err != nil {
		return fmt.Errorf("register Nacos service %s: %w", serviceName, err)
	}
	go heartbeat(ctx, serviceName, ip, port)
	return nil
}

// Deregister removes an instance from Nacos.
func Deregister(ctx context.Context, serviceName, address string) error {
	client := getClient()
	serviceName = namingService(client, serviceName)
	ip, port, err := instanceAddress(client, address)
	if err != nil {
		return err
	}
	_, err = do(ctx, http.MethodDelete, "/v1/ns/instance", instanceQuery(serviceName, ip, port), nil)
	return err
}

// Discover returns a healthy instance address for serviceName.
func Discover(ctx context.Context, serviceName string) (string, error) {
	return DiscoverInNamespace(ctx, serviceName, getClient().namespace)
}

// DiscoverInNamespace discovers a service without changing the client's registration
// or configuration namespace. An empty namespace or "public" selects the public namespace.
func DiscoverInNamespace(ctx context.Context, serviceName, namespace string) (string, error) {
	client := getClient()
	serviceName = namingService(client, serviceName)
	query := url.Values{
		"serviceName": {serviceName},
		"groupName":   {defaultGroup},
		"healthyOnly": {"true"},
		"namespaceId": {namespace},
	}
	body, err := do(ctx, http.MethodGet, "/v1/ns/instance/list", query, nil)
	if err != nil {
		return "", fmt.Errorf("discover Nacos service %s in namespace %q: %w", serviceName, namespace, err)
	}
	var reply struct {
		Hosts []struct {
			IP      string `json:"ip"`
			Port    int    `json:"port"`
			Healthy bool   `json:"healthy"`
			Enabled bool   `json:"enabled"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return "", fmt.Errorf("decode Nacos service %s instances: %w", serviceName, err)
	}
	var healthy []int
	for i, instance := range reply.Hosts {
		if instance.Healthy && instance.Enabled && instance.IP != "" && instance.Port > 0 {
			healthy = append(healthy, i)
		}
	}
	if len(healthy) == 0 {
		return "", fmt.Errorf("Nacos service %s in namespace %q has no healthy instances", serviceName, namespace)
	}
	index := healthy[(discoverySequence.Add(1)-1)%uint64(len(healthy))]
	instance := reply.Hosts[index]
	return net.JoinHostPort(instance.IP, strconv.Itoa(instance.Port)), nil
}

func namingService(client *client, serviceName string) string {
	if serviceName == "" {
		return client.serviceName
	}
	return serviceName
}

func instanceQuery(serviceName, ip string, port int) url.Values {
	return url.Values{
		"serviceName": {serviceName},
		"groupName":   {defaultGroup},
		"ip":          {ip},
		"port":        {strconv.Itoa(port)},
		"clusterName": {"DEFAULT"},
		"ephemeral":   {"true"},
	}
}

func heartbeat(ctx context.Context, serviceName, ip string, port int) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			beat, _ := json.Marshal(map[string]any{
				"serviceName": serviceName,
				"ip":          ip,
				"port":        port,
				"cluster":     "DEFAULT",
			})
			query := instanceQuery(serviceName, ip, port)
			query.Set("beat", string(beat))
			if _, err := do(ctx, http.MethodPut, "/v1/ns/instance/beat", query, nil); err != nil && ctx.Err() == nil {
				logx.Errorf("Nacos %s heartbeat: %v", serviceName, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func instanceAddress(client *client, address string) (string, int, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("invalid service address %q: %w", address, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid service port %q", portText)
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = strings.TrimSpace(os.Getenv("NACOS_SERVICE_IP"))
		if host == "" {
			host, err = outboundIP(client)
			if err != nil {
				return "", 0, err
			}
		}
	}
	return host, port, nil
}

func outboundIP(client *client) (string, error) {
	port := client.baseURL.Port()
	if port == "" {
		if client.baseURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	conn, err := net.DialTimeout("udp", net.JoinHostPort(client.baseURL.Hostname(), port), 2*time.Second)
	if err != nil {
		return "", fmt.Errorf("determine service IP; set NACOS_SERVICE_IP: %w", err)
	}
	defer conn.Close()
	address, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || address.IP == nil {
		return "", fmt.Errorf("determine service IP; set NACOS_SERVICE_IP")
	}
	return address.IP.String(), nil
}
