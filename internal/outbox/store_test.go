package outbox

import (
	"errors"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func testEvent(t *testing.T, id, message string) alerts.EncodedEvent {
	t.Helper()
	event, err := alerts.BuildEvent("site-a", id, ddae.AlertDetail{ID: id, Message: &message}, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestEnqueueDeduplicateAcknowledgeAndReplay(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(Options{StateDir: dir, MaxBytes: 1 << 20, MaxEvents: 10, MaxCheckpoints: 10, Retention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	event := testEvent(t, "alert-1", "first")
	inserted, err := store.Enqueue(event, "marker-1", time.Unix(100, 0))
	if err != nil || !inserted {
		t.Fatalf("enqueue: inserted=%v err=%v", inserted, err)
	}
	inserted, err = store.Enqueue(event, "marker-1", time.Unix(101, 0))
	if err != nil || inserted {
		t.Fatalf("duplicate: inserted=%v err=%v", inserted, err)
	}
	records, err := store.Records(10)
	if err != nil || len(records) != 1 {
		t.Fatalf("records=%d err=%v", len(records), err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(Options{StateDir: dir, MaxBytes: 1 << 20, MaxEvents: 10, MaxCheckpoints: 10, Retention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	records, err = store.Records(10)
	if err != nil || len(records) != 1 {
		t.Fatalf("replay records=%d err=%v", len(records), err)
	}
	if err := store.Acknowledge(records[0].Sequence); err != nil {
		t.Fatal(err)
	}
	stats, err := store.Stats()
	if err != nil || stats.Events != 0 {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
}

func TestOutboxFullPreservesExistingRecord(t *testing.T) {
	store, err := Open(Options{StateDir: t.TempDir(), MaxBytes: 1 << 20, MaxEvents: 1, MaxCheckpoints: 10, Retention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Enqueue(testEvent(t, "alert-1", "first"), "", time.Now()); err != nil {
		t.Fatal(err)
	}
	_, err = store.Enqueue(testEvent(t, "alert-2", "second"), "", time.Now())
	var full FullError
	if !errors.As(err, &full) {
		t.Fatalf("expected FullError, got %v", err)
	}
	records, _ := store.Records(10)
	if len(records) != 1 || records[0].AlertID != "alert-1" {
		t.Fatalf("existing outbox changed: %#v", records)
	}
}

func TestCheckpointLifecycleAndRetention(t *testing.T) {
	store, err := Open(Options{StateDir: t.TempDir(), MaxBytes: 1 << 20, MaxEvents: 10, MaxCheckpoints: 2, Retention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(1000, 0)
	if err := store.MarkSeen("alert-1", "marker-1", now); err != nil {
		t.Fatal(err)
	}
	exists, marker, fetched, err := store.FetchState("alert-1")
	if err != nil || !exists || marker != "marker-1" || !fetched.IsZero() {
		t.Fatalf("state exists=%v marker=%q fetched=%v err=%v", exists, marker, fetched, err)
	}
	event := testEvent(t, "alert-1", "updated")
	if inserted, err := store.Enqueue(event, "marker-2", now.Add(time.Minute)); err != nil || !inserted {
		t.Fatalf("enqueue inserted=%v err=%v", inserted, err)
	}
	checkpoint, exists, err := store.Checkpoint("alert-1")
	if err != nil || !exists || checkpoint.PendingHash == "" || checkpoint.ListMarker != "marker-2" {
		t.Fatalf("checkpoint=%#v exists=%v err=%v", checkpoint, exists, err)
	}
	records, _ := store.Records(1)
	if err := store.Acknowledge(records[0].Sequence); err != nil {
		t.Fatal(err)
	}
	checkpoint, _, _ = store.Checkpoint("alert-1")
	if checkpoint.PendingHash != "" || checkpoint.DeliveredHash != event.ContentHash {
		t.Fatalf("ack checkpoint = %#v", checkpoint)
	}
	if err := store.ReconcileListed(map[string]struct{}{}, now.Add(2*time.Hour), true); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileListed(map[string]struct{}{}, now.Add(4*time.Hour), true); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.Checkpoint("alert-1"); err != nil || exists {
		t.Fatalf("expired checkpoint exists=%v err=%v", exists, err)
	}
}

func TestReconcileIncompleteDoesNotMarkAbsentAndCheckpointLimitFailsClosed(t *testing.T) {
	store, err := Open(Options{StateDir: t.TempDir(), MaxBytes: 1 << 20, MaxEvents: 10, MaxCheckpoints: 1, Retention: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now()
	if err := store.MarkSeen("listed", "", now); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileListed(map[string]struct{}{}, now, false); err != nil {
		t.Fatal(err)
	}
	cp, _, _ := store.Checkpoint("listed")
	if cp.AbsentSince != 0 {
		t.Fatal("incomplete list changed absence state")
	}
	var full FullError
	if err := store.MarkSeen("also-listed", "", now); !errors.As(err, &full) {
		t.Fatalf("expected checkpoint FullError, got %v", err)
	}
}

func TestOpenValidationByteLimitAndIdempotentMissingAck(t *testing.T) {
	if _, err := Open(Options{}); err == nil {
		t.Fatal("invalid options accepted")
	}
	store, err := Open(Options{StateDir: t.TempDir(), MaxBytes: 1, MaxEvents: 10, MaxCheckpoints: 10, Retention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Enqueue(testEvent(t, "alert-1", "large"), "", time.Now()); err == nil {
		t.Fatal("byte limit did not reject event")
	}
	if err := store.Acknowledge(999); err != nil {
		t.Fatalf("missing acknowledgement was not idempotent: %v", err)
	}
	stats, err := store.Stats()
	if err != nil || stats.Events != 0 || stats.Bytes != 0 {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
	events, full, err := store.Health()
	if err != nil || events != 0 || full {
		t.Fatalf("health events=%d full=%v err=%v", events, full, err)
	}
	classified := FullError{}
	if classified.Error() == "" || classified.FailureClass() == "" {
		t.Fatal("full error lacks bounded classification")
	}
}
