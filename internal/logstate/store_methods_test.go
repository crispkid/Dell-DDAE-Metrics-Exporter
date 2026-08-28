package logstate

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

func TestStoreStateMethodsAndCapacity(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := testStore(t, directory)
	defer store.Close()
	at := time.Now().UTC()

	if err := store.MarkSeen("log-seen", "marker", at); err != nil {
		t.Fatal(err)
	}
	exists, marker, lastFetched, err := store.FetchState("log-seen")
	if err != nil || !exists || marker != "marker" || !lastFetched.IsZero() {
		t.Fatalf("fetch exists=%v marker=%q fetched=%v err=%v", exists, marker, lastFetched, err)
	}
	if err := store.ReconcileListed(nil, at.Add(2*time.Hour), false); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.Checkpoint("log-seen"); err != nil || !exists {
		t.Fatalf("incomplete reconciliation removed checkpoint exists=%v err=%v", exists, err)
	}
	if _, err := store.Records(0); err == nil {
		t.Fatal("non-positive record limit was accepted")
	}
	if err := store.Acknowledge(999); err != nil {
		t.Fatalf("missing acknowledgement was not idempotent: %v", err)
	}
	events, full, err := store.Health()
	if err != nil || events != 0 || full {
		t.Fatalf("empty health events=%d full=%v err=%v", events, full, err)
	}
}

func TestStoreRejectsInvalidOptionsAndReportsFullClasses(t *testing.T) {
	if _, err := Open(Options{}); err == nil {
		t.Fatal("invalid options were accepted")
	}
	fullError := FullError{}
	if fullError.Error() == "" || fullError.FailureClass() != observability.ClassBufferFull {
		t.Fatalf("full error classification = %q %q", fullError.Error(), fullError.FailureClass())
	}
	corrupt := CorruptionError{reason: "test"}
	if corrupt.Error() == "" || corrupt.FailureClass() != observability.ClassInternal {
		t.Fatalf("corruption classification = %q %q", corrupt.Error(), corrupt.FailureClass())
	}

	store, err := Open(Options{
		StateDir: filepath.Join(t.TempDir(), "state"), MaxBytes: 1 << 20,
		MaxEvents: 1, MaxCheckpoints: 10, Retention: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().UTC()
	if inserted, err := store.Enqueue(testEvent(t, "log-1", "A", at), "", at); err != nil || !inserted {
		t.Fatalf("first enqueue inserted=%v err=%v", inserted, err)
	}
	if _, err := store.Enqueue(testEvent(t, "log-2", "B", at), "", at); err == nil {
		t.Fatal("event capacity was exceeded")
	} else {
		var classified observability.Classified
		if !errors.As(err, &classified) || classified.FailureClass() != observability.ClassBufferFull {
			t.Fatalf("capacity error = %v", err)
		}
	}
	events, full, err := store.Health()
	if err != nil || events != 1 || !full {
		t.Fatalf("full health events=%d full=%v err=%v", events, full, err)
	}
}

func TestMarkSeenHonorsCheckpointCapacity(t *testing.T) {
	store, err := Open(Options{
		StateDir: filepath.Join(t.TempDir(), "state"), MaxBytes: 1 << 20,
		MaxEvents: 10, MaxCheckpoints: 1, Retention: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().UTC()
	if err := store.MarkSeen("log-1", "", at); err != nil {
		t.Fatal(err)
	}
	if events, full, err := store.Health(); err != nil || events != 0 || !full {
		t.Fatalf("checkpoint capacity health events=%d full=%v err=%v", events, full, err)
	}
	if err := store.MarkSeen("log-2", "", at); err == nil {
		t.Fatal("checkpoint capacity was exceeded")
	}
}
