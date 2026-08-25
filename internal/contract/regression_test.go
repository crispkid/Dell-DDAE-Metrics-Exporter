package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/metrics"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ddae-1.5.0", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestDDAE150TypedFixtureAndKafkaGoldenContract(t *testing.T) {
	var detail ddae.AlertDetail
	if err := json.Unmarshal(fixture(t, "alert-detail.json"), &detail); err != nil {
		t.Fatal(err)
	}
	encoded, err := alerts.BuildEvent("fixture-site", "alert-1", detail, time.Date(2026, 8, 24, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	want := bytes.TrimSpace(fixture(t, "alert-event.golden.json"))
	if !bytes.Equal(encoded.Payload, want) {
		t.Fatalf("Kafka event changed\nwant: %s\n got: %s", want, encoded.Payload)
	}
	for _, excluded := range []string{"excluded-synthetic-value", "synthetic.example.invalid", "synthetic-contact@example.invalid", "labels", "links", "unknownNested"} {
		if bytes.Contains(encoded.Payload, []byte(excluded)) {
			t.Fatalf("golden event contains excluded field/value %q", excluded)
		}
	}
}

func TestDDAE150TypedMetricFixtureContract(t *testing.T) {
	var clusters []ddae.Cluster
	if err := json.Unmarshal(fixture(t, "clusters.json"), &clusters); err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 2 || clusters[0].ID != "cluster-1" || clusters[0].Coordinator.CPU == nil {
		t.Fatalf("typed fixture = %#v", clusters)
	}
	now := time.Now()
	store := snapshot.NewStore()
	store.RecordClusters([]snapshot.Cluster{{
		ID: "cluster-1", State: "available",
		CoordinatorCPU:    snapshot.OptionalFloat{Value: 2, Valid: true},
		CoordinatorMemory: snapshot.OptionalFloat{Value: 4 * 1024 * 1024 * 1024, Valid: true},
		WorkerCPU:         snapshot.OptionalFloat{Value: 8, Valid: true},
		WorkerMemory:      snapshot.OptionalFloat{Value: 16 * 1024 * 1024 * 1024, Valid: true},
	}}, true, true, now, 0)
	registry, err := metrics.NewRegistry(store, time.Minute, metrics.BuildInfo{Version: "fixture", GoVersion: "go-test"})
	if err != nil {
		t.Fatal(err)
	}
	want := `
# HELP ddae_cluster_coordinator_configured_cpu_cores Configured DDAE coordinator CPU cores, not usage.
# TYPE ddae_cluster_coordinator_configured_cpu_cores gauge
ddae_cluster_coordinator_configured_cpu_cores{cluster="cluster-1"} 2
# HELP ddae_cluster_worker_configured_memory_bytes Memory quantity returned by the cluster worker configuration object, converted to bytes; no aggregate or utilization meaning is implied.
# TYPE ddae_cluster_worker_configured_memory_bytes gauge
ddae_cluster_worker_configured_memory_bytes{cluster="cluster-1"} 1.7179869184e+10
`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(want),
		"ddae_cluster_coordinator_configured_cpu_cores", "ddae_cluster_worker_configured_memory_bytes"); err != nil {
		t.Fatal(err)
	}
}

func TestDDAE150MalformedFixtureRemainsInvalid(t *testing.T) {
	var ping ddae.PingResponse
	if err := json.Unmarshal(fixture(t, "malformed.json"), &ping); err == nil {
		t.Fatal("malformed fixture was accepted")
	}
}
