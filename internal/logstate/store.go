package logstate

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/serviceability"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketMeta        = []byte("meta-v1")
	bucketOutbox      = []byte("outbox-v1")
	bucketCheckpoints = []byte("checkpoints-v1")
	keySchemaVersion  = []byte("schema-version")
	keyEventCount     = []byte("event-count")
	keyEventBytes     = []byte("event-bytes")
	currentSchema     = []byte("1")
)

type FullError struct{}

func (FullError) Error() string                     { return "serviceability log outbox is full" }
func (FullError) FailureClass() observability.Class { return observability.ClassBufferFull }

type CorruptionError struct{ reason string }

func (e CorruptionError) Error() string {
	if e.reason == "" {
		return "serviceability log state is corrupt"
	}
	return "serviceability log state is corrupt: " + e.reason
}
func (CorruptionError) FailureClass() observability.Class { return observability.ClassInternal }

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
	LogID       string `json:"log_id"`
	RecordKey   []byte `json:"record_key"`
	Payload     []byte `json:"payload"`
	ContentHash string `json:"content_hash"`
	CreatedAt   int64  `json:"created_at_unix_nano"`
}

type Checkpoint struct {
	LogID         string `json:"log_id"`
	ListMarker    string `json:"list_marker,omitempty"`
	PendingHash   string `json:"pending_hash,omitempty"`
	DeliveredHash string `json:"delivered_hash,omitempty"`
	LastFetchedAt int64  `json:"last_fetched_at_unix_nano,omitempty"`
	LastSeenAt    int64  `json:"last_seen_at_unix_nano,omitempty"`
	AbsentSince   int64  `json:"absent_since_unix_nano,omitempty"`
}

type Stats struct {
	Events      int
	Bytes       int64
	Checkpoints int
	Full        bool
}

