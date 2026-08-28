package serviceability

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

func TestFairSelectionProgressesBothClasses(t *testing.T) {
	now := time.Now()
	pipeline := &Pipeline{maxPerCycle: 4}
	tasks := []detailTask{
		{id: "new-b", priority: 0, lastFetched: now},
		{id: "new-a", priority: 0, lastFetched: now.Add(-time.Second)},
		{id: "new-c", priority: 0, lastFetched: now.Add(time.Second)},
		{id: "refresh-a", priority: 1, lastFetched: now.Add(-time.Hour)},
		{id: "refresh-b", priority: 1, lastFetched: now},
	}
	selected := pipeline.selectFair(tasks)
	if len(selected) != 4 || selected[0].id != "new-a" || selected[3].id != "refresh-a" {
		t.Fatalf("selection = %#v", selected)
	}
	pipeline.maxPerCycle = 1
	first := pipeline.selectFair(tasks)
	second := pipeline.selectFair(tasks)
	if len(first) != 1 || len(second) != 1 || first[0].priority == second[0].priority {
		t.Fatalf("limit-one turns = %#v then %#v", first, second)
	}
}

type boundedAPI struct {
	list        ddae.ServiceabilityLogList
	started     chan struct{}
	release     chan struct{}
	mu          sync.Mutex
	calls       int
	inFlight    int
	maxInFlight int
}

func (a *boundedAPI) ServiceabilityLogList(context.Context) (ddae.ServiceabilityLogList, error) {
	return a.list, nil
}

func (a *boundedAPI) ServiceabilityLogDetail(ctx context.Context, id string) (ddae.ServiceabilityLogDetail, error) {
	a.mu.Lock()
	a.calls++
	a.inFlight++
	if a.inFlight > a.maxInFlight {
		a.maxInFlight = a.inFlight
	}
	a.mu.Unlock()
	a.started <- struct{}{}
	select {
	case <-a.release:
	case <-ctx.Done():
		a.mu.Lock()
		a.inFlight--
		a.mu.Unlock()
		return ddae.ServiceabilityLogDetail{}, ctx.Err()
	}
	a.mu.Lock()
	a.inFlight--
	a.mu.Unlock()
	return ddae.ServiceabilityLogDetail{ID: id}, nil
}

func TestPollBoundsRequestsConcurrencyAndReportsDeferredWork(t *testing.T) {
	total := int64(5)
	results := make([]ddae.ServiceabilityLogListItem, 5)
	for index := range results {
		results[index].ID = "log-" + string(rune('a'+index))
	}
	api := &boundedAPI{
		list:    ddae.ServiceabilityLogList{Results: results, TotalRecords: &total},
		started: make(chan struct{}, 2), release: make(chan struct{}),
	}
	state := &memoryState{}
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(api, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: time.Second,
		RefreshInterval: time.Hour, MaxPerCycle: 2, Concurrency: 2,
	}, nil)
	done := make(chan struct{})
	go func() {
		pipeline.poll(context.Background())
		close(done)
	}()
	for range 2 {
		select {
		case <-api.started:
		case <-time.After(time.Second):
			t.Fatal("bounded detail workers did not start")
		}
	}
	close(api.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bounded poll did not stop")
	}
	api.mu.Lock()
	calls, maximum := api.calls, api.maxInFlight
	api.mu.Unlock()
	view := diagnostics.Load()
	if calls != 2 || maximum != 2 || len(state.enqueued) != 2 {
		t.Fatalf("calls=%d concurrency=%d enqueued=%v", calls, maximum, state.enqueued)
	}
	if !view.ServiceabilityLogListComplete || !view.ServiceabilityLogCollectionReady || view.ServiceabilityLogDeferred != 3 {
		t.Fatalf("bounded diagnostics = %#v", view)
	}
}

type mismatchAPI struct{}

func (mismatchAPI) ServiceabilityLogList(context.Context) (ddae.ServiceabilityLogList, error) {
	total := int64(1)
	return ddae.ServiceabilityLogList{Results: []ddae.ServiceabilityLogListItem{{ID: "log-1"}}, TotalRecords: &total}, nil
}
func (mismatchAPI) ServiceabilityLogDetail(context.Context, string) (ddae.ServiceabilityLogDetail, error) {
	return ddae.ServiceabilityLogDetail{ID: "different"}, nil
}

func TestReturnedDetailIdentityMismatchFailsOnlyTheLogCycle(t *testing.T) {
	state := &memoryState{}
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(mismatchAPI{}, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: time.Second,
		RefreshInterval: time.Hour, MaxPerCycle: 1, Concurrency: 1,
	}, nil)
	pipeline.poll(context.Background())
	view := diagnostics.Load()
	if len(state.enqueued) != 0 || view.ServiceabilityLogCollectionReady || view.Collectors["serviceability_log_detail"].Success {
		t.Fatalf("identity mismatch state=%v diagnostics=%#v", state.enqueued, view)
	}
}

type deadlineAPI struct{}

func (deadlineAPI) ServiceabilityLogList(context.Context) (ddae.ServiceabilityLogList, error) {
	total := int64(1)
	return ddae.ServiceabilityLogList{Results: []ddae.ServiceabilityLogListItem{{ID: "log-1"}}, TotalRecords: &total}, nil
}
func (deadlineAPI) ServiceabilityLogDetail(ctx context.Context, _ string) (ddae.ServiceabilityLogDetail, error) {
	<-ctx.Done()
	return ddae.ServiceabilityLogDetail{}, ctx.Err()
}

func TestDetailWorkHonorsCycleDeadline(t *testing.T) {
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(deadlineAPI{}, &memoryState{}, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: 10 * time.Millisecond,
		RefreshInterval: time.Hour, MaxPerCycle: 1, Concurrency: 1,
	}, nil)
	started := time.Now()
	pipeline.poll(context.Background())
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cycle deadline elapsed=%v", elapsed)
	}
	if diagnostics.Load().ServiceabilityLogCollectionReady {
		t.Fatal("timed-out detail cycle reported ready")
	}
}
