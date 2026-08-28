package server

import (
	"net/http"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

func TestPersistentStateReadinessComponentsAreIndependentAndSticky(t *testing.T) {
	state := snapshot.NewStore()
	state.SetAlertCollectionReady(true)
	state.SetAlertPipelineStateHealthy(true)
	state.SetAlertPublisherStateHealthy(true)
	if status := readyStatus(t, state, PipelineMode{AlertsEnabled: true}); status != http.StatusOK {
		t.Fatalf("initial status=%d", status)
	}

	state.SetAlertPipelineStateHealthy(false)
	state.SetAlertPublisherStateHealthy(false)
	state.SetAlertPublisherStateHealthy(true)
	if status := readyStatus(t, state, PipelineMode{AlertsEnabled: true}); status != http.StatusServiceUnavailable {
		t.Fatalf("publisher overwrote pipeline failure: status=%d", status)
	}
	state.SetAlertPipelineStateHealthy(true)
	if status := readyStatus(t, state, PipelineMode{AlertsEnabled: true}); status != http.StatusOK {
		t.Fatalf("pipeline recovery status=%d", status)
	}

	state.SetAlertPublisherStateHealthy(false)
	state.SetAlertCollectionReady(false)
	state.SetAlertCollectionReady(true)
	state.SetAlertPipelineStateHealthy(true)
	if status := readyStatus(t, state, PipelineMode{AlertsEnabled: true}); status != http.StatusServiceUnavailable {
		t.Fatalf("pipeline overwrote publisher failure: status=%d", status)
	}
	state.SetAlertPublisherStateHealthy(true)
	state.SetAlertStateFull(true)
	if status := readyStatus(t, state, PipelineMode{AlertsEnabled: true}); status != http.StatusServiceUnavailable {
		t.Fatalf("full state reported ready: status=%d", status)
	}
	state.SetAlertStateFull(false)
	if status := readyStatus(t, state, PipelineMode{AlertsEnabled: true}); status != http.StatusOK {
		t.Fatalf("full-state recovery status=%d", status)
	}
}
