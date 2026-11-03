package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RomiChan/protobuf/proto"
	"github.com/RomiChan/protobuf/rpc"
	"github.com/RomiChan/protobuf/rpc/rpctest"
	"github.com/bihua-university/alisten/internal/music"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
)

// stubBackend satisfies RPCBackend; tests override only the methods they use.
type stubBackend struct{}

func (*stubBackend) Load(context.Context, *pb.SongRequest) (*pb.Song, error) {
	return nil, errors.New("not implemented")
}

func (*stubBackend) Search(context.Context, *pb.SearchRequest) (*pb.MusicResult, error) {
	return nil, errors.New("not implemented")
}

func (*stubBackend) Playlists(context.Context, *pb.SearchRequest) (*pb.PlaylistResult, error) {
	return nil, errors.New("not implemented")
}

func (*stubBackend) SongList(context.Context, *pb.SearchRequest) (*pb.MusicResult, error) {
	return nil, errors.New("not implemented")
}

func (*stubBackend) Loudness(context.Context, *pb.SongRequest) (*pb.CoefficientReply, error) {
	return nil, errors.New("not implemented")
}

func (*stubBackend) Similar(context.Context, *pb.SongRequest) (*pb.IDsReply, error) {
	return nil, errors.New("not implemented")
}

func (*stubBackend) CookieStatus(context.Context, *pb.Empty) (*pb.BoolReply, error) {
	return nil, errors.New("not implemented")
}

func (*stubBackend) SetCookie(context.Context, *pb.CookieRequest) (*pb.BoolReply, error) {
	return nil, errors.New("not implemented")
}

type concurrentBackend struct {
	stubBackend
	gate    chan struct{}
	started chan struct{}
}

