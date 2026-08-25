package outbox

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketMeta        = []byte("meta-v1")
	bucketOutbox      = []byte("outbox-v1")
	bucketCheckpoints = []byte("checkpoints-v1")
	keyEventCount     = []byte("event-count")
	keyEventBytes     = []byte("event-bytes")
)

type FullError struct{}

func (FullError) Error() string                     { return "durable outbox is full" }
func (FullError) FailureClass() observability.Class { return observability.ClassBufferFull }

type Options struct {
	StateDir       string
	MaxBytes       int64
	MaxEvents      int
	MaxCheckpoints int
	Retention      time.Duration
}

type Store struct {
	db             *bolt.DB
	maxBytes       int64
	maxEvents      int
	maxCheckpoints int
	retention      time.Duration
}

type Record struct {
	Sequence    uint64 `json:"sequence"`
	AlertID     string `json:"alert_id"`
	RecordKey   []byte `json:"record_key"`
	Payload     []byte `json:"payload"`
	ContentHash string `json:"content_hash"`
	CreatedAt   int64  `json:"created_at_unix_nano"`
}

type Checkpoint struct {
	AlertID       string `json:"alert_id"`
	ListMarker    string `json:"list_marker,omitempty"`
	PendingHash   string `json:"pending_hash,omitempty"`
	DeliveredHash string `json:"delivered_hash,omitempty"`
	LastFetchedAt int64  `json:"last_fetched_at_unix_nano,omitempty"`
	LastSeenAt    int64  `json:"last_seen_at_unix_nano,omitempty"`
	AbsentSince   int64  `json:"absent_since_unix_nano,omitempty"`
}

type Stats struct {
	Events int
	Bytes  int64
	Full   bool
}

func Open(options Options) (*Store, error) {
	if options.StateDir == "" || options.MaxBytes <= 0 || options.MaxEvents <= 0 || options.MaxCheckpoints <= 0 || options.Retention <= 0 {
		return nil, errors.New("invalid outbox options")
	}
	if err := os.MkdirAll(options.StateDir, 0o700); err != nil {
		return nil, errors.New("cannot create STATE_DIR")
	}
	if err := os.Chmod(options.StateDir, 0o700); err != nil {
		return nil, errors.New("cannot protect STATE_DIR")
	}
	path := filepath.Join(options.StateDir, "state.db")
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, errors.New("cannot open persistent state")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, errors.New("cannot protect persistent state")
	}
	store := &Store{db: db, maxBytes: options.MaxBytes, maxEvents: options.MaxEvents, maxCheckpoints: options.MaxCheckpoints, retention: options.Retention}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketMeta, bucketOutbox, bucketCheckpoints} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, errors.New("cannot initialize persistent state")
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Enqueue(event alerts.EncodedEvent, marker string, observedAt time.Time) (bool, error) {
	inserted := false
	err := s.db.Update(func(tx *bolt.Tx) error {
		outbox := tx.Bucket(bucketOutbox)
		checkpoints := tx.Bucket(bucketCheckpoints)
		meta := tx.Bucket(bucketMeta)
		checkpoint, err := readCheckpoint(checkpoints.Get([]byte(event.Event.AlertID)))
		if err != nil {
			return err
		}
		if checkpoint.AlertID == "" {
			if checkpoints.Stats().KeyN >= s.maxCheckpoints {
				return FullError{}
			}
			checkpoint.AlertID = event.Event.AlertID
		}
		checkpoint.LastSeenAt = observedAt.UnixNano()
		checkpoint.LastFetchedAt = observedAt.UnixNano()
		checkpoint.ListMarker = marker
		checkpoint.AbsentSince = 0
		if checkpoint.PendingHash == event.ContentHash ||
			(checkpoint.PendingHash == "" && checkpoint.DeliveredHash == event.ContentHash) {
			return putCheckpoint(checkpoints, checkpoint)
		}
		count := int(readUint64(meta.Get(keyEventCount)))
		bytesUsed := int64(readUint64(meta.Get(keyEventBytes)))
		record := Record{
			AlertID: event.Event.AlertID, RecordKey: append([]byte(nil), event.RecordKey...),
			Payload: append([]byte(nil), event.Payload...), ContentHash: event.ContentHash,
			CreatedAt: observedAt.UnixNano(),
		}
		sequence, err := outbox.NextSequence()
		if err != nil {
			return err
		}
		record.Sequence = sequence
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		encodedSize := int64(len(encoded))
		if count >= s.maxEvents || bytesUsed+encodedSize > s.maxBytes {
			return FullError{}
		}
		if err := outbox.Put(uint64Key(sequence), encoded); err != nil {
			return err
		}
		checkpoint.PendingHash = event.ContentHash
		if err := putCheckpoint(checkpoints, checkpoint); err != nil {
			return err
		}
		if err := meta.Put(keyEventCount, uint64Value(uint64(count+1))); err != nil {
			return err
		}
		if err := meta.Put(keyEventBytes, uint64Value(uint64(bytesUsed+encodedSize))); err != nil {
			return err
		}
		inserted = true
		return nil
	})
	return inserted, err
}

func (s *Store) MarkSeen(alertID, marker string, observedAt time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketCheckpoints)
		checkpoint, err := readCheckpoint(bucket.Get([]byte(alertID)))
		if err != nil {
			return err
		}
		if checkpoint.AlertID == "" {
			if bucket.Stats().KeyN >= s.maxCheckpoints {
				return FullError{}
			}
			checkpoint.AlertID = alertID
		}
		checkpoint.ListMarker = marker
		checkpoint.LastSeenAt = observedAt.UnixNano()
		checkpoint.AbsentSince = 0
		return putCheckpoint(bucket, checkpoint)
	})
}