func Open(options Options) (*Store, error) {
	if options.StateDir == "" || options.MaxBytes <= 0 || options.MaxEvents <= 0 || options.MaxCheckpoints <= 0 || options.Retention <= 0 {
		return nil, errors.New("invalid serviceability log state options")
	}
	if err := os.MkdirAll(options.StateDir, 0o700); err != nil {
		return nil, errors.New("cannot create STATE_DIR")
	}
	if err := os.Chmod(options.StateDir, 0o700); err != nil {
		return nil, errors.New("cannot protect STATE_DIR")
	}
	path := filepath.Join(options.StateDir, "serviceability-logs.db")
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, errors.New("cannot open serviceability log state")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, errors.New("cannot protect serviceability log state")
	}
	store := &Store{db: db, maxBytes: options.MaxBytes, maxEvents: options.MaxEvents, maxCheckpoints: options.MaxCheckpoints, retention: options.Retention}
	if err := db.Update(func(tx *bolt.Tx) error {
		present := 0
		for _, name := range [][]byte{bucketMeta, bucketOutbox, bucketCheckpoints} {
			if tx.Bucket(name) != nil {
				present++
			}
		}
		if present != 0 && present != 3 {
			return CorruptionError{reason: "required bucket is missing"}
		}
		if present == 0 {
			for _, name := range [][]byte{bucketMeta, bucketOutbox, bucketCheckpoints} {
				if _, err := tx.CreateBucket(name); err != nil {
					return err
				}
			}
		}
		return validateAndMigrate(tx)
	}); err != nil {
		_ = db.Close()
		var classified observability.Classified
		if errors.As(err, &classified) {
			return nil, err
		}
		return nil, errors.New("cannot initialize serviceability log state")
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Enqueue(event serviceability.EncodedEvent, marker string, observedAt time.Time) (bool, error) {
	inserted := false
	err := s.db.Update(func(tx *bolt.Tx) error {
		outbox := tx.Bucket(bucketOutbox)
		checkpoints := tx.Bucket(bucketCheckpoints)
		meta := tx.Bucket(bucketMeta)
		checkpoint, err := readCheckpoint(checkpoints.Get([]byte(event.Event.LogID)))
		if err != nil {
			return err
		}
		if checkpoint.LogID == "" {
			if checkpoints.Stats().KeyN >= s.maxCheckpoints {
				return FullError{}
			}
			checkpoint.LogID = event.Event.LogID
		}
		checkpoint.LastSeenAt = observedAt.UnixNano()
		checkpoint.LastFetchedAt = observedAt.UnixNano()
		checkpoint.ListMarker = marker
		checkpoint.AbsentSince = 0
		if checkpoint.PendingHash == event.ContentHash || (checkpoint.PendingHash == "" && checkpoint.DeliveredHash == event.ContentHash) {
			return putCheckpoint(checkpoints, checkpoint)
		}
		count := int(readUint64(meta.Get(keyEventCount)))
		bytesUsed := int64(readUint64(meta.Get(keyEventBytes)))
		sequence, err := outbox.NextSequence()
		if err != nil {
			return err
		}
		record := Record{
			Sequence: sequence, LogID: event.Event.LogID,
			RecordKey: append([]byte(nil), event.RecordKey...), Payload: append([]byte(nil), event.Payload...),
			ContentHash: event.ContentHash, CreatedAt: observedAt.UnixNano(),
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		encodedSize := int64(len(encoded))
		if count >= s.maxEvents || encodedSize > s.maxBytes-bytesUsed {
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

func (s *Store) MarkSeen(logID, marker string, observedAt time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketCheckpoints)
		checkpoint, err := readCheckpoint(bucket.Get([]byte(logID)))
		if err != nil {
			return err
		}
		if checkpoint.LogID == "" {
			if bucket.Stats().KeyN >= s.maxCheckpoints {
				return FullError{}
			}
			checkpoint.LogID = logID
		}
		checkpoint.ListMarker = marker
		checkpoint.LastSeenAt = observedAt.UnixNano()
		checkpoint.AbsentSince = 0
		return putCheckpoint(bucket, checkpoint)
	})
}

func (s *Store) Checkpoint(logID string) (Checkpoint, bool, error) {
	var result Checkpoint
	err := s.db.View(func(tx *bolt.Tx) error {
		var err error
		result, err = readCheckpoint(tx.Bucket(bucketCheckpoints).Get([]byte(logID)))
		return err
	})
	return result, result.LogID != "", err
}

func (s *Store) FetchState(logID string) (bool, string, time.Time, error) {
	checkpoint, exists, err := s.Checkpoint(logID)
	if err != nil || !exists {
		return exists, "", time.Time{}, err
	}
	if checkpoint.LastFetchedAt == 0 {
		return true, checkpoint.ListMarker, time.Time{}, nil
	}
	return true, checkpoint.ListMarker, time.Unix(0, checkpoint.LastFetchedAt), nil
}

func (s *Store) Records(limit int) ([]Record, error) {
	if limit < 1 {
		return nil, errors.New("record limit must be positive")
	}
	result := make([]Record, 0, limit)
	err := s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketOutbox).Cursor()
		for key, value := cursor.First(); key != nil && len(result) < limit; key, value = cursor.Next() {
			var record Record
			if err := json.Unmarshal(value, &record); err != nil {
				return CorruptionError{reason: "invalid outbox record"}
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
			return CorruptionError{reason: "invalid outbox record"}
		}
		canonical, err := json.Marshal(record)
		if err != nil || !bytes.Equal(canonical, value) || record.Sequence != sequence || validateStoredRecord(record) != nil {
			return CorruptionError{reason: "invalid outbox record"}
		}
		checkpoints := tx.Bucket(bucketCheckpoints)
		checkpoint, err := readCheckpoint(checkpoints.Get([]byte(record.LogID)))
		if err != nil {
			return err
		}
		if checkpoint.LogID == "" || checkpoint.LogID != record.LogID {
			return CorruptionError{reason: "outbox record has no matching checkpoint"}
		}
		newestPending := ""
		var newestSequence uint64
		cursor := outbox.Cursor()
		for _, candidate := cursor.First(); candidate != nil; _, candidate = cursor.Next() {
			var queued Record
			if err := json.Unmarshal(candidate, &queued); err != nil {
				return CorruptionError{reason: "invalid outbox record"}
			}
			if queued.LogID == record.LogID {
				newestPending = queued.ContentHash
				newestSequence = queued.Sequence
			}
		}
		if checkpoint.PendingHash != newestPending {
			return CorruptionError{reason: "checkpoint pending hash mismatch"}
		}
		checkpoint.DeliveredHash = record.ContentHash
		if newestSequence == record.Sequence {
			checkpoint.PendingHash = ""
		}
		if err := putCheckpoint(checkpoints, checkpoint); err != nil {
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
			return CorruptionError{reason: "derived byte count underflow"}
		}
		bytesUsed -= encodedSize
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
		checkpoints := tx.Bucket(bucketCheckpoints)
		if meta == nil || checkpoints == nil {
			return CorruptionError{reason: "required bucket is missing"}
		}
		result.Events = int(readUint64(meta.Get(keyEventCount)))
		result.Bytes = int64(readUint64(meta.Get(keyEventBytes)))
		result.Checkpoints = checkpoints.Stats().KeyN
		result.Full = result.Events >= s.maxEvents || result.Bytes >= s.maxBytes || result.Checkpoints >= s.maxCheckpoints
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
	full := false
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketCheckpoints)
		var deleteKeys [][]byte
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			checkpoint, err := readCheckpoint(value)
			if err != nil {
				return err
			}
			if _, exists := listed[checkpoint.LogID]; exists {
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
			}
		}
		for _, key := range deleteKeys {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
		full = bucket.Stats().KeyN >= s.maxCheckpoints
		return nil
	})
	if err != nil {
		return err
	}
	if full {
		return FullError{}
	}
	return nil
}

