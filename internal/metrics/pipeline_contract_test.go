package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func gatheredNames(t *testing.T, registry *prometheus.Registry) map[string]bool {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(families))
	for _, family := range families {
		names[family.GetName()] = true
	}
	return names
}

func TestPipelineEnableMetricAndConditionalFamilies(t *testing.T) {
	for name, mode := range map[string]PipelineMode{
		"resource-only": {ResourcesEnabled: true},
		"alert-only":    {AlertsEnabled: true},
		"log-only":      {ServiceabilityLogsEnabled: true},
		"dual":          {ResourcesEnabled: true, AlertsEnabled: true},
		"all":           {ResourcesEnabled: true, AlertsEnabled: true, ServiceabilityLogsEnabled: true},
	} {
		t.Run(name, func(t *testing.T) {
			registry, err := NewRegistry(snapshot.NewStore(), time.Minute, BuildInfo{Version: "test", GoVersion: "go-test"}, mode)
			if err != nil {
				t.Fatal(err)
			}
			expected := `
# HELP ddae_monitoring_enabled Configured monitoring pipeline is enabled (1) or disabled (0).
# TYPE ddae_monitoring_enabled gauge
ddae_monitoring_enabled{pipeline="alerts"} ` + boolText(mode.AlertsEnabled) + `
ddae_monitoring_enabled{pipeline="resources"} ` + boolText(mode.ResourcesEnabled) + `
ddae_monitoring_enabled{pipeline="serviceability_logs"} ` + boolText(mode.ServiceabilityLogsEnabled) + `
`
			if err := testutil.GatherAndCompare(registry, strings.NewReader(expected), "ddae_monitoring_enabled"); err != nil {
				t.Fatal(err)
			}
			names := gatheredNames(t, registry)
			if names["ddae_up"] != mode.ResourcesEnabled {
				t.Fatal("resource family presence did not match selection")
			}
			if names["ddae_alert_pipeline_ready"] != mode.AlertsEnabled || names["ddae_kafka_buffered_events"] != mode.AlertsEnabled {
				t.Fatal("alert family presence did not match selection")
			}
			if names["ddae_serviceability_log_pipeline_ready"] != mode.ServiceabilityLogsEnabled || names["ddae_serviceability_log_buffered_records"] != mode.ServiceabilityLogsEnabled {
				t.Fatal("serviceability log family presence did not match selection")
			}
			if !names["ddae_build_info"] || !names["ddae_monitoring_enabled"] {
				t.Fatal("always-on metric family is missing")
			}
		})
	}
}

func boolText(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
