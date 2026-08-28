package outbox

import (
	"errors"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestReconcileNeverEvictsPendingOrIneligibleCheckpoints(t *testing.T) {
	options := integrityOptions(t.TempDir())
	options.MaxCheckpoints = 1
	store, err := Open(options)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(1000, 0)
	if _, err := store.Enqueue(testEvent(t, "pending", "keep"), "marker", now); err != nil {
		t.Fatal(err)
	}
	if err := store.db.Update(func(tx *bolt.Tx) error {
		return putCheckpoint(tx.Bucket(bucketCheckpoints), Checkpoint{AlertID: "recent", LastSeenAt: now.UnixNano()})
	}); err != nil {
		t.Fatal(err)
	}

	err = store.ReconcileListed(map[string]struct{}{}, now.Add(2*time.Hour), true)
	var full FullError
	if !errors.As(err, &full) {
		t.Fatalf("expected visible full-state failure, got %v", err)
	}
	for _, id := range []string{"pending", "recent"} {
		if _, exists, err := store.Checkpoint(id); err != nil || !exists {
			t.Fatalf("checkpoint %q was evicted: exists=%v err=%v", id, exists, err)
		}
	}
}
