package musichub

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/RomiChan/protobuf/rpc"
	"github.com/RomiChan/protobuf/rpc/rpctest"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
)

// rpctest serves the fake peer over in-memory connections, so tests never bind
// a port. The client under test is the production wrapper around rpc.Client.
func musichubTestClient(t *testing.T, handler rpc.Handler) *Client {
	t.Helper()
	return &Client{rpc: rpctest.NewServer(t, handler).Client(rpc.ClientOptions{CallTimeout: time.Second})}
}

// The fake peer accepts generated protobuf requests directly. This catches
// accidental envelopes, wrong wire names and lossy domain conversions.
func TestMusichubClientContract(t *testing.T) {
	ctx := context.Background()
	client := musichubTestClient(t, rpc.HandlerFunc(func(ctx context.Context, req *rpc.Request) (any, error) {
		switch req.Method {
		case "musichub.v1.Load":
			p, err := req.UnmarshalParams[pb.SongRequest]()
			if err != nil {
				return nil, err
			}
			if p.Source != "netease" || p.Id != "song" || !p.UseCache {
				return nil, errors.New("wrong song request")
			}
			return &pb.Song{Id: p.Id, Source: p.Source, Type: "music", Url: "audio", WebUrl: "web", PictureUrl: "cover", Duration: 1234, Lyric: "歌词", Artist: "artist", Name: "name", Album: "album"}, nil
		case "musichub.v1.Search", "musichub.v1.SongList":
			p, err := req.UnmarshalParams[pb.SearchRequest]()
			if err != nil {
				return nil, err
			}
			if p.Id != "list" || p.Keyword != "query" || p.Source != -1 || p.Page != 2 || p.PageSize != 30 {
				return nil, errors.New("wrong search request")
			}
			return &pb.MusicResult{Total: 100, Data: []*pb.Music{{Id: "song", Name: "name", Artist: "artist", Album: "album", Duration: 1234, Cover: "cover", Source: -1}}}, nil
		case "musichub.v1.Playlists":
			return &pb.PlaylistResult{Total: 50, Data: []*pb.Playlist{{Id: "list", Name: "list name", PictureUrl: "picture", Desc: "desc", Creator: "creator", CreatorUid: "uid", PlayCount: 1000, SongCount: 20}}}, nil
		case "musichub.v1.Loudness":
			return &pb.CoefficientReply{Coefficient: 0.75}, nil
		case "musichub.v1.Similar":
			return &pb.IDsReply{Ids: []string{"one", "two"}}, nil
		case "musichub.v1.CookieStatus":
			return &pb.BoolReply{Value: true}, nil
		case "musichub.v1.SetCookie":
			p, err := req.UnmarshalParams[pb.CookieRequest]()
			if err != nil {
				return nil, err
			}
			if p.Cookie != "cookie" {
				return nil, errors.New("wrong cookie")
			}
			return &pb.BoolReply{Value: true}, nil
		}
		return nil, &rpc.Error{Code: rpc.MethodNotFound, Message: "unknown method"}
	}))
	song, err := client.Load(ctx, &pb.SongRequest{Source: "netease", Id: "song", UseCache: true})
	wantSong := &pb.Song{Id: "song", Source: "netease", Type: "music", Url: "audio", WebUrl: "web", PictureUrl: "cover", Duration: 1234, Lyric: "歌词", Artist: "artist", Name: "name", Album: "album"}
	if err != nil || !reflect.DeepEqual(song, wantSong) {
		t.Fatalf("Load = %+v, %v", song, err)
	}
	request := &pb.SearchRequest{Id: "list", Source: -1, Keyword: "query", Page: 2, PageSize: 30}
	wantResult := pb.MusicResult{Total: 100, Data: []*pb.Music{{Id: "song", Name: "name", Artist: "artist", Album: "album", Duration: 1234, Cover: "cover", Source: -1}}}
	for _, search := range []func(context.Context, *pb.SearchRequest) (pb.MusicResult, error){client.Search, client.SongList} {
		r, err := search(ctx, request)
		if err != nil || !reflect.DeepEqual(r, wantResult) {
			t.Fatalf("search = %+v, %v", r, err)
		}
	}
	playlists, err := client.Playlists(ctx, request)
	wantPlaylists := pb.PlaylistResult{Total: 50, Data: []*pb.Playlist{{Id: "list", Name: "list name", PictureUrl: "picture", Desc: "desc", Creator: "creator", CreatorUid: "uid", PlayCount: 1000, SongCount: 20}}}
	if err != nil || !reflect.DeepEqual(playlists, wantPlaylists) {
		t.Fatalf("Playlists = %+v, %v", playlists, err)
	}
	if v, err := client.Loudness(ctx, "netease", "song"); err != nil || v != 0.75 {
		t.Fatalf("Loudness = %v, %v", v, err)
	}
	if v, err := client.Similar(ctx, "song"); err != nil || !reflect.DeepEqual(v, []string{"one", "two"}) {
		t.Fatalf("Similar = %v, %v", v, err)
	}
	if v, err := client.CookieStatus(ctx); err != nil || !v {
		t.Fatalf("CookieStatus = %v, %v", v, err)
	}
	if v, err := client.SetCookie(ctx, "cookie"); err != nil || !v {
		t.Fatalf("SetCookie = %v, %v", v, err)
	}
}