func validateAndMigrate(tx *bolt.Tx) error {
	meta := tx.Bucket(bucketMeta)
	outbox := tx.Bucket(bucketOutbox)
	checkpoints := tx.Bucket(bucketCheckpoints)
	if meta == nil || outbox == nil || checkpoints == nil {
		return CorruptionError{reason: "required bucket is missing"}
	}
	version := meta.Get(keySchemaVersion)
	if version != nil && !bytes.Equal(version, currentSchema) {
		return CorruptionError{reason: "unsupported schema version"}
	}
	count, bytesUsed, newestPending, recordsByLog, err := validateRecords(outbox)
	if err != nil {
		return err
	}
	if err := validateCheckpoints(checkpoints, newestPending, recordsByLog); err != nil {
		return err
	}
	if err := meta.Put(keyEventCount, uint64Value(count)); err != nil {
		return err
	}
	if err := meta.Put(keyEventBytes, uint64Value(bytesUsed)); err != nil {
		return err
	}
	return meta.Put(keySchemaVersion, currentSchema)
}

func validateRecords(bucket *bolt.Bucket) (uint64, uint64, map[string]string, map[string]bool, error) {
	var count, bytesUsed uint64
	newestPending := make(map[string]string)
	recordsByLog := make(map[string]bool)
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		if len(key) != 8 {
			return 0, 0, nil, nil, CorruptionError{reason: "invalid outbox key"}
		}
		sequence := binary.BigEndian.Uint64(key)
		if sequence == 0 {
			return 0, 0, nil, nil, CorruptionError{reason: "invalid outbox sequence"}
		}
		var record Record
		if err := json.Unmarshal(value, &record); err != nil {
			return 0, 0, nil, nil, CorruptionError{reason: "invalid outbox record"}
		}
		canonical, err := json.Marshal(record)
		if err != nil || !bytes.Equal(canonical, value) || record.Sequence != sequence || validateStoredRecord(record) != nil {
			return 0, 0, nil, nil, CorruptionError{reason: "outbox record invariant failed"}
		}
		count++
		if uint64(len(value)) > ^uint64(0)-bytesUsed {
			return 0, 0, nil, nil, CorruptionError{reason: "outbox byte count overflow"}
		}
		bytesUsed += uint64(len(value))
		newestPending[record.LogID] = record.ContentHash
		recordsByLog[record.LogID] = true
	}
	return count, bytesUsed, newestPending, recordsByLog, nil
}

func validateStoredRecord(record Record) error {
	if record.Sequence == 0 || record.CreatedAt <= 0 || ddae.ValidateServiceabilityLogID(record.LogID) != nil || !validHash(record.ContentHash) {
		return errors.New("invalid record identity")
	}
	if len(record.RecordKey) != 64 || !validHash(string(record.RecordKey)) {
		return errors.New("invalid record key")
	}
	event, err := serviceability.DecodeStoredEvent(record.Payload)
	if err != nil {
		return err
	}
	if event.LogID != record.LogID || event.ContentHashSHA256 != record.ContentHash {
		return errors.New("payload identity mismatch")
	}
	canonical, err := json.Marshal(event.Log)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(canonical)
	if hex.EncodeToString(hash[:]) != record.ContentHash {
		return errors.New("payload content hash mismatch")
	}
	keyHash := sha256.New()
	_, _ = keyHash.Write([]byte(event.SourceInstance))
	_, _ = keyHash.Write([]byte{0})
	_, _ = keyHash.Write([]byte(serviceability.RecordKind))
	_, _ = keyHash.Write([]byte{0})
	_, _ = keyHash.Write([]byte(event.LogID))
	if !bytes.Equal(record.RecordKey, []byte(hex.EncodeToString(keyHash.Sum(nil)))) {
		return errors.New("record key mismatch")
	}
	return nil
}

func validateCheckpoints(bucket *bolt.Bucket, newestPending map[string]string, recordsByLog map[string]bool) error {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		checkpoint, err := readCheckpoint(value)
		if err != nil {
			return CorruptionError{reason: "invalid checkpoint"}
		}
		if checkpoint.LogID == "" || string(key) != checkpoint.LogID || ddae.ValidateServiceabilityLogID(checkpoint.LogID) != nil {
			return CorruptionError{reason: "checkpoint identity mismatch"}
		}
		canonical, err := json.Marshal(checkpoint)
		if err != nil || !bytes.Equal(canonical, value) {
			return CorruptionError{reason: "non-canonical checkpoint"}
		}
		for _, hash := range []string{checkpoint.PendingHash, checkpoint.DeliveredHash} {
			if hash != "" && !validHash(hash) {
				return CorruptionError{reason: "invalid checkpoint hash"}
			}
		}
		for _, timestamp := range []int64{checkpoint.LastFetchedAt, checkpoint.LastSeenAt, checkpoint.AbsentSince} {
			if timestamp < 0 {
				return CorruptionError{reason: "invalid checkpoint timestamp"}
			}
		}
		if checkpoint.PendingHash != newestPending[checkpoint.LogID] {
			return CorruptionError{reason: "checkpoint pending hash mismatch"}
		}
		delete(recordsByLog, checkpoint.LogID)
	}
	if len(recordsByLog) != 0 {
		return CorruptionError{reason: "outbox record has no checkpoint"}
	}
	return nil
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
	return bucket.Put([]byte(checkpoint.LogID), encoded)
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

func validHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}
