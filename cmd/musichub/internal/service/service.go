package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"

	"github.com/bihua-university/alisten/cmd/musichub/internal/config"
	"github.com/bihua-university/alisten/cmd/musichub/internal/loudness"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/music"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
)

// Backend implements the musichub RPC methods on top of the platform clients.
type Backend struct {
	cookieMu sync.Mutex
}

var _ RPCBackend = (*Backend)(nil)

// NewBackend returns a Backend ready to serve musichub RPC requests.
func NewBackend() *Backend {
	return &Backend{}
}

func (b *Backend) Load(ctx context.Context, p *pb.SongRequest) (*pb.Song, error) {
	return GetMusic(ctx, p.Source, p.Id, p.UseCache)
}

func (b *Backend) Search(_ context.Context, p *pb.SearchRequest) (*pb.MusicResult, error) {
	normalize(p)
	provider, err := providerFor(music.Source(p.Source))
	if err != nil {
		return nil, err
	}
	return provider.Search(p)
}

func (b *Backend) Playlists(_ context.Context, p *pb.SearchRequest) (*pb.PlaylistResult, error) {
	normalize(p)
	provider, err := providerFor(music.Source(p.Source))
	if err != nil {
		return nil, err
	}
	searcher, ok := provider.(PlaylistSearcher)
	if !ok {
		return nil, fmt.Errorf("source %q does not support playlist search", music.Source(p.Source))
	}
	return searcher.SearchPlaylist(p)
}

func (b *Backend) SongList(_ context.Context, p *pb.SearchRequest) (*pb.MusicResult, error) {
	provider, err := providerFor(music.Source(p.Source))
	if err != nil {
		return nil, err
	}
	lister, ok := provider.(SongLister)
	if !ok {
		return nil, fmt.Errorf("source %q does not support song list", music.Source(p.Source))
	}
	return lister.SongList(p.Id)
}

// Loudness 返回响度归一化系数；未分析过时同步完成分析，异步化由调用方负责。
func (b *Backend) Loudness(_ context.Context, p *pb.SongRequest) (*pb.CoefficientReply, error) {
	return &pb.CoefficientReply{Coefficient: GetLoudnessCoefficient(p.Source, p.Id)}, nil
}

func (b *Backend) Similar(_ context.Context, p *pb.SongRequest) (*pb.IDsReply, error) {
	syncConfig()
	ids, err := neteaseAPI.GetSimilarSongs(p.Id)
	return &pb.IDsReply{Ids: ids}, err
}

func (b *Backend) CookieStatus(_ context.Context, _ *pb.Empty) (*pb.BoolReply, error) {
	b.cookieMu.Lock()
	defer b.cookieMu.Unlock()
	return &pb.BoolReply{Value: config.MusicConfig.Get().Music.Cookie != ""}, nil
}

func (b *Backend) SetCookie(ctx context.Context, p *pb.CookieRequest) (*pb.BoolReply, error) {
	cookie := strings.TrimSpace(p.Cookie)
	if cookie == "" {
		return nil, errors.New("cookie cannot be empty")
	}
	b.cookieMu.Lock()
	defer b.cookieMu.Unlock()
	err := config.SetCookie(ctx, cookie)
	SetNeteaseCookie(cookie)
	if err != nil {
		logx.Error("persist music Cookie:", err)
		return &pb.BoolReply{Value: false}, nil
	}
	return &pb.BoolReply{Value: true}, nil
}

// loudnessCache 缓存响度分析结果;结果不会变化,只做 LRU 淘汰。
var loudnessCache = mustLRU[string, *loudness.Result](1024)

func mustLRU[K comparable, V any](size int) *lru.Cache[K, V] {
	c, err := lru.New[K, V](size)
	if err != nil {
		panic(err)
	}
	return c
}

func loudnessKey(source, id string) string {
	return source + "OvO" + id
}

var (
	loudnessGroup      singleflight.Group
	loudnessSema       = make(chan struct{}, 4) // 限制并发下载/解码
	loudnessHTTPClient = &http.Client{Timeout: 2 * time.Minute}
)

// GetLoudnessCoefficient 返回响度归一化的线性音量系数 (Spotify 策略)。
// 未缓存时同步计算;同一首歌的并发请求共享一次计算,失败结果也会缓存,避免反复下载。
func GetLoudnessCoefficient(source, id string) float64 {
	key := loudnessKey(source, id)
	if r, ok := loudnessCache.Get(key); ok {
		return coefficientOf(r)
	}
	value, _, _ := loudnessGroup.Do(key, func() (any, error) {
		if r, ok := loudnessCache.Get(key); ok { // 等待期间可能已完成
			return r, nil
		}
		loudnessSema <- struct{}{}
		defer func() { <-loudnessSema }()
		r, err := loudnessCompute(source, id)
		if err != nil {
			r = nil
		}
		loudnessCache.Add(key, r)
		return r, nil
	})
	r, _ := value.(*loudness.Result)
	return coefficientOf(r)
}

func coefficientOf(r *loudness.Result) float64 {
	if r == nil {
		return 1
	}
	return r.Coefficient()
}

// loudnessCompute 下载歌曲音频并做 BS.1770 响度分析;测试可注入假实现。
var loudnessCompute = func(source, id string) (*loudness.Result, error) {
	m, err := GetMusic(context.Background(), source, id, true)
	if err != nil {
		return nil, fmt.Errorf("获取歌曲信息失败: %w", err)
	}
	if m.Url == "" {
		return nil, fmt.Errorf("source %s id %s: 没有可下载的播放地址", source, id)
	}

	resp, err := loudnessHTTPClient.Get(m.Url)
	if err != nil {
		return nil, fmt.Errorf("下载音频失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载音频失败: HTTP %d", resp.StatusCode)
	}

	return loudness.AnalyzeReader(resp.Body)
}
