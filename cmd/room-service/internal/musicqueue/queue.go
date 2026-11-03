package musicqueue

import (
	"slices"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/protocol"
)

type Queue struct {
	items      []Track
	current    Track
	preparing  Track
	end        time.Time
	votes      []auth.User
	generation uint64
	playback   protocol.PlaybackData
}

func New() *Queue {
	return &Queue{
		items:    make([]Track, 0),
		votes:    make([]auth.User, 0),
		playback: protocol.PlaybackData{VolumeCoefficient: 1},
	}
}

func (q *Queue) Items() []Track {
	return slices.Clone(q.items)
}

func (q *Queue) Playback() (protocol.PlaybackData, Track) {
	return q.playback, q.current
}

func (q *Queue) Snapshot(online int64) protocol.QueueData {
	data := protocol.QueueData{Items: make([]protocol.QueueItem, 0, len(q.items)), OnlineCount: online}
	if q.current.ID != "" {
		current := q.current.item()
		data.Current = &current
	}
	for _, track := range q.items {
		data.Items = append(data.Items, track.item())
	}
	return data
}

func (q *Queue) CanRecommend(generation uint64) bool {
	return q.generation == generation && len(q.items) == 0
}

func (q *Queue) Deadline() time.Time {
	return q.end
}
