package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricContractAndBoundedStates(t *testing.T) {
	store := snapshot.NewStore()
	now := time.Now()
	store.RecordPing(true, true, true, now, time.Millisecond)
	store.RecordClusters([]snapshot.Cluster{{
		ID: "cluster-1", State: "available",
		CoordinatorCPU: snapshot.OptionalFloat{Value: 2, Valid: true},
	}}, true, true, now, time.Millisecond)
	store.RecordNodes([]snapshot.Node{{ID: "node-1", State: "ready"}}, true, true, now, time.Millisecond)
	store.RecordLock(false, true, true, now, time.Millisecond)
	store.RecordPower(snapshot.Power{ControlPlaneReady: true, NodesReady: 2, TotalNodes: 3}, true, true, now, time.Millisecond)
	store.CompleteRequiredCycle(now, true)
	registry, err := NewRegistry(store, 2*time.Minute, BuildInfo{Version: "test", GoVersion: "go-test"})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	expected := `
# HELP ddae_up 1 only when the approved target-success policy is satisfied by a current snapshot; otherwise 0.
# TYPE ddae_up gauge
ddae_up 1
# HELP ddae_cluster_state_info One-hot state using available or unknown; cluster is the Dell cluster id.
# TYPE ddae_cluster_state_info gauge
ddae_cluster_state_info{cluster="cluster-1",state="available"} 1
ddae_cluster_state_info{cluster="cluster-1",state="unknown"} 0
# HELP ddae_cluster_coordinator_configured_cpu_cores Configured DDAE coordinator CPU cores, not usage.
# TYPE ddae_cluster_coordinator_configured_cpu_cores gauge
ddae_cluster_coordinator_configured_cpu_cores{cluster="cluster-1"} 2
`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(expected),
		"ddae_up", "ddae_cluster_state_info", "ddae_cluster_coordinator_configured_cpu_cores"); err != nil {
		t.Fatal(err)
	}
}

func TestStaleTargetFamiliesAreWithheld(t *testing.T) {
	store := snapshot.NewStore()
	old := time.Now().Add(-time.Hour)
	store.RecordClusters([]snapshot.Cluster{{ID: "stale", State: "available"}}, true, false, old, time.Millisecond)
	registry, err := NewRegistry(store, time.Minute, BuildInfo{Version: "test", GoVersion: "go-test"})
	if err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "ddae_cluster_state_info" {
			t.Fatal("stale cluster family was exposed")
		}
	}
}
