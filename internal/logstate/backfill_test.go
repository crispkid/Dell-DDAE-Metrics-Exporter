package logstate

import (
	"testing"
	"time"
)

// TEST-DDAE-10-005: an older detail must not replace a newer pending checkpoint.
func TestBackfillOlderMarker(t *testing.T) {
	s := testStore(t, t.TempDir())
	defer s.Close()
	now := time.Now().UTC()
	newer := now.Format(time.RFC3339Nano)
	older := now.Add(-time.Hour).Format(time.RFC3339Nano)
	if _, e := s.Enqueue(testEvent(t, "x", "new", now), newer, now); e != nil {
		t.Fatal(e)
	}
	if inserted, e := s.Enqueue(testEvent(t, "x", "old", now), older, now.Add(time.Second)); e != nil || inserted {
		t.Fatal(inserted, e)
	}
	if e := s.MarkSeen("x", older, now.Add(2*time.Second)); e != nil {
		t.Fatal(e)
	}
	cp, _, e := s.Checkpoint("x")
	rows, _ := s.Records(10)
	if e != nil || cp.ListMarker != newer || len(rows) != 1 {
		t.Fatal(cp, len(rows), e)
	}
}
