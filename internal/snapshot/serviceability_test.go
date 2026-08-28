package snapshot

import (
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

func TestServiceabilityDiagnosticsAndReadinessAreIndependent(t *testing.T) {
	store := NewStore()
	store.RecordServiceabilityLogList(true, true, time.Second)
	store.RecordServiceabilityLogDetail(true, 2*time.Second, 3)
	store.SetServiceabilityLogCollectionReady(true)
	store.SetServiceabilityLogPipelineStateHealthy(true)
	store.SetServiceabilityLogPublisherStateHealthy(true)
	store.SetServiceabilityLogStateFull(false)
	store.RecordServiceabilityLogPublish(true, 3*time.Second, 2, "", 4)

	view := store.Load()
	if !view.ServiceabilityLogListComplete || !view.Collectors["serviceability_log_detail"].Success || view.ServiceabilityLogDeferred != 3 {
		t.Fatalf("collector diagnostics = %#v", view)
	}
	if !view.ServiceabilityLogPipelineReady || !view.ServiceabilityLogPublishSuccess || view.ServiceabilityLogPublishedTotal != 2 || view.ServiceabilityLogBuffered != 4 {
		t.Fatalf("pipeline diagnostics = %#v", view)
	}
	if !store.ReadyFor(time.Now(), time.Minute, false, false, true) {
		t.Fatal("healthy logs-only mode was not ready")
	}

	store.SetServiceabilityLogStateFull(true)
	if store.ReadyFor(time.Now(), time.Minute, false, false, true) {
		t.Fatal("full log state reported ready")
	}
	store.SetServiceabilityLogStateFull(false)
	store.SetServiceabilityLogCollectionReady(false)
	if store.ReadyFor(time.Now(), time.Minute, false, false, true) {
		t.Fatal("incomplete log collection reported ready")
	}
}

func TestServiceabilityFailuresAndLoadedMapsAreIsolated(t *testing.T) {
	store := NewStore()
	store.RecordServiceabilityLogList(false, true, time.Second)
	store.RecordServiceabilityLogDetail(false, 2*time.Second, 5)
	store.RecordServiceabilityLogPublish(false, 3*time.Second, 0, observability.ClassTimeout, 7)
	store.SetServiceabilityLogBuffered(8)

	first := store.Load()
	if first.ServiceabilityLogListComplete || first.ServiceabilityLogPublishSuccess || first.ServiceabilityLogFailedTotal[observability.ClassTimeout] != 1 || first.ServiceabilityLogBuffered != 8 {
		t.Fatalf("failure diagnostics = %#v", first)
	}
	first.ServiceabilityLogFailedTotal[observability.ClassTimeout] = 99
	if store.Load().ServiceabilityLogFailedTotal[observability.ClassTimeout] != 1 {
		t.Fatal("loaded Serviceability Log failure map mutated the store")
	}
}

func TestComponentReadinessConjunctions(t *testing.T) {
	store := NewStore()
	if store.ReadyFor(time.Now(), time.Minute, false, false) {
		t.Fatal("no enabled pipelines reported ready")
	}
	store.SetAlertCollectionReady(true)
	store.SetAlertPipelineStateHealthy(true)
	store.SetAlertPublisherStateHealthy(true)
	if !store.ReadyFor(time.Now(), time.Minute, false, true) {
		t.Fatal("healthy alert-only mode was not ready")
	}
	store.SetAlertStateFull(true)
	if store.ReadyFor(time.Now(), time.Minute, false, true) {
		t.Fatal("full alert state reported ready")
	}
	store.SetAlertStateFull(false)
	store.SetAlertPublisherStateHealthy(false)
	if store.ReadyFor(time.Now(), time.Minute, false, true) {
		t.Fatal("unhealthy alert publisher reported ready")
	}
}
