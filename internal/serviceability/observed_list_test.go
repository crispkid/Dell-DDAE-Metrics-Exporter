package serviceability

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type observedListAPI struct {
	list    ddae.ServiceabilityLogList
	mu      sync.Mutex
	fetched []string
}

func (a *observedListAPI) ServiceabilityLogList(context.Context) (ddae.ServiceabilityLogList, error) {
	return a.list, nil
}

func (a *observedListAPI) ServiceabilityLogDetail(_ context.Context, id string) (ddae.ServiceabilityLogDetail, error) {
	a.mu.Lock()
	a.fetched = append(a.fetched, id)
	a.mu.Unlock()
	severity := "Informational"
	return ddae.ServiceabilityLogDetail{ID: id, Type: &severity}, nil
}

func TestObservedIncompleteListFetchesDetailWithoutReconcilingAbsence(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ddae-1.5.0", "serviceability-observed-structure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list ddae.ServiceabilityLogList
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}
	api := &observedListAPI{list: list}
	state := &memoryState{}
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(api, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: time.Second,
		RefreshInterval: time.Hour, MaxPerCycle: 10, Concurrency: 2,
	}, nil)
	pipeline.poll(context.Background())

	api.mu.Lock()
	fetched := append([]string(nil), api.fetched...)
	api.mu.Unlock()
	view := diagnostics.Load()
	if len(fetched) != 1 || fetched[0] != "log-synthetic-1" || len(state.enqueued) != 1 {
		t.Fatalf("fetched=%v enqueued=%v", fetched, state.enqueued)
	}
	if view.ServiceabilityLogListComplete || view.ServiceabilityLogCollectionReady || state.reconciled {
		t.Fatalf("incomplete list became complete: view=%#v reconciled=%v", view, state.reconciled)
	}
}

func TestDuplicateOnlyServiceabilityListIsIncomplete(t *testing.T) {
	total := int64(1)
	api := &observedListAPI{list: ddae.ServiceabilityLogList{
		Results:      []ddae.ServiceabilityLogListItem{{ID: "log-1"}, {ID: "log-1"}},
		TotalRecords: &total,
	}}
	state := &memoryState{}
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(api, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: time.Second,
		RefreshInterval: time.Hour, MaxPerCycle: 10, Concurrency: 2,
	}, nil)
	pipeline.poll(context.Background())
	view := diagnostics.Load()
	if view.ServiceabilityLogListComplete || view.ServiceabilityLogCollectionReady || state.reconciled {
		t.Fatalf("duplicate-only list became complete: view=%#v reconciled=%v", view, state.reconciled)
	}
}
