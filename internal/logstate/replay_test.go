package logstate

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/serviceability"
)

func testStore(t *testing.T, directory string) *Store {
	t.Helper()
	store, err := Open(Options{
		StateDir: directory, MaxBytes: 1 << 20, MaxEvents: 100,
		MaxCheckpoints: 100, Retention: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func testEvent(t *testing.T, id, message string, at time.Time) serviceability.EncodedEvent {
	t.Helper()
	detail := ddae.ServiceabilityLogDetail{ID: id, Message: &message}
	event, err := serviceability.BuildEvent("site-a", id, detail, at)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestPendingRecordReplaysAfterRestartAndAcknowledges(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	at := time.Now().UTC()
	store := testStore(t, directory)
	event := testEvent(t, "log-1", "A", at)
	inserted, err := store.Enqueue(event, "marker", at)
	if err != nil || !inserted {
		t.Fatalf("enqueue inserted=%v err=%v", inserted, err)
	}
	records, err := store.Records(10)
	if err != nil || len(records) != 1 {
		t.Fatalf("records=%v err=%v", records, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store = testStore(t, directory)
	defer store.Close()
	replayed, err := store.Records(10)
	if err != nil || len(replayed) != 1 || replayed[0].Sequence != records[0].Sequence {
		t.Fatalf("replayed=%v err=%v", replayed, err)
	}
	if err := store.Acknowledge(replayed[0].Sequence); err != nil {
		t.Fatal(err)
	}
	if remaining, _ := store.Records(10); len(remaining) != 0 {
		t.Fatalf("remaining=%v", remaining)
	}
	checkpoint, exists, err := store.Checkpoint("log-1")
	if err != nil || !exists || checkpoint.PendingHash != "" || checkpoint.DeliveredHash != event.ContentHash {
		t.Fatalf("checkpoint=%#v exists=%v err=%v", checkpoint, exists, err)
	}
}
