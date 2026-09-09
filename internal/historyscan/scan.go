package historyscan

import (
	"context"
	"errors"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

var ErrSaturated = errors.New("history timestamp interval saturated")
var ErrExpired = errors.New("history interval predates retention")

type Page struct {
	Items    []historystate.Item
	Complete bool
}
type Source interface {
	List(context.Context, historystate.Window) (Page, error)
	Record(context.Context, historystate.Item) error
	Close()
}
type Worker struct {
	store            *historystate.Store
	key, identity    string
	cfg              config.BackfillConfig
	retention        time.Duration
	source           Source
	runMu            sync.Mutex
	mu               sync.RWMutex
	success, blocked bool
	at               time.Time
	progress         historystate.Progress
}

func New(st *historystate.Store, key, identity string, c config.BackfillConfig, retention time.Duration, source Source) (*Worker, error) {
	p, e := st.Load(key, identity)
	if e != nil {
		return nil, e
	}
	if p.Lookback != 0 && p.Overlap != c.Overlap {
		return nil, errors.New("history window configuration changed; restore overlap or use a separate state directory")
	}
	return &Worker{store: st, key: key, identity: identity, cfg: c, retention: retention, source: source, progress: p}, nil
}
func (w *Worker) Close() { w.source.Close() }
func (w *Worker) Ready() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.success && !w.blocked && !w.at.IsZero() && time.Since(w.at) <= 3*w.cfg.Interval
}
func (w *Worker) Run(ctx context.Context) {
	w.Poll(ctx)
	t := time.NewTicker(w.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Poll(ctx)
		}
	}
}
func (w *Worker) update(f func(*historystate.Progress) error) error {
	err := w.store.Update(w.key, w.identity, w.cfg.MaxPending, f)
	if err != nil && !errors.Is(err, historystate.ErrFull) {
		return errors.Join(historystate.ErrCorrupt, err)
	}
	return err
}
func (w *Worker) load() (historystate.Progress, error) { return w.store.Load(w.key, w.identity) }
func (w *Worker) Poll(parent context.Context) {
	if !w.runMu.TryLock() {
		return
	}
	defer w.runMu.Unlock()
	ctx, cancel := context.WithTimeout(parent, w.cfg.CycleTimeout)
	defer cancel()
	err := w.cycle(ctx)
	p, loadErr := w.load()
	w.mu.Lock()
	defer w.mu.Unlock()
	w.at = time.Now()
	w.success = err == nil && loadErr == nil
	w.blocked = loadErr != nil || p.Block != "" || errors.Is(err, historystate.ErrFull) || errors.Is(err, historystate.ErrCorrupt) || errors.Is(err, ErrExpired)
	if loadErr == nil {
		w.progress = p
	}
}
func (w *Worker) cycle(ctx context.Context) error {
	now := time.Now().UTC()
	end := now.Add(time.Second - time.Nanosecond).Truncate(time.Second)
	if err := w.update(func(p *historystate.Progress) error {
		if p.Lookback != w.cfg.Lookback {
			p.Windows = nil
			p.Pending = nil
			p.Page = nil
			p.Block = ""
			p.LastCompleted = time.Time{}
			p.LastSweep = time.Time{}
			p.Lookback = w.cfg.Lookback
			p.Overlap = w.cfg.Overlap
		}
		if p.Block != "" {
			return nil
		}
		if len(p.Windows) == 0 && len(p.Pending) == 0 && p.Page == nil {
			start := p.LastCompleted.Add(-w.cfg.Overlap)
			if p.LastCompleted.IsZero() || now.Sub(p.LastSweep) >= w.cfg.RescanInterval {
				start = now.Add(-w.cfg.Lookback).Truncate(time.Second)
				p.LastSweep = end
			}
			if !end.After(p.LastCompleted) {
				return nil
			}
			p.Start = start
			p.End = end
			p.Windows = []historystate.Window{{Start: start, End: end}}
		}
		return nil
	}); err != nil {
		return err
	}
	details := 0
	pages := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		p, err := w.load()
		if err != nil {
			return err
		}
		if p.Block != "" {
			return ErrSaturated
		}
		if p.Page != nil && p.Page.Start.Before(now.Add(-w.retention).Truncate(time.Second)) {
			if e := w.update(func(p *historystate.Progress) error { p.Block = "expired"; return nil }); e != nil {
				return e
			}
			return ErrExpired
		}
		if len(p.Pending) > 0 {
			if details >= w.cfg.DetailMax {
				return nil
			}
			n := min(w.cfg.DetailMax-details, len(p.Pending))
			batch := append([]historystate.Item(nil), p.Pending[:n]...)
			jobs := make(chan historystate.Item)
			results := make(chan error, len(batch))
			var wg sync.WaitGroup
			for i := 0; i < min(w.cfg.Concurrency, len(batch)); i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for item := range jobs {
						e := w.source.Record(ctx, item)
						// Acknowledgement happens only after the source's durable Record commits.
						if e == nil {
							e = w.update(func(p *historystate.Progress) error {
								for j, x := range p.Pending {
									if x.ID == item.ID {
										p.Pending = append(p.Pending[:j], p.Pending[j+1:]...)
										break
									}
								}
								return nil
							})
						}
						results <- e
					}
				}()
			}
			sent := 0
		dispatch:
			for _, item := range batch {
				select {
				case <-ctx.Done():
					break dispatch
				case jobs <- item:
					sent++
				}
			}
			close(jobs)
			wg.Wait()
			close(results)
			details += sent
			var failure error
			for e := range results {
				if e != nil {
					failure = e
				}
			}
			if failure != nil {
				if errors.Is(failure, ErrExpired) {
					_ = w.update(func(p *historystate.Progress) error { p.Block = "expired"; return nil })
				}
				return failure
			}
			if sent < len(batch) {
				return ctx.Err()
			}
			continue
		}
		if p.Page != nil {
			if err = w.update(func(p *historystate.Progress) error {
				p.Page = nil
				if len(p.Windows) == 0 {
					p.LastCompleted = p.End
				}
				return nil
			}); err != nil {
				return err
			}
			continue
		}
		if len(p.Windows) == 0 {
			return nil
		}
		window := p.Windows[0]
		if window.Start.Before(now.Add(-w.retention).Truncate(time.Second)) {
			if err = w.update(func(p *historystate.Progress) error { p.Block = "expired"; return nil }); err != nil {
				return err
			}
			return ErrExpired
		}
		if pages >= w.cfg.MaxPages {
			return nil
		}
		page, err := w.source.List(ctx, window)
		pages++
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, item := range page.Items {
			if item.ID == "" || seen[item.ID] || item.When.Before(window.Start.Truncate(time.Second)) || item.When.After(window.End.Add(time.Second-time.Nanosecond).Truncate(time.Second)) {
				return errors.New("history page identity or time invalid")
			}
			seen[item.ID] = true
		}
		if !page.Complete {
			if window.End.Sub(window.Start) <= time.Second {
				if err = w.update(func(p *historystate.Progress) error { p.Block = "saturated"; return nil }); err != nil {
					return err
				}
				return ErrSaturated
			}
			mid := window.Start.Add(window.End.Sub(window.Start) / 2).Truncate(time.Second)
			if !mid.After(window.Start) {
				mid = window.Start.Add(time.Second)
			}
			if !mid.Before(window.End) {
				return ErrSaturated
			}
			if err = w.update(func(p *historystate.Progress) error {
				p.Windows = append([]historystate.Window{{Start: mid, End: window.End}, {Start: window.Start, End: mid}}, p.Windows[1:]...)
				return nil
			}); err != nil {
				return err
			}
			continue
		}
		if err = w.update(func(p *historystate.Progress) error {
			p.Pending = page.Items
			p.Page = &window
			p.Windows = p.Windows[1:]
			return nil
		}); err != nil {
			return err
		}
	}
}

