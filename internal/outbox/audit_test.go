package outbox

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// TEST-DDAE-12-001: all repeated states are pending before the first ack.
func TestAuditPendingRepeatedStatesSurviveEveryAckAndReopen(t *testing.T) {
	options := Options{StateDir: t.TempDir(), MaxBytes: 1 << 20, MaxEvents: 20, MaxCheckpoints: 10, Retention: time.Hour}
	s, err := Open(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s != nil {
			s.Close()
		}
	})
	for _, item := range [][2]string{{"one", "A"}, {"one", "B"}, {"other", "X"}, {"one", "A"}, {"one", "B"}, {"one", "A"}} {
		if inserted, err := s.Enqueue(testEvent(t, item[0], item[1]), "", time.Now()); err != nil || !inserted {
			t.Fatalf("enqueue: %v %v", inserted, err)
		}
	}
	records, err := s.Records(20)
	if err != nil || len(records) != 6 {
		t.Fatalf("records=%d error=%v", len(records), err)
	}
	for i := range records {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		s, err = Open(options)
		if err != nil {
			t.Fatalf("reopen before ack %d: %v", i, err)
		}
		if err := s.Acknowledge(records[i].Sequence); err != nil {
			t.Fatalf("ack %d: %v", i, err)
		}
		if err := s.Acknowledge(records[i].Sequence); err != nil {
			t.Fatalf("repeat ack: %v", err)
		}
		remaining, err := s.Records(20)
		if err != nil || len(remaining) != len(records)-i-1 {
			t.Fatalf("remaining: %d %v", len(remaining), err)
		}
		var size int64
		for j, r := range remaining {
			if r.Sequence != records[i+1+j].Sequence {
				t.Fatal("sequence changed")
			}
			b, _ := json.Marshal(r)
			size += int64(len(b))
		}
		stats, err := s.Stats()
		if err != nil || stats.Events != len(remaining) || stats.Bytes != size {
			t.Fatalf("accounting: %+v %v", stats, err)
		}
		for _, id := range []string{"one", "other"} {
			cp, exists, err := s.Checkpoint(id)
			if err != nil || !exists {
				t.Fatalf("checkpoint: %v", err)
			}
			want := ""
			for _, r := range remaining {
				if r.AlertID == id {
					want = r.ContentHash
				}
			}
			if cp.PendingHash != want {
				t.Fatalf("ack %d: pending cleared before newest record for %s", i, id)
			}
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(options)
	if err != nil {
		t.Fatal(err)
	}
}

// TEST-DDAE-12-007: equality is full; pending data cannot be pruned to make room.
func TestAuditCheckpointCapacityHealthAndSafePruning(t *testing.T) {
	s, err := Open(Options{StateDir: t.TempDir(), MaxBytes: 1 << 20, MaxEvents: 10, MaxCheckpoints: 1, Retention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now()
	if _, full, err := s.Health(); err != nil || full {
		t.Fatalf("empty health: %v %v", full, err)
	}
	if _, err := s.Enqueue(testEvent(t, "one", "pending"), "", now); err != nil {
		t.Fatal(err)
	}
	if events, full, err := s.Health(); err != nil || !full || events != 1 {
		t.Errorf("at capacity health: %d %v %v", events, full, err)
	}
	var capacity FullError
	for _, at := range []time.Time{now, now.Add(2 * time.Hour)} {
		if err := s.ReconcileListed(map[string]struct{}{}, at, true); !errors.As(err, &capacity) {
			t.Errorf("full reconcile: %v", err)
		}
	}
	records, _ := s.Records(10)
	if len(records) != 1 {
		t.Fatal("pending record pruned")
	}
	if err := s.Acknowledge(records[0].Sequence); err != nil {
		t.Fatal(err)
	}
	if events, full, err := s.Health(); err != nil || !full || events != 0 {
		t.Errorf("checkpoint-only full: %d %v %v", events, full, err)
	}
	if err := s.ReconcileListed(map[string]struct{}{}, now.Add(3*time.Hour), true); err != nil {
		t.Fatal(err)
	}
	if _, full, err := s.Health(); err != nil || full {
		t.Fatalf("pruned health: %v %v", full, err)
	}
}
