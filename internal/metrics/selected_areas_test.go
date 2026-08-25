package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

func TestSelectedMetricAreasAndNoAlertContent(t *testing.T) {
	store := snapshot.NewStore()
	now := time.Now()
	falseValue := false
	store.RecordPing(true, true, true, now, 0)
	store.RecordClusters([]snapshot.Cluster{{
		ID: "cluster-a", State: "available",
		CoordinatorCPU:    snapshot.OptionalFloat{Value: 2, Valid: true},
		CoordinatorMemory: snapshot.OptionalFloat{Value: 4 << 30, Valid: true},
		WorkerCPU:         snapshot.OptionalFloat{Value: 8, Valid: true},
		WorkerMemory:      snapshot.OptionalFloat{Value: 16 << 30, Valid: true},
	}}, true, true, now, 0)
	store.RecordNodes([]snapshot.Node{{
		ID: "node-a", State: "ready",
		CapacityCPU:       snapshot.OptionalFloat{Value: 16, Valid: true},
		AllocatableMemory: snapshot.OptionalFloat{Value: 32 << 30, Valid: true},
		DiskPressure:      &falseValue,
	}}, true, true, now, 0)
	store.RecordLock(false, true, true, now, 0)
	store.RecordPower(snapshot.Power{ControlPlaneReady: true, NodesReady: 2, TotalNodes: 3}, true, true, now, 0)
	store.CompleteRequiredCycle(now, true)
	registry, err := NewRegistry(store, time.Minute, BuildInfo{Version: "test", GoVersion: "go-test"})
	if err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(families))
	var output strings.Builder
	for _, family := range families {
		names[family.GetName()] = true
		output.WriteString(family.String())
	}
	text := output.String()
	for _, required := range []string{
		"ddae_cluster_state_info", "ddae_cluster_coordinator_configured_cpu_cores",
		"ddae_node_state_info", "ddae_node_capacity_cpu_cores", "ddae_system_locked",
		"ddae_control_plane_ready", "ddae_nodes_ready",
	} {
		if !names[required] {
			t.Errorf("missing selected metric %s", required)
		}
	}
	for _, forbidden := range []string{"alert_id", "message=", "query_latency", "_utilization", "service_health", "supportassist", "password"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Errorf("metric output contains forbidden term %q", forbidden)
		}
	}
}
