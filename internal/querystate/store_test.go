package querystate

import (
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/queryclient"
	bolt "go.etcd.io/bbolt"
	"path/filepath"
	"testing"
	"time"
)

func TestQueryAtomicDedupAndBackpressure(t *testing.T) {
	opt := Options{Dir: filepath.Join(t.TempDir(), "state"), Source: "test", MaxEvents: 1, MaxBytes: 1 << 20, MaxCheckpoints: 100, Retention: time.Hour, Events: true}
	s, err := Open(opt)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	d := .008
	e := queryclient.Event{SourceInstance: "test", QueryID: "q1", User: "user", State: "finished", Submitted: now, Completed: &now, Observed: now, Elapsed: &d}
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	e.QueryID = "q2"
	if err = s.Record(e); err == nil {
		t.Fatal("full outbox accepted")
	}
	s.Close()
	s, err = Open(opt)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.Stats()
	if err != nil || a.Completed["finished"] != 1 || a.Pending != 1 {
		t.Fatal(a, err)
	}
	rows, err := s.Records(10)
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	if err = s.Ack(rows[0].Sequence); err != nil {
		t.Fatal(err)
	}
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	a, _ = s.Stats()
	if a.Completed["finished"] != 2 {
		t.Fatal("lost aggregate")
	}
}

func TestQueryRetentionSchemaAndSourceIsolation(t *testing.T) {
	opt := Options{Dir: filepath.Join(t.TempDir(), "state"), Source: "test", MaxEvents: 10, MaxBytes: 1 << 20, MaxCheckpoints: 10, Retention: time.Hour}
	s, err := Open(opt)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	d := 0.0
	e := queryclient.Event{SourceInstance: "test", QueryID: "q", User: "u", State: "finished", Submitted: now, Completed: &now, Observed: now, Elapsed: &d}
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	if err = s.Prune(now.Add(2 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = s.Record(e); err != ErrExpired {
		t.Fatal("expired record counted", err)
	}
	stats, _ := s.Stats()
	if stats.Completed["finished"] != 1 || stats.Pending != 0 {
		t.Fatal(stats)
	}
	s.Close()
	wrong := opt
	wrong.Source = "other"
	if other, err := Open(wrong); err == nil {
		other.Close()
		t.Fatal("source changed")
	}
	s, err = Open(opt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Records(0); err == nil {
		t.Fatal("unbounded batch")
	}
	if err = s.Ack(987); err != nil {
		t.Fatal(err)
	}
	if err = s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(meta).Put([]byte("version"), []byte("future")) }); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if other, err := Open(opt); err == nil {
		other.Close()
		t.Fatal("future schema opened")
	}
}
func TestQueryCheckpointCapacityAndStateUpdates(t *testing.T) {
	s, err := Open(Options{Dir: filepath.Join(t.TempDir(), "state"), Source: "test", MaxEvents: 10, MaxBytes: 1 << 20, MaxCheckpoints: 1, Retention: time.Hour, Events: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	e := queryclient.Event{SourceInstance: "test", QueryID: "q", User: "u", State: "queued", Submitted: now, Observed: now}
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	e.Observed = now.Add(time.Second)
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.Records(10)
	if len(rows) != 1 {
		t.Fatal("unchanged state re-enqueued")
	}
	e.State = "running"
	if err = s.Record(e); err != nil {
		t.Fatal(err)
	}
	e.QueryID = "q2"
	if err = s.Record(e); err != ErrFull {
		t.Fatal("checkpoint cap", err)
	}
	a, _ := s.Stats()
	if a.Pending != 2 || len(a.Completed) != 0 {
		t.Fatal(a)
	}
}

// TEST-DDAE-10-005: an older in-flight observation cannot roll back newer nonterminal state.
func TestBackfillOlderQueryObservation(t *testing.T) {
	s, e := Open(Options{Dir: t.TempDir(), Source: "test", MaxEvents: 10, MaxBytes: 1 << 20, MaxCheckpoints: 10, Retention: time.Hour, Events: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	event := queryclient.Event{SourceInstance: "test", QueryID: "q1", User: "synthetic", State: "running", Submitted: now.Add(-time.Minute), Observed: now}
	if e = s.Record(event); e != nil {
		t.Fatal(e)
	}
	event.State = "queued"
	event.Observed = now.Add(-time.Second)
	if e = s.Record(event); e != nil {
		t.Fatal(e)
	}
	rows, e := s.Records(10)
	if e != nil || len(rows) != 1 {
		t.Fatal(len(rows), e)
	}
}
