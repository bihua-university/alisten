package types

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
