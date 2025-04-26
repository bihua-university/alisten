package music

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
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Artist    string    `json:"artist"`
	Album     Album     `json:"album"`
	Duration  int64     `json:"duration"`
	Privilege Privilege `json:"privilege"`
	Source    Source    `json:"source"` // qq/163/kuwo
}

type Album struct {
	Name string `json:"name"`
}

type Privilege struct {
	St int `json:"st"`
	Fl int `json:"fl"`
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
