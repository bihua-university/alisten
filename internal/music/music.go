package music

import "fmt"

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

type H = map[string]interface{}

type Source int

const (
	QQ Source = iota
	NetEase
	KuWo
)

func (s Source) String() string {
	switch s {
	case QQ:
		return "qq"
	case NetEase:
		return "netease"
	case KuWo:
		return "kuwo"
	default:
		return "unknown"
	}
}

type Music struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int64  `json:"duration"`
	Cover    string `json:"cover"`
	Source   Source `json:"source"`
}

type Playlist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PictureURL string `json:"pictureUrl"`
	Desc       string `json:"desc"`
	Creator    string `json:"creator"`
	CreatorUid string `json:"creatorUid"`
	PlayCount  int64  `json:"playCount"`
	SongCount  int64  `json:"songCount"`
}

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
