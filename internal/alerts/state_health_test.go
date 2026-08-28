package alerts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type failingState struct {
	fail        string
	exists      bool
	lastFetched time.Time
}

func (s *failingState) failure(operation string) error {
	if s.fail == operation {
		return errors.New("state-canary")
	}
	return nil
}

func (s *failingState) FetchState(string) (bool, string, time.Time, error) {
	return s.exists, "", s.lastFetched, s.failure("fetch")
}
func (s *failingState) Enqueue(EncodedEvent, string, time.Time) (bool, error) {
	return true, s.failure("enqueue")
}
func (s *failingState) MarkSeen(string, string, time.Time) error {
	return s.failure("mark")
}
func (s *failingState) ReconcileListed(map[string]struct{}, time.Time, bool) error {
	return s.failure("reconcile")
}
func (s *failingState) Health() (int, bool, error) { return 0, false, s.failure("health") }

func TestPipelineStateFailuresAreStickyUntilPipelineRecovery(t *testing.T) {
	for _, operation := range []string{"fetch", "enqueue", "mark", "reconcile", "health"} {
		t.Run(operation, func(t *testing.T) {
			total := int64(1)
			api := &fakeAlertAPI{
				list:    ddae.AlertList{Results: []ddae.AlertListItem{{ID: "alert-1"}}, TotalRecords: &total},
				details: map[string]ddae.AlertDetail{"alert-1": {ID: "alert-1"}},
			}
			state := &failingState{fail: operation}
			if operation == "mark" {
				state.exists = true
				state.lastFetched = time.Now()
			}
			diagnostics := snapshot.NewStore()
			diagnostics.SetAlertPublisherStateHealthy(true)
			pipeline := NewPipeline(api, state, diagnostics, Options{
				SourceInstance: "site-a", Interval: time.Hour, CycleTimeout: time.Second,
				RefreshInterval: time.Hour, MaxPerCycle: 1, Concurrency: 1,
			}, nil)
			pipeline.poll(context.Background())
			if diagnostics.Load().AlertPipelineStateOK || diagnostics.Load().AlertPipelineReady {
				t.Fatalf("%s failure reported healthy: %#v", operation, diagnostics.Load())
			}
			diagnostics.SetAlertPublisherStateHealthy(false)
			state.fail = ""
			if operation == "enqueue" {
				api.detailErr = map[string]error{"alert-1": errors.New("api-canary")}
				pipeline.poll(context.Background())
				if diagnostics.Load().AlertPipelineStateOK {
					t.Fatalf("incomplete state sequence cleared sticky failure: %#v", diagnostics.Load())
				}
				api.detailErr = nil
			}
			pipeline.poll(context.Background())
			view := diagnostics.Load()
			if !view.AlertPipelineStateOK || view.AlertPipelineReady {
				t.Fatalf("pipeline recovery overwrote publisher state: %#v", view)
			}
			diagnostics.SetAlertPublisherStateHealthy(true)
			if !diagnostics.Load().AlertPipelineReady {
				t.Fatalf("independent publisher recovery did not restore readiness: %#v", diagnostics.Load())
			}
		})
	}
}
