package logstate

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestDedicatedStateRepairsDerivedCountersButRejectsFutureSchema(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := testStore(t, directory)
	at := time.Now().UTC()
	if _, err := store.Enqueue(testEvent(t, "log-1", "A", at), "", at); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "serviceability-logs.db")
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("state permissions info=%v err=%v", info, err)
	}
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketMeta).Put(keyEventCount, uint64Value(999))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store = testStore(t, directory)
	stats, err := store.Stats()
	if err != nil || stats.Events != 1 {
		t.Fatalf("repaired stats=%#v err=%v", stats, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketMeta).Put(keySchemaVersion, []byte("99"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(Options{StateDir: directory, MaxBytes: 1 << 20, MaxEvents: 100, MaxCheckpoints: 100, Retention: time.Hour}); err == nil {
		t.Fatal("future schema was accepted")
	}
}

func TestPrimaryOutboxCorruptionFailsClosed(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := testStore(t, directory)
	at := time.Now().UTC()
	if _, err := store.Enqueue(testEvent(t, "log-1", "A", at), "", at); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "serviceability-logs.db")
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, 1)
		return tx.Bucket(bucketOutbox).Put(key, []byte(`{"corrupt":true}`))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(Options{StateDir: directory, MaxBytes: 1 << 20, MaxEvents: 100, MaxCheckpoints: 100, Retention: time.Hour}); err == nil {
		t.Fatal("primary corruption was accepted")
	}
}
