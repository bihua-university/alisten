package musicqueue

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/protocol"
)

func (q *Queue) Vote(user auth.User, online int, random bool) (protocol.VoteResult, Transition, VoteResult) {
	var result protocol.VoteResult
	if q.preparing.ID != "" {
		return result, Transition{}, VotePreparing
	}
	if q.current.ID == "" {
		return result, Transition{}, VoteNoCurrent
	}
	if slices.Contains(q.votes, user) {
		return result, Transition{}, VoteDuplicate
	}
	q.votes = append(q.votes, user)
	result.CurrentVotes = int64(len(q.votes))
	result.RequiredVotes = int64(max((online+2)/3, 1))
	if result.CurrentVotes >= result.RequiredVotes {
		transition := q.next(true, random)
		result.Skipped = transition.Changed
		return result, transition, VoteAccepted
	}
	return result, Transition{}, VoteAccepted
}

func (q *Queue) Next(force, random bool) Transition {
	return q.next(force, random)
}

func (q *Queue) next(force, random bool) Transition {
	if q.preparing.ID != "" {
		return Transition{}
	}
	if !force && q.current.ID != "" && q.end.After(time.Now()) {
		return Transition{}
	}
	if len(q.items) == 0 {
		if q.current.ID == "" {
			return Transition{}
		}
		q.current = Track{}
		q.end = time.Time{}
		q.votes = nil
		q.generation++
		q.playback = protocol.PlaybackData{VolumeCoefficient: 1}
		return Transition{Generation: q.generation, Changed: true}
	}
	index := 0
	if random {
		index = rand.IntN(len(q.items))
	}
	play := q.items[index]
	q.items = append(q.items[:index], q.items[index+1:]...)
	q.preparing = play
	q.generation++
	q.votes = nil
	return Transition{Track: play, Generation: q.generation, Changed: true}
}

func (q *Queue) LoadFailed(generation uint64, random bool) (Transition, bool) {
	if q.generation != generation {
		return Transition{}, false
	}
	q.preparing = Track{}
	return q.next(true, random), true
}

func (q *Queue) Apply(track Track, generation uint64, coefficient float64) bool {
	if q.generation != generation {
		return false
	}
	now := time.Now()
	q.current = track
	q.preparing = Track{}
	q.votes = nil
	pushTime := now.Add(200 * time.Millisecond).UnixMilli()
	q.end = time.UnixMilli(pushTime).Add(time.Duration(track.Song.Duration) * time.Millisecond)
	playable := protocol.PlayableData(track.Song)
	user := protocol.UserData(track.User)
	q.playback = protocol.PlaybackData{
		Song:              &playable,
		User:              &user,
		StartedAtMS:       pushTime,
		VolumeCoefficient: coefficient,
	}
	return true
}
