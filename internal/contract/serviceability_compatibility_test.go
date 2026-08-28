package contract

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/metrics"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestDisabledServiceabilityLogsPreserveAlertBytesAndAddOneEnableSeries(t *testing.T) {
	var detail ddae.AlertDetail
	if err := json.Unmarshal(fixture(t, "alert-detail.json"), &detail); err != nil {
		t.Fatal(err)
	}
	event, err := alerts.BuildEvent("fixture-site", "alert-1", detail, time.Date(2026, 8, 24, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(event.Payload, bytes.TrimSpace(fixture(t, "alert-event.golden.json"))) {
		t.Fatal("existing alert event changed")
	}
	registry, err := metrics.NewRegistry(snapshot.NewStore(), time.Minute, metrics.BuildInfo{Version: "test", GoVersion: "go-test"}, metrics.PipelineMode{ResourcesEnabled: true, AlertsEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `
# HELP ddae_monitoring_enabled Configured monitoring pipeline is enabled (1) or disabled (0).
# TYPE ddae_monitoring_enabled gauge
ddae_monitoring_enabled{pipeline="alerts"} 1
ddae_monitoring_enabled{pipeline="resources"} 1
ddae_monitoring_enabled{pipeline="serviceability_logs"} 0
`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(want), "ddae_monitoring_enabled"); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if strings.HasPrefix(family.GetName(), "ddae_serviceability_log_") {
			t.Fatalf("disabled log family emitted: %s", family.GetName())
		}
	}
}
