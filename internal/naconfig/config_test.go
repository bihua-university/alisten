package naconfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bihua-university/alisten/internal/nacos"
)

type testConfig struct {
	Value string `json:"value"`
}

type configServer struct {
	server   *httptest.Server
	content  atomic.Value
	requests atomic.Int32
}

func newConfigServer(t *testing.T, content string) *configServer {
	t.Helper()
	s := &configServer{}
	s.content.Store(content)
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v1/cs/configs" || r.URL.Query().Get("dataId") != "test.json" {
			http.NotFound(w, r)
			return
		}
		s.requests.Add(1)
		_, _ = w.Write([]byte(s.content.Load().(string)))
	}))
	t.Cleanup(s.server.Close)
	t.Setenv("NACOS_SERVER_ADDR", s.server.URL)
	if err := nacos.Init("test"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestJSONLazyLoadAndRefresh(t *testing.T) {
	s := newConfigServer(t, `{"value":"old"}`)
	config := Json[testConfig]("test.json")
	if got := s.requests.Load(); got != 0 {
		t.Fatalf("declaration made %d requests", got)
	}
	if got := config.Get().Value; got != "old" {
		t.Fatalf("initial value = %q", got)
	}

	s.content.Store(`{"value":"new"}`)
	if err := config.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := config.Get().Value; got != "new" {
		t.Fatalf("refreshed value = %q", got)
	}

	s.content.Store(`{invalid`)
	if err := config.Refresh(context.Background()); err == nil {
		t.Fatal("invalid JSON refresh succeeded")
	}
	if got := config.Get().Value; got != "new" {
		t.Fatalf("failed refresh replaced snapshot with %q", got)
	}
}

func TestJSONRefreshAfterExpiry(t *testing.T) {
	oldTTL := refreshTTL
	refreshTTL = 20 * time.Millisecond
	t.Cleanup(func() { refreshTTL = oldTTL })

	s := newConfigServer(t, `{"value":"old"}`)
	config := Json[testConfig]("test.json")
	if got := config.Get().Value; got != "old" {
		t.Fatal(got)
	}
	s.content.Store(`{"value":"new"}`)
	if got := config.Get().Value; got != "old" {
		t.Fatalf("unexpired value = %q", got)
	}
	time.Sleep(30 * time.Millisecond)
	if got := s.requests.Load(); got != 1 {
		t.Fatalf("expiry triggered a background request: %d", got)
	}
	if got := config.Get().Value; got != "new" {
		t.Fatalf("lazy refreshed value = %q", got)
	}

	s.content.Store(`{invalid`)
	time.Sleep(30 * time.Millisecond)
	if got := config.Get().Value; got != "new" {
		t.Fatalf("failed lazy refresh replaced snapshot with %q", got)
	}
}

func TestJSONConcurrentInitialLoad(t *testing.T) {
	s := newConfigServer(t, `{"value":"loaded"}`)
	config := Json[testConfig]("test.json")
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := config.Get().Value; got != "loaded" {
				t.Errorf("value = %q", got)
			}
		}()
	}
	wg.Wait()
	if got := s.requests.Load(); got != 1 {
		t.Fatalf("initial load made %d requests", got)
	}
}

func TestConfigConcurrentReadsAndUpdates(t *testing.T) {
	config := Json[testConfig]("unused.json")
	config.Update(testConfig{Value: "initial"})
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 1000 {
				_ = config.Get()
			}
		}()
		go func() {
			defer wg.Done()
			for range 1000 {
				config.Update(testConfig{Value: "updated"})
			}
		}()
	}
	wg.Wait()
}
