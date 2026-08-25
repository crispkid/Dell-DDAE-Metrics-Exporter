package app

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/kafka"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/outbox"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type fakeWorker struct {
	started chan struct{}
	stop    chan struct{}
	once    sync.Once
}

func (w *fakeWorker) Run(ctx context.Context) {
	w.once.Do(func() { close(w.started) })
	if w.stop == nil {
		<-ctx.Done()
		return
	}
	<-w.stop
}

type fakeServer struct {
	stopped chan struct{}
	once    sync.Once
	err     error
}

func (s *fakeServer) ListenAndServe() error {
	<-s.stopped
	return s.err
}
func (s *fakeServer) Shutdown(context.Context) error {
	s.once.Do(func() { close(s.stopped) })
	return nil
}

type fakeProducerCloser struct{ closed bool }

func (c *fakeProducerCloser) Close() { c.closed = true }

type shutdownBlackholeProducer struct {
	started chan struct{}
	closed  bool
}

func (p *shutdownBlackholeProducer) Publish(ctx context.Context, _ outbox.Record) error {
	close(p.started)
	<-ctx.Done()
	return ctx.Err()
}

func (p *shutdownBlackholeProducer) Close() { p.closed = true }

type retainedShutdownOutbox struct {
	record outbox.Record
	acked  bool
}

func (s *retainedShutdownOutbox) Records(int) ([]outbox.Record, error) {
	return []outbox.Record{s.record}, nil
}

func (s *retainedShutdownOutbox) Acknowledge(uint64) error {
	s.acked = true
	return nil
}

func (*retainedShutdownOutbox) Health() (int, bool, error) { return 1, false, nil }

type fakeClientCloser struct{ closed bool }

func (c *fakeClientCloser) CloseIdleConnections() { c.closed = true }

type fakeStateCloser struct {
	closed bool
	err    error
}

func (c *fakeStateCloser) Close() error { c.closed = true; return c.err }

func newLifecycleApp(grace time.Duration, workers ...*fakeWorker) (*App, *fakeProducerCloser, *fakeClientCloser, *fakeStateCloser) {
	producer := &fakeProducerCloser{}
	client := &fakeClientCloser{}
	state := &fakeStateCloser{}
	return &App{
		config: config.Config{ShutdownGracePeriod: grace},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		ddae:   client, producer: producer, outbox: state,
		server:  &fakeServer{stopped: make(chan struct{})},
		manager: workers[0], alerts: workers[1], publisher: workers[2],
	}, producer, client, state
}

func TestRunCancelsWorkersAndClosesResources(t *testing.T) {
	workers := []*fakeWorker{
		{started: make(chan struct{})}, {started: make(chan struct{})}, {started: make(chan struct{})},
	}
	application, producer, client, state := newLifecycleApp(time.Second, workers...)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	for _, worker := range workers {
		select {
		case <-worker.started:
		case <-time.After(time.Second):
			t.Fatal("worker did not start")
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !producer.closed || !client.closed || !state.closed {
		t.Fatalf("resource close state producer=%v client=%v state=%v", producer.closed, client.closed, state.closed)
	}
}

func TestRunReturnsGraceErrorBeforeClosingInUseResources(t *testing.T) {
	block := make(chan struct{})
	workers := []*fakeWorker{
		{started: make(chan struct{}), stop: block}, {started: make(chan struct{})}, {started: make(chan struct{})},
	}
	application, producer, client, state := newLifecycleApp(10*time.Millisecond, workers...)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	for _, worker := range workers {
		<-worker.started
	}
	cancel()
	err := <-done
	close(block)
	if err == nil || !strings.Contains(err.Error(), "SHUTDOWN_GRACE_PERIOD") {
		t.Fatalf("Run error = %v", err)
	}
	if producer.closed || client.closed || state.closed {
		t.Fatal("resources were closed while a worker remained in flight")
	}
}

func TestRunStopsInFlightBlackholePublisherWithinGraceAndRetainsRecord(t *testing.T) {
	producer := &shutdownBlackholeProducer{started: make(chan struct{})}
	store := &retainedShutdownOutbox{record: outbox.Record{
		Sequence: 1, RecordKey: []byte("stable-key"), Payload: []byte(`{"event":"retained"}`),
	}}
	state := &fakeStateCloser{}
	application := &App{
		config:   config.Config{ShutdownGracePeriod: time.Second},
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		ddae:     &fakeClientCloser{},
		producer: producer,
		outbox:   state,
		server:   &fakeServer{stopped: make(chan struct{})},
		publisher: kafka.NewPublisher(
			producer, store, snapshot.NewStore(), nil,
		),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	select {
	case <-producer.started:
	case <-time.After(time.Second):
		t.Fatal("publisher did not enter the in-flight blackhole")
	}

	started := time.Now()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("application exceeded shutdown grace tolerance")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("shutdown exceeded grace tolerance: %v", elapsed)
	}
	if store.acked {
		t.Fatal("uncertain outbox record was acknowledged")
	}
	if !producer.closed || !state.closed {
		t.Fatalf("resources not closed producer=%v state=%v", producer.closed, state.closed)
	}
}

func TestNewBuildsAllLocalComponentsWithoutConnecting(t *testing.T) {
	base, _ := url.Parse("https://ddae.example.test")
	cfg := config.Config{
		ResourceMonitoringEnabled: true, AlertMonitoringEnabled: true,
		DDAEBaseURL: base, SourceInstance: "site-a", ListenAddress: "127.0.0.1:0",
		CollectionInterval: time.Minute, RequestTimeout: time.Second, CycleTimeout: 5 * time.Second,
		ResponseMaxBytes: 1024, RetryMax: 0, StaleAfter: 2 * time.Minute,
		AlertListResponseMaxBytes: 1024, AlertDetailResponseMaxBytes: 1024,
		AlertDetailRefreshInterval: time.Minute, AlertDetailMaxPerCycle: 10, AlertDetailConcurrency: 2,
		KafkaBrokers: []string{"127.0.0.1:1"}, KafkaTopic: "test", KafkaClientID: "test",
		KafkaPublishTimeout: time.Second, StateDir: filepath.Join(t.TempDir(), "state"),
		KafkaOutboxMaxBytes: 1 << 20, KafkaOutboxMaxEvents: 10,
		CheckpointRetention: time.Hour, CheckpointMaxAlerts: 10, ShutdownGracePeriod: time.Second,
	}
	application, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), BuildInfo{Version: "test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	application.producer.Close()
	application.ddae.CloseIdleConnections()
	if err := application.outbox.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewRejectsInvalidCAAndState(t *testing.T) {
	base, _ := url.Parse("https://ddae.example.test")
	cfg := config.Config{
		ResourceMonitoringEnabled: true, AlertMonitoringEnabled: true,
		DDAEBaseURL: base, DDAECAFile: filepath.Join(t.TempDir(), "missing-ca"),
	}
	if _, err := New(cfg, slog.Default(), BuildInfo{}); err == nil {
		t.Fatal("missing CA was accepted")
	}
	cfg.DDAECAFile = ""
	cfg.StateDir = ""
	if _, err := New(cfg, slog.Default(), BuildInfo{}); err == nil {
		t.Fatal("invalid persistent state was accepted")
	}
}
