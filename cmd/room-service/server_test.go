package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bihua-university/alisten/cmd/room-service/internal/config"
	"github.com/bihua-university/alisten/internal/music"
	"github.com/bihua-university/alisten/internal/nacos"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
	"github.com/bihua-university/alisten/protocol"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/tools/txtar"
)

type testRecommender struct{}

func (testRecommender) AddHistory(string) {}
func (testRecommender) Mark(string)       {}
func (testRecommender) Recommend(context.Context, []string) []string {
	return []string{"recommended", "bad"}
}

func testHouse(t *testing.T, configure func(*House)) (*House, string) {
	t.Helper()
	h := newHouse("test", "description", "pw", true)
	h.recommander = testRecommender{}
	h.orderLimiter, h.likeLimiter = nil, nil
	h.backend = musicBackend{
		Load: func(ctx context.Context, req *pb.SongRequest) (*pb.Song, error) {
			if req.Source == "kuwo" || req.Id == "bad" || (req.Id == "expires" && !req.UseCache) {
				return nil, errors.New("provider unavailable")
			}
			return &pb.Song{
				Type: "music", Id: req.Id, Source: req.Source, Name: "同名歌曲", Artist: "artist",
				Url: "https://example.invalid/" + req.Id, Duration: 3_600_000, Lyric: "[00:00]lyric",
			}, nil
		},
		Search: func(ctx context.Context, req *pb.SearchRequest) (pb.MusicResult, error) {
			if req.Source == int32(music.KuWo) {
				return pb.MusicResult{}, errors.New("provider unavailable")
			}
			return pb.MusicResult{Total: 1, Data: []*pb.Music{{
				Id: "searched", Name: req.Keyword, Source: req.Source, Duration: 1000,
			}}}, nil
		},
		Playlists: func(ctx context.Context, req *pb.SearchRequest) (pb.PlaylistResult, error) {
			return pb.PlaylistResult{Total: 1, Data: []*pb.Playlist{{
				Id: "list", Name: req.Keyword, Desc: "description", CreatorUid: "creator",
			}}}, nil
		},
		SongList: func(ctx context.Context, req *pb.SearchRequest) (pb.MusicResult, error) {
			return pb.MusicResult{Total: 1, Data: []*pb.Music{{Id: req.Id + "-song", Source: req.Source}}}, nil
		},
		Loudness: func(context.Context, string, string) (float64, error) { return 0.75, nil },
	}
	if configure != nil {
		configure(h)
	}
	id := uuid.NewString()
	housesMu.Lock()
	houses[id] = h
	housesMu.Unlock()
	h.Start()
	t.Cleanup(func() {
		h.closeHouse()
		select {
		case <-h.stopped:
		case <-time.After(5 * time.Second):
			t.Error("room did not stop")
		}
	})
	return h, id
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	config.ServerConfig.Update(config.File{})
	server := httptest.NewServer(newHandler())
	t.Cleanup(server.Close)
	return server
}

func goldenFiles(t *testing.T, name string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", name, "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("golden directory has no cases")
	}
	return files
}

func goldenArchive(t *testing.T, path string) *txtar.Archive {
	t.Helper()
	archive, err := txtar.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.Files) == 0 {
		t.Fatal("golden fixture has no sections")
	}
	for _, section := range archive.Files {
		if section.Name == "response" {
			return archive
		}
	}
	t.Fatal("golden fixture has no expected response")
	return archive
}

func checkGolden(t *testing.T, want, got []byte) {
	t.Helper()
	var expected, actual any
	if err := json.Unmarshal(want, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatal(err)
	}
	normalizeTimes(actual)
	expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
	actualJSON, _ := json.MarshalIndent(actual, "", "  ")
	if string(expectedJSON) != string(actualJSON) {
		t.Errorf("golden mismatch\nwant:\n%s\ngot:\n%s", expectedJSON, actualJSON)
	}
}

