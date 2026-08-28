package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceabilityLogOnlyConstructsNoAlertDependencies(t *testing.T) {
	cfg := pipelineTestConfig(t)
	cfg.ResourceMonitoringEnabled = false
	cfg.AlertMonitoringEnabled = false
	cfg.ServiceabilityLogMonitoringEnabled = true
	application, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), BuildInfo{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if application.manager != nil || application.alerts != nil || application.publisher != nil || application.producer != nil || application.outbox != nil {
		t.Fatal("disabled resource/alert dependency was constructed")
	}
	if application.logs == nil || application.logPublisher == nil || application.logProducer == nil || application.logOutbox == nil {
		t.Fatal("enabled serviceability log dependency is missing")
	}
	if _, err := os.Stat(filepath.Join(cfg.StateDir, "serviceability-logs.db")); err != nil {
		t.Fatalf("dedicated state: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.StateDir, "state.db")); !os.IsNotExist(err) {
		t.Fatalf("disabled alert state exists: %v", err)
	}
	application.logProducer.Close()
	application.ddae.CloseIdleConnections()
	if err := application.logOutbox.Close(); err != nil {
		t.Fatal(err)
	}
}
