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

	"github.com/bihua-university/alisten/cmd/room-service/internal/musicqueue"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
	"github.com/bihua-university/alisten/protocol"
	"github.com/tidwall/gjson"
)

func playbackHouse(t *testing.T, ids ...string) *House {
	t.Helper()
	h := newHouse("playback", "", "", true)
	h.backend.Load = func(ctx context.Context, req *pb.SongRequest) (*pb.Song, error) {
		switch req.Id {
		case "error":
			return nil, errors.New("expired URL")
		case "nil":
			return nil, nil
		case "empty-url":
			return &pb.Song{}, nil
		}
		return &pb.Song{Id: req.Id, Source: req.Source, Url: "https://example.invalid/" + req.Id, Duration: 60000}, nil
	}
	h.backend.Loudness = func(context.Context, string, string) (float64, error) { return 0.75, nil }
	current := Order{ID: "current", Source: "qq", Song: pb.Song{
		Id: "current", Source: "qq", Url: "https://example.invalid/current", Duration: 60000,
	}}
	if !h.queue.Apply(current, 0, 0.75) {
		t.Fatal("failed to initialize playback")
	}
	for _, id := range ids {
		if result := h.queue.Add(Order{ID: id, Source: "qq"}, 0); result != musicqueue.Added {
			t.Fatalf("failed to queue %s: %v", id, result)
		}
	}
	t.Cleanup(h.closeHouse)
	return h
}

func expirePlayback(t *testing.T, h *House) {
	t.Helper()
	playback, current := h.queue.Playback()
	current.Song.Duration = -1000
	if !h.queue.Apply(current, 0, playback.VolumeCoefficient) {
		t.Fatal("failed to expire playback")
	}
}

func playbackVote(h *House, name string) *httptest.ResponseRecorder {
	r := httptest.NewRecorder()
	data, _ := json.Marshal(map[string]any{"user": map[string]string{"name": name}})
	voteSkip(&Context{ctx: context.Background(), house: h, hw: r, data: gjson.ParseBytes(data)})
	return r
}

func assertIdlePlayback(t *testing.T, h *House) {
	t.Helper()
	playback, current := h.queue.Playback()
	if current.ID != "" || playback.Song != nil || playback.User != nil || playback.StartedAtMS != 0 ||
		len(h.queue.Items()) != 0 || h.queue.Snapshot(0).Current != nil {
		t.Fatalf("stale playback after stop: current=%+v playback=%+v", current, playback)
	}
	if h.Skip(false) || h.Skip(true) {
		t.Fatal("idle room reported another transition")
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
				expirePlayback(t, h)
				if !h.Skip(false) {
					t.Fatal("finished song was not cleared")
				}
			}
			assertIdlePlayback(t, h)
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
			h.backend.Load = func(ctx context.Context, req *pb.SongRequest) (*pb.Song, error) {
				loaded = append(loaded, req.Id)
				return load(ctx, req)
			}
			if force {
				if r := playbackVote(h, "alice"); r.Code != http.StatusOK {
					t.Fatalf("skip vote failed: %s", r.Body.String())
				}
			} else {
				expirePlayback(t, h)
				h.Update()
				if !h.Skip(false) {
					t.Fatal("finished song did not advance")
				}
			}
			playback, current := h.queue.Playback()
			items := h.queue.Items()
			if !slices.Equal(loaded, []string{"error", "nil", "empty-url", "next"}) ||
				current.ID != "next" || playback.Song.ID != "next" ||
				len(items) != 1 || items[0].ID != "last" {
				t.Fatalf("skip failed to reach next playable song: loaded=%v current=%s queue=%v", loaded, current.ID, items)
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
}

func TestPlaybackVotesDuringLoad(t *testing.T) {
	h := playbackHouse(t, "next", "last")
	entered, release := make(chan struct{}), make(chan struct{})
	load := h.backend.Load
	h.backend.Load = func(ctx context.Context, req *pb.SongRequest) (*pb.Song, error) {
		if req.Id == "next" {
			close(entered)
			<-release
		}
		return load(ctx, req)
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
	if len(h.queue.Items()) != 1 {
		t.Fatal("in-flight vote changed the queue")
	}
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
	_, current := h.queue.Playback()
	if r := playbackVote(h, "alice"); r.Code != http.StatusOK || current.ID != "next" {
		t.Fatalf("loading vote leaked to the next song: %s current=%s", r.Body.String(), current.ID)
	}
	_, current = h.queue.Playback()
	if current.ID != "last" {
		t.Fatalf("vote did not advance to last song: %s", current.ID)
	}
}

func TestPlaybackStopPublishesIdle(t *testing.T) {
	h := playbackHouse(t)
	conn := &Connection{done: make(chan struct{}), send: make(chan []byte, 8)}
	h.Connection = []*Connection{conn}
	if !h.Skip(true) {
		t.Fatal("stop failed")
	}
	select {
	case raw := <-conn.send:
		message := gjson.ParseBytes(raw)
		if message.Get("type").String() != protocol.EventPlaybackUpdated {
			t.Fatalf("unexpected event: %s", raw)
		}
		if message.Get("data.song").Type != gjson.Null {
			t.Fatalf("stop published stale song: %s", raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("missing playback.updated event")
	}
	// 停止不再重发整份队列，等待队列的清理由前端本地完成。
	select {
	case raw := <-conn.send:
		t.Fatalf("stop re-sent a snapshot: %s", raw)
	default:
	}
}

func TestPlaybackTransitionPublishesDelta(t *testing.T) {
	h := playbackHouse(t, "next", "last")
	conn := &Connection{done: make(chan struct{}), send: make(chan []byte, 8)}
	h.Connection = []*Connection{conn}
	if !h.Skip(true) {
		t.Fatal("skip failed")
	}
	// 切歌只下发被取走的歌曲与播放快照，等待队列不再整份同步。
	select {
	case raw := <-conn.send:
		message := gjson.ParseBytes(raw)
		if message.Get("type").String() != protocol.EventQueueRemoved ||
			message.Get("data.id").String() != "next" ||
			message.Get("data.source").String() != "qq" {
			t.Fatalf("unexpected removal event: %s", raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("missing queue.removed event")
	}
	select {
	case raw := <-conn.send:
		message := gjson.ParseBytes(raw)
		if message.Get("type").String() != protocol.EventPlaybackUpdated ||
			message.Get("data.song.id").String() != "next" {
			t.Fatalf("unexpected playback event: %s", raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("missing playback.updated event")
	}
	select {
	case raw := <-conn.send:
		t.Fatalf("transition re-sent a queue snapshot: %s", raw)
	default:
	}
}

func TestPlaybackVoteReset(t *testing.T) {
	h := playbackHouse(t, "next", "last")
	if r := playbackVote(h, "alice"); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	_, current := h.queue.Playback()
	if current.ID != "next" {
		t.Fatalf("unexpected current song: %s", current.ID)
	}
	if r := playbackVote(h, "alice"); r.Code != http.StatusOK {
		t.Fatalf("vote was not reset for the next song: %s", r.Body.String())
	}
}
