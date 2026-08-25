package alerts

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

func newTestPipeline(api *fakeAlertAPI, state *memoryState, diagnostics *snapshot.Store, logger Logger) *Pipeline {
	return NewPipeline(api, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Hour, CycleTimeout: time.Second,
		RefreshInterval: time.Minute, MaxPerCycle: 10, Concurrency: 2,
	}, logger)
}

func TestPipelineListAndDetailFailuresDegradeOnlyAlertReadiness(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	diagnostics := snapshot.NewStore()
	state := &memoryState{checkpoints: make(map[string]struct {
		marker string
		at     time.Time
	})}
	api := &fakeAlertAPI{listErr: errors.New("raw-response-canary")}
	pipeline := newTestPipeline(api, state, diagnostics, logger)
	pipeline.poll(context.Background())
	view := diagnostics.Load()
	if view.Collectors["alert_list"].Success || view.AlertPipelineReady {
		t.Fatalf("list failure diagnostics = %#v", view)
	}
	if bytes.Contains(logs.Bytes(), []byte("raw-response-canary")) {
		t.Fatalf("log leaked raw error: %s", logs.String())
	}

	total := int64(1)
	api.listErr = nil
	api.list = ddae.AlertList{Results: []ddae.AlertListItem{{ID: "alert-1"}}, TotalRecords: &total}
	api.detailErr = map[string]error{"alert-1": errors.New("detail-body-canary")}
	pipeline.poll(context.Background())
	view = diagnostics.Load()
	if !view.Collectors["alert_list"].Success || view.Collectors["alert_detail"].Success || view.AlertPipelineReady || len(state.events) != 0 {
		t.Fatalf("detail failure diagnostics = %#v events=%d", view, len(state.events))
	}
}

func TestPipelineDeduplicatesAndMarksIncompleteList(t *testing.T) {
	total := int64(3)
	api := &fakeAlertAPI{
		list:    ddae.AlertList{Results: []ddae.AlertListItem{{ID: "valid"}, {ID: "valid"}, {ID: "../unsafe"}}, TotalRecords: &total},
		details: map[string]ddae.AlertDetail{"valid": {ID: "valid"}},
	}
	state := &memoryState{checkpoints: make(map[string]struct {
		marker string
		at     time.Time
	})}
	diagnostics := snapshot.NewStore()
	newTestPipeline(api, state, diagnostics, nil).poll(context.Background())
	if len(api.fetched) != 1 || api.fetched[0] != "valid" {
		t.Fatalf("detail requests = %v", api.fetched)
	}
	view := diagnostics.Load()
	if view.AlertListComplete || view.AlertPipelineReady || len(state.events) != 1 {
		t.Fatalf("incomplete list diagnostics=%#v events=%d", view, len(state.events))
	}
}

func TestPipelineRunReturnsOnCancellation(t *testing.T) {
	total := int64(0)
	api := &fakeAlertAPI{list: ddae.AlertList{TotalRecords: &total}}
	state := &memoryState{checkpoints: make(map[string]struct {
		marker string
		at     time.Time
	})}
	pipeline := newTestPipeline(api, state, snapshot.NewStore(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { pipeline.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pipeline did not stop")
	}
}
