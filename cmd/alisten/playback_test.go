package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/music"
	"github.com/bihua-university/alisten/internal/syncx"
	"github.com/bihua-university/alisten/protocol"
	"github.com/tidwall/gjson"
)

// No ticker or external providers: each test controls exactly when playback advances.
func playbackHouse(t *testing.T, ids ...string) *House {
	t.Helper()
	h := newHouse("playback", "", "", true)
	h.backend.Load = func(source, id string, cached bool) (*music.Song, error) {
		switch id {
		case "error":
			return nil, errors.New("expired URL")
		case "nil":
			return nil, nil
		case "empty-url":
			return &music.Song{}, nil
		}
		return &music.Song{ID: id, Source: source, URL: "https://example.invalid/" + id, Duration: 60000}, nil
	}
	h.backend.Loudness = func(string, string) float64 { return 0.75 }
	h.Current = Order{id: "current", source: "qq"}
	current := protocol.PlayableSong{ID: "current", Source: protocol.Source("qq")}
	h.PushTime = time.Now().UnixMilli()
	h.End = time.Now().Add(time.Minute)
	h.playback = protocol.PlaybackData{Song: &current, StartedAtMS: h.PushTime, VolumeCoefficient: 0.75}
	for _, id := range ids {
		h.Playlist = append(h.Playlist, Order{id: id, source: "qq"})
	}
	t.Cleanup(h.closeHouse)
	return h
}

func playbackVote(h *House, name string) *httptest.ResponseRecorder {
	r := httptest.NewRecorder()
	data, _ := json.Marshal(map[string]any{"user": map[string]string{"name": name}})
	voteSkip(&Context{ctx: context.Background(), house: h, hw: r, data: gjson.ParseBytes(data)})
	return r
}

func assertIdlePlayback(t *testing.T, h *House) {
	t.Helper()
	if h.Current.id != "" || h.preparing.id != "" || h.playback.Song != nil ||
		h.playback.User != nil || h.playback.StartedAtMS != 0 || h.PushTime != 0 ||
		!h.End.IsZero() || len(h.VoteSkip) != 0 || h.queueLocked().Current != nil {
		t.Fatalf("stale playback after stop: current=%+v playback=%+v votes=%v", h.Current, h.playback, h.VoteSkip)
	}
}

func TestPlaybackEmptyQueue(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "natural-end", true: "vote"}[force], func(t *testing.T) {
			h := playbackHouse(t)
			if h.Skip(false) {
				t.Fatal("stopped before the song ended")
			}
			if force {
				if r := playbackVote(h, "alice"); r.Code != http.StatusOK {
					t.Fatalf("skip vote failed: %s", r.Body.String())
				}
			} else {
				h.End = time.Now().Add(-time.Second)
				if !h.Skip(false) {
					t.Fatal("finished song was not cleared")
				}
			}
			assertIdlePlayback(t, h)
			if h.Skip(false) || h.Skip(true) {
				t.Fatal("idle room reported another transition")
			}
			if r := playbackVote(h, "alice"); r.Code != http.StatusNotFound {
				t.Fatalf("idle vote should fail: %d %s", r.Code, r.Body.String())
			}
			assertIdlePlayback(t, h)
		})
	}
}

func TestPlaybackSkipsUnavailableSongs(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "natural-end", true: "vote"}[force], func(t *testing.T) {
			h := playbackHouse(t, "error", "nil", "empty-url", "next", "last")
			var loaded []string
			load := h.backend.Load
			h.backend.Load = func(source, id string, cached bool) (*music.Song, error) {
				loaded = append(loaded, id)
				return load(source, id, cached)
			}
			if force {
				if r := playbackVote(h, "alice"); r.Code != http.StatusOK {
					t.Fatalf("skip vote failed: %s", r.Body.String())
				}
			} else {
				h.End = time.Now().Add(-time.Second)
				h.Update()
			}
			if !slices.Equal(loaded, []string{"error", "nil", "empty-url", "next"}) ||
				h.Current.id != "next" || h.playback.Song.ID != "next" ||
				len(h.Playlist) != 1 || h.Playlist[0].id != "last" {
				t.Fatalf("skip failed to reach next playable song: loaded=%v current=%s queue=%v", loaded, h.Current.id, h.Playlist)
			}
			if h.End.UnixMilli() != h.playback.StartedAtMS+h.playback.Song.DurationMS {
				t.Fatal("song end must include the scheduled start delay")
			}
			if h.Skip(false) {
				t.Fatal("timer consumed another song immediately after the skip")
			}
		})
	}
}

func TestPlaybackUnavailableQueueStops(t *testing.T) {
	h := playbackHouse(t, "error", "nil", "empty-url")
	if !h.Skip(true) {
		t.Fatal("skip did not stop the old song")
	}
	assertIdlePlayback(t, h)
	if len(h.Playlist) != 0 {
		t.Fatal("unavailable songs remain queued")
	}
}

func TestPlaybackVotesDuringLoad(t *testing.T) {
	h := playbackHouse(t, "next", "last")
	entered, release := make(chan struct{}), make(chan struct{})
	load := h.backend.Load
	h.backend.Load = func(source, id string, cached bool) (*music.Song, error) {
		if id == "next" {
			close(entered)
			<-release
		}
		return load(source, id, cached)
	}
	done := make(chan bool, 1)
	go func() { done <- h.Skip(true) }()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("load did not start")
	}
	if r := playbackVote(h, "alice"); r.Code == http.StatusOK {
		t.Fatal("vote was accepted while switching tracks")
	}
	h.lock(func() {
		if len(h.VoteSkip) != 0 || len(h.Playlist) != 1 {
			t.Fatal("in-flight vote changed the next song's votes or queue")
		}
	})
	if h.Skip(false) || h.Skip(true) {
		t.Fatal("concurrent skip consumed another song")
	}
	close(release)
	select {
	case changed := <-done:
		if !changed {
			t.Fatal("reserved song did not start")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("load did not finish")
	}
	if r := playbackVote(h, "alice"); r.Code != http.StatusOK || h.Current.id != "last" {
		t.Fatalf("loading vote leaked to the next song: %s current=%s", r.Body.String(), h.Current.id)
	}
}

func TestPlaybackStopPublishesIdle(t *testing.T) {
	h := playbackHouse(t)
	conn := &Connection{done: make(chan struct{}), send: syncx.NewUnboundedChan[[]byte](8)}
	h.Connection = []*Connection{conn}
	h.VoteSkip = []auth.User{{Name: "alice"}}
	if !h.Skip(true) {
		t.Fatal("stop failed")
	}
	for _, event := range []string{protocol.EventPlaybackUpdated, protocol.EventQueueUpdated} {
		select {
		case raw := <-conn.send.Out():
			message := gjson.ParseBytes(raw)
			if message.Get("type").String() != event {
				t.Fatalf("unexpected event: %s", raw)
			}
			if message.Get("data.song").Type != gjson.Null || message.Get("data.current").Type != gjson.Null {
				t.Fatalf("stop published stale song: %s", raw)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("missing %s event", event)
		}
	}
}
