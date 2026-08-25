package outbox

import (
	"testing"
	"time"
)

func TestEnqueueReturnToDeliveredHashPreservesLatestState(t *testing.T) {
	dir := t.TempDir()
	options := Options{
		StateDir: dir, MaxBytes: 1 << 20, MaxEvents: 10,
		MaxCheckpoints: 10, Retention: time.Hour,
	}
	store, err := Open(options)
	if err != nil {
		t.Fatal(err)
	}

	eventA := testEvent(t, "alert-1", "state-a")
	eventB := testEvent(t, "alert-1", "state-b")
	if inserted, err := store.Enqueue(eventA, "marker-a-1", time.Unix(100, 0)); err != nil || !inserted {
		t.Fatalf("enqueue initial A: inserted=%v err=%v", inserted, err)
	}
	records, err := store.Records(10)
	if err != nil || len(records) != 1 {
		t.Fatalf("initial records=%v err=%v", records, err)
	}
	if err := store.Acknowledge(records[0].Sequence); err != nil {
		t.Fatal(err)
	}

	if inserted, err := store.Enqueue(eventB, "marker-b", time.Unix(101, 0)); err != nil || !inserted {
		t.Fatalf("enqueue B: inserted=%v err=%v", inserted, err)
	}
	if inserted, err := store.Enqueue(eventA, "marker-a-2", time.Unix(102, 0)); err != nil || !inserted {
		t.Fatalf("enqueue returned A: inserted=%v err=%v", inserted, err)
	}
	if inserted, err := store.Enqueue(eventA, "marker-a-3", time.Unix(103, 0)); err != nil || inserted {
		t.Fatalf("duplicate pending A: inserted=%v err=%v", inserted, err)
	}

	records, err = store.Records(10)
	if err != nil || len(records) != 2 {
		t.Fatalf("transition records=%v err=%v", records, err)
	}
	if records[0].ContentHash != eventB.ContentHash || records[1].ContentHash != eventA.ContentHash {
		t.Fatalf("transition order hashes=%q,%q", records[0].ContentHash, records[1].ContentHash)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(options)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	records, err = store.Records(10)
	if err != nil || len(records) != 2 {
		t.Fatalf("reopened records=%v err=%v", records, err)
	}
	if err := store.Acknowledge(records[0].Sequence); err != nil {
		t.Fatal(err)
	}
	checkpoint, exists, err := store.Checkpoint("alert-1")
	if err != nil || !exists {
		t.Fatalf("checkpoint after B: exists=%v err=%v", exists, err)
	}
	if checkpoint.DeliveredHash != eventB.ContentHash || checkpoint.PendingHash != eventA.ContentHash {
		t.Fatalf("checkpoint after B=%#v", checkpoint)
	}
	if err := store.Acknowledge(records[1].Sequence); err != nil {
		t.Fatal(err)
	}
	checkpoint, exists, err = store.Checkpoint("alert-1")
	if err != nil || !exists {
		t.Fatalf("checkpoint after final A: exists=%v err=%v", exists, err)
	}
	if checkpoint.DeliveredHash != eventA.ContentHash || checkpoint.PendingHash != "" {
		t.Fatalf("final checkpoint=%#v", checkpoint)
	}
	stats, err := store.Stats()
	if err != nil || stats.Events != 0 || stats.Bytes != 0 {
		t.Fatalf("final stats=%#v err=%v", stats, err)
	}
}
