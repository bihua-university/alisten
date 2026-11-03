// Package music defines the shared music domain.
// Platform clients, caches and audio analysis belong to cmd/musichub/internal.
package music

import "fmt"

// Source 表示音乐平台来源。
type Source int32

const (
	QQ      Source = iota // 保持既有数值，兼容前端协议
	NetEase               // 1
	KuWo                  // 2

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
