//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/kafka"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/outbox"
)

func TestAuthorizedIntegration(t *testing.T) {
	if os.Getenv("DDAE_INTEGRATION_ENABLED") != "1" {
		t.Fatal("authorized integration environment is not enabled")
	}
	if os.Getenv("DDAE_TEST_SOFTWARE_VERSION") != "1.5.0" {
		t.Fatal("DDAE_TEST_SOFTWARE_VERSION must identify the authorized DDAE 1.5.0 environment")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("authorized integration configuration is invalid: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.CycleTimeout)
	defer cancel()
	client, err := ddae.NewClient(cfg)
	if err != nil {
		t.Fatalf("create DDAE client: %v", err)
	}
	defer client.CloseIdleConnections()
	if _, err := client.Ping(ctx); err != nil {
		t.Fatalf("DDAE ping failed with bounded error: %v", err)
	}
	if _, err := client.Clusters(ctx); err != nil {
		t.Fatalf("DDAE cluster GET failed with bounded error: %v", err)
	}
	if _, err := client.Nodes(ctx); err != nil {
		t.Fatalf("DDAE node GET failed with bounded error: %v", err)
	}
	if _, err := client.Lock(ctx); err != nil {
		t.Fatalf("DDAE lock GET failed with bounded error: %v", err)
	}
	if _, err := client.Power(ctx); err != nil {
		t.Fatalf("DDAE power GET failed with bounded error: %v", err)
	}
	list, err := client.AlertList(ctx)
	if err != nil {
		t.Fatalf("DDAE alert-list GET failed with bounded error: %v", err)
	}
	if list.TotalRecords == nil || *list.TotalRecords != int64(len(list.Results)) || len(list.Results) == 0 {
		t.Fatal("authorized fixture must provide one complete, non-empty alert list without retaining its contents")
	}
	id := list.Results[0].ID
	detail, err := client.AlertDetail(ctx, id)
	if err != nil {
		t.Fatalf("DDAE alert-detail GET failed with bounded error: %v", err)
	}
	event, err := alerts.BuildEvent(cfg.SourceInstance, id, detail, time.Now())
	if err != nil {
		t.Fatalf("typed alert normalization failed with bounded error: %v", err)
	}
	producer, err := kafka.NewProducer(cfg)
	if err != nil {
		t.Fatalf("create isolated Kafka producer: %v", err)
	}
	defer producer.Close()
	if err := producer.Publish(ctx, outbox.Record{RecordKey: event.RecordKey, Payload: event.Payload}); err != nil {
		t.Fatalf("isolated Kafka publish failed with bounded error: %v", err)
	}
}
