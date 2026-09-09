package historyscan

import (
	"context"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"testing"
	"time"
)

// TEST-DDAE-10-008: gap reporting, fixed windows and interval-only restart.
func TestBackfillRetentionAndRestart(t *testing.T) {
	dir := t.TempDir()
	st, e := historystate.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	cfg := testConfig()
	at := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Second)
	if e = st.Update("queries", "test", 1000, func(p *historystate.Progress) error {
		p.Lookback = cfg.Lookback
		p.Overlap = cfg.Overlap
		p.Start = at
		p.End = at.Add(time.Hour)
		p.Windows = []historystate.Window{{Start: p.Start, End: p.End}}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	st.Close()
	st, e = historystate.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	cfg.Interval = time.Minute
	src := &testSource{cap: 1000, seen: map[string]int{}}
	w, _ := New(st, "queries", "test", cfg, 2*time.Hour, src)
	w.Poll(context.Background())
	p, _ := st.Load("queries", "test")
	if p.Block != "expired" || !p.Start.Equal(at) || src.calls != 0 || w.Ready() {
		t.Fatal(p, src.calls)
	}
	cfg.Lookback = 2 * time.Hour
	w, _ = New(st, "queries", "test", cfg, 3*time.Hour, src)
	w.Poll(context.Background())
	if !w.Ready() {
		t.Fatal("explicit lookback change did not create new sweep")
	}
}
func TestBackfillRejectsOutOfWindow(t *testing.T) {
	st, e := historystate.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	src := callbackSource{list: func(_ context.Context, w historystate.Window) (Page, error) {
		return Page{Complete: true, Items: []historystate.Item{{ID: "x", When: w.End.Add(time.Hour)}}}, nil
	}}
	w, _ := New(st, "queries", "test", testConfig(), 2*time.Hour, src)
	w.Poll(context.Background())
	p, _ := st.Load("queries", "test")
	if w.Ready() || !p.LastCompleted.IsZero() || len(p.Windows) != 1 {
		t.Fatal("invalid page advanced")
	}
}

func TestBackfillLateArrivalRescanAndWindowFingerprint(t *testing.T) {
	st, e := historystate.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	cfg := testConfig()
	src := &testSource{cap: 1000, seen: map[string]int{}}
	w, e := New(st, "queries", "test", cfg, 2*time.Hour, src)
	if e != nil {
		t.Fatal(e)
	}
	w.Poll(context.Background())
	at := time.Now().UTC().Truncate(time.Second)
	src.all = []historystate.Item{{ID: "late", When: at.Add(-30 * time.Minute)}}
	if e = st.Update("queries", "test", 1000, func(p *historystate.Progress) error { p.LastCompleted = at.Add(-2 * time.Second); return nil }); e != nil {
		t.Fatal(e)
	}
	w.Poll(context.Background())
	if src.seen["late"] != 0 {
		t.Fatal("incremental bounds not respected")
	}
	if e = st.Update("queries", "test", 1000, func(p *historystate.Progress) error {
		p.LastCompleted = at.Add(-2 * time.Second)
		p.LastSweep = at.Add(-2 * time.Hour)
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	w.Poll(context.Background())
	if src.seen["late"] != 1 {
		t.Fatal("rescan missed late arrival")
	}
	cfg.Overlap = 2 * time.Minute
	if _, e = New(st, "queries", "test", cfg, 2*time.Hour, src); e == nil {
		t.Fatal("window fingerprint mismatch accepted")
	}
}
