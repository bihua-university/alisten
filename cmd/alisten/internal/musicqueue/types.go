package musicqueue

import (
	"github.com/bihua-university/alisten/internal/auth"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
	"github.com/bihua-university/alisten/protocol"
)

type Track struct {
	Source string
	ID     string
	User   auth.User
	Likes  int
	Song   pb.Song
}

type Transition struct {
	Track      Track
	Generation uint64
	Changed    bool
}

type AddResult uint8

const (
	Added AddResult = iota
	Full
	Duplicate
)

type VoteResult uint8

const (
	VoteAccepted VoteResult = iota
	VotePreparing
	VoteNoCurrent
	VoteDuplicate
)

func (t Track) item() protocol.QueueItem {
	song := t.Song
	song.Id, song.Source = t.ID, t.Source
	song.Duration = max(song.Duration, 0)
	return protocol.QueueItem{Song: protocol.SongData(song), User: protocol.UserData(t.User), Likes: int64(t.Likes)}
}
