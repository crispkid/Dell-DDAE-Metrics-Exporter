package kafka

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/outbox"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/twmb/franz-go/pkg/kerr"
)

func TestProducerConfigurationIsBoundedAndTLSValidated(t *testing.T) {
	base := config.Config{
		KafkaBrokers: []string{"127.0.0.1:1"}, KafkaTopic: "test-topic",
		KafkaClientID: "test-client", KafkaPublishTimeout: time.Second,
	}
	for _, mechanism := range []string{"", "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"} {
		cfg := base
		cfg.KafkaSASLMechanism = mechanism
		producer, err := NewProducer(cfg)
		if err != nil {
			t.Fatalf("NewProducer %q: %v", mechanism, err)
		}
		producer.Close()
	}
	badCA := filepath.Join(t.TempDir(), "bad-ca.pem")
	if err := os.WriteFile(badCA, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := base
	cfg.KafkaCAFile = badCA
	if _, err := NewProducer(cfg); err == nil {
		t.Fatal("invalid Kafka CA was accepted")
	}
	cfg.KafkaCAFile = ""
	cfg.KafkaClientCertFile = badCA
	cfg.KafkaClientKeyFile = badCA
	if _, err := NewProducer(cfg); err == nil {
		t.Fatal("invalid Kafka client certificate pair was accepted")
	}
}

func TestPublisherFailurePathsKeepRecordsAndBoundLogs(t *testing.T) {
	for name, store := range map[string]*fakeOutbox{
		"records": {recordsErr: errors.New("state-canary")},
		"health":  {healthErr: errors.New("health-canary")},
		"ack":     {records: []outbox.Record{{Sequence: 1}}, ackErr: errors.New("ack-canary")},
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			diagnostics := snapshot.NewStore()
			publisher := NewPublisher(fakeProducer{}, store, diagnostics, slog.New(slog.NewJSONHandler(&output, nil)))
			publisher.flush(context.Background())
			if diagnostics.Load().KafkaFailedTotal[observability.ClassInternal] != 1 {
				t.Fatalf("diagnostics = %#v", diagnostics.Load())
			}
			for _, canary := range []string{"state-canary", "health-canary", "ack-canary"} {
				if bytes.Contains(output.Bytes(), []byte(canary)) {
					t.Fatalf("log leaked error canary: %s", output.String())
				}
			}
		})
	}
}

func TestPublisherRunStopsAndClassifiedErrorsAreStable(t *testing.T) {
	diagnostics := snapshot.NewStore()
	publisher := NewPublisher(fakeProducer{}, &fakeOutbox{}, diagnostics, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { publisher.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher did not stop")
	}
	for class, err := range map[observability.Class]error{
		observability.ClassKafkaTimeout:  Error{class: observability.ClassKafkaTimeout},
		observability.ClassKafkaRejected: Error{class: observability.ClassKafkaRejected},
	} {
		if observability.Classify(err) != class || err.Error() == "" {
			t.Fatalf("classification for %s = %s", class, observability.Classify(err))
		}
	}
	deadlineContext, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if got := publishFailureClass(errors.New("timeout-canary"), deadlineContext); got != observability.ClassKafkaTimeout {
		t.Fatalf("timeout class = %s", got)
	}
	if got := publishFailureClass(kerr.SaslAuthenticationFailed, context.Background()); got != observability.ClassKafkaAuth {
		t.Fatalf("authentication class = %s", got)
	}
	if got := publishFailureClass(errors.New("broker-canary"), context.Background()); got != observability.ClassKafkaRejected {
		t.Fatalf("rejected class = %s", got)
	}
}
