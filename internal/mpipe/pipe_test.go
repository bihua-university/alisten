package mpipe

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type fakeDownloader struct {
	mu    sync.Mutex
	calls int
	url   string
	err   error
}

func (d *fakeDownloader) GetDownloadURL(songID string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	return d.url, d.err
}

func (d *fakeDownloader) callsCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	p := NewWithClient(&fakeDownloader{}, 1, time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/music/file?source=netease&id=1", nil)
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestServeHTTP_BadRequest(t *testing.T) {
	p := NewWithClient(&fakeDownloader{}, 1, time.Hour)
	cases := []string{
		"/music/file?source=netease",
		"/music/file?id=123",
		"/music/file?source=kuwo&id=123",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path %s: expected 400, got %d", path, rec.Code)
		}
	}
}

func TestServeHTTP_DownloadAndCache(t *testing.T) {
	wantBody := []byte("hello world")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", "11")
		if _, err := w.Write(wantBody); err != nil {
			t.Logf("upstream write error: %v", err)
		}
	}))
	defer srv.Close()

	dl := &fakeDownloader{url: srv.URL + "/song"}
	p := NewWithClient(dl, 1, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/music/file?source=netease&id=1", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Fatalf("expected Content-Type audio/mpeg, got %s", ct)
	}
	if got := rec.Body.Bytes(); string(got) != string(wantBody) {
		t.Fatalf("expected body %q, got %q", wantBody, got)
	}
	if dl.callsCount() != 1 {
		t.Fatalf("expected 1 download URL call, got %d", dl.callsCount())
	}

	// 第二次请求应命中缓存，不再请求下载链接。
	req2 := httptest.NewRequest(http.MethodGet, "/music/file?source=netease&id=1", nil)
	rec2 := httptest.NewRecorder()
	p.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("second request expected 200, got %d", rec2.Code)
	}
	if got := rec2.Body.Bytes(); string(got) != string(wantBody) {
		t.Fatalf("second request expected body %q, got %q", wantBody, got)
	}
	if dl.callsCount() != 1 {
		t.Fatalf("expected still 1 download URL call after cache hit, got %d", dl.callsCount())
	}
}

func TestServeHTTP_ConcurrentRequests(t *testing.T) {
	wantBody := []byte("concurrent stream data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", "22")
		if _, err := w.Write(wantBody); err != nil {
			t.Logf("upstream write error: %v", err)
		}
	}))
	defer srv.Close()

	dl := &fakeDownloader{url: srv.URL + "/song"}
	p := NewWithClient(dl, 1, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/music/file?source=netease&id=1", nil)
			rec := httptest.NewRecorder()
			p.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
			if got := rec.Body.Bytes(); string(got) != string(wantBody) {
				t.Errorf("expected body %q, got %q", wantBody, got)
			}
		}()
	}
	wg.Wait()

	if dl.callsCount() != 1 {
		t.Fatalf("expected 1 download URL call for concurrent requests, got %d", dl.callsCount())
	}
}

func TestServeHTTP_DownloadURLError(t *testing.T) {
	dl := &fakeDownloader{err: errors.New("not found")}
	p := NewWithClient(dl, 1, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/music/file?source=netease&id=1", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestServeHTTP_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dl := &fakeDownloader{url: srv.URL + "/song"}
	p := NewWithClient(dl, 1, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/music/file?source=netease&id=1", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestServeHTTP_SlowUpstream(t *testing.T) {
	wantBody := []byte("slow stream data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", "16")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flush")
		}
		for i := 0; i < len(wantBody); i++ {
			if _, err := w.Write(wantBody[i : i+1]); err != nil {
				t.Logf("upstream write error: %v", err)
				return
			}
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	dl := &fakeDownloader{url: srv.URL + "/song"}
	p := NewWithClient(dl, 1, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/music/file?source=netease&id=1", nil)
			rec := httptest.NewRecorder()
			p.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
				return
			}
			if got := rec.Body.Bytes(); string(got) != string(wantBody) {
				t.Errorf("expected body %q, got %q", wantBody, got)
			}
		}()
	}
	wg.Wait()

	if dl.callsCount() != 1 {
		t.Fatalf("expected 1 download URL call for slow upstream, got %d", dl.callsCount())
	}
}

func TestServeHTTP_ReaderReturnsFullBody(t *testing.T) {
	wantBody := []byte("full body check")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		if _, err := w.Write(wantBody); err != nil {
			t.Logf("upstream write error: %v", err)
		}
	}))
	defer srv.Close()

	dl := &fakeDownloader{url: srv.URL + "/song"}
	p := NewWithClient(dl, 1, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/music/file?source=netease&id=1", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if string(body) != string(wantBody) {
		t.Fatalf("expected body %q, got %q", wantBody, body)
	}
}
