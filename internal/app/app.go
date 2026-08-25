package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/collector"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/kafka"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/metrics"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/outbox"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/server"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type BuildInfo struct {
	Version   string
	Revision  string
	BuildDate string
}

type worker interface {
	Run(context.Context)
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

type producerCloser interface{ Close() }
type clientCloser interface{ CloseIdleConnections() }
type stateCloser interface{ Close() error }

type App struct {
	config    config.Config
	logger    *slog.Logger
	ddae      clientCloser
	producer  producerCloser
	outbox    stateCloser
	server    httpServer
	manager   worker
	alerts    worker
	publisher worker
}

func New(cfg config.Config, logger *slog.Logger, build BuildInfo) (*App, error) {
	resourcesEnabled := cfg.ResourceMonitoringEnabled
	alertsEnabled := cfg.AlertMonitoringEnabled
	if !resourcesEnabled && !alertsEnabled {
		return nil, errors.New("at least one monitoring pipeline must be enabled")
	}
	resourceInterval := cfg.ResourceCollectionInterval
	if resourceInterval == 0 {
		resourceInterval = cfg.CollectionInterval
	}
	alertInterval := cfg.AlertCollectionInterval
	if alertInterval == 0 {
		alertInterval = cfg.CollectionInterval
	}
	state := snapshot.NewStore()
	ddaeClient, err := ddae.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create DDAE client: %w", err)
	}
	var store *outbox.Store
	var producer *kafka.Producer
	if alertsEnabled {
		store, err = outbox.Open(outbox.Options{
			StateDir: cfg.StateDir, MaxBytes: cfg.KafkaOutboxMaxBytes,
			MaxEvents: cfg.KafkaOutboxMaxEvents, MaxCheckpoints: cfg.CheckpointMaxAlerts,
			Retention: cfg.CheckpointRetention,
		})
		if err != nil {
			ddaeClient.CloseIdleConnections()
			return nil, err
		}
		producer, err = kafka.NewProducer(cfg)
		if err != nil {
			_ = store.Close()
			ddaeClient.CloseIdleConnections()
			return nil, err
		}
	}
	registry, err := metrics.NewRegistry(
		state, cfg.StaleAfter,
		metrics.BuildInfo{Version: build.Version, GoVersion: runtime.Version()},
		metrics.PipelineMode{ResourcesEnabled: resourcesEnabled, AlertsEnabled: alertsEnabled},
	)
	if err != nil {
		if producer != nil {
			producer.Close()
		}
		if store != nil {
			_ = store.Close()
		}
		ddaeClient.CloseIdleConnections()
		return nil, err
	}
	var manager worker
	if resourcesEnabled {
		manager = collector.NewManager(ddaeClient, state, cfg.CycleTimeout, resourceInterval, logger)
	}
	var alertPipeline worker
	var publisher worker
	if alertsEnabled {
		alertPipeline = alerts.NewPipeline(ddaeClient, store, state, alerts.Options{
			SourceInstance: cfg.SourceInstance, Interval: alertInterval,
			CycleTimeout: cfg.CycleTimeout, RefreshInterval: cfg.AlertDetailRefreshInterval,
			MaxPerCycle: cfg.AlertDetailMaxPerCycle, Concurrency: cfg.AlertDetailConcurrency,
		}, logger)
		publisher = kafka.NewPublisher(producer, store, state, logger)
	}
	httpServer := server.New(
		cfg.ListenAddress, registry, state, cfg.StaleAfter,
		server.PipelineMode{ResourcesEnabled: resourcesEnabled, AlertsEnabled: alertsEnabled},
	)
	application := &App{
		config: cfg, logger: logger, ddae: ddaeClient,
		server: httpServer, manager: manager, alerts: alertPipeline,
		publisher: publisher,
	}
	// Avoid storing typed nil pointers in interface fields. A typed nil interface
	// compares non-nil and would make resource-only shutdown call absent resources.
	application.producer = nil
	application.outbox = nil
	if producer != nil {
		application.producer = producer
	}
	if store != nil {
		application.outbox = store
	}
	return application, nil
}

func (a *App) Run(ctx context.Context) error {
	workerContext, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	activeWorkers := make([]worker, 0, 3)
	for _, candidate := range []worker{a.manager, a.alerts, a.publisher} {
		if candidate != nil {
			activeWorkers = append(activeWorkers, candidate)
		}
	}
	var workers sync.WaitGroup
	workers.Add(len(activeWorkers))
	for _, activeWorker := range activeWorkers {
		go func(current worker) {
			defer workers.Done()
			current.Run(workerContext)
		}(activeWorker)
	}

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- a.server.ListenAndServe() }()
	a.logger.Info("DDAE exporter started", "component", "server")

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-serverErrors:
		if runErr != nil {
			cancelWorkers()
		}
	}
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), a.config.ShutdownGracePeriod)
	defer cancelShutdown()
	if err := a.server.Shutdown(shutdownContext); err != nil && runErr == nil {
		runErr = err
	}
	cancelWorkers()
	workersDone := make(chan struct{})
	go func() {
		workers.Wait()
		close(workersDone)
	}()
	select {
	case <-workersDone:
	case <-shutdownContext.Done():
		if runErr == nil {
			runErr = errors.New("workers did not stop within SHUTDOWN_GRACE_PERIOD")
		}
		return runErr
	}
	if a.producer != nil {
		a.producer.Close()
	}
	a.ddae.CloseIdleConnections()
	if a.outbox != nil {
		if err := a.outbox.Close(); err != nil && runErr == nil {
			runErr = err
		}
	}
	if errors.Is(runErr, context.Canceled) {
		return nil
	}
	a.logger.Info("DDAE exporter stopped", "component", "server")
	return runErr
}
