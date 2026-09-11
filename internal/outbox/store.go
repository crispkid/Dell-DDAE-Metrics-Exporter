package outbox

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

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
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

func (FullError) Error() string                     { return "durable outbox is full" }
func (FullError) FailureClass() observability.Class { return observability.ClassBufferFull }

type CorruptionError struct{ reason string }

func (e CorruptionError) Error() string {
	if e.reason == "" {
		return "persistent state is corrupt"
	}
	return "persistent state is corrupt: " + e.reason
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
		canonical, err := json.Marshal(record)
		if err != nil || !bytes.Equal(canonical, value) || record.Sequence != sequence {
			return CorruptionError{reason: "invalid outbox record"}
		}
		if err := validateStoredRecord(record); err != nil {
			return CorruptionError{reason: "invalid outbox record"}
		}
		checkpointBucket := tx.Bucket(bucketCheckpoints)
		checkpoint, err := readCheckpoint(checkpointBucket.Get([]byte(record.AlertID)))
		if err != nil {
			return err
		}
		if checkpoint.AlertID == "" || checkpoint.AlertID != record.AlertID {
			return CorruptionError{reason: "outbox record has no matching checkpoint"}
		}
		newestPending := ""
		var newestPendingSequence uint64
		cursor := outbox.Cursor()
		for _, candidate := cursor.First(); candidate != nil; _, candidate = cursor.Next() {
			var queued Record
			if err := json.Unmarshal(candidate, &queued); err != nil {
				return CorruptionError{reason: "invalid outbox record"}
			}
			if queued.AlertID == record.AlertID {
				newestPending = queued.ContentHash
				newestPendingSequence = queued.Sequence
			}
		}
		if checkpoint.PendingHash != newestPending {
			return CorruptionError{reason: "checkpoint pending hash mismatch"}
		}
		checkpoint.DeliveredHash = record.ContentHash
		if sequence == newestPendingSequence {
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
		result.Full = result.Events >= s.maxEvents || result.Bytes >= s.maxBytes || tx.Bucket(bucketCheckpoints).Stats().KeyN >= s.maxCheckpoints
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
		count := 0
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			count++
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
			}
		}
		for _, key := range deleteKeys {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
		// Bucket.Stats may still describe the pre-delete pages in this transaction.
		full = count-len(deleteKeys) >= s.maxCheckpoints
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

	count, bytesUsed, newestPending, recordsByAlert, err := validateRecords(outbox)
	if err != nil {
		return err
	}
	if err := validateCheckpoints(checkpoints, newestPending, recordsByAlert); err != nil {
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
	var count uint64
	var bytesUsed uint64
	newestPending := make(map[string]string)
	recordsByAlert := make(map[string]bool)
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
		if err != nil || !bytes.Equal(canonical, value) {
			return 0, 0, nil, nil, CorruptionError{reason: "non-canonical outbox record"}
		}
		if record.Sequence != sequence || validateStoredRecord(record) != nil {
			return 0, 0, nil, nil, CorruptionError{reason: "outbox record invariant failed"}
		}
		count++
		if uint64(len(value)) > ^uint64(0)-bytesUsed {
			return 0, 0, nil, nil, CorruptionError{reason: "outbox byte count overflow"}
		}
		bytesUsed += uint64(len(value))
		newestPending[record.AlertID] = record.ContentHash
		recordsByAlert[record.AlertID] = true
	}
	return count, bytesUsed, newestPending, recordsByAlert, nil
}

func validateStoredRecord(record Record) error {
	if record.Sequence == 0 || record.CreatedAt <= 0 || ddaeAlertIDInvalid(record.AlertID) || !validHash(record.ContentHash) {
		return errors.New("invalid record identity")
	}
	if len(record.Payload) == 0 || len(record.Payload) > alerts.MaxEventBytes || len(record.RecordKey) != 64 || !validHash(string(record.RecordKey)) {
		return errors.New("invalid record payload")
	}
	var event alerts.Event
	if err := json.Unmarshal(record.Payload, &event); err != nil {
		return err
	}
	canonicalEvent, err := json.Marshal(event)
	if err != nil || !bytes.Equal(canonicalEvent, record.Payload) {
		return errors.New("non-canonical event payload")
	}
	if err := alerts.ValidateStoredEvent(event); err != nil {
		return err
	}
	if event.SchemaVersion != alerts.SchemaVersion || event.EventType != alerts.EventType || event.SourceSystem != alerts.SourceSystem ||
		event.AlertID != record.AlertID || event.ContentHashSHA256 != record.ContentHash {
		return errors.New("payload identity mismatch")
	}
	canonical, err := json.Marshal(event.Alert)
	if err != nil {
		return err
	}
	contentHash := sha256.Sum256(canonical)
	if hex.EncodeToString(contentHash[:]) != record.ContentHash {
		return errors.New("payload content hash mismatch")
	}
	keyHash := sha256.New()
	_, _ = keyHash.Write([]byte(event.SourceInstance))
	_, _ = keyHash.Write([]byte{0})
	_, _ = keyHash.Write([]byte(event.AlertID))
	if !bytes.Equal(record.RecordKey, []byte(hex.EncodeToString(keyHash.Sum(nil)))) {
		return errors.New("record key mismatch")
	}
	return nil
}

func validateCheckpoints(bucket *bolt.Bucket, newestPending map[string]string, recordsByAlert map[string]bool) error {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		checkpoint, err := readCheckpoint(value)
		if err != nil {
			return CorruptionError{reason: "invalid checkpoint"}
		}
		if checkpoint.AlertID == "" || string(key) != checkpoint.AlertID || ddaeAlertIDInvalid(checkpoint.AlertID) {
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
		if checkpoint.PendingHash != newestPending[checkpoint.AlertID] {
			return CorruptionError{reason: "checkpoint pending hash mismatch"}
		}
		delete(recordsByAlert, checkpoint.AlertID)
	}
	if len(recordsByAlert) != 0 {
		return CorruptionError{reason: "outbox record has no checkpoint"}
	}
	return nil
}

func validHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, ch := range value {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	return true
}

func ddaeAlertIDInvalid(id string) bool {
	if len(id) < 1 || len(id) > 256 || id == "." || id == ".." {
		return true
	}
	for _, ch := range id {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '.' || ch == '_' || ch == ':' || ch == '-') {
			return true
		}
	}
	return false
}
