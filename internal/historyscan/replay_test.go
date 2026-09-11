package historyscan

import (
	"context"
	"errors"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/queryclient"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/querystate"
	"testing"
	"time"
)

// TEST-DDAE-10-005: durable event precedes ack; replay shares foreground terminal dedup.
func TestBackfillCrashAfterRecord(t *testing.T) {
	dir := t.TempDir()
	st, e := historystate.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	opt := querystate.Options{Dir: dir, Source: "test", Retention: 2 * time.Hour, MaxCheckpoints: 100, MaxEvents: 100, MaxBytes: 1 << 20, Events: true}
	events, e := querystate.Open(opt)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	elapsed := 60.0
	event := queryclient.Event{SourceInstance: "test", QueryID: "q1", User: "synthetic", State: "finished", Submitted: now.Add(-time.Minute), Completed: &now, Observed: now, Elapsed: &elapsed}
	committed := false
	src := callbackSource{list: func(_ context.Context, w historystate.Window) (Page, error) {
		return Page{Complete: true, Items: []historystate.Item{{ID: "q1", When: now}}}, nil
	}, record: func(context.Context, historystate.Item) error {
		if e := events.Record(event); e != nil {
			return e
		}
		committed = true
		return errors.New("simulated interruption before progress ack")
	}}
	w, _ := New(st, "queries", "test", testConfig(), 2*time.Hour, src)
	w.Poll(context.Background())
	p, _ := st.Load("queries", "test")
	if !committed || len(p.Pending) != 1 || !p.LastCompleted.IsZero() {
		t.Fatal("unsafe ack")
	}
	st.Close()
	events.Close()
	st, e = historystate.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	events, e = querystate.Open(opt)
	if e != nil {
		t.Fatal(e)
	}
	defer events.Close()
	src.record = func(context.Context, historystate.Item) error { return events.Record(event) }
	w, _ = New(st, "queries", "test", testConfig(), 2*time.Hour, src)
	w.Poll(context.Background())
	a, e := events.Stats()
	p, _ = st.Load("queries", "test")
	if e != nil || a.Completed["finished"] != 1 || a.Pending != 1 || p.LastCompleted.IsZero() || len(p.Pending) != 0 {
		t.Fatal(a, p, e)
	}
}
