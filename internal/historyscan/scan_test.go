package historyscan

import (
	"context"
	"fmt"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"path/filepath"
	"testing"
	"time"
)

type testSource struct {
	all   []historystate.Item
	seen  map[string]int
	cap   int
	calls int
}

func (s *testSource) List(_ context.Context, w historystate.Window) (Page, error) {
	s.calls++
	p := Page{Complete: true}
	for _, i := range s.all {
		if !i.When.Before(w.Start) && !i.When.After(w.End) {
			p.Items = append(p.Items, i)
		}
	}
	if len(p.Items) >= s.cap {
		p.Items = p.Items[:s.cap]
		p.Complete = false
	}
	return p, nil
}
func (s *testSource) Record(_ context.Context, i historystate.Item) error { s.seen[i.ID]++; return nil }
func (s *testSource) Close()                                              {}
func testConfig() config.BackfillConfig {
	return config.BackfillConfig{Enabled: true, Lookback: time.Hour, Overlap: time.Minute, Interval: 30 * time.Second, CycleTimeout: time.Second, RescanInterval: time.Hour, MaxPages: 4, DetailMax: 25, Concurrency: 1, MaxPending: 1000}
}

// TEST-DDAE-10-003: split, overlap and bounded progress beyond a page.
func TestBackfillSplitAndResume(t *testing.T) {
	st, e := historystate.Open(filepath.Join(t.TempDir(), "state"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	now := time.Now().UTC().Truncate(time.Second)
	src := &testSource{seen: map[string]int{}, cap: 3}
	for n := 0; n < 9; n++ {
		src.all = append(src.all, historystate.Item{ID: string(rune('a' + n)), When: now.Add(time.Duration(-100-n*100) * time.Second)})
	}
	w, e := New(st, "queries", "test", testConfig(), 2*time.Hour, src)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 30; n++ {
		before := src.calls
		w.Poll(context.Background())
		if src.calls-before > 4 {
			t.Fatal("page budget")
		}
		p, _ := st.Load("queries", "test")
		if !p.LastCompleted.IsZero() {
			break
		}
	}
	if len(src.seen) != 9 || !w.Ready() {
		t.Fatal(len(src.seen), w.Ready())
	}
}
func TestBackfillTimestampSaturation(t *testing.T) {
	st, _ := historystate.Open(filepath.Join(t.TempDir(), "state"))
	defer st.Close()
	at := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	src := &testSource{seen: map[string]int{}, cap: 1000}
	for n := 0; n < 1000; n++ {
		src.all = append(src.all, historystate.Item{ID: fmt.Sprint(n), When: at})
	}
	w, _ := New(st, "queries", "test", testConfig(), 2*time.Hour, src)
	for n := 0; n < 15; n++ {
		w.Poll(context.Background())
	}
	p, _ := st.Load("queries", "test")
	if p.Block != "saturated" || w.Ready() {
		t.Fatal(p.Block, w.Ready())
	}
}