func (b *concurrentBackend) Load(ctx context.Context, p *pb.SongRequest) (*pb.Song, error) {
	b.started <- struct{}{}
	select {
	case <-b.gate:
		return &pb.Song{Id: p.Id, Source: p.Source}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *concurrentBackend) CookieStatus(context.Context, *pb.Empty) (*pb.BoolReply, error) {
	return &pb.BoolReply{Value: true}, nil
}

func TestRPCHandlerConcurrentRequests(t *testing.T) {
	for _, poolSize := range []int{1, 8} {
		t.Run(fmt.Sprintf("pool=%d", poolSize), func(t *testing.T) {
			const calls = 32
			gate := make(chan struct{})
			var release sync.Once
			defer release.Do(func() { close(gate) })
			started := make(chan struct{}, calls)
			service := &concurrentBackend{gate: gate, started: started}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client := rpctest.NewServer(t, NewServer(service)).Client(rpc.ClientOptions{
				PoolSize:    poolSize,
				CallTimeout: 5 * time.Second,
			})
			results := make(chan error, calls)
			for i := 0; i < calls; i++ {
				go func() {
					id := fmt.Sprint(i)
					song, err := client.Call[pb.Song](ctx, "musichub.v1.Load", &pb.SongRequest{Source: "qq", Id: id})
					if err == nil && (song.Id != id || song.Source != "qq") {
						err = fmt.Errorf("response does not match request %s: %+v", id, song)
					}
					results <- err
				}()
			}
			for i := 0; i < calls; i++ {
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatalf("only %d/%d backend calls ran concurrently", i, calls)
				}
			}
			// A quick method must complete while all Load handlers are still
			// blocked, including with a single connection.
			status, err := client.Call[pb.BoolReply](ctx, "musichub.v1.CookieStatus", &pb.Empty{})
			if err != nil || !status.Value {
				t.Fatalf("quick call blocked by slow handlers: %+v, %v", status, err)
			}
			release.Do(func() { close(gate) })
			for i := 0; i < calls; i++ {
				if err := <-results; err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

type scriptedBackend struct{}

func (*scriptedBackend) Load(_ context.Context, p *pb.SongRequest) (*pb.Song, error) {
	if p.Source != "qq" || p.Id != "song" || !p.UseCache {
		return nil, errors.New("invalid Load arguments")
	}
	return &pb.Song{Id: p.Id, Source: p.Source, Type: "music", Url: "url", WebUrl: "web", PictureUrl: "picture",
		Duration: 123, Lyric: "lyric", Artist: "artist", Name: "name", Album: "album"}, nil
}

func (*scriptedBackend) Search(_ context.Context, p *pb.SearchRequest) (*pb.MusicResult, error) {
	if p.Keyword != "query" || p.Source != int32(music.NetEase) || p.Page != 2 || p.PageSize != 30 {
		return nil, errors.New("invalid Search arguments")
	}
	return &pb.MusicResult{Total: 5, Data: []*pb.Music{{Id: "found", Source: int32(music.NetEase), Name: "name",
		Artist: "artist", Album: "album", Duration: 123, Cover: "cover"}}}, nil
}

func (*scriptedBackend) Playlists(_ context.Context, p *pb.SearchRequest) (*pb.PlaylistResult, error) {
	return &pb.PlaylistResult{Total: 8, Data: []*pb.Playlist{{Id: p.Id, Name: "list", PictureUrl: "cover",
		Desc: "desc", Creator: "creator", CreatorUid: "uid", PlayCount: 12, SongCount: 5}}}, nil
}

func (*scriptedBackend) SongList(_ context.Context, p *pb.SearchRequest) (*pb.MusicResult, error) {
	return &pb.MusicResult{Total: 1, Data: []*pb.Music{{Id: p.Id}}}, nil
}

func (*scriptedBackend) Loudness(context.Context, *pb.SongRequest) (*pb.CoefficientReply, error) {
	return &pb.CoefficientReply{Coefficient: 0.5}, nil
}

func (*scriptedBackend) Similar(context.Context, *pb.SongRequest) (*pb.IDsReply, error) {
	return &pb.IDsReply{Ids: []string{"similar"}}, nil
}

func (*scriptedBackend) CookieStatus(context.Context, *pb.Empty) (*pb.BoolReply, error) {
	return &pb.BoolReply{Value: true}, nil
}

func (*scriptedBackend) SetCookie(_ context.Context, p *pb.CookieRequest) (*pb.BoolReply, error) {
	return &pb.BoolReply{Value: p.Cookie == "new-cookie"}, nil
}

func TestRPCHandlerGeneratedMessages(t *testing.T) {
	service := &scriptedBackend{}
	client := rpctest.NewServer(t, NewServer(service)).Client(rpc.ClientOptions{CallTimeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	song, err := client.Call[pb.Song](ctx, "musichub.v1.Load", &pb.SongRequest{Source: "qq", Id: "song", UseCache: true})
	if err != nil || song.Id != "song" || song.Url != "url" || song.PictureUrl != "picture" || song.Lyric != "lyric" || song.Duration != 123 {
		t.Fatalf("Load = %+v, %v", song, err)
	}
	args := &pb.SearchRequest{Id: "list", Source: 1, Keyword: "query", Page: 2, PageSize: 30}
	r, err := client.Call[pb.MusicResult](ctx, "musichub.v1.Search", args)
	if err != nil || r.Total != 5 || len(r.Data) != 1 || r.Data[0].Source != 1 || r.Data[0].Cover != "cover" {
		t.Fatalf("Search = %+v, %v", r, err)
	}
	list, err := client.Call[pb.PlaylistResult](ctx, "musichub.v1.Playlists", args)
	if err != nil || list.Total != 8 || len(list.Data) != 1 || list.Data[0].CreatorUid != "uid" {
		t.Fatalf("Playlists = %+v, %v", list, err)
	}
	r, err = client.Call[pb.MusicResult](ctx, "musichub.v1.SongList", args)
	if err != nil || len(r.Data) != 1 || r.Data[0].Id != "list" {
		t.Fatalf("SongList = %+v, %v", r, err)
	}
	coef, err := client.Call[pb.CoefficientReply](ctx, "musichub.v1.Loudness", &pb.SongRequest{})
	if err != nil || coef.Coefficient != 0.5 {
		t.Fatalf("Loudness = %v, %v", coef, err)
	}
	ids, err := client.Call[pb.IDsReply](ctx, "musichub.v1.Similar", &pb.SongRequest{})
	if err != nil || len(ids.Ids) != 1 || ids.Ids[0] != "similar" {
		t.Fatalf("Similar = %v, %v", ids, err)
	}
	set, err := client.Call[pb.BoolReply](ctx, "musichub.v1.CookieStatus", &pb.Empty{})
	if err != nil || !set.Value {
		t.Fatalf("CookieStatus = %v, %v", set, err)
	}
	persisted, err := client.Call[pb.BoolReply](ctx, "musichub.v1.SetCookie", &pb.CookieRequest{Cookie: "new-cookie"})
	if err != nil || !persisted.Value {
		t.Fatalf("SetCookie = %v, %v", persisted, err)
	}
	_, err = client.Call[pb.Empty](ctx, "musichub.v1.Unknown", &pb.Empty{})
	var remote *rpc.Error
	if !errors.As(err, &remote) || remote.Code != rpc.MethodNotFound {
		t.Fatalf("unknown method: %v", err)
	}
	// A varint on SongRequest.source (a string) must fail parameter decoding.
	_, err = NewServer(service).Handle(ctx, &rpc.Request{Method: "musichub.v1.Load", Params: proto.RawMessage{0x08, 0x01}})
	if !errors.As(err, &remote) || remote.Code != rpc.InvalidParams {
		t.Fatalf("invalid params: %v", err)
	}
}
