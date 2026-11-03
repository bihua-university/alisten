// Package service 是音乐服务的统一门面，对外提供搜索、歌单与播放信息查询。
// 各平台差异由内部 Provider 抽象屏蔽；RPC 层直接收发 pb 消息，不再转换为领域类型。
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bihua-university/alisten/cmd/musichub/internal/config"
	"github.com/bihua-university/alisten/cmd/musichub/internal/kuwo"
	"github.com/bihua-university/alisten/cmd/musichub/internal/netease"
	"github.com/bihua-university/alisten/cmd/musichub/internal/qq"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/music"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
)

// Provider 统一抽象一个音乐平台的核心能力，直接收发 RPC 消息。
type Provider interface {
	// Source 返回该 provider 对应的平台来源
	Source() music.Source
	// Search 搜索歌曲
	Search(*pb.SearchRequest) (*pb.MusicResult, error)
	// GetSong 获取一首歌的完整播放信息
	GetSong(ctx context.Context, id string) (*pb.Song, error)
}

// PlaylistSearcher 是可选能力：歌单搜索。
type PlaylistSearcher interface {
	SearchPlaylist(*pb.SearchRequest) (*pb.PlaylistResult, error)
}

// SongLister 是可选能力：获取歌单全部歌曲。
type SongLister interface {
	SongList(playlistID string) (*pb.MusicResult, error)
}

// 全局 Provider，由 Init 初始化一次。
var (
	neteaseAPI      *netease.Client // 供 Provider 与 Recommender 共用
	neteaseP        *neteaseProvider
	qqP             *qqProvider
	neteaseConfigMu sync.Mutex
	neteaseCookie   string
)

// Init 初始化音乐平台 Provider，须在配置加载后调用一次。
func Init() {
	neteaseAPI = netease.New("")
	neteaseP = &neteaseProvider{client: neteaseAPI}
	qqP = &qqProvider{client: qq.New(), kuwo: kuwo.New()}
	syncConfig()
}

func syncConfig() {
	neteaseConfigMu.Lock()
	defer neteaseConfigMu.Unlock()
	cookie := config.MusicConfig.Get().Music.Cookie
	if cookie == neteaseCookie {
		return
	}
	neteaseAPI.SetCookie(cookie)
	neteaseCookie = cookie
	cache.Purge()
}

// SetNeteaseCookie 更新网易云客户端的 Cookie，运行时生效。
func SetNeteaseCookie(cookie string) {
	neteaseConfigMu.Lock()
	defer neteaseConfigMu.Unlock()
	neteaseAPI.SetCookie(cookie)
	neteaseCookie = cookie
	cache.Purge()
}

// providerFor 返回指定来源的 Provider。
func providerFor(s music.Source) (Provider, error) {
	syncConfig()
	switch s {
	case music.NetEase:
		return neteaseP, nil
	case music.QQ:
		return qqP, nil
	default:
		return nil, fmt.Errorf("unsupported music source: %q", s)
	}
}

// normalize 补全搜索请求的分页参数。
func normalize(p *pb.SearchRequest) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
}

// paginate 对全量列表做内存分页。
func paginate[T any](data []*T, p *pb.SearchRequest) []*T {
	start := (p.Page - 1) * p.PageSize
	if start >= int64(len(data)) {
		return nil
	}
	end := min(start+p.PageSize, int64(len(data)))
	return data[start:end]
}

// neteaseProvider 网易云音乐平台实现。
type neteaseProvider struct {
	client *netease.Client
}

func (p *neteaseProvider) Source() music.Source { return music.NetEase }

func (p *neteaseProvider) Search(req *pb.SearchRequest) (*pb.MusicResult, error) {
	r, err := p.client.Search(req.Keyword)
	if err != nil {
		return nil, err
	}
	r.Data = paginate(r.Data, req)
	return r, nil
}

func (p *neteaseProvider) SearchPlaylist(req *pb.SearchRequest) (*pb.PlaylistResult, error) {
	r, err := p.client.SearchPlaylist(req.Keyword)
	if err != nil {
		return nil, err
	}
	r.Data = paginate(r.Data, req)
	return r, nil
}

func (p *neteaseProvider) SongList(playlistID string) (*pb.MusicResult, error) {
	songs, err := p.client.PlaylistSongs(playlistID)
	if err != nil {
		return nil, err
	}
	return &pb.MusicResult{Total: int64(len(songs)), Data: songs}, nil
}

func (p *neteaseProvider) GetSong(ctx context.Context, id string) (*pb.Song, error) {
	songs, err := p.client.GetSongDetail([]string{id})
	if err != nil {
		return nil, err
	}
	if len(songs) == 0 {
		return nil, errors.New("song not found")
	}
	m := songs[0]

	// 下载链接与歌词失败不阻塞，保持与历史行为一致
	url, _ := p.client.GetDownloadURL(id)
	lyric, _ := p.client.GetLyrics(id)

	return &pb.Song{
		Type:       "music",
		Url:        url,
		WebUrl:     music.NetEase.WebURL(id),
		PictureUrl: m.Cover,
		Duration:   m.Duration,
		Source:     music.NetEase.String(),
		Lyric:      lyric,
		Artist:     m.Artist,
		Name:       m.Name,
		Album:      m.Album,
		Id:         id,
	}, nil
}

// qqProvider QQ 音乐平台实现。
// QQ 官方接口不提供稳定的下载地址，播放链接通过酷我搜索补齐。
type qqProvider struct {
	client *qq.Client
	kuwo   *kuwo.Client
}

func (p *qqProvider) Source() music.Source { return music.QQ }

func (p *qqProvider) Search(req *pb.SearchRequest) (*pb.MusicResult, error) {
	r, err := p.client.Search(req.Keyword)
	if err != nil {
		return nil, err
	}
	r.Data = paginate(r.Data, req)
	return r, nil
}

func (p *qqProvider) SearchPlaylist(req *pb.SearchRequest) (*pb.PlaylistResult, error) {
	r, err := p.client.SearchPlaylist(req.Keyword)
	if err != nil {
		return nil, err
	}
	r.Data = paginate(r.Data, req)
	return r, nil
}

func (p *qqProvider) GetSong(ctx context.Context, id string) (*pb.Song, error) {
	detail, err := p.client.GetSongDetail(id)
	if err != nil {
		return nil, err
	}
	lyric, _ := p.client.GetLyrics(id)

	key := detail.Artist + " " + detail.Name
	klist, err := p.kuwo.Search(key)
	if err != nil || klist == nil || len(klist.Data) == 0 {
		logx.Error("ksearch", key, err)
		return nil, errors.New("qq song: no playable source found")
	}

	rid := klist.Data[0].Id
	url, err := p.kuwo.GetDownloadURL(rid)
	if err != nil {
		logx.Error("kdownload", key, "->", rid, err)
		return nil, errors.New("qq song: download url unavailable")
	}

	return &pb.Song{
		Type:       "music",
		Url:        url,
		WebUrl:     music.QQ.WebURL(id),
		PictureUrl: detail.Cover,
		Duration:   detail.Duration,
		Source:     music.QQ.String(),
		Lyric:      lyric,
		Artist:     detail.Artist,
		Name:       detail.Name,
		Album:      detail.Album,
		Id:         id,
	}, nil
}
