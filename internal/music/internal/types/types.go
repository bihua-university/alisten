// Package types 定义音乐领域的核心类型，供 internal/music 及其子包共用。
// 该包不依赖项目内其他包，以避免循环引用。
package types

import "fmt"

// Source 表示音乐平台来源。
type Source int

const (
	QQ        Source = iota // 保持既有数值，兼容前端协议
	NetEase                 // 1
	KuWo                    // 2
	URLCommon               // 3

	Unknown Source = -1
)

// ParseSource 解析前端传入的来源字符串。
// 兼容历史别名：wy→NetEase。
func ParseSource(s string) Source {
	switch s {
	case "qq":
		return QQ
	case "wy", "netease":
		return NetEase
	case "kuwo":
		return KuWo
	case "url_common":
		return URLCommon
	default:
		return Unknown
	}
}

func (s Source) String() string {
	switch s {
	case QQ:
		return "qq"
	case NetEase:
		return "netease"
	case KuWo:
		return "kuwo"
	case URLCommon:
		return "url_common"
	default:
		return "unknown"
	}
}

// WebURL 返回该来源歌曲的网页地址。
func (s Source) WebURL(id string) string {
	switch s {
	case NetEase:
		return fmt.Sprintf("https://music.163.com/#/song?id=%s", id)
	case QQ:
		return fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", id)
	default:
		return ""
	}
}

// Music 是歌曲的基础元信息，用于搜索与歌单列表。
type Music struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int64  `json:"duration"`
	Cover    string `json:"cover"`
	Source   Source `json:"source"`
}

// Playlist 是歌单的元信息。
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

// Song 是一首可播放歌曲的完整信息。
// JSON 字段名与前端协议保持一致。
type Song struct {
	Type       string `json:"type"`
	URL        string `json:"url"`
	WebURL     string `json:"webUrl"`
	PictureURL string `json:"pictureUrl"`
	Duration   int64  `json:"duration"`
	Source     string `json:"source"`
	Lyric      string `json:"lyric"`
	Artist     string `json:"artist"`
	Name       string `json:"name"`
	Album      string `json:"album"`
	ID         string `json:"id"`
}

// Result 是分页查询结果。
type Result[T any] struct {
	Total int64
	Data  []*T
}