// Retain zero and every other field; positive server timestamps become 1.
// A missing timestamp or an unexpected zero still fails the golden comparison.
func normalizeTimes(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "serverTimeMs", "sentAtMs", "startedAtMs", "pushTime":
				if number, ok := child.(float64); ok && number > 0 {
					value[key] = float64(1)
				}
			default:
				normalizeTimes(child)
			}
		}
	case []any:
		for _, child := range value {
			normalizeTimes(child)
		}
	}
}

type wsSession struct {
	conn    *websocket.Conn
	pending []protocol.Incoming
}

func (s *wsSession) receive(t *testing.T, want protocol.Incoming) []byte {
	t.Helper()
	match := func(message protocol.Incoming) bool {
		return message.Kind == want.Kind && message.Type == want.Type && message.RequestID == want.RequestID
	}
	for i, message := range s.pending {
		if match(message) {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			raw, _ := json.Marshal(message)
			return raw
		}
	}
	for {
		_ = s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var message protocol.Incoming
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatal(err)
		}
		if message.Kind == "response" && !match(message) {
			t.Fatalf("unexpected response type or requestId: %s", raw)
		}
		if match(message) {
			return raw
		}
		s.pending = append(s.pending, message)
	}
}

func TestWebsocketGolden(t *testing.T) {
	covered := make(map[string]bool)
	for _, path := range goldenFiles(t, "websocket") {
		archive := goldenArchive(t, path)
		for _, section := range archive.Files {
			if section.Name == "setup" || section.Name == "request" {
				var request protocol.Incoming
				if json.Unmarshal(section.Data, &request) == nil {
					covered[request.Type] = true
				}
			}
		}
		t.Run(strings.TrimSuffix(filepath.Base(path), ".txt"), func(t *testing.T) {
			_, id := testHouse(t, nil)
			server := testServer(t)
			url := "ws" + strings.TrimPrefix(server.URL, "http") + "/server?houseId=" + id + "&housePwd=pw"
			conn, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			session := wsSession{conn: conn}
			// Cases that start with an input skip the initial room snapshots.
			// The connection case starts with responses and checks them explicitly.
			if archive.Files[0].Name != "response" {
				for _, event := range []string{
					protocol.EventPlaybackUpdated, protocol.EventSettingsUpdated,
					protocol.EventQueueUpdated, protocol.EventMembersUpdated,
				} {
					session.receive(t, protocol.Incoming{Kind: "event", Type: event})
				}
			}
			for _, section := range archive.Files {
				switch section.Name {
				case "setup", "request":
					if err := conn.WriteMessage(websocket.TextMessage, section.Data); err != nil {
						t.Fatal(err)
					}
					if section.Name == "setup" {
						var request protocol.Incoming
						if err := json.Unmarshal(section.Data, &request); err != nil {
							t.Fatal(err)
						}
						request.Kind = "response"
						raw := session.receive(t, request)
						var response protocol.Incoming
						_ = json.Unmarshal(raw, &response)
						if response.Error != nil {
							t.Fatalf("setup failed: %s", raw)
						}
						session.pending = nil
					}
				case "response":
					var want protocol.Incoming
					if err := json.Unmarshal(section.Data, &want); err != nil {
						t.Fatal(err)
					}
					checkGolden(t, section.Data, session.receive(t, want))
				default:
					t.Fatalf("unknown golden section %q", section.Name)
				}
			}
		})
	}
	for name := range protocol.Requests {
		if !covered[name] {
			t.Errorf("add a golden case for protocol request %q", name)
		}
	}
}

