package outbox

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func integrityOptions(dir string) Options {
	return Options{StateDir: dir, MaxBytes: 1 << 20, MaxEvents: 10, MaxCheckpoints: 10, Retention: time.Hour}
}

func TestStateSchemaMigratesLegacyAndRepairsDerivedCounters(t *testing.T) {
	dir := t.TempDir()
	options := integrityOptions(dir)
	store, err := Open(options)
	if err != nil {
		t.Fatal(err)
	}
	if inserted, err := store.Enqueue(testEvent(t, "alert-1", "legacy"), "marker", time.Unix(200, 0)); err != nil || !inserted {
		t.Fatalf("enqueue inserted=%v err=%v", inserted, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := bolt.Open(filepath.Join(dir, "state.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket(bucketMeta)
		if err := meta.Delete(keySchemaVersion); err != nil {
			return err
		}
		if err := meta.Put(keyEventCount, []byte("malformed")); err != nil {
			return err
		}
		return meta.Put(keyEventBytes, uint64Value(1))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(options)
	if err != nil {
		t.Fatalf("open migrated state: %v", err)
	}
	defer store.Close()
	stats, err := store.Stats()
	if err != nil || stats.Events != 1 || stats.Bytes <= 1 {
		t.Fatalf("repaired stats=%#v err=%v", stats, err)
	}
	if err := store.db.View(func(tx *bolt.Tx) error {
		if got := string(tx.Bucket(bucketMeta).Get(keySchemaVersion)); got != string(currentSchema) {
			t.Fatalf("schema version=%q", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStateOpenFailsClosedForFutureVersionAndPrimaryCorruption(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*bolt.Tx) error
	}{
		{name: "future version", mutate: func(tx *bolt.Tx) error {
			return tx.Bucket(bucketMeta).Put(keySchemaVersion, []byte("2"))
		}},
		{name: "corrupt record", mutate: func(tx *bolt.Tx) error {
			key, _ := tx.Bucket(bucketOutbox).Cursor().First()
			return tx.Bucket(bucketOutbox).Put(key, []byte("{"))
		}},
		{name: "payload unknown field", mutate: func(tx *bolt.Tx) error {
			key, value := tx.Bucket(bucketOutbox).Cursor().First()
			var record Record
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			record.Payload = append(record.Payload[:len(record.Payload)-1], []byte(`,"unknown":"state-canary"}`)...)
			encoded, err := json.Marshal(record)
			if err != nil {
				return err
			}
			return tx.Bucket(bucketOutbox).Put(key, encoded)
		}},
		{name: "corrupt checkpoint", mutate: func(tx *bolt.Tx) error {
			return tx.Bucket(bucketCheckpoints).Put([]byte("alert-1"), []byte("{"))
		}},
		{name: "mismatched pending checkpoint", mutate: func(tx *bolt.Tx) error {
			value := tx.Bucket(bucketCheckpoints).Get([]byte("alert-1"))
			var checkpoint Checkpoint
			if err := json.Unmarshal(value, &checkpoint); err != nil {
				return err
			}
			checkpoint.PendingHash = strings.Repeat("a", 64)
			return putCheckpoint(tx.Bucket(bucketCheckpoints), checkpoint)
		}},
		{name: "missing checkpoint", mutate: func(tx *bolt.Tx) error {
			return tx.Bucket(bucketCheckpoints).Delete([]byte("alert-1"))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			options := integrityOptions(dir)
			store, err := Open(options)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Enqueue(testEvent(t, "alert-1", "primary"), "marker", time.Unix(200, 0)); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := bolt.Open(filepath.Join(dir, "state.db"), 0o600, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Update(test.mutate); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			_, err = Open(options)
			var corruption CorruptionError
			if !errors.As(err, &corruption) {
				t.Fatalf("expected bounded corruption error, got %v", err)
			}
		})
	}
}

func TestAcknowledgeRequiresMatchingCheckpointAndPreservesRecord(t *testing.T) {
	store, err := Open(integrityOptions(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Enqueue(testEvent(t, "alert-1", "pending"), "marker", time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	records, err := store.Records(1)
	if err != nil || len(records) != 1 {
		t.Fatalf("records=%v err=%v", records, err)
	}
	if err := store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketCheckpoints).Delete([]byte("alert-1"))
	}); err != nil {
		t.Fatal(err)
	}
	err = store.Acknowledge(records[0].Sequence)
	var corruption CorruptionError
	if !errors.As(err, &corruption) {
		t.Fatalf("expected corruption error, got %v", err)
	}
	remaining, err := store.Records(10)
	if err != nil || len(remaining) != 1 || remaining[0].Sequence != records[0].Sequence {
		t.Fatalf("record changed after failed ack: %v err=%v", remaining, err)
	}
	stats, err := store.Stats()
	if err != nil || stats.Events != 1 || stats.Bytes == 0 {
		t.Fatalf("stats changed after failed ack: %#v err=%v", stats, err)
	}
}
