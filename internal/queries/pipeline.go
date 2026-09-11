package queries

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/kafka"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/queryclient"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/querystate"
	"github.com/prometheus/client_golang/prometheus"
)

type api interface {
	Get(context.Context, string) ([]byte, error)
	Scope(context.Context) error
	CloseIdleConnections()
}
type sender interface {
	PublishQuery(context.Context, []byte, []byte) error
	Close()
}
type Pipeline struct {
	cfg                                             config.QueryConfig
	source                                          string
	client                                          api
	store                                           *querystate.Store
	producer                                        sender
	logger                                          *slog.Logger
	mu                                              sync.RWMutex
	sample                                          queryclient.Sample
	scope, overviewOK, detailOK, stateOK, publishOK bool
	detailAt                                        time.Time
	stats                                           querystate.Stats
	cursor                                          int
}

func New(cfg config.Config, logger *slog.Logger) (*Pipeline, error) {
	client, err := queryclient.New(cfg.Query, cfg.AllowInsecureTLS)
	if err != nil {
		return nil, err
	}
	s, err := querystate.Open(querystate.Options{Dir: cfg.StateDir, Source: cfg.SourceInstance, MaxEvents: cfg.Query.MaxEvents, MaxBytes: cfg.Query.MaxBytes, MaxCheckpoints: cfg.Query.MaxCheckpoints, Retention: cfg.Query.Retention, Events: cfg.Query.Events})
	if err != nil {
		client.CloseIdleConnections()
		return nil, err
	}
	p := &Pipeline{cfg: cfg.Query, source: cfg.SourceInstance, client: client, store: s, logger: logger, publishOK: !cfg.Query.Events}
	if cfg.Query.Events {
		producer, err := kafka.NewQueryProducer(cfg)
		if err != nil {
			s.Close()
			client.CloseIdleConnections()
			return nil, err
		}
		p.producer = producer
	}
	p.stats, err = s.Stats()
	if err != nil {
		p.Close()
		return nil, err
	}
	p.stateOK = true
	return p, nil
}
func (p *Pipeline) Close() error {
	if p.producer != nil {
		p.producer.Close()
	}
	if p.client != nil {
		p.client.CloseIdleConnections()
	}
	if p.store != nil {
		return p.store.Close()
	}
	return nil
}
func (p *Pipeline) Run(ctx context.Context) {
	var wg sync.WaitGroup
	if p.producer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.flush(ctx)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					p.flush(ctx)
				}
			}
		}()
	}
	defer wg.Wait()
	p.Poll(ctx)
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.Poll(ctx)
		}
	}
}
func (p *Pipeline) failure(stage string) {
	if p.logger != nil {
		p.logger.Warn("query collection failed", "component", "queries", "stage", stage)
	}
}
func (p *Pipeline) Poll(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, p.cfg.CycleTimeout)
	defer cancel()
	if err := p.client.Scope(ctx); err != nil {
		p.mu.Lock()
		p.scope = false
		p.overviewOK = false
		p.detailOK = false
		p.mu.Unlock()
		p.failure("scope")
		return
	}
	p.mu.Lock()
	p.scope = true
	p.mu.Unlock()
	b, err := p.client.Get(ctx, "/ui/api/insights/overview/queries")
	var sample queryclient.Sample
	if err == nil {
		sample, err = queryclient.DecodeOverview(b, time.Now(), p.cfg.StaleAfter)
	}
	p.mu.Lock()
	p.overviewOK = err == nil
	if err == nil {
		p.sample = sample
	}
	p.mu.Unlock()
	if err != nil {
		p.failure("overview")
	}
	if err = p.store.Prune(time.Now()); err != nil {
		p.mu.Lock()
		p.stateOK = false
		p.detailOK = false
		p.mu.Unlock()
		p.failure("state")
		return
	}
	b, err = p.client.Get(ctx, "/ui/api/insights/history/queries")
	var items []queryclient.HistoryItem
	if err == nil {
		items, err = queryclient.DecodeHistory(b, p.cfg.MaxHistory)
	}
	ok := err == nil
	if ok && len(items) > 0 {
		ids := make([]string, 0, p.cfg.MaxPerCycle)
		examined := 0
		for examined < len(items) && len(ids) < p.cfg.MaxPerCycle {
			item := items[(p.cursor+examined)%len(items)]
			examined++
			final, _, fetchErr := p.store.Fetch(item.ID)
			if fetchErr != nil {
				ok = false
				break
			}
			if !final {
				ids = append(ids, item.ID)
			}
		}
		p.cursor = (p.cursor + examined) % len(items)
		jobs := make(chan string)
		results := make(chan error, len(ids))
		var wg sync.WaitGroup
		for i := 0; i < p.cfg.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for id := range jobs {
					event, err := queryclient.FetchDetail(ctx, p.client, id, p.source)
					if err == nil {
						err = p.store.Record(event)
					}
					results <- err
				}
			}()
		}
		for _, id := range ids {
			jobs <- id
		}
		close(jobs)
		wg.Wait()
		close(results)
		for err := range results {
			if errors.Is(err, querystate.ErrExpired) {
				p.failure("retention_gap")
			}
			if err != nil && !errors.Is(err, querystate.ErrExpired) {
				ok = false
			}
		}
	}
	stats, stateErr := p.store.Stats()
	p.mu.Lock()
	p.detailOK = ok && stateErr == nil
	p.detailAt = time.Now()
	p.stateOK = stateErr == nil
	if stateErr == nil && stats.Revision >= p.stats.Revision {
		p.stats = stats
	}
	p.mu.Unlock()
	if !ok {
		p.failure("details")
	}
}
func (p *Pipeline) flush(ctx context.Context) {
	rows, err := p.store.Records(100)
	if err == nil {
		for _, r := range rows {
			if err = p.producer.PublishQuery(ctx, r.Key, r.Payload); err != nil {
				break
			}
			if err = p.store.Ack(r.Sequence); err != nil {
				break
			}
		}
	}
	stats, stateErr := p.store.Stats()
	p.mu.Lock()
	p.publishOK = err == nil && stateErr == nil
	p.stateOK = stateErr == nil
	if stateErr == nil && stats.Revision >= p.stats.Revision {
		p.stats = stats
	}
	p.mu.Unlock()
	if err != nil {
		p.failure("publish")
	}
}
func (p *Pipeline) Ready() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	fresh := !p.sample.At.IsZero() && time.Since(p.sample.At) <= p.cfg.StaleAfter
	return fresh && p.scope && p.overviewOK && p.detailOK && p.stateOK && time.Since(p.detailAt) <= p.cfg.StaleAfter && (!p.cfg.Events || p.publishOK)
}

