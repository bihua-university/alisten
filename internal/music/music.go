// Package music 是音乐服务的统一门面，对外提供搜索、歌单与播放信息查询。
// 各平台差异由内部 Provider 抽象屏蔽。
package music

import (
	"fmt"

	"github.com/bihua-university/alisten/internal/music/internal/types"
)

// 领域类型统一 re-export
type (
	Source   = types.Source
	Music    = types.Music
	Playlist = types.Playlist
	Song     = types.Song
)

type Result[T any] = types.Result[T]

const (
	Unknown   = types.Unknown
	QQ        = types.QQ
	NetEase   = types.NetEase
	KuWo      = types.KuWo
	URLCommon = types.URLCommon
)

// ParseSource 解析前端传入的来源字符串。
func ParseSource(s string) Source {
	return types.ParseSource(s)
}

// SearchOption 是搜索/列表查询的参数。
type SearchOption struct {
	ID       string // 歌单 ID（GetSongList 使用）
	Source   Source
	Keyword  string
	Page     int64
	PageSize int64
}

func (o *SearchOption) normalize() {
	if o.Page <= 0 {
		o.Page = 1
	}
	if o.PageSize <= 0 {
		o.PageSize = 20
	}
}

// SearchMusic 按关键词搜索歌曲。
func SearchMusic(o SearchOption) (Result[Music], error) {
	o.normalize()
	p, err := providerFor(o.Source)
	if err != nil {
		return Result[Music]{}, err
	}
	r, err := p.Search(o)
	if err != nil {
		return Result[Music]{}, err
	}
	return *r, nil
}

// SearchPlaylist 按关键词搜索歌单。
func SearchPlaylist(o SearchOption) (Result[Playlist], error) {
	o.normalize()
	p, err := providerFor(o.Source)
	if err != nil {
		return Result[Playlist]{}, err
	}
	ps, ok := p.(PlaylistSearcher)
	if !ok {
		return Result[Playlist]{}, fmt.Errorf("source %q does not support playlist search", o.Source)
	}
	r, err := ps.SearchPlaylist(o)
	if err != nil {
		return Result[Playlist]{}, err
	}
	return *r, nil
}

// GetSongList 获取歌单的全部歌曲。
func GetSongList(o SearchOption) (Result[Music], error) {
	p, err := providerFor(o.Source)
	if err != nil {
		return Result[Music]{}, err
	}
	sl, ok := p.(SongLister)
	if !ok {
		return Result[Music]{}, fmt.Errorf("source %q does not support song list", o.Source)
	}
	r, err := sl.SongList(o.ID)
	if err != nil {
		return Result[Music]{}, err
	}
	return *r, nil
}
