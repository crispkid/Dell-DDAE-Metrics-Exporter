package logpublisher

import (
	"context"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/logstate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type Producer interface {
	PublishServiceabilityLog(context.Context, []byte, []byte) error
}

type Outbox interface {
	Records(limit int) ([]logstate.Record, error)
	Acknowledge(sequence uint64) error
	Health() (events int, full bool, err error)
}

type Publisher struct {
	producer              Producer
	outbox                Outbox
	diagnostics           *snapshot.Store
	logger                interface{ Error(string, ...any) }
	stateRecoveryRequired bool
}

func New(producer Producer, outbox Outbox, diagnostics *snapshot.Store, logger interface{ Error(string, ...any) }) *Publisher {
	return &Publisher{producer: producer, outbox: outbox, diagnostics: diagnostics, logger: logger}
}

func (p *Publisher) Run(ctx context.Context) {
	p.flush(ctx)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.flush(ctx)
		}
	}
}

func (p *Publisher) flush(ctx context.Context) {
	records, err := p.outbox.Records(100)
	if err != nil {
		p.stateRecoveryRequired = true
		p.diagnostics.SetServiceabilityLogPublisherStateHealthy(false)
		p.failed(err, 0)
		return
	}
	if len(records) == 0 {
		events, full, healthErr := p.outbox.Health()
		if healthErr != nil {
			p.stateRecoveryRequired = true
			p.diagnostics.SetServiceabilityLogPublisherStateHealthy(false)
			p.failed(healthErr, events)
			return
		}
		p.diagnostics.SetServiceabilityLogBuffered(events)
		p.diagnostics.SetServiceabilityLogStateFull(full)
		p.diagnostics.SetServiceabilityLogPublisherStateHealthy(true)
		p.stateRecoveryRequired = false
		return
	}
	for _, record := range records {
		started := time.Now()
		if err := p.producer.PublishServiceabilityLog(ctx, record.RecordKey, record.Payload); err != nil {
			events, full, healthErr := p.outbox.Health()
			if healthErr != nil {
				p.stateRecoveryRequired = true
				p.diagnostics.SetServiceabilityLogPublisherStateHealthy(false)
				p.failed(healthErr, events)
				return
			}
			p.diagnostics.SetServiceabilityLogStateFull(full)
			if !p.stateRecoveryRequired {
				p.diagnostics.SetServiceabilityLogPublisherStateHealthy(true)
			}
			p.diagnostics.RecordServiceabilityLogPublish(false, time.Since(started), 0, observability.Classify(err), events)
			p.log(err)
			return
		}
		if err := p.outbox.Acknowledge(record.Sequence); err != nil {
			p.stateRecoveryRequired = true
			p.diagnostics.SetServiceabilityLogPublisherStateHealthy(false)
			p.failed(err, len(records))
			return
		}
		events, full, healthErr := p.outbox.Health()
		if healthErr != nil {
			p.stateRecoveryRequired = true
			p.diagnostics.SetServiceabilityLogPublisherStateHealthy(false)
			p.failed(healthErr, events)
			return
		}
		p.diagnostics.SetServiceabilityLogStateFull(full)
		p.diagnostics.RecordServiceabilityLogPublish(true, time.Since(started), 1, "", events)
	}
	p.diagnostics.SetServiceabilityLogPublisherStateHealthy(true)
	p.stateRecoveryRequired = false
}

func (p *Publisher) failed(err error, buffered int) {
	p.diagnostics.RecordServiceabilityLogPublish(false, 0, 0, observability.Classify(err), buffered)
	p.log(err)
}

func (p *Publisher) log(err error) {
	if p.logger != nil {
		p.logger.Error("serviceability log Kafka publisher failed", "component", "serviceability_log_kafka", "failure_class", observability.Classify(err))
	}
}
