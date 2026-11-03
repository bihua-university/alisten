package musicqueue

import "sort"

func (q *Queue) Add(track Track, limit int) AddResult {
	if limit > 0 && len(q.items) >= limit {
		return Full
	}
	if q.preparing.ID == track.ID && q.preparing.Source == track.Source {
		return Duplicate
	}
	for _, queued := range q.items {
		if queued.ID == track.ID && queued.Source == track.Source {
			return Duplicate
		}
	}
	q.items = append(q.items, track)
	return Added
}

func (q *Queue) AddRecommendation(track Track, generation uint64) bool {
	if q.generation != generation || len(q.items) != 0 ||
		q.current.ID == track.ID || q.preparing.ID == track.ID {
		return false
	}
	q.items = append(q.items, track)
	return true
}

func (q *Queue) Remove(id, source, legacyName string, legacy bool) (Track, bool) {
	for i, track := range q.items {
		matches := track.ID == id && track.Source == source
		if legacy {
			matches = track.Song.Name == legacyName
		}
		if matches {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return track, true
		}
	}
	return Track{}, false
}

func (q *Queue) AddLike(id, source string, legacyIndex int64, legacy bool) (Track, bool) {
	index := -1
	if legacy {
		if legacyIndex >= 1 && legacyIndex <= int64(len(q.items)) {
			index = int(legacyIndex - 1)
		}
	} else {
		for i, track := range q.items {
			if track.ID == id && track.Source == source {
				index = i
				break
			}
		}
	}
	if index < 0 {
		return Track{}, false
	}
	q.items[index].Likes++
	liked := q.items[index]
	sort.SliceStable(q.items, func(i, j int) bool { return q.items[i].Likes > q.items[j].Likes })
	return liked, true
}