func postGolden(t *testing.T, server *httptest.Server, id string, input []byte) (int, []byte) {
	t.Helper()
	header, payload, ok := strings.Cut(string(input), "\n")
	path, post := strings.CutPrefix(strings.TrimSpace(header), "POST ")
	if !ok || !post {
		t.Fatal("HTTP request must start with POST /path")
	}
	body := map[string]any{"houseId": id, "password": "pw"}
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Post(server.URL+path, "application/json", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err = io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, raw
}

func TestHTTPGolden(t *testing.T) {
	for _, path := range goldenFiles(t, "http") {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".txt"), func(t *testing.T) {
			archive := goldenArchive(t, path)
			_, id := testHouse(t, nil)
			server := testServer(t)
			var status int
			var response []byte
			for _, section := range archive.Files {
				switch section.Name {
				case "setup", "request":
					status, response = postGolden(t, server, id, section.Data)
					if section.Name == "setup" && status != http.StatusOK {
						t.Fatalf("setup returned %d: %s", status, response)
					}
				case "response":
					code, body, ok := strings.Cut(string(section.Data), "\n")
					if !ok || strconv.Itoa(status) != strings.TrimSpace(code) {
						t.Fatalf("status = %d, want %s: %s", status, code, response)
					}
					checkGolden(t, []byte(body), response)
				default:
					t.Fatalf("unknown golden section %q", section.Name)
				}
			}
		})
	}
}

func TestPlaybackReservationAndClose(t *testing.T) {
	entered, release := make(chan struct{}, 1), make(chan struct{})
	h, _ := testHouse(t, func(h *House) {
		load := h.backend.Load
		h.backend.Load = func(ctx context.Context, req *pb.SongRequest) (*pb.Song, error) {
			entered <- struct{}{}
			<-release
			return load(ctx, req)
		}
		current := Order{ID: "current", Source: "qq", Song: pb.Song{
			Id: "current", Source: "qq", Duration: int64(time.Hour / time.Millisecond),
		}}
		h.queue.Apply(current, 0, 1)
		h.queue.Add(Order{ID: "next", Source: "qq"}, 0)
		h.queue.Add(Order{ID: "last", Source: "qq"}, 0)
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	done := make(chan bool, 1)
	go func() { done <- h.Skip(true) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("playback load did not start")
	}
	if h.Skip(true) {
		t.Fatal("concurrent skip consumed a second song while loading")
	}
	h.closeHouse()
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("playback load did not finish")
	}
	_, current := h.queue.Playback()
	items := h.queue.Items()
	if current.ID != "current" || len(items) != 1 || items[0].ID != "last" {
		t.Fatalf("late load changed a closed room: current=%s queue=%+v", current.ID, items)
	}
}

func TestMusichubDiscoveryNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("namespaceId") == "canary" {
			_, _ = w.Write([]byte(`{"hosts":[{"ip":"127.0.0.1","port":9091,"healthy":true,"enabled":true}]}`))
		} else if r.URL.Query().Get("namespaceId") == "" {
			_, _ = w.Write([]byte(`{"hosts":[{"ip":"127.0.0.1","port":9090,"healthy":true,"enabled":true}]}`))
		} else {
			_, _ = w.Write([]byte(`{"hosts":[]}`))
		}
	}))
	defer server.Close()
	t.Setenv("NACOS_SERVER_ADDR", server.URL+"?namespace=canary")
	if err := nacos.Init("room-service"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ namespace, address string }{
		{"", "127.0.0.1:9091"}, {"public", "127.0.0.1:9090"}, {" canary ", "127.0.0.1:9091"},
	} {
		t.Setenv("MUSICHUB_NACOS_NAMESPACE", test.namespace)
		for _, discover := range []func(context.Context) (string, error){discoverMusichub, discoverMusichubOnce} {
			address, err := discover(context.Background())
			if err != nil || address != test.address {
				t.Fatalf("namespace %q: address=%q err=%v", test.namespace, address, err)
			}
		}
	}
	t.Setenv("MUSICHUB_NACOS_NAMESPACE", "missing")
	if _, err := discoverMusichubOnce(context.Background()); err == nil {
		t.Fatal("missing namespace fell back")
	}
}
