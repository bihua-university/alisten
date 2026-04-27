package music

import (
	"fmt"

	"github.com/bihua-university/alisten/internal/music/types"
)

// GenerateWebURL generates the web URL for a music track based on source and ID
func GenerateWebURL(source, id string) string {
	switch source {
	case "wy", "netease":
		return fmt.Sprintf("https://music.163.com/#/song?id=%s", id)
	case "qq":
		return fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", id)
	case "db":
		return fmt.Sprintf("https://www.bilibili.com/video/%s", id)
	default:
		return ""
	}
}

type Source = types.Source

const (
	QQ Source = iota
	NetEase
	KuWo
)

type Music = types.Music

type Playlist = types.Playlist

type H = map[string]interface{}

type SearchOption struct {
	ID       string
	Source   string
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

type SearchResult[T any] struct {
	Total int64
	Data  []*T
}
