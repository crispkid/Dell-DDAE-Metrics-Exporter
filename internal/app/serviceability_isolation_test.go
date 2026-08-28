package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestAlertAndServiceabilityLogStateFilesAreIsolated(t *testing.T) {
	cfg := pipelineTestConfig(t)
	cfg.ResourceMonitoringEnabled = false
	cfg.AlertMonitoringEnabled = true
	cfg.ServiceabilityLogMonitoringEnabled = true
	application, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), BuildInfo{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"state.db", "serviceability-logs.db"} {
		info, statErr := os.Stat(filepath.Join(cfg.StateDir, name))
		if statErr != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s is not an independent protected state file: info=%v err=%v", name, info, statErr)
		}
	}
	application.producer.Close()
	application.logProducer.Close()
	application.ddae.CloseIdleConnections()
	if err := application.outbox.Close(); err != nil {
		t.Fatal(err)
	}
	if err := application.logOutbox.Close(); err != nil {
		t.Fatal(err)
	}
}