func TestMusichubClientConcurrentCallsAndCancellation(t *testing.T) {
	gate := make(chan struct{})
	defer close(gate)
	client := musichubTestClient(t, rpc.HandlerOf(func(ctx context.Context, p *pb.SongRequest) (*pb.Song, error) {
		if p.Id == "slow" {
			<-gate
		}
		if p.Id == "bad" {
			return nil, &rpc.Error{Code: -32000, Message: "provider unavailable"}
		}
		return &pb.Song{Id: p.Id, Source: p.Source}, nil
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.Load(ctx, &pb.SongRequest{Source: "qq", Id: "slow", UseCache: true}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := client.Load(context.Background(), &pb.SongRequest{Source: "qq", Id: "fast", UseCache: true})
			if err != nil || s.Id != "fast" {
				t.Errorf("concurrent Load = %+v, %v", s, err)
			}
		}()
	}
	wg.Wait()
	_, err := client.Load(context.Background(), &pb.SongRequest{Source: "qq", Id: "bad", UseCache: true})
	var remote *rpc.Error
	if !errors.As(err, &remote) || remote.Code != -32000 {
		t.Fatalf("remote error: %v", err)
	}
}

func TestMusichubClientClose(t *testing.T) {
	client := musichubTestClient(t, rpc.HandlerOf(func(ctx context.Context, p *pb.SongRequest) (*pb.Song, error) {
		return &pb.Song{Id: p.Id}, nil
	}))
	if _, err := client.Load(context.Background(), &pb.SongRequest{Source: "qq", Id: "song", UseCache: true}); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Load(context.Background(), &pb.SongRequest{Source: "qq", Id: "song", UseCache: true}); !errors.Is(err, rpc.ErrClientClosed) {
		t.Fatalf("call after Close = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close = %v", err)
	}
}

func discoveryTestClient(t *testing.T, handlers map[string]rpc.Handler) *Client {
	t.Helper()
	client := NewDiscoveryClient("old", nil, time.Second)
	client.dial = func(address string, timeout time.Duration) *rpc.Client {
		return rpctest.NewServer(t, handlers[address]).Client(rpc.ClientOptions{CallTimeout: timeout})
	}
	return client
}

func discoverySong(id string) rpc.Handler {
	return rpc.HandlerOf(func(ctx context.Context, p *pb.SongRequest) (*pb.Song, error) {
		return &pb.Song{Id: id}, nil
	})
}

func TestDiscoveryClientRefreshesAfterTransportError(t *testing.T) {
	client := discoveryTestClient(t, map[string]rpc.Handler{
		"old": discoverySong("old"),
		"new": discoverySong("new"),
	})
	client.discover = func(context.Context) (string, error) { return "new", nil }
	client.afterCall(errors.New("connection reset"))
	song, err := client.Load(context.Background(), &pb.SongRequest{})
	if err != nil || song.Id != "new" {
		t.Fatalf("Load after reconnect = %+v, %v", song, err)
	}
}
