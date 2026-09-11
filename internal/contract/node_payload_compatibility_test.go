package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/collector"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/metrics"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type nodeCompatibilityAPI struct {
	nodes []ddae.InfrastructureNode
}

func (a nodeCompatibilityAPI) Ping(context.Context) (ddae.PingResponse, error) {
	return ddae.PingResponse{Status: "ok"}, nil
}
func (a nodeCompatibilityAPI) Clusters(context.Context) ([]ddae.Cluster, error) {
	return []ddae.Cluster{}, nil
}
func (a nodeCompatibilityAPI) Nodes(context.Context) ([]ddae.InfrastructureNode, error) {
	return a.nodes, nil
}
func (a nodeCompatibilityAPI) Lock(context.Context) (ddae.LockResponse, error) {
	var result ddae.LockResponse
	_ = json.Unmarshal([]byte(`{"status":false}`), &result)
	return result, nil
}
func (a nodeCompatibilityAPI) Power(context.Context) (ddae.PowerResponse, error) {
	ready, nodesReady, totalNodes := true, int64(1), int64(1)
	return ddae.PowerResponse{ControlPlaneReady: &ready, NodesReady: &nodesReady, TotalNodes: &totalNodes}, nil
}

func TestDocumentedAndLegacyNodePayloadsProduceIdenticalMetrics(t *testing.T) {
	const documented = `{
		"results":[{
			"id":"node-equivalent","state":"Ready",
			"capacity":{"cpu":12,"memory":"20Gi","ephemeralStorage":"100Gi"},
			"allocatable":{"cpu":10,"memory":"16Gi","ephemeralStorage":"80Gi"},
			"conditions":{"diskPressure":"False","memoryPressure":"True"},
			"address":"192.0.2.20","excluded":"excluded-node-contract-canary"
		}]
	}`
	const legacy = `[{
		"id":"node-equivalent","state":"Ready",
		"capacity":{"cpu":"12","memory":"20Gi","ephemeral-storage":"100Gi"},
		"allocatable":{"cpu":"10","memory":"16Gi","ephemeral-storage":"80Gi"},
		"conditions":[{"type":"DiskPressure","status":"False"},{"type":"MemoryPressure","status":"True"}]
	}]`

	var documentedEnvelope struct {
		Results []ddae.InfrastructureNode `json:"results"`
	}
	if err := json.Unmarshal([]byte(documented), &documentedEnvelope); err != nil {
		t.Fatal(err)
	}
	var legacyNodes []ddae.InfrastructureNode
	if err := json.Unmarshal([]byte(legacy), &legacyNodes); err != nil {
		t.Fatal(err)
	}
	documentedMetrics := gatherNodeMetrics(t, documentedEnvelope.Results)
	legacyMetrics := gatherNodeMetrics(t, legacyNodes)
	if documentedMetrics != legacyMetrics {
		t.Fatalf("node metrics differ\ndocumented:\n%s\nlegacy:\n%s", documentedMetrics, legacyMetrics)
	}
	if strings.Contains(documentedMetrics, "excluded-node-contract-canary") || strings.Contains(documentedMetrics, "192.0.2.20") {
		t.Fatalf("node metrics leaked excluded input: %s", documentedMetrics)
	}
}

func gatherNodeMetrics(t *testing.T, nodes []ddae.InfrastructureNode) string {
	t.Helper()
	store := snapshot.NewStore()
	manager := collector.NewManager(nodeCompatibilityAPI{nodes: nodes}, store, time.Second, time.Hour, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		manager.Run(ctx)
		close(done)
	}()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for store.Load().LastCompleteAt.IsZero() {
		select {
		case <-deadline.C:
			cancel()
			<-done
			t.Fatal("resource collection did not complete")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	<-done

	registry, err := metrics.NewRegistry(store, time.Minute, metrics.BuildInfo{Version: "test", GoVersion: "go-test"}, metrics.PipelineMode{ResourcesEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for _, family := range families {
		if strings.HasPrefix(family.GetName(), "ddae_node_") {
			fmt.Fprintln(&output, family.String())
		}
	}
	return output.String()
}

// TEST-DDAE-12-004: represented null is invalid in both supported layouts.
func TestAuditNullNodePressureDegradesCollector(t *testing.T) {
	for _, conditions := range []string{`{"diskPressure":null}`, `{"memoryPressure":null}`, `[{"type":"DiskPressure","status":null}]`, `{}`} {
		t.Run(conditions, func(t *testing.T) {
			var node ddae.InfrastructureNode
			if err := json.Unmarshal([]byte(`{"id":"one","state":"Ready","capacity":{"cpu":2},"conditions":`+conditions+`}`), &node); err != nil {
				t.Fatal(err)
			}
			store := snapshot.NewStore()
			manager := collector.NewManager(nodeCompatibilityAPI{nodes: []ddae.InfrastructureNode{node}}, store, time.Second, time.Hour, nil)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { manager.Run(ctx); close(done) }()
			t.Cleanup(func() { cancel(); <-done })
			deadline := time.After(time.Second)
			for !store.Load().Nodes.Present || !store.Load().Ping.Present || !store.Load().Clusters.Present || !store.Load().Lock.Present || !store.Load().Power.Present {
				select {
				case <-deadline:
					t.Fatal("collection timed out")
				case <-time.After(time.Millisecond):
				}
			}
			cancel()
			<-done
			view := store.Load()
			want := conditions == `{}`
			if view.Collectors["nodes"].Success != want || store.ReadyFor(time.Now(), time.Minute, true, false) != want {
				t.Fatalf("null readiness/collector mismatch: success=%v", view.Collectors["nodes"].Success)
			}
			if !view.Nodes.Present || len(view.Nodes.Data) != 1 {
				t.Fatal("lost usable node data")
			}
		})
	}
}
