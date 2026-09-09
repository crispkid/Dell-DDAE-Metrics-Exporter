package app

import (
	"context"
	"fmt"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/querystate"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQueryOnlyLifecycle(t *testing.T) {
	for k, v := range map[string]string{"QUERY_ENABLED": "true", "QUERY_BASE_URL": "https://engine.invalid", "QUERY_AUTH_URL": "https://auth.invalid", "QUERY_REALM": "ddae", "QUERY_ROLE": "monitoring", "QUERY_USERNAME": "synthetic", "QUERY_PASSWORD": "synthetic", "DDAE_RESOURCE_MONITORING_ENABLED": "false", "DDAE_ALERT_MONITORING_ENABLED": "false", "DDAE_SOURCE_INSTANCE": "test", "STATE_DIR": t.TempDir()} {
		t.Setenv(k, v)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ListenAddress = "127.0.0.1:0"
	a, err := New(cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)), BuildInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if a.ddae != nil || a.producer != nil || a.outbox != nil || a.query == nil {
		t.Fatal("query-only resource isolation failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	state, err := querystate.Open(querystate.Options{Dir: cfg.StateDir, Source: "test", MaxEvents: 10, MaxCheckpoints: 100, MaxBytes: 1 << 20, Retention: time.Hour})
	if err != nil {
		t.Fatal("state lock leaked", err)
	}
	state.Close()
}

// TEST-DDAE-10-001 and 006: disabled creates no history DB; enabled cancels and releases it.
func TestBackfillAppLifecycle(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			for k, v := range map[string]string{"QUERY_ENABLED": "true", "QUERY_BASE_URL": "https://engine.invalid", "QUERY_AUTH_URL": "https://auth.invalid", "QUERY_REALM": "ddae", "QUERY_ROLE": "monitoring", "QUERY_USERNAME": "synthetic", "QUERY_PASSWORD": "synthetic", "DDAE_RESOURCE_MONITORING_ENABLED": "false", "DDAE_ALERT_MONITORING_ENABLED": "false", "DDAE_SOURCE_INSTANCE": "test", "STATE_DIR": t.TempDir(), "QUERY_BACKFILL_ENABLED": fmt.Sprint(enabled)} {
				t.Setenv(k, v)
			}
			cfg, e := config.Load()
			if e != nil {
				t.Fatal(e)
			}
			cfg.ListenAddress = "127.0.0.1:0"
			a, e := New(cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)), BuildInfo{})
			if e != nil {
				t.Fatal(e)
			}
			if (a.historyState != nil) != enabled {
				t.Fatal("unexpected history state")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if e = a.Run(ctx); e != nil {
				t.Fatal(e)
			}
			if enabled {
				s, e := historystate.Open(cfg.StateDir)
				if e != nil {
					t.Fatal("state lock leaked", e)
				}
				s.Close()
			} else {
				if _, e := os.Stat(filepath.Join(cfg.StateDir, "history-backfill.db")); !os.IsNotExist(e) {
					t.Fatal("disabled created DB")
				}
			}
		})
	}
}
