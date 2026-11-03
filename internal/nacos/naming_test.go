package nacos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRegisterDiscoverAndDeregister(t *testing.T) {
	var registered, deregistered atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("namespaceId"); got != "development" {
			t.Errorf("namespaceId = %q", got)
		}
		switch r.URL.Path {
		case "/nacos/v1/ns/instance":
			switch r.Method {
			case http.MethodPost:
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if r.Form.Get("serviceName") != "musichub" || r.Form.Get("ip") != "127.0.0.1" || r.Form.Get("port") != "9090" {
					t.Errorf("register form = %v", r.Form)
				}
				registered.Store(true)
				_, _ = w.Write([]byte("ok"))
			case http.MethodDelete:
				if r.URL.Query().Get("serviceName") != "musichub" || r.URL.Query().Get("ip") != "127.0.0.1" || r.URL.Query().Get("port") != "9090" {
					t.Errorf("instance query = %v", r.URL.Query())
				}
				deregistered.Store(true)
				_, _ = w.Write([]byte("ok"))
			default:
				t.Errorf("method = %q", r.Method)
			}
		case "/nacos/v1/ns/instance/list":
			_, _ = w.Write([]byte(`{"hosts":[{"ip":"127.0.0.1","port":9090,"healthy":true,"enabled":true}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NACOS_SERVER_ADDR", server.URL+"?namespace=development")
	t.Setenv("NACOS_SERVICE_IP", "127.0.0.1")
	if err := Init("musichub"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := Register(ctx, "", ":9090"); err != nil {
		t.Fatal(err)
	}
	if !registered.Load() {
		t.Fatal("instance was not registered")
	}
	address, err := Discover(context.Background(), "")
	if err != nil || address != "127.0.0.1:9090" {
		t.Fatalf("Discover = %q, %v", address, err)
	}
	if err := Deregister(context.Background(), "", ":9090"); err != nil {
		t.Fatal(err)
	}
	if !deregistered.Load() {
		t.Fatal("instance was not deregistered")
	}
}

func TestDiscoverRequiresHealthyInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hosts":[{"ip":"127.0.0.1","port":9090,"healthy":false,"enabled":true}]}`))
	}))
	defer server.Close()
	t.Setenv("NACOS_SERVER_ADDR", server.URL)
	if err := Init("room-service"); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(context.Background(), "musichub"); err == nil {
		t.Fatal("unhealthy instance accepted")
	}
}
