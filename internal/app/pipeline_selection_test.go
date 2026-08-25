package app

import (
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
)

func pipelineTestConfig(t *testing.T) config.Config {
	t.Helper()
	base, err := url.Parse("https://ddae.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return config.Config{
		ResourceCollectionInterval:  time.Minute,
		AlertCollectionInterval:     2 * time.Minute,
		DDAEBaseURL:                 base,
		SourceInstance:              "site-a",
		ListenAddress:               "127.0.0.1:0",
		RequestTimeout:              time.Second,
		CycleTimeout:                5 * time.Second,
		ResponseMaxBytes:            1024,
		RetryMax:                    0,
		StaleAfter:                  3 * time.Minute,
		AlertListResponseMaxBytes:   1024,
		AlertDetailResponseMaxBytes: 1024,
		AlertDetailRefreshInterval:  10 * time.Minute,
		AlertDetailMaxPerCycle:      10,
		AlertDetailConcurrency:      2,
		KafkaBrokers:                []string{"127.0.0.1:1"},
		KafkaTopic:                  "test",
		KafkaClientID:               "test",
		KafkaPublishTimeout:         time.Second,
		StateDir:                    filepath.Join(t.TempDir(), "state"),
		KafkaOutboxMaxBytes:         1 << 20,
		KafkaOutboxMaxEvents:        10,
		CheckpointRetention:         time.Hour,
		CheckpointMaxAlerts:         10,
		ShutdownGracePeriod:         time.Second,
	}
}

func TestNewConstructsOnlySelectedPipeline(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for name, mode := range map[string]struct {
		resources bool
		alerts    bool
	}{
		"resource-only": {resources: true},
		"alert-only":    {alerts: true},
		"dual":          {resources: true, alerts: true},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := pipelineTestConfig(t)
			cfg.ResourceMonitoringEnabled = mode.resources
			cfg.AlertMonitoringEnabled = mode.alerts
			if !mode.alerts {
				cfg.StateDir = ""
				cfg.KafkaBrokers = nil
				cfg.KafkaTopic = ""
			}
			application, err := New(cfg, logger, BuildInfo{Version: "test"})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if (application.manager != nil) != mode.resources {
				t.Fatal("resource manager construction did not match selection")
			}
			if (application.alerts != nil) != mode.alerts || (application.publisher != nil) != mode.alerts {
				t.Fatal("alert worker construction did not match selection")
			}
			if (application.producer != nil) != mode.alerts || (application.outbox != nil) != mode.alerts {
				t.Fatal("Kafka/state construction did not match alert selection")
			}
			if application.producer != nil {
				application.producer.Close()
			}
			application.ddae.CloseIdleConnections()
			if application.outbox != nil {
				if err := application.outbox.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestNewRejectsBothPipelinesDisabled(t *testing.T) {
	cfg := pipelineTestConfig(t)
	cfg.ResourceMonitoringEnabled = false
	cfg.AlertMonitoringEnabled = false
	if _, err := New(cfg, slog.Default(), BuildInfo{}); err == nil {
		t.Fatal("both-disabled application was accepted")
	}
}
