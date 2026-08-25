package snapshot

import (
	"testing"
	"time"
)

func TestFailedFamilyKeepsPriorTimestampAndReportsFailure(t *testing.T) {
	store := NewStore()
	old := time.Unix(100, 0)
	current := old.Add(time.Minute)
	store.RecordClusters([]Cluster{{ID: "prior", State: "available"}}, true, true, old, time.Second)
	store.CompleteRequiredCycle(old, true)
	store.RecordClusters(nil, false, false, current, 2*time.Second)
	store.RecordNodes([]Node{{ID: "current", State: "ready"}}, true, true, current, time.Second)
	store.CompleteRequiredCycle(current, false)
	view := store.Load()
	if view.Clusters.CollectedAt != old || len(view.Clusters.Data) != 1 || view.Collectors["clusters"].Success {
		t.Fatalf("failed family state = %#v", view)
	}
	if view.LastCompleteAt != old || !view.Collectors["nodes"].Success {
		t.Fatalf("partial cycle state = %#v", view)
	}
}

func TestLoadReturnsIndependentCollectionsAndDiagnostics(t *testing.T) {
	store := NewStore()
	store.RecordClusters([]Cluster{{ID: "safe"}}, true, true, time.Now(), 0)
	store.RecordKafkaPublish(false, time.Second, 0, "internal", 3)
	first := store.Load()
	first.Clusters.Data[0].ID = "changed"
	first.Collectors["clusters"] = CollectorStatus{}
	first.KafkaFailedTotal["internal"] = 99
	second := store.Load()
	if second.Clusters.Data[0].ID != "safe" || !second.Collectors["clusters"].Success || second.KafkaFailedTotal["internal"] != 1 {
		t.Fatalf("store view was mutable: %#v", second)
	}
}

func TestAlertAndKafkaDiagnosticSetters(t *testing.T) {
	store := NewStore()
	store.RecordAlertList(true, true, time.Second)
	store.RecordAlertDetail(true, 2*time.Second, 4)
	store.SetKafkaBuffered(5)
	view := store.Load()
	if !view.AlertListComplete || !view.Collectors["alert_detail"].Success || view.AlertDeferred != 4 || view.KafkaBuffered != 5 {
		t.Fatalf("diagnostics = %#v", view)
	}
}
