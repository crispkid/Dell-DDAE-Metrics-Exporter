package collector

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type schedulerAPI struct {
	mu         sync.Mutex
	active     int
	maxActive  int
	delay      time.Duration
	clusterErr error
}

func (f *schedulerAPI) call(ctx context.Context) error {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()
	if f.delay == 0 {
		return nil
	}
	timer := time.NewTimer(f.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (f *schedulerAPI) Ping(ctx context.Context) (ddae.PingResponse, error) {
	return ddae.PingResponse{Status: "ok"}, f.call(ctx)
}
func (f *schedulerAPI) Clusters(ctx context.Context) ([]ddae.Cluster, error) {
	err := f.call(ctx)
	if err == nil {
		err = f.clusterErr
	}
	return []ddae.Cluster{{ID: "cluster-1", ClusterStatus: "available"}}, err
}
func (f *schedulerAPI) Nodes(ctx context.Context) ([]ddae.InfrastructureNode, error) {
	return []ddae.InfrastructureNode{{ID: "node-1", State: "Ready"}}, f.call(ctx)
}
func (f *schedulerAPI) Lock(ctx context.Context) (ddae.LockResponse, error) {
	var result ddae.LockResponse
	_ = json.Unmarshal([]byte(`{"status":"unlocked"}`), &result)
	return result, f.call(ctx)
}
func (f *schedulerAPI) Power(ctx context.Context) (ddae.PowerResponse, error) {
	ready, count := true, int64(1)
	return ddae.PowerResponse{ControlPlaneReady: &ready, NodesReady: &count, TotalNodes: &count}, f.call(ctx)
}

func TestSchedulerCollectsBoundedParallelCycle(t *testing.T) {
	api := &schedulerAPI{}
	store := snapshot.NewStore()
	manager := NewManager(api, store, time.Second, time.Minute, nil)
	manager.collect(context.Background())
	view := store.Load()
	if view.LastCompleteAt.IsZero() || !snapshot.RequiredCurrent(view, time.Now(), time.Minute) {
		t.Fatalf("cycle not current: %#v", view)
	}
	api.mu.Lock()
	maxActive := api.maxActive
	api.mu.Unlock()
	if maxActive < 1 || maxActive > len(RequiredCollectors()) {
		t.Fatalf("max active calls = %d", maxActive)
	}
}

func TestSchedulerDoesNotOverlapCyclesAndReturnsOnCancel(t *testing.T) {
	api := &schedulerAPI{delay: 15 * time.Millisecond}
	manager := NewManager(api, snapshot.NewStore(), 50*time.Millisecond, time.Millisecond, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		manager.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("scheduler workers did not return after cancellation")
	}
	api.mu.Lock()
	maxActive := api.maxActive
	api.mu.Unlock()
	if maxActive > len(RequiredCollectors()) {
		t.Fatalf("overlapping cycles observed: %d active calls", maxActive)
	}
}

func TestSchedulerIsolatesCollectorFailure(t *testing.T) {
	api := &schedulerAPI{clusterErr: errors.New("synthetic cluster failure")}
	store := snapshot.NewStore()
	NewManager(api, store, time.Second, time.Minute, nil).collect(context.Background())
	view := store.Load()
	if view.Collectors["clusters"].Success || !view.Collectors["nodes"].Success || !view.LastCompleteAt.IsZero() {
		t.Fatalf("partial failure state = %#v", view)
	}
}
