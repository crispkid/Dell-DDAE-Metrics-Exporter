package serviceability

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type listAPI struct{ list ddae.ServiceabilityLogList }

func (a listAPI) ServiceabilityLogList(context.Context) (ddae.ServiceabilityLogList, error) {
	return a.list, nil
}

func TestWeakListRetainsSafeItemsButCannotReconcileMalformedResponse(t *testing.T) {
	var list ddae.ServiceabilityLogList
	if err := json.Unmarshal([]byte(`{
		"results":[{"id":"log-safe"},42,{"id":true}],
		"threshold":10,
		"totalRecords":"unknown"
	}`), &list); err != nil {
		t.Fatal(err)
	}
	if !list.Malformed || len(list.Results) != 1 || list.Results[0].ID != "log-safe" {
		t.Fatalf("weak list = %#v", list)
	}

	state := &memoryState{}
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(listAPI{list: list}, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: time.Second,
		RefreshInterval: time.Hour, MaxPerCycle: 10, Concurrency: 2,
	}, nil)
	pipeline.poll(context.Background())
	view := diagnostics.Load()
	if len(state.enqueued) != 1 || state.enqueued[0] != "log-safe" {
		t.Fatalf("enqueued = %v", state.enqueued)
	}
	if view.ServiceabilityLogListComplete || state.reconciled || view.ServiceabilityLogCollectionReady {
		t.Fatalf("malformed list became complete: %#v reconciled=%v", view, state.reconciled)
	}
}
func (a listAPI) ServiceabilityLogDetail(_ context.Context, id string) (ddae.ServiceabilityLogDetail, error) {
	return ddae.ServiceabilityLogDetail{ID: id}, nil
}

type memoryState struct {
	mu         sync.Mutex
	enqueued   []string
	reconciled bool
}

func (*memoryState) FetchState(string) (bool, string, time.Time, error) {
	return false, "", time.Time{}, nil
}
func (s *memoryState) Enqueue(event EncodedEvent, _ string, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enqueued = append(s.enqueued, event.Event.LogID)
	return true, nil
}
func (*memoryState) MarkSeen(string, string, time.Time) error { return nil }
func (s *memoryState) ReconcileListed(_ map[string]struct{}, _ time.Time, complete bool) error {
	s.reconciled = complete
	return nil
}
func (*memoryState) Health() (int, bool, error) { return 0, false, nil }

func TestListDeduplicatesSafeIDsAndReportsIncompleteTotals(t *testing.T) {
	total := int64(3)
	state := &memoryState{}
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(listAPI{list: ddae.ServiceabilityLogList{
		Results:      []ddae.ServiceabilityLogListItem{{ID: "log-1"}, {ID: "log-1"}, {ID: "bad\x00id"}},
		TotalRecords: &total,
	}}, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: time.Second,
		RefreshInterval: time.Hour, MaxPerCycle: 10, Concurrency: 2,
	}, nil)
	pipeline.poll(context.Background())
	view := diagnostics.Load()
	if len(state.enqueued) != 1 || state.enqueued[0] != "log-1" {
		t.Fatalf("enqueued = %v", state.enqueued)
	}
	if view.ServiceabilityLogListComplete || state.reconciled || view.ServiceabilityLogCollectionReady {
		t.Fatalf("incomplete state = %#v reconciled=%v", view, state.reconciled)
	}
}
