package collector

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/metrics"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

// AC-DDAE-8-001/002: equivalent transport shapes preserve normalized values,
// metric output and the existing distinction between decode and validation.
func TestClusterResponseNormalizationAndMetrics(t *testing.T) {
	const body = `[{"id":"cluster-synthetic","clusterStatus":"Available","coordinator":{"cpu":"2000m","memory":"4Gi"},"worker":{"cpu":"8","memory":"16Gi"}}]`
	const nested = `[{"id":"cluster-synthetic","clusterStatus":{"status":"Available","message":"must-not-be-exported"},"coordinator":{"resources":{"cpu":2,"memory":"4Gi"}},"worker":{"resources":{"cpu":8,"memory":"16Gi"}}}]`
	const mixedRoles = `[{"id":"cluster-synthetic","clusterStatus":"Available","coordinator":{"cpu":"2","memory":"4Gi"},"worker":{"resources":{"cpu":8,"memory":"16Gi"}}}]`
	var baseline []snapshot.Cluster
	for _, input := range []string{body, `{"results":` + body + `}`, nested, `{"results":` + nested + `}`, mixedRoles} {
		raw, err := decodeClusterFixture([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		clusters, usable, err := normalizeClusters(raw.([]ddae.Cluster))
		if err != nil || !usable || len(clusters) != 1 {
			t.Fatalf("normalize: usable=%v error=%v value=%#v", usable, err, clusters)
		}
		if baseline != nil && !reflect.DeepEqual(baseline, clusters) {
			t.Fatal("response shape changed normalized fields")
		}
		baseline = clusters
		store := snapshot.NewStore()
		store.RecordClusters(clusters, true, true, time.Now(), 0)
		registry, err := metrics.NewRegistry(store, time.Minute, metrics.BuildInfo{}, metrics.PipelineMode{ResourcesEnabled: true})
		if err != nil {
			t.Fatal(err)
		}
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]float64{
			"ddae_cluster_coordinator_configured_cpu_cores":    2,
			"ddae_cluster_coordinator_configured_memory_bytes": 4 << 30,
			"ddae_cluster_worker_configured_cpu_cores":         8,
			"ddae_cluster_worker_configured_memory_bytes":      16 << 30,
		}
		for _, family := range families {
			if value, ok := want[family.GetName()]; ok {
				if len(family.Metric) != 1 || family.Metric[0].GetGauge().GetValue() != value {
					t.Fatalf("incorrect metric %s", family.GetName())
				}
				delete(want, family.GetName())
			}
		}
		if len(want) != 0 || clusters[0].State != "available" {
			t.Fatalf("missing metrics %v or incorrect cluster state", want)
		}
	}
}

func TestClusterResponseRetainsSemanticValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body  string
		usable, bad bool
	}{
		{"missing-id", `[{}]`, false, true},
		{"duplicate-id", `[{"id":"same"},{"id":"same"}]`, false, true},
		{"missing-optional", `[{"id":"c","clusterStatus":"future"}]`, true, false},
		{"invalid-cpu", `[{"id":"c","coordinator":{"cpu":"broken"}}]`, true, true},
		{"nested-missing-optional", `[{"id":"c","clusterStatus":{"status":"future"},"worker":{"resources":{}}}]`, true, false},
		{"nested-invalid-cpu", `[{"id":"c","coordinator":{"resources":{"cpu":"broken"}}}]`, true, true},
		{"nested-invalid-memory", `[{"id":"c","worker":{"resources":{"memory":"broken"}}}]`, true, true},
		{"invalid-memory", `[{"id":"c","worker":{"memory":"broken"}}]`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, input := range []string{tc.body, `{"results":` + tc.body + `}`} {
				raw, err := decodeClusterFixture([]byte(input))
				if err != nil {
					t.Fatal(err)
				}
				_, usable, err := normalizeClusters(raw.([]ddae.Cluster))
				if usable != tc.usable || (err != nil) != tc.bad {
					t.Fatalf("usable=%v error=%v", usable, err)
				}
			}
		})
	}
}

// Transport shape rejection is covered by internal/ddae client tests.
func decodeClusterFixture(body []byte) (any, error) {
	var envelope struct {
		Results json.RawMessage `json:"results"`
	}
	if len(body) > 0 && body[0] == '{' {
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, err
		}
		body = envelope.Results
	}
	var clusters []ddae.Cluster
	err := json.Unmarshal(body, &clusters)
	return clusters, err
}