func (s *Store) Checkpoint(alertID string) (Checkpoint, bool, error) {
	var result Checkpoint
	err := s.db.View(func(tx *bolt.Tx) error {
		var err error
		result, err = readCheckpoint(tx.Bucket(bucketCheckpoints).Get([]byte(alertID)))
		return err
	})
	return result, result.AlertID != "", err
}

func (s *Store) FetchState(alertID string) (bool, string, time.Time, error) {
	checkpoint, exists, err := s.Checkpoint(alertID)
	if err != nil || !exists {
		return exists, "", time.Time{}, err
	}
	var lastFetched time.Time
	if checkpoint.LastFetchedAt != 0 {
		lastFetched = time.Unix(0, checkpoint.LastFetchedAt)
	}
	return true, checkpoint.ListMarker, lastFetched, nil
}

func (s *Store) Records(limit int) ([]Record, error) {
	result := make([]Record, 0, limit)
	err := s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketOutbox).Cursor()
		for key, value := cursor.First(); key != nil && len(result) < limit; key, value = cursor.Next() {
			var record Record
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			result = append(result, record)
		}
		return nil
	})
	return result, err
}

func (s *Store) Acknowledge(sequence uint64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		outbox := tx.Bucket(bucketOutbox)
		value := outbox.Get(uint64Key(sequence))
		if value == nil {
			return nil
		}
		var record Record
		if err := json.Unmarshal(value, &record); err != nil {
			return err
		}
		checkpointBucket := tx.Bucket(bucketCheckpoints)
		checkpoint, err := readCheckpoint(checkpointBucket.Get([]byte(record.AlertID)))
		if err != nil {
			return err
		}
		checkpoint.DeliveredHash = record.ContentHash
		if checkpoint.PendingHash == record.ContentHash {
			checkpoint.PendingHash = ""
		}
		if err := putCheckpoint(checkpointBucket, checkpoint); err != nil {
			return err
		}
		meta := tx.Bucket(bucketMeta)
		count := readUint64(meta.Get(keyEventCount))
		bytesUsed := readUint64(meta.Get(keyEventBytes))
		encodedSize := uint64(len(value))
		if err := outbox.Delete(uint64Key(sequence)); err != nil {
			return err
		}
		if count > 0 {
			count--
		}
		if encodedSize > bytesUsed {
			bytesUsed = 0
		} else {
			bytesUsed -= encodedSize
		}
		if err := meta.Put(keyEventCount, uint64Value(count)); err != nil {
			return err
		}
		return meta.Put(keyEventBytes, uint64Value(bytesUsed))
	})
}

func (s *Store) Stats() (Stats, error) {
	var result Stats
	err := s.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket(bucketMeta)
		result.Events = int(readUint64(meta.Get(keyEventCount)))
		result.Bytes = int64(readUint64(meta.Get(keyEventBytes)))
		result.Full = result.Events >= s.maxEvents || result.Bytes >= s.maxBytes
		return nil
	})
	return result, err
}

func (s *Store) Health() (int, bool, error) {
	stats, err := s.Stats()
	return stats.Events, stats.Full, err
}

func (s *Store) ReconcileListed(listed map[string]struct{}, now time.Time, complete bool) error {
	if !complete {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketCheckpoints)
		var deleteKeys [][]byte
		type absent struct {
			key []byte
			at  int64
		}
		var absentRows []absent
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			checkpoint, err := readCheckpoint(value)
			if err != nil {
				return err
			}
			if _, ok := listed[checkpoint.AlertID]; ok {
				continue
			}
			if checkpoint.AbsentSince == 0 {
				checkpoint.AbsentSince = now.UnixNano()
				if err := putCheckpoint(bucket, checkpoint); err != nil {
					return err
				}
			}
			if now.Sub(time.Unix(0, checkpoint.AbsentSince)) >= s.retention && checkpoint.PendingHash == "" {
				deleteKeys = append(deleteKeys, append([]byte(nil), key...))
			} else {
				absentRows = append(absentRows, absent{key: append([]byte(nil), key...), at: checkpoint.AbsentSince})
			}
		}
		for _, key := range deleteKeys {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
		count := bucket.Stats().KeyN
		if count <= s.maxCheckpoints {
			return nil
		}
		sort.Slice(absentRows, func(i, j int) bool { return absentRows[i].at < absentRows[j].at })
		for _, row := range absentRows {
			if count <= s.maxCheckpoints {
				break
			}
			if err := bucket.Delete(row.key); err != nil {
				return err
			}
			count--
		}
		if count > s.maxCheckpoints {
			return FullError{}
		}
		return nil
	})
}

func readCheckpoint(value []byte) (Checkpoint, error) {
	if value == nil {
		return Checkpoint{}, nil
	}
	var result Checkpoint
	if err := json.Unmarshal(value, &result); err != nil {
		return Checkpoint{}, fmt.Errorf("decode checkpoint: %w", err)
	}
	return result, nil
}

func putCheckpoint(bucket *bolt.Bucket, checkpoint Checkpoint) error {
	encoded, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return bucket.Put([]byte(checkpoint.AlertID), encoded)
}

func uint64Key(value uint64) []byte { return uint64Value(value) }
func uint64Value(value uint64) []byte {
	result := make([]byte, 8)
	binary.BigEndian.PutUint64(result, value)
	return result
}
func readUint64(value []byte) uint64 {
	if len(value) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(value)
}