var metricNames = []string{"enabled", "success", "pending_windows", "pending_records", "last_completed_timestamp_seconds", "incomplete", "blocked"}
var metricHelp = []string{"Bounded history recovery is enabled; not an audit completeness claim.", "Last bounded recovery cycle succeeded; not global history completeness.", "Durable intervals awaiting bounded history recovery.", "Durable records awaiting bounded history delivery.", "Upper time bound of last walked window; not permanent history completeness.", "Current bounded recovery window is pending or incomplete.", "Bounded recovery cannot advance due to saturation, retention, capacity or state failure."}

func (w *Worker) desc(i int) *prometheus.Desc {
	return prometheus.NewDesc("ddae_history_backfill_"+metricNames[i], metricHelp[i], nil, prometheus.Labels{"pipeline": w.key})
}
func (w *Worker) Describe(ch chan<- *prometheus.Desc) {
	for i := range metricNames {
		ch <- w.desc(i)
	}
}
func flag(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
func (w *Worker) Collect(ch chan<- prometheus.Metric) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	p := w.progress
	incomplete := p.LastCompleted.IsZero() || len(p.Windows) > 0 || len(p.Pending) > 0 || p.Page != nil || w.blocked
	values := []float64{1, flag(w.success), float64(len(p.Windows)), float64(len(p.Pending)), float64(p.LastCompleted.Unix()), flag(incomplete), flag(w.blocked)}
	for i, v := range values {
		if i == 4 && p.LastCompleted.IsZero() {
			continue
		}
		ch <- prometheus.MustNewConstMetric(w.desc(i), prometheus.GaugeValue, v)
	}
}
