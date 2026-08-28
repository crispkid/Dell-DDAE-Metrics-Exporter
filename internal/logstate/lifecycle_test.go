package logstate

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLifecyclePreservesReturnToDeliveredContentAndNoDeletionEvent(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := testStore(t, directory)
	defer store.Close()
	at := time.Now().UTC()
	sequence := []struct {
		message  string
		inserted bool
	}{{"A", true}, {"A", false}, {"B", true}, {"A", true}}
	for index, step := range sequence {
		inserted, err := store.Enqueue(testEvent(t, "log-1", step.message, at.Add(time.Duration(index)*time.Second)), "marker", at.Add(time.Duration(index)*time.Second))
		if err != nil || inserted != step.inserted {
			t.Fatalf("step %d inserted=%v err=%v", index, inserted, err)
		}
	}
	records, err := store.Records(10)
	if err != nil || len(records) != 3 {
		t.Fatalf("records=%v err=%v", records, err)
	}
	for _, record := range records {
		if err := store.Acknowledge(record.Sequence); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ReconcileListed(map[string]struct{}{}, at.Add(10*time.Second), true); err != nil {
		t.Fatal(err)
	}
	if records, _ := store.Records(10); len(records) != 0 {
		t.Fatal("disappearance synthesized an event")
	}
	if err := store.ReconcileListed(map[string]struct{}{}, at.Add(2*time.Hour), true); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.Checkpoint("log-1"); err != nil || exists {
		t.Fatalf("retained checkpoint exists=%v err=%v", exists, err)
	}
}

func TestPendingCheckpointIsNeverEvicted(t *testing.T) {
	store := testStore(t, filepath.Join(t.TempDir(), "state"))
	defer store.Close()
	at := time.Now().UTC()
	if _, err := store.Enqueue(testEvent(t, "log-1", "pending", at), "", at); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileListed(map[string]struct{}{}, at, true); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileListed(map[string]struct{}{}, at.Add(2*time.Hour), true); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.Checkpoint("log-1"); err != nil || !exists {
		t.Fatalf("pending checkpoint exists=%v err=%v", exists, err)
	}
}
