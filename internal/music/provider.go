package music

import (
	"errors"
	"fmt"
	"time"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/music/internal/kuwo"
	"github.com/bihua-university/alisten/internal/music/internal/netease"
	"github.com/bihua-university/alisten/internal/music/internal/qq"
	"github.com/bihua-university/alisten/internal/music/internal/types"
	"github.com/bihua-university/alisten/internal/task"
)

// Provider 统一抽象一个音乐平台的核心能力。
type Provider interface {
	// Source 返回该 provider 对应的平台来源
	Source() types.Source
	// Search 搜索歌曲
	Search(o SearchOption) (*Result[Music], error)
	// GetSong 获取一首歌的完整播放信息
	GetSong(id string) (*Song, error)
}

// PlaylistSearcher 是可选能力：歌单搜索。
type PlaylistSearcher interface {
	SearchPlaylist(o SearchOption) (*Result[Playlist], error)
}

// SongLister 是可选能力：获取歌单全部歌曲。
type SongLister interface {
	SongList(playlistID string) (*Result[Music], error)
}

// Downloader 提供歌曲下载地址，供音乐文件代理（mpipe）使用。
type Downloader interface {
	GetDownloadURL(songID string) (string, error)
}

// NewNeteaseDownloader 创建一个网易云下载器。
func NewNeteaseDownloader(cookie string) Downloader {
	return netease.New(cookie)
}

// 全局 Provider，由 Init 初始化一次（urlCommonP 零值即可用，无需初始化）。
var (
	neteaseAPI *netease.Client // 供 Provider 与 Recommender 共用
	neteaseP   *neteaseProvider
	qqP        *qqProvider
	urlCommonP taskProvider
)

// Init 初始化音乐平台 Provider，依赖 base.Config，须在配置加载后调用一次。
func Init() {
	neteaseAPI = netease.New(base.Config.Cookie)
	neteaseP = &neteaseProvider{client: neteaseAPI}
	qqP = &qqProvider{client: qq.New(), kuwo: kuwo.New()}
}

// providerFor 返回指定来源的 Provider。
func providerFor(s types.Source) (Provider, error) {
	switch s {
	case types.NetEase:
		return neteaseP, nil
	case types.QQ:
		return qqP, nil
	case types.URLCommon:
		return urlCommonP, nil
	default:
		return nil, fmt.Errorf("unsupported music source: %q", s)
	}
}

// paginate 对全量列表做内存分页。
func paginate[T any](data []*T, o SearchOption) []*T {
	start := (o.Page - 1) * o.PageSize
	if start >= int64(len(data)) {
		return nil
	}
	end := min(start+o.PageSize, int64(len(data)))
	return data[start:end]
}

// neteaseProvider 网易云音乐平台实现。
type neteaseProvider struct {
	client *netease.Client
}

func (p *neteaseProvider) Source() types.Source { return types.NetEase }

func (p *neteaseProvider) Search(o SearchOption) (*Result[Music], error) {
	r, err := p.client.Search(o.Keyword)
	if err != nil {
		return nil, err
	}
	r.Data = paginate(r.Data, o)
	return r, nil
}

func (p *neteaseProvider) SearchPlaylist(o SearchOption) (*Result[Playlist], error) {
	r, err := p.client.SearchPlaylist(o.Keyword)
	if err != nil {
		return nil, err
	}
	r.Data = paginate(r.Data, o)
	return r, nil
}

func (p *neteaseProvider) SongList(playlistID string) (*Result[Music], error) {
	songs, err := p.client.PlaylistSongs(playlistID)
	if err != nil {
		return nil, err
	}
	return &Result[Music]{Total: int64(len(songs)), Data: songs}, nil
}

func (p *neteaseProvider) GetSong(id string) (*Song, error) {
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

	return &Song{
		Type:       "music",
		URL:        url,
		WebURL:     types.NetEase.WebURL(id),
		PictureURL: m.Cover,
		Duration:   m.Duration,
		Source:     types.NetEase.String(),
		Lyric:      lyric,
		Artist:     m.Artist,
		Name:       m.Name,
		Album:      m.Album,
		ID:         id,
	}, nil
}

// qqProvider QQ 音乐平台实现。
// QQ 官方接口不提供稳定的下载地址，播放链接通过酷我搜索补齐。
type qqProvider struct {
	client *qq.Client
	kuwo   *kuwo.Client
}

func (p *qqProvider) Source() types.Source { return types.QQ }

func (p *qqProvider) Search(o SearchOption) (*Result[Music], error) {
	r, err := p.client.Search(o.Keyword)
	if err != nil {
		return nil, err
	}
	r.Data = paginate(r.Data, o)
	return r, nil
}

func (p *qqProvider) SearchPlaylist(o SearchOption) (*Result[Playlist], error) {
	r, err := p.client.SearchPlaylist(o.Keyword)
	if err != nil {
		return nil, err
	}
	r.Data = paginate(r.Data, o)
	return r, nil
}

func (p *qqProvider) GetSong(id string) (*Song, error) {
	detail, err := p.client.GetSongDetail(id)
	if err != nil {
		return nil, err
	}
	lyric, _ := p.client.GetLyrics(id)

	key := detail.Artist + " " + detail.Name
	klist, err := p.kuwo.Search(key)
	if len(klist) == 0 || err != nil {
		logx.Error("ksearch", key, err)
		return nil, errors.New("qq song: no playable source found")
	}

	rid := klist[0].ID
	url, err := p.kuwo.GetDownloadURL(rid)
	if err != nil {
		logx.Error("kdownload", key, "->", rid, err)
		return nil, errors.New("qq song: download url unavailable")
	}

	return &Song{
		Type:       "music",
		URL:        url,
		WebURL:     types.QQ.WebURL(id),
		PictureURL: detail.Cover,
		Duration:   detail.Duration,
		Source:     types.QQ.String(),
		Lyric:      lyric,
		Artist:     detail.Artist,
		Name:       detail.Name,
		Album:      detail.Album,
		ID:         id,
	}, nil
}

// taskProvider 通过 task 调度器（外部 worker）获取通用 url(url_common) 来源的数据。
// 零值即可使用。
type taskProvider struct{}

func (taskProvider) Source() types.Source { return types.URLCommon }

func (taskProvider) Search(SearchOption) (*Result[Music], error) {
	return nil, errors.New("url_common does not support search")
}

func (taskProvider) GetSong(id string) (*Song, error) {
	t := task.Scheduler.NewTask("url_common:get_music", map[string]string{"url": id})
	r := task.Scheduler.Call(t, 3*time.Minute)
	if r == nil || r.Result == nil {
		return nil, errors.New("get song task failed")
	}

	rg := gjson.ParseBytes(r.Result)
	return &Song{
		Type:       rg.Get("type").String(),
		URL:        rg.Get("url").String(),
		WebURL:     id, // url_common 的 id 即原始 url
		PictureURL: rg.Get("pictureUrl").String(),
		Duration:   rg.Get("duration").Int(),
		Source:     types.URLCommon.String(),
		Artist:     rg.Get("artist").String(),
		Name:       rg.Get("name").String(),
		Album:      rg.Get("al.name").String(),
		ID:         id,
	}, nil
}
