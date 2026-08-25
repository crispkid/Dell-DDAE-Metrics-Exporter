package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/metrics"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

func readyStatus(t *testing.T, state *snapshot.Store, mode PipelineMode) int {
	t.Helper()
	registry, err := metrics.NewRegistry(
		state, time.Minute, metrics.BuildInfo{},
		metrics.PipelineMode{ResourcesEnabled: mode.ResourcesEnabled, AlertsEnabled: mode.AlertsEnabled},
	)
	if err != nil {
		t.Fatal(err)
	}
	applicationServer := New("127.0.0.1:0", registry, state, time.Minute, mode)
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	applicationServer.http.Handler.ServeHTTP(response, request)
	return response.Code
}

func resourceStateAt(collectedAt time.Time) *snapshot.Store {
	state := snapshot.NewStore()
	state.RecordPing(true, true, true, collectedAt, time.Millisecond)
	state.RecordClusters(nil, true, true, collectedAt, time.Millisecond)
	state.RecordNodes(nil, true, true, collectedAt, time.Millisecond)
	state.RecordLock(false, true, true, collectedAt, time.Millisecond)
	state.RecordPower(snapshot.Power{}, true, true, collectedAt, time.Millisecond)
	state.CompleteRequiredCycle(collectedAt, true)
	return state
}

func currentResourceState() *snapshot.Store { return resourceStateAt(time.Now()) }

func TestReadinessUsesOnlyEnabledPipelines(t *testing.T) {
	resourceState := currentResourceState()
	if status := readyStatus(t, resourceState, PipelineMode{ResourcesEnabled: true}); status != http.StatusOK {
		t.Fatalf("resource-only status = %d", status)
	}

	alertState := snapshot.NewStore()
	alertState.SetAlertPipelineReady(true)
	if status := readyStatus(t, alertState, PipelineMode{AlertsEnabled: true}); status != http.StatusOK {
		t.Fatalf("alert-only status = %d", status)
	}

	if status := readyStatus(t, currentResourceState(), PipelineMode{ResourcesEnabled: true, AlertsEnabled: true}); status != http.StatusServiceUnavailable {
		t.Fatalf("dual status without alert readiness = %d", status)
	}
	dualState := currentResourceState()
	dualState.SetAlertPipelineReady(true)
	if status := readyStatus(t, dualState, PipelineMode{ResourcesEnabled: true, AlertsEnabled: true}); status != http.StatusOK {
		t.Fatalf("dual healthy status = %d", status)
	}
	if status := readyStatus(t, snapshot.NewStore(), PipelineMode{}); status != http.StatusServiceUnavailable {
		t.Fatalf("both-disabled status = %d", status)
	}
	if status := readyStatus(t, resourceStateAt(time.Now().Add(-2*time.Minute)), PipelineMode{ResourcesEnabled: true}); status != http.StatusServiceUnavailable {
		t.Fatalf("stale resource-only status = %d", status)
	}
	failedResource := currentResourceState()
	failedResource.RecordNodes(nil, false, false, time.Now(), time.Millisecond)
	if status := readyStatus(t, failedResource, PipelineMode{ResourcesEnabled: true}); status != http.StatusServiceUnavailable {
		t.Fatalf("failed resource-only status = %d", status)
	}
}
