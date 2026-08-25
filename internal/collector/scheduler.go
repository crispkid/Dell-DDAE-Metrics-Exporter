package collector

import (
	"context"
	"sync"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type Manager struct {
	api          API
	store        *snapshot.Store
	cycleTimeout time.Duration
	interval     time.Duration
	logger       Logger
}

type Logger interface {
	Error(msg string, args ...any)
}

func NewManager(api API, store *snapshot.Store, cycleTimeout, interval time.Duration, logger Logger) *Manager {
	return &Manager{api: api, store: store, cycleTimeout: cycleTimeout, interval: interval, logger: logger}
}

// Run serializes cycles: the next ticker event cannot start collection until
// all workers from the current cycle have returned.
func (m *Manager) Run(ctx context.Context) {
	m.collect(ctx)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collect(ctx)
		}
	}
}

func (m *Manager) collect(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, m.cycleTimeout)
	defer cancel()
	type outcome struct {
		name     string
		value    any
		usable   bool
		success  bool
		duration time.Duration
		err      error
	}
	outcomes := make(chan outcome, 5)
	var workers sync.WaitGroup
	workers.Add(5)
	go func() {
		defer workers.Done()
		start := time.Now()
		value, err := m.api.Ping(ctx)
		outcomes <- outcome{name: "ping", value: value, usable: err == nil, success: err == nil, duration: time.Since(start), err: err}
	}()
	go func() {
		defer workers.Done()
		start := time.Now()
		raw, err := m.api.Clusters(ctx)
		var value []snapshot.Cluster
		usable, success := false, false
		if err == nil {
			value, usable, err = normalizeClusters(raw)
			success = err == nil
		}
		outcomes <- outcome{name: "clusters", value: value, usable: usable, success: success, duration: time.Since(start), err: err}
	}()
	go func() {
		defer workers.Done()
		start := time.Now()
		raw, err := m.api.Nodes(ctx)
		var value []snapshot.Node
		usable, success := false, false
		if err == nil {
			value, usable, err = normalizeNodes(raw)
			success = err == nil
		}
		outcomes <- outcome{name: "nodes", value: value, usable: usable, success: success, duration: time.Since(start), err: err}
	}()
	go func() {
		defer workers.Done()
		start := time.Now()
		raw, err := m.api.Lock(ctx)
		value, usable := false, false
		if err == nil {
			value, usable = raw.Status.Value()
			if !usable {
				err = validationError("lock")
			}
		}
		outcomes <- outcome{name: "lock", value: value, usable: usable, success: err == nil, duration: time.Since(start), err: err}
	}()
	go func() {
		defer workers.Done()
		start := time.Now()
		raw, err := m.api.Power(ctx)
		value, usable := snapshot.Power{}, false
		if err == nil {
			value, usable, err = normalizePower(raw)
		}
		outcomes <- outcome{name: "power", value: value, usable: usable, success: err == nil, duration: time.Since(start), err: err}
	}()
	go func() {
		workers.Wait()
		close(outcomes)
	}()

	cycleSuccess := true
	completedAt := time.Now()
	for result := range outcomes {
		completedAt = time.Now()
		if !result.success {
			cycleSuccess = false
			if m.logger != nil {
				m.logger.Error("DDAE collector failed", "collector", result.name, "failure_class", observability.Classify(result.err))
			}
		}
		switch result.name {
		case "ping":
			_, pingOK := result.value.(ddae.PingResponse)
			m.store.RecordPing(pingOK, result.usable, result.success, completedAt, result.duration)
		case "clusters":
			m.store.RecordClusters(result.value.([]snapshot.Cluster), result.usable, result.success, completedAt, result.duration)
		case "nodes":
			m.store.RecordNodes(result.value.([]snapshot.Node), result.usable, result.success, completedAt, result.duration)
		case "lock":
			m.store.RecordLock(result.value.(bool), result.usable, result.success, completedAt, result.duration)
		case "power":
			m.store.RecordPower(result.value.(snapshot.Power), result.usable, result.success, completedAt, result.duration)
		}
	}
	m.store.CompleteRequiredCycle(completedAt, cycleSuccess)
}
