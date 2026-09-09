package contract

import (
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/serviceability"
)

func TestInformationalSeverityUsesExistingInfoContract(t *testing.T) {
	for _, source := range []string{"Informational", " informational ", "INFORMATIONAL"} {
		source := source
		alert, err := alerts.BuildEvent("site-a", "alert-1", ddae.AlertDetail{ID: "alert-1", Type: &source}, time.Unix(1, 0))
		if err != nil {
			t.Fatal(err)
		}
		logEvent, err := serviceability.BuildEvent("site-a", "log-1", ddae.ServiceabilityLogDetail{ID: "log-1", Type: &source}, time.Unix(1, 0))
		if err != nil {
			t.Fatal(err)
		}
		if alert.Event.Alert.Severity != "info" || logEvent.Event.Log.Severity != "info" {
			t.Fatalf("source %q normalized to alert=%q log=%q", source, alert.Event.Alert.Severity, logEvent.Event.Log.Severity)
		}
	}

	unsupported := "future"
	alert, err := alerts.BuildEvent("site-a", "alert-1", ddae.AlertDetail{ID: "alert-1", Type: &unsupported}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	logEvent, err := serviceability.BuildEvent("site-a", "log-1", ddae.ServiceabilityLogDetail{ID: "log-1", Type: &unsupported}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if alert.Event.Alert.Severity != "unknown" || logEvent.Event.Log.Severity != "unknown" {
		t.Fatalf("unsupported severity changed: alert=%q log=%q", alert.Event.Alert.Severity, logEvent.Event.Log.Severity)
	}
}
