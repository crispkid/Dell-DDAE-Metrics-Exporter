package historyscan

import (
	"context"
	"errors"
	"fmt"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"sync/atomic"
	"testing"
	"time"
)

type callbackSource struct {
	list   func(context.Context, historystate.Window) (Page, error)
	record func(context.Context, historystate.Item) error
}

func (s callbackSource) List(c context.Context, w historystate.Window) (Page, error) {
	return s.list(c, w)
}
func (s callbackSource) Record(c context.Context, i historystate.Item) error { return s.record(c, i) }
func (s callbackSource) Close()                                              {}

// TEST-DDAE-10-006: independent deadlines, non-overlap and budgeted detail work.
func TestBackfillBudgetsAndCancel(t *testing.T) {
	st, e := historystate.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	cfg := testConfig()
	cfg.DetailMax = 3
	cfg.Concurrency = 2
	var active, peak, calls atomic.Int32
	src := callbackSource{list: func(_ context.Context, w historystate.Window) (Page, error) {
		p := Page{Complete: true}
		for n := 0; n < 10; n++ {
			p.Items = append(p.Items, historystate.Item{ID: fmt.Sprint(n), When: w.End})
		}
		return p, nil
	}, record: func(ctx context.Context, _ historystate.Item) error {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		calls.Add(1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
			return nil
		}
	}}
	w, _ := New(st, "queries", "test", cfg, 2*time.Hour, src)
	w.Poll(context.Background())
	p, _ := st.Load("queries", "test")
	if calls.Load() != 3 || peak.Load() > 2 || len(p.Pending) != 7 || !w.Ready() || !p.LastCompleted.IsZero() {
		t.Fatal(calls.Load(), peak.Load(), p)
	}
	entered := make(chan struct{})
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	w.source = callbackSource{record: func(c context.Context, _ historystate.Item) error {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-c.Done()
		return c.Err()
	}}
	go func() { w.Poll(ctx); close(done) }()
	<-entered
	w.Poll(context.Background())
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel stalled")
	}
	if w.Ready() {
		t.Fatal("cancel reported healthy")
	}
	p, _ = st.Load("queries", "test")
	if len(p.Pending) != 7 {
		t.Fatal("cancellation lost IDs")
	}
	w.source = callbackSource{record: func(context.Context, historystate.Item) error { return errors.New("synthetic detail failure") }}
	w.Poll(context.Background())
	if w.Ready() {
		t.Fatal("failure reported healthy")
	}
}
func TestBackfillRealisticCounts(t *testing.T) {
	for _, tc := range []struct{ n, cap int }{{720, 500}, {1474, 1000}} {
		t.Run(fmt.Sprint(tc.n), func(t *testing.T) {
			st, e := historystate.Open(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			defer st.Close()
			src := &testSource{seen: map[string]int{}, cap: tc.cap}
			at := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
			for n := 0; n < tc.n; n++ {
				src.all = append(src.all, historystate.Item{ID: fmt.Sprint(n), When: at.Add(-time.Duration(n) * time.Second).Add(-500 * time.Millisecond)})
			}
			cfg := testConfig()
			cfg.DetailMax = 1000
			cfg.CycleTimeout = 10 * time.Second
			w, _ := New(st, "queries", "test", cfg, 2*time.Hour, src)
			for n := 0; n < 20; n++ {
				w.Poll(context.Background())
				p, _ := st.Load("queries", "test")
				if !p.LastCompleted.IsZero() {
					break
				}
			}
			if len(src.seen) != tc.n || !w.Ready() {
				t.Fatal(len(src.seen), w.Ready())
			}
		})
	}
}
