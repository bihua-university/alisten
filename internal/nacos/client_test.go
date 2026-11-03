package nacos

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	pb "github.com/bihua-university/alisten/internal/proto/nacos"
)

func TestUninitializedPanics(t *testing.T) {
	old := shared
	shared = nil
	t.Cleanup(func() { shared = old })
	defer func() {
		if recover() == nil {
			t.Fatal("uninitialized GetConfig did not panic")
		}
	}()
	_, _ = GetConfig(context.Background(), &pb.ConfigRequest{DataId: "alisten.json"})
}

func TestClientConfigAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v1/cs/configs" {
			t.Errorf("path = %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("tenant"); got != "development" {
			t.Errorf("tenant = %q", got)
		}
		switch r.Method {
		case http.MethodGet:
			if got := r.URL.Query().Get("dataId"); got != "alisten.json" {
				t.Errorf("dataId = %q", got)
			}
			if got := r.URL.Query().Get("group"); got != defaultGroup {
				t.Errorf("group = %q", got)
			}
			_, _ = w.Write([]byte(`{"addr":":8080"}`))
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm: %v", err)
			}
			if got := r.Form.Get("dataId"); got != "alisten.json" {
				t.Errorf("publish dataId = %q", got)
			}
			if got := r.Form.Get("group"); got != defaultGroup {
				t.Errorf("publish group = %q", got)
			}
			if got := r.Form.Get("type"); got != "json" {
				t.Errorf("publish type = %q", got)
			}
			if got := r.Form.Get("content"); got != `{"debug":true}` {
				t.Errorf("publish content = %q", got)
			}
			_, _ = w.Write([]byte("true"))
		default:
			t.Errorf("method = %q", r.Method)
		}
	}))
	defer server.Close()

	t.Setenv("NACOS_SERVER_ADDR", server.URL+"?namespace=development")
	if err := Init("alisten"); err != nil {
		t.Fatal(err)
	}
	reply, err := GetConfig(context.Background(), &pb.ConfigRequest{DataId: "alisten.json"})
	if err != nil || reply.Content != `{"addr":":8080"}` {
		t.Fatalf("GetConfig = %+v, %v", reply, err)
	}
	_, err = PublishConfig(context.Background(), &pb.PublishConfigRequest{DataId: "alisten.json", Content: `{"debug":true}`})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientAuthenticationAndRetry(t *testing.T) {
	var logins atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos/v1/auth/login":
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm: %v", err)
			}
			if r.Form.Get("username") != "user" || r.Form.Get("password") != "pass" {
				t.Errorf("login form = %v", r.Form)
			}
			token := logins.Add(1)
			fmt.Fprintf(w, `{"accessToken":"token-%d","tokenTtl":3600}`, token)
		case "/nacos/v1/cs/configs":
			token := r.URL.Query().Get("accessToken")
			if token == "token-1" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if token != "token-2" {
				t.Errorf("accessToken = %q", token)
			}
			_, _ = w.Write([]byte("config"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("NACOS_SERVER_ADDR", strings.Replace(server.URL, "://", "://user:pass@", 1))
	if err := Init("alisten"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		reply, err := GetConfig(context.Background(), &pb.ConfigRequest{DataId: "alisten.json"})
		if err != nil || reply.Content != "config" {
			t.Fatalf("GetConfig = %+v, %v", reply, err)
		}
	}
	if got := logins.Load(); got != 2 {
		t.Fatalf("login count = %d", got)
	}
}

func TestClientErrorsAndCancellation(t *testing.T) {
	t.Setenv("NACOS_SERVER_ADDR", "")
	if err := Init("alisten"); err == nil {
		t.Fatal("empty server address accepted")
	}
	t.Setenv("NACOS_SERVER_ADDR", "ftp://localhost")
	if err := Init("alisten"); err == nil {
		t.Fatal("invalid scheme accepted")
	}
	t.Setenv("NACOS_SERVER_ADDR", "localhost:8848")
	if err := Init("alisten"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetConfig(context.Background(), &pb.ConfigRequest{}); err == nil {
		t.Fatal("empty data ID accepted")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte("false"))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("NACOS_SERVER_ADDR", server.URL)
	if err := Init("alisten"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetConfig(context.Background(), &pb.ConfigRequest{DataId: "alisten.json"}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("GetConfig error = %v", err)
	}
	if _, err := PublishConfig(context.Background(), &pb.PublishConfigRequest{DataId: "alisten.json"}); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("PublishConfig error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GetConfig(ctx, &pb.ConfigRequest{DataId: "alisten.json"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}
