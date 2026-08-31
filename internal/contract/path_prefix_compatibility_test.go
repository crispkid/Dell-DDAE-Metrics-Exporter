package contract

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestRC2PathPrefixesPreserveRoutesAndAlertEventContract(t *testing.T) {
	operations, err := ddae.ApprovedOperationsForPrefixes("/rest/v1", "/rest/v1")
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := map[string]string{
		"ping":                      "/rest/v1/ping",
		"clusters":                  "/rest/v1/ddae-clusters",
		"nodes":                     "/rest/v1/infrastructure-nodes",
		"lock":                      "/rest/v1/system-lock",
		"power":                     "/rest/v1/system-shutdown",
		"alert_list":                "/rest/v1/serviceability-issues",
		"alert_detail":              "/rest/v1/serviceability-issues/{id}",
		"serviceability_log_list":   "/rest/v1/serviceability-events",
		"serviceability_log_detail": "/rest/v1/serviceability-events/{id}",
	}
	if len(operations) != len(wantPaths) {
		t.Fatalf("operations = %#v", operations)
	}
	for _, operation := range operations {
		if operation.Method != "GET" || wantPaths[operation.Collector] != operation.Path {
			t.Fatalf("RC2 operation = %#v", operation)
		}
	}

	var detail ddae.AlertDetail
	if err := json.Unmarshal(fixture(t, "alert-detail.json"), &detail); err != nil {
		t.Fatal(err)
	}
	encoded, err := alerts.BuildEvent(
		"fixture-site",
		"alert-1",
		detail,
		time.Date(2026, 8, 24, 3, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := bytes.TrimSpace(fixture(t, "alert-event.golden.json")); !bytes.Equal(encoded.Payload, want) {
		t.Fatalf("RC2-compatible prefixes changed the alert event\nwant: %s\n got: %s", want, encoded.Payload)
	}
}