var help = map[string]string{
	"ddae_queries_running":                  "Running queries in the latest fresh upstream sample with all-query scope.",
	"ddae_queries_queued":                   "Queued queries in the latest fresh upstream sample with all-query scope.",
	"ddae_query_sample_timestamp_seconds":   "Upstream query sample timestamp in Unix seconds, not fetch time.",
	"ddae_query_collection_success":         "Whether the latest query overview collection succeeded and remains fresh.",
	"ddae_query_detail_collection_success":  "Whether the latest bounded query detail collection succeeded and remains fresh.",
	"ddae_query_scope_all":                  "Whether fresh query collection has confirmed all-query visibility.",
	"ddae_query_history_complete":           "Whether complete query history coverage is proven; this version reports zero (unknown coverage).",
	"ddae_query_events_pending":             "Query detail events pending in the durable outbox.",
	"ddae_query_event_publish_success":      "Whether the latest query event publish attempt succeeded.",
	"ddae_queries_observed_completed_total": "Deduplicated terminal queries observed within collection coverage, not all cluster queries.",
	"ddae_query_observed_elapsed_seconds":   "Elapsed seconds of observed terminal queries, not a complete cluster distribution.",
	"ddae_query_observed_execution_seconds": "Execution seconds of observed terminal queries with a reported value.",
	"ddae_query_observed_queued_seconds":    "Queued seconds of observed terminal queries with a reported value.",
}

func desc(name string) *prometheus.Desc {
	var labels []string
	if strings.Contains(name, "observed_") {
		labels = []string{"state"}
	}
	return prometheus.NewDesc(name, help[name], labels, nil)
}
func (p *Pipeline) Describe(ch chan<- *prometheus.Desc) {
	for name := range help {
		ch <- desc(name)
	}
}
func boolean(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
func (p *Pipeline) Collect(ch chan<- prometheus.Metric) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	emit := func(n string, v float64) { ch <- prometheus.MustNewConstMetric(desc(n), prometheus.GaugeValue, v) }
	fresh := !p.sample.At.IsZero() && time.Since(p.sample.At) <= p.cfg.StaleAfter
	emit("ddae_query_collection_success", boolean(p.overviewOK && fresh && p.scope))
	emit("ddae_query_detail_collection_success", boolean(p.detailOK && p.stateOK && time.Since(p.detailAt) <= p.cfg.StaleAfter))
	emit("ddae_query_scope_all", boolean(p.scope && fresh))
	emit("ddae_query_history_complete", 0)
	if p.scope && fresh {
		emit("ddae_queries_running", float64(p.sample.Running))
		emit("ddae_queries_queued", float64(p.sample.Queued))
	}
	if !p.sample.At.IsZero() {
		emit("ddae_query_sample_timestamp_seconds", float64(p.sample.At.UnixNano())/1e9)
	}
	emit("ddae_query_events_pending", float64(p.stats.Pending))
	if p.cfg.Events {
		emit("ddae_query_event_publish_success", boolean(p.publishOK))
	}
	for _, state := range []string{"finished", "failed", "canceled"} {
		ch <- prometheus.MustNewConstMetric(desc("ddae_queries_observed_completed_total"), prometheus.CounterValue, float64(p.stats.Completed[state]), state)
		for _, field := range []string{"elapsed", "execution", "queued"} {
			h, ok := p.stats.Histograms[field+":"+state]
			if !ok {
				continue
			}
			buckets := map[float64]uint64{}
			for i, b := range querystate.Bounds {
				if i < len(h.Buckets) {
					buckets[b] = h.Buckets[i]
				}
			}
			ch <- prometheus.MustNewConstHistogram(desc("ddae_query_observed_"+field+"_seconds"), h.Count, h.Sum, buckets, state)
		}
	}
}

// HistoryStore shares durable deduplication with the independent history client.
func (p *Pipeline) HistoryStore() *querystate.Store { return p.store }
