// Package musicqueue manages one room's playlist and playback schedule.
//
// Queue is a state machine with no locks, timers, goroutines, or channels. The
// caller must serialize access: House is the room's only concurrency boundary.
// RPCs for loading, searching, recommending, and loudness analysis always run
// outside House.mu.
//
// # Locking
//
// House.mu protects the room's connections, settings, closed state, playback
// timer, and its Queue. HTTP and WebSocket handlers change the queue or read
// snapshots only through House methods; they never touch the lock or the Queue
// directly.
//
// Queue never calls back into House, and the playback timer belongs entirely
// to House. House's registry mutex (housesMu) is never nested with House.mu,
// and connections are closed after House.mu is released.
//
// # Queue operations
//
// Add enforces the queue limit and rejects duplicates. Remove deletes by
// source and ID, and supports deletion by song name for the legacy HTTP API.
// AddLike increments Likes and then stable-sorts the list by likes; the legacy
// API addresses a track by 1-based index instead. Vote rejects duplicate
// voters, derives the required votes from the online count, and reserves the
// next track once the threshold is reached. Items, Playback, and Snapshot
// return copies of the state, so callers cannot modify the internal list.
//
// # Transitions and the timer
//
// Next reserves the next track inside House's critical section and increments
// generation, so a timer tick and a vote cannot consume the same track twice.
// Next and LoadFailed report a Transition carrying the reserved track, the
// generation, and whether anything changed. Loading runs outside the lock: a
// failed load continues with the next track through LoadFailed, a successful
// one commits playback through Apply, and loads whose generation is stale are
// dropped. Deadline exposes the playback end time, so House can schedule Skip
// with time.AfterFunc, stopping the old timer on skip or room close instead of
// polling.
//
// House keeps a once-per-minute idle-room check; it never drives playback.
//
// # Events
//
// Queue only returns business results and state copies; it never produces
// broadcast events. House turns those results into events: a transition
// announces only the track that left the waiting queue (queue.removed), a
// successful load broadcasts the playback snapshot, a stop broadcasts the idle
// playback snapshot, and ordinary queue changes broadcast a full QueueData
// snapshot so clients can still resync.
//
// House mutates state, fills in the online count, and broadcasts inside one
// critical section, keeping event order identical to state order. Broadcasts
// never perform synchronous WebSocket writes.
//
// Every connection has a bounded send channel of 64 slots; a full buffer
// closes the slow connection so memory cannot grow without bound.
// Connection.Close only closes done, the send channel is never closed, and the
// writer goroutine exits when it observes done, so concurrent sends cannot
// panic.
//
// The queue needs no event channel, overflow coalescing, separate event
// consumer, or extra resync protocol.
//
// # Usage
//
// Queue has no internal synchronization, so call it while holding the room
// lock and run the backend RPC that loads a track while holding nothing:
//
//	q := musicqueue.New()
//	if q.Add(order, 10) != musicqueue.Added {
//		// Full or Duplicate; nothing was queued.
//	}
//
//	// Inside the room lock: reserve a track and keep its generation.
//	transition := q.Next(true, false)
//	if !transition.Changed || transition.Track.ID == "" {
//		return // nothing to play
//	}
//
//	// Outside the lock: load the reserved track.
//	song, err := load(transition.Track)
//
//	// Inside the room lock again: commit this generation, or skip it.
//	if err != nil {
//		q.LoadFailed(transition.Generation, false)
//		return
//	}
//	transition.Track.Song = *song
//	q.Apply(transition.Track, transition.Generation, 1)
//	playback, current := q.Playback()
package musicqueue
