package musicqueue

import "testing"

func TestQueueAddLikeAndRemove(t *testing.T) {
	q := New()
	if result := q.Add(Track{ID: "a", Source: "qq"}, 2); result != Added || len(q.Snapshot(0).Items) != 1 {
		t.Fatalf("add a: result=%v snapshot=%+v", result, q.Snapshot(0))
	}
	if result := q.Add(Track{ID: "a", Source: "qq"}, 2); result != Duplicate {
		t.Fatalf("duplicate: %v", result)
	}
	if result := q.Add(Track{ID: "b", Source: "qq"}, 2); result != Added {
		t.Fatalf("add b: %v", result)
	}
	if result := q.Add(Track{ID: "c", Source: "qq"}, 2); result != Full {
		t.Fatalf("full result: %v", result)
	}
	liked, ok := q.AddLike("b", "qq", 0, false)
	if !ok || liked.Likes != 1 || q.Snapshot(0).Items[0].Song.ID != "b" {
		t.Fatalf("add like: track=%+v ok=%v snapshot=%+v", liked, ok, q.Snapshot(0))
	}
	items := q.Items()
	if len(items) != 2 || items[0].ID != "b" || items[0].Likes != 1 {
		t.Fatalf("likes were not sorted: %+v", items)
	}
	removed, ok := q.Remove("b", "qq", "", false)
	if !ok || removed.ID != "b" || len(q.Items()) != 1 {
		t.Fatalf("remove: track=%+v ok=%v items=%+v", removed, ok, q.Items())
	}
}

func TestQueueTransitionAndDeadline(t *testing.T) {
	q := New()
	if !q.Apply(Track{ID: "current", Source: "qq"}, 0, 1) || q.Deadline().IsZero() {
		t.Fatal("apply failed")
	}
	q.Add(Track{ID: "next", Source: "qq"}, 0)
	transition := q.Next(true, false)
	if !transition.Changed || transition.Track.ID != "next" || transition.Generation != 1 {
		t.Fatalf("unexpected transition: %+v", transition)
	}
	if !q.Apply(transition.Track, transition.Generation, 1) {
		t.Fatal("failed to apply transition")
	}
	transition, valid := q.LoadFailed(transition.Generation, false)
	if !valid || !transition.Changed || transition.Track.ID != "" || !q.Deadline().IsZero() {
		t.Fatalf("load failure did not stop playback: transition=%+v valid=%v", transition, valid)
	}
}
