package snapshot

import "time"

// Song represents a song in the playlist
type Song struct {
	Source string `json:"source"`
	ID     string `json:"id"`
	User   string `json:"user"`
	Likes  int    `json:"likes"`
}

// RoomState represents the state of a single room
type RoomState struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Desc      string `json:"desc"`
	Password  string `json:"password"`
	Mode      string `json:"mode"`
	Current   Song   `json:"current"`
	Playlist  []Song `json:"playlist"`
	PushTime  int64  `json:"pushTime"`
	UpdatedAt int64  `json:"updatedAt"`
}

// Snapshot represents a full state snapshot
type Snapshot struct {
	Rooms   map[string]*RoomState `json:"rooms"`
	SavedAt int64                 `json:"savedAt"`
	Version int                   `json:"version"`
}

// NewSnapshot creates a new empty snapshot
func NewSnapshot() *Snapshot {
	return &Snapshot{
		Rooms:   make(map[string]*RoomState),
		SavedAt: time.Now().UnixMilli(),
		Version: 1,
	}
}
