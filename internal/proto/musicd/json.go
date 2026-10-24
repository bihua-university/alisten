package musicd

import "encoding/json"

// The HTTP API predates the RPC split and returns these messages directly as
// JSON. These marshalers keep its lowercase field names and zero values, which
// the former music model defined with struct tags. Protobuf encoding does not
// use encoding/json, so the RPC wire format is unaffected.

func (m Music) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Artist   string `json:"artist"`
		Album    string `json:"album"`
		Duration int64  `json:"duration"`
		Cover    string `json:"cover"`
		Source   int32  `json:"source"`
	}{m.Id, m.Name, m.Artist, m.Album, m.Duration, m.Cover, m.Source})
}

func (p Playlist) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		PictureURL string `json:"pictureUrl"`
		Desc       string `json:"desc"`
		Creator    string `json:"creator"`
		CreatorUID string `json:"creatorUid"`
		PlayCount  int64  `json:"playCount"`
		SongCount  int64  `json:"songCount"`
	}{p.Id, p.Name, p.PictureUrl, p.Desc, p.Creator, p.CreatorUid, p.PlayCount, p.SongCount})
}

func (s Song) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
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
	}{s.Type, s.Url, s.WebUrl, s.PictureUrl, s.Duration, s.Source, s.Lyric, s.Artist, s.Name, s.Album, s.Id})
}
