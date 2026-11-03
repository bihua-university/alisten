package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/bihua-university/alisten/internal/naconfig"
	"github.com/bihua-university/alisten/internal/nacos"
)

type fakeNacos struct {
	server *httptest.Server

	mu           sync.Mutex
	content      string
	published    string
	publishReply string
}

func newFakeNacos(t *testing.T, content string) *fakeNacos {
	t.Helper()
	f := &fakeNacos{content: content, publishReply: "true"}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v1/cs/configs" {
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("dataId") != dataID || r.URL.Query().Get("group") != "DEFAULT_GROUP" {
				t.Errorf("query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(f.content))
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm: %v", err)
			}
			f.published = r.Form.Get("content")
			if f.publishReply == "true" {
				f.content = f.published
			}
			_, _ = w.Write([]byte(f.publishReply))
		default:
			t.Errorf("method = %q", r.Method)
		}
	}))
	t.Cleanup(f.server.Close)
	t.Setenv("NACOS_SERVER_ADDR", f.server.URL)
	t.Setenv("MUSICHUB_CONFIG", "musichub.json")
	if err := nacos.Init("musichub"); err != nil {
		t.Fatal(err)
	}
	previous := MusicConfig
	previousDataID := dataID
	dataID = "musichub.json"
	MusicConfig = naconfig.Json[File](dataID)
	t.Cleanup(func() {
		MusicConfig = previous
		dataID = previousDataID
	})
	return f
}

func TestMusicConfig(t *testing.T) {
	newFakeNacos(t, `{"music":{"cookie":"MUSIC_U=abc"}}`)
	if got := MusicConfig.Get().Music.Cookie; got != "MUSIC_U=abc" {
		t.Errorf("unexpected cookie: %q", got)
	}
}

func TestSetCookiePublishesNacosConfig(t *testing.T) {
	f := newFakeNacos(t, `{"token":"token","music":{"cookie":"old","netease":"http://x"}}`)
	if err := SetCookie(context.Background(), "new-cookie"); err != nil {
		t.Fatal(err)
	}
	if got := MusicConfig.Get().Music.Cookie; got != "new-cookie" {
		t.Errorf("memory not updated: %q", got)
	}

	f.mu.Lock()
	published := f.published
	f.mu.Unlock()
	var document map[string]any
	if err := json.Unmarshal([]byte(published), &document); err != nil {
		t.Fatal(err)
	}
	music := document["music"].(map[string]any)
	if music["cookie"] != "new-cookie" || music["netease"] != "http://x" {
		t.Errorf("publish failed: %s", published)
	}
	if document["token"] != "token" {
		t.Errorf("other field lost: %s", published)
	}
}

func TestSetCookiePublishFailureKeepsRuntimeValue(t *testing.T) {
	f := newFakeNacos(t, `{"music":{"cookie":"old"}}`)
	f.publishReply = "false"
	if err := SetCookie(context.Background(), "new"); err == nil {
		t.Fatal("publish failure not returned")
	}
	if got := MusicConfig.Get().Music.Cookie; got != "new" {
		t.Fatalf("runtime Cookie = %q", got)
	}
}
