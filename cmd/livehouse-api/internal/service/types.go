package service

type ErrorResponse struct {
	Error string `json:"error"`
}

type ItemsResponse[T any] struct {
	Items     []T   `json:"items"`
	TotalSize int64 `json:"totalSize"`
}

type RoomRequest struct {
	HouseID  string `json:"houseId"`
	Password string `json:"password"`
}

type CreateRoomRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Password string `json:"password"`
	Ultimate bool   `json:"ultimate"`
}

type CreateRoomResponse struct {
	HouseID string `json:"houseId"`
}

type RoomInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"desc"`
	Population  int    `json:"population"`
	CreateTime  int64  `json:"createTime"`
	NeedPwd     bool   `json:"needPwd"`
	Ultimate    bool   `json:"ultimate"`
}

type ProtocolUser struct {
	Name      string `json:"name"`
	EmailHash string `json:"emailHash"`
}

type UserProfile struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ProtocolSong struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Name       string `json:"name"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMS int64  `json:"durationMs"`
	PictureURL string `json:"pictureUrl"`
	WebURL     string `json:"webUrl"`
}

type PlayableSong struct {
	ID         string   `json:"id"`
	Source     string   `json:"source"`
	Name       string   `json:"name"`
	Artist     string   `json:"artist"`
	Album      string   `json:"album"`
	DurationMS int64    `json:"durationMs"`
	PictureURL string   `json:"pictureUrl"`
	WebURL     string   `json:"webUrl"`
	URL        string   `json:"url"`
	Lyric      string   `json:"lyric"`
	Gain       *float64 `json:"gain,omitempty"` // dB, nil when unavailable
}

type Track struct {
	Playable PlayableSong `json:"playable"`
	User     ProtocolUser `json:"user"`
	Likes    uint64       `json:"likes"`
	Voters   []string     `json:"voters"`
}

type QueueItem struct {
	Song  ProtocolSong `json:"song"`
	User  ProtocolUser `json:"user"`
	Likes int          `json:"likes"`
}

type QueueData struct {
	Current *QueueItem  `json:"current"`
	Items   []QueueItem `json:"items"`
}

type PickRequest struct {
	HouseID  string       `json:"houseId"`
	Password string       `json:"password"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Source   string       `json:"source"`
	User     *UserProfile `json:"user"`
}

type PickResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Source string `json:"source"`
}

type SearchRequest struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Keyword  string `json:"keyword"`
	Page     int64  `json:"pageIndex"`
	PageSize int64  `json:"pageSize"`
}

type Playlist struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Name        string `json:"name"`
	PictureURL  string `json:"pictureUrl"`
	Description string `json:"description"`
	Creator     string `json:"creator"`
	CreatorID   string `json:"creatorId"`
	PlayCount   int64  `json:"playCount"`
	SongCount   int64  `json:"songCount"`
}

type RecommendRequest struct {
	ID       string `json:"id"`
	HouseID  string `json:"houseId"`
	Password string `json:"password"`
}

type ResolveRequest struct {
	Source string `json:"source"`
	ID     string `json:"id"`
}
