package querystate

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/queryclient"
	bolt "go.etcd.io/bbolt"
)

func auditOptions(t *testing.T) Options {
	t.Helper()
	return Options{Dir: t.TempDir(), Source: "test", MaxEvents: 20, MaxCheckpoints: 20, MaxBytes: 1 << 20, Retention: time.Hour, Events: true}
}
func auditEvent() queryclient.Event {
	now := time.Now().UTC()
	elapsed := 1.0
	return queryclient.Event{SourceInstance: "test", QueryID: "q1", User: "synthetic", State: "finished", Submitted: now.Add(-time.Second), Completed: &now, Observed: now.Add(-10 * time.Second), Elapsed: &elapsed}
}
func auditPutStats(tx *bolt.Tx, mutate func(*Stats)) error {
	var a Stats
	if err := json.Unmarshal(tx.Bucket(meta).Get([]byte("stats")), &a); err != nil {
		return err
	}
	mutate(&a)
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return tx.Bucket(meta).Put([]byte("stats"), b)
}
func auditEditPayload(tx *bolt.Tx, edit func([]byte) []byte) error {
	b := tx.Bucket(events)
	k, v := b.Cursor().First()
	before := len(v)
	after := edit(append([]byte(nil), v...))
	if err := b.Put(k, after); err != nil {
		return err
	}
	return auditPutStats(tx, func(a *Stats) { a.Bytes += int64(len(after) - before) })
}

// TEST-DDAE-12-005: existing state failures must not initialize or prune the DB.
func TestAuditRejectsCorruptQueryStateWithoutWriting(t *testing.T) {
	mutations := map[string]func(*bolt.Tx) error{
		"missing-final-checkpoint": func(tx *bolt.Tx) error { return tx.Bucket(checkpoints).Delete([]byte("q1")) },
		"missing-meta":             func(tx *bolt.Tx) error { return tx.DeleteBucket(meta) },
		"missing-checkpoints":      func(tx *bolt.Tx) error { return tx.DeleteBucket(checkpoints) },
		"missing-events":           func(tx *bolt.Tx) error { return tx.DeleteBucket(events) },
		"future-version":           func(tx *bolt.Tx) error { return tx.Bucket(meta).Put([]byte("version"), []byte("2")) },
		"wrong-source":             func(tx *bolt.Tx) error { return tx.Bucket(meta).Put([]byte("source"), []byte("other")) },
		"count":                    func(tx *bolt.Tx) error { return auditPutStats(tx, func(a *Stats) { a.Pending++ }) },
		"bytes":                    func(tx *bolt.Tx) error { return auditPutStats(tx, func(a *Stats) { a.Bytes++ }) },
		"unknown-counter": func(tx *bolt.Tx) error {
			return auditPutStats(tx, func(a *Stats) { a.Completed["unbounded-label"] = 1 })
		},
		"bad-histogram": func(tx *bolt.Tx) error {
			return auditPutStats(tx, func(a *Stats) {
				h := a.Histograms["elapsed:finished"]
				h.Buckets[0] = h.Count + 1
				a.Histograms["elapsed:finished"] = h
			})
		},
		"missing-checkpoint-fields": func(tx *bolt.Tx) error { return tx.Bucket(checkpoints).Put([]byte("q1"), []byte(`{}`)) },
		"wrong-final-hash": func(tx *bolt.Tx) error {
			var cp checkpoint
			json.Unmarshal(tx.Bucket(checkpoints).Get([]byte("q1")), &cp)
			cp.Hash[0] ^= 1
			b, _ := json.Marshal(cp)
			return tx.Bucket(checkpoints).Put([]byte("q1"), b)
		},
		"sequence-behind-key": func(tx *bolt.Tx) error { return tx.Bucket(events).SetSequence(0) },
		"zero-key": func(tx *bolt.Tx) error {
			b := tx.Bucket(events)
			k, v := b.Cursor().First()
			v = append([]byte(nil), v...)
			if err := b.Delete(k); err != nil {
				return err
			}
			return b.Put(make([]byte, 8), v)
		},
		"extra-SQL-field": func(tx *bolt.Tx) error {
			return auditEditPayload(tx, func(b []byte) []byte { return append([]byte(`{"queryText":"SQL_CANARY",`), b[1:]...) })
		},
		"duplicate-key": func(tx *bolt.Tx) error {
			return auditEditPayload(tx, func(b []byte) []byte { return append([]byte(`{"query_id":"q1",`), b[1:]...) })
		},
		"event-source": func(tx *bolt.Tx) error {
			return auditEditPayload(tx, func(b []byte) []byte {
				return bytes.Replace(b, []byte(`"source_instance":"test"`), []byte(`"source_instance":"other"`), 1)
			})
		},
		"event-user-null": func(tx *bolt.Tx) error {
			return auditEditPayload(tx, func(b []byte) []byte { return bytes.Replace(b, []byte(`"user":"synthetic"`), []byte(`"user":null`), 1) })
		},
		"event-state": func(tx *bolt.Tx) error {
			return auditEditPayload(tx, func(b []byte) []byte {
				return bytes.Replace(b, []byte(`"state":"finished"`), []byte(`"state":"arbitrary"`), 1)
			})
		},
		"trailing-json": func(tx *bolt.Tx) error {
			return auditEditPayload(tx, func(b []byte) []byte { return append(b, []byte(` null`)...) })
		},
		"invalid-utf8": func(tx *bolt.Tx) error {
			return auditEditPayload(tx, func(b []byte) []byte { return bytes.Replace(b, []byte("synthetic"), []byte{0xff}, 1) })
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			o := auditOptions(t)
			s, err := Open(o)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Record(auditEvent()); err != nil {
				t.Fatal(err)
			}
			if err = s.db.Update(mutate); err != nil {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(o.Dir, "query-events.db")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(o)
			if err == nil {
				reopened.Close()
				t.Error("corrupt state accepted")
			}
			if err != nil && strings.Contains(err.Error(), "SQL_CANARY") {
				t.Error("error leaked payload")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Error("failed open changed database bytes")
			}
		})
	}
}

