package alerts

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type fakeAlertAPI struct {
	list      ddae.AlertList
	listErr   error
	details   map[string]ddae.AlertDetail
	detailErr map[string]error
	mu        sync.Mutex
	fetched   []string
}

func (f *fakeAlertAPI) AlertList(context.Context) (ddae.AlertList, error) { return f.list, f.listErr }
func (f *fakeAlertAPI) AlertDetail(_ context.Context, id string) (ddae.AlertDetail, error) {
	f.mu.Lock()
	f.fetched = append(f.fetched, id)
	f.mu.Unlock()
	return f.details[id], f.detailErr[id]
}

type memoryState struct {
	mu          sync.Mutex
	checkpoints map[string]struct {
		marker string
		at     time.Time
	}
	events []EncodedEvent
}

func (m *memoryState) FetchState(id string) (bool, string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.checkpoints[id]
	return ok, value.marker, value.at, nil
}
func (m *memoryState) Enqueue(event EncodedEvent, marker string, at time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	m.checkpoints[event.Event.AlertID] = struct {
		marker string
		at     time.Time
	}{marker, at}
	return true, nil
}
func (m *memoryState) MarkSeen(id, marker string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	value := m.checkpoints[id]
	value.marker = marker
	m.checkpoints[id] = value
	return nil
}
func (*memoryState) ReconcileListed(map[string]struct{}, time.Time, bool) error { return nil }
func (m *memoryState) Health() (int, bool, error)                               { return len(m.events), false, nil }

func TestPipelinePrioritizesNewAlertsAndDefersBoundedly(t *testing.T) {
	api := &fakeAlertAPI{
		list:    ddae.AlertList{Results: []ddae.AlertListItem{{ID: "b"}, {ID: "a"}}, TotalRecords: pointer[int64](2)},
		details: map[string]ddae.AlertDetail{"a": {ID: "a"}, "b": {ID: "b"}},
	}
	state := &memoryState{checkpoints: make(map[string]struct {
		marker string
		at     time.Time
	})}
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(api, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: time.Second,
		RefreshInterval: time.Minute, MaxPerCycle: 1, Concurrency: 1,
	}, nil)
	pipeline.poll(context.Background())
	view := diagnostics.Load()
	if len(api.fetched) != 1 || api.fetched[0] != "a" {
		t.Fatalf("fetched = %v", api.fetched)
	}
	if view.AlertDeferred != 1 || !view.Collectors["alert_detail"].Success {
		t.Fatalf("diagnostics = %#v", view)
	}
}