func TestAuditExistingEmptyAndTruncatedQueryFilesArePreserved(t *testing.T) {
	for _, data := range [][]byte{nil, make([]byte, 128)} {
		o := auditOptions(t)
		path := filepath.Join(o.Dir, "query-events.db")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		s, err := Open(o)
		if err == nil {
			s.Close()
			t.Error("existing invalid file initialized")
		}
		after, e := os.ReadFile(path)
		if e != nil || !bytes.Equal(after, data) {
			t.Fatal("invalid file modified", e)
		}
	}
}

func TestAuditQueryRetentionAndHistoricalPendingCompatibility(t *testing.T) {
	o := auditOptions(t)
	s, err := Open(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s != nil {
			s.Close()
		}
	})
	e := auditEvent()
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	if err = s.Prune(e.Completed.Add(2 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(o)
	if err != nil {
		t.Fatal("legal pruned pending rejected", err)
	}
	a, err := s.Stats()
	if err != nil || a.Completed["finished"] != 1 || a.Pending != 1 {
		t.Fatal("lost cumulative state", err)
	}
	rows, err := s.Records(10)
	if err != nil || len(rows) != 1 {
		t.Fatal("lost expired pending", err)
	}
	if err = s.Ack(rows[0].Sequence); err != nil {
		t.Fatal(err)
	}
	if err = s.Record(e); !errors.Is(err, ErrExpired) {
		t.Fatal("recounted expired record", err)
	}
}

func TestAuditQueryMultipleHistoricalStatesAndMetricsOnlyUpdate(t *testing.T) {
	o := auditOptions(t)
	s, err := Open(o)
	if err != nil {
		t.Fatal(err)
	}
	e := auditEvent()
	completion := *e.Completed
	e.State = "running"
	e.Elapsed = nil
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	// Nonterminal completion is optional; removing it can move retention time earlier.
	e.Completed = nil
	e.State = "queued"
	e.Observed = e.Observed.Add(time.Second)
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	o.Events = false
	s, err = Open(o)
	if err != nil {
		t.Fatal(err)
	}
	e.State = "planning"
	e.Observed = e.Observed.Add(time.Second)
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	if err = s.Prune(completion.Add(time.Hour - 500*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(o)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.Records(10)
	if err != nil || len(rows) != 2 {
		t.Fatal("historical hashes rejected", err)
	}
}

func TestAuditQueryStrictSchemaBoundaries(t *testing.T) {
	e := auditEvent()
	raw, _ := json.Marshal(e)
	for _, body := range [][]byte{
		[]byte(`null`), []byte(`[]`), []byte(`{"query_id":`),
		bytes.Replace(raw, []byte(`"query_id"`), []byte(`"QUERY_ID"`), 1),
		bytes.Replace(raw, []byte(`"user":"synthetic"`), []byte(`"user":{"nested":{"x":1,"x":2}}`), 1),
		append([]byte(`{"extra":`+strings.Repeat("[", 18)+`0`+strings.Repeat("]", 18)+`,`), raw[1:]...),
		bytes.Repeat([]byte(" "), maxEventBytes+1),
	} {
		if _, err := decodeEvent(body, "test"); err == nil {
			t.Error("invalid schema accepted")
		}
	}
	for _, state := range []string{"unknown", "queued", "running", "planning", "starting", "finishing", "waiting_for_resources", "dispatching"} {
		e.State = state
		e.Completed = nil
		e.Elapsed = nil
		raw, _ := json.Marshal(e)
		if _, err := decodeEvent(raw, "test"); err != nil {
			t.Errorf("valid state %s: %v", state, err)
		}
	}
	for _, duration := range []float64{0, maxDurationSeconds, maxDurationSeconds + 1} {
		e.Elapsed = &duration
		err := validateEvent(e, "test")
		if (err == nil) != (duration <= maxDurationSeconds) {
			t.Error("duration bound", duration, err)
		}
	}
	e.Elapsed = nil
	e.User = strings.Repeat("a", 1024)
	if err := validateEvent(e, "test"); err != nil {
		t.Fatal("maximum user rejected")
	}
	e.User += "a"
	if err := validateEvent(e, "test"); err == nil {
		t.Fatal("oversized user accepted")
	}
	e = auditEvent()
	bad := e.Submitted.Add(-time.Second)
	e.Completed = &bad
	if err := validateEvent(e, "test"); err == nil {
		t.Fatal("completion before submission")
	}
}

func TestAuditQueryAggregateAndCheckpointNullsRejected(t *testing.T) {
	s, err := Open(auditOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Record(auditEvent()); err != nil {
		t.Fatal(err)
	}
	err = s.db.View(func(tx *bolt.Tx) error {
		raw := append([]byte(nil), tx.Bucket(meta).Get([]byte("stats"))...)
		for _, b := range [][]byte{
			bytes.Replace(raw, []byte(`"finished":1`), []byte(`"finished":null`), 1),
			bytes.Replace(raw, []byte(`"Buckets":[0`), []byte(`"Buckets":[null`), 1),
			bytes.Replace(raw, []byte(`"elapsed:finished"`), []byte(`"invalid:finished"`), 1),
			bytes.Replace(raw, []byte(`"Count":1`), []byte(`"Count":null`), 1),
		} {
			if bytes.Equal(b, raw) {
				t.Fatal("mutation did not apply")
			}
			if _, err := decodeStats(b); err == nil {
				t.Error("invalid aggregate accepted")
			}
		}
		var fields map[string]json.RawMessage
		cpRaw := tx.Bucket(checkpoints).Get([]byte("q1"))
		json.Unmarshal(cpRaw, &fields)
		for _, badHash := range []string{`[]`, `[null]`, strings.Repeat(`0,`, 32) + `0`} {
			value := badHash
			if strings.HasPrefix(value, "0") {
				value = "[" + value + "]"
			}
			fields["Hash"] = json.RawMessage(value)
			b, _ := json.Marshal(fields)
			if _, err := decodeCheckpoint([]byte("q1"), b); err == nil {
				t.Error("invalid hash length accepted")
			}
		}
		fields["Hash"] = json.RawMessage("[null," + strings.Repeat("0,", 30) + "1]")
		b, _ := json.Marshal(fields)
		if _, err := decodeCheckpoint([]byte("q1"), b); err == nil {
			t.Error("null hash entry accepted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if final, at, err := s.Fetch("q1"); err != nil || !final || at.IsZero() {
		t.Fatal("valid checkpoint fetch", err)
	}
	if final, at, err := s.Fetch("missing"); err != nil || final || !at.IsZero() {
		t.Fatal("absent checkpoint fetch", err)
	}
}

func TestAuditQueryPathAndSequenceBounds(t *testing.T) {
	o := auditOptions(t)
	s, err := Open(o)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.db.Update(func(tx *bolt.Tx) error { return auditPutStats(tx, func(a *Stats) { a.Revision = math.MaxUint64 }) }); err != nil {
		t.Fatal(err)
	}
	if err = s.Prune(time.Now()); err == nil {
		t.Error("revision wrapped")
	}
	s.Close()
	o = auditOptions(t)
	path := filepath.Join(o.Dir, "query-events.db")
	target := filepath.Join(o.Dir, "other")
	if err = os.WriteFile(target, []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(o); err == nil {
		s.Close()
		t.Fatal("symlink opened")
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "preserved" {
		t.Fatal("symlink target changed", err)
	}
}

func TestAuditQueryRecordAndReplayValidation(t *testing.T) {
	for _, kind := range []string{"source", "user", "state", "observed", "submitted", "completed", "duration", "nan", "infinity"} {
		t.Run(kind, func(t *testing.T) {
			s, err := Open(auditOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			e := auditEvent()
			switch kind {
			case "source":
				e.SourceInstance = "other"
			case "user":
				e.User = ""
			case "state":
				e.State = "not-allowed"
			case "observed":
				e.Observed = time.Time{}
			case "submitted":
				e.Submitted = time.Time{}
			case "completed":
				e.Completed = nil
			case "duration":
				v := -1.0
				e.Elapsed = &v
			case "nan":
				v := math.NaN()
				e.Elapsed = &v
			case "infinity":
				v := math.Inf(1)
				e.Elapsed = &v
			}
			if err = s.Record(e); err == nil {
				t.Error("invalid event stored")
			}
			a, err := s.Stats()
			if err != nil || a.Pending != 0 || len(a.Completed) != 0 {
				t.Fatal("invalid write changed state", err)
			}
		})
	}
	s, err := Open(auditOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Record(auditEvent()); err != nil {
		t.Fatal(err)
	}
	if err = s.db.Update(func(tx *bolt.Tx) error {
		return auditEditPayload(tx, func(b []byte) []byte { return append([]byte(`{"queryText":"SQL_CANARY",`), b[1:]...) })
	}); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.Records(10); err == nil || len(rows) != 0 {
		t.Error("polluted payload available for replay")
	}
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, 1)
	if err = s.Ack(1); err == nil {
		t.Error("polluted payload silently acknowledged")
	}
	s.db.View(func(tx *bolt.Tx) error {
		if tx.Bucket(events).Get(key) == nil {
			t.Error("polluted record removed")
		}
		return nil
	})
}
