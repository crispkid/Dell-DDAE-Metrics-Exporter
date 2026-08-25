package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/outbox"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

type Error struct{ class observability.Class }

func (e Error) Error() string                     { return "Kafka publish failed" }
func (e Error) FailureClass() observability.Class { return e.class }

type Producer struct {
	client  *kgo.Client
	topic   string
	timeout time.Duration
}

func NewProducer(cfg config.Config) (*Producer, error) {
	return newProducer(cfg)
}

func newProducer(cfg config.Config, extraOptions ...kgo.Opt) (*Producer, error) {
	if cfg.KafkaTLSInsecureSkipVerify && !cfg.AllowInsecureTLS {
		return nil, errors.New("Kafka insecure TLS requires the global acknowledgement")
	}
	if cfg.KafkaTLSInsecureSkipVerify && cfg.KafkaCAFile != "" {
		return nil, errors.New("Kafka custom CA conflicts with insecure TLS")
	}
	tlsConfig, err := producerTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	options := []kgo.Opt{
		kgo.SeedBrokers(cfg.KafkaBrokers...),
		kgo.ClientID(cfg.KafkaClientID),
		kgo.DialTLSConfig(tlsConfig),
		kgo.DialTimeout(cfg.KafkaPublishTimeout),
		kgo.RequestTimeoutOverhead(cfg.KafkaPublishTimeout),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.AllowIdempotentProduceCancellation(),
		kgo.MaxProduceRequestsInflightPerBroker(1),
		kgo.MaxBufferedRecords(128),
		kgo.MaxBufferedBytes(32 * 1024 * 1024),
		kgo.ProducerBatchMaxBytes(512 * 1024),
		kgo.ProducerLinger(0),
		kgo.ProduceRequestTimeout(cfg.KafkaPublishTimeout),
		kgo.RecordDeliveryTimeout(cfg.KafkaPublishTimeout),
		kgo.RecordRetries(3),
		kgo.UnknownTopicRetries(3),
	}
	switch cfg.KafkaSASLMechanism {
	case "PLAIN":
		options = append(options, kgo.SASL(plain.Auth{User: cfg.KafkaSASLUsername.Value(), Pass: cfg.KafkaSASLPassword.Value()}.AsMechanism()))
	case "SCRAM-SHA-256":
		options = append(options, kgo.SASL(scram.Auth{User: cfg.KafkaSASLUsername.Value(), Pass: cfg.KafkaSASLPassword.Value()}.AsSha256Mechanism()))
	case "SCRAM-SHA-512":
		options = append(options, kgo.SASL(scram.Auth{User: cfg.KafkaSASLUsername.Value(), Pass: cfg.KafkaSASLPassword.Value()}.AsSha512Mechanism()))
	}
	options = append(options, extraOptions...)
	client, err := kgo.NewClient(options...)
	if err != nil {
		return nil, Error{class: observability.ClassKafkaRejected}
	}
	return &Producer{client: client, topic: cfg.KafkaTopic, timeout: cfg.KafkaPublishTimeout}, nil
}

func producerTLSConfig(cfg config.Config) (*tls.Config, error) {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if cfg.KafkaCAFile != "" {
		pem, err := os.ReadFile(cfg.KafkaCAFile)
		if err != nil || !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("KAFKA_CA_FILE is invalid")
		}
	}
	result := &tls.Config{
		MinVersion:         config.TLSMinVersion(),
		RootCAs:            roots,
		InsecureSkipVerify: cfg.KafkaTLSInsecureSkipVerify, // #nosec G402 -- guarded by validated global and target opt-ins.
	}
	if cfg.KafkaClientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.KafkaClientCertFile, cfg.KafkaClientKeyFile)
		if err != nil {
			return nil, errors.New("Kafka client certificate pair is invalid")
		}
		result.Certificates = []tls.Certificate{certificate}
	}
	return result, nil
}

func (p *Producer) Publish(ctx context.Context, record outbox.Record) error {
	publishContext, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	result := p.client.ProduceSync(publishContext, &kgo.Record{
		Topic: p.topic,
		Key:   append([]byte(nil), record.RecordKey...),
		Value: append([]byte(nil), record.Payload...),
		Headers: []kgo.RecordHeader{
			{Key: "content-type", Value: []byte("application/json")},
			{Key: "ddae-schema-version", Value: []byte("1.0")},
		},
	})
	if err := result.FirstErr(); err != nil {
		return Error{class: publishFailureClass(err, publishContext)}
	}
	return nil
}

func publishFailureClass(err error, ctx context.Context) observability.Class {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return observability.ClassKafkaTimeout
	}
	if errors.Is(err, kerr.SaslAuthenticationFailed) {
		return observability.ClassKafkaAuth
	}
	return observability.ClassKafkaRejected
}

func (p *Producer) Close() { p.client.Close() }

type Outbox interface {
	Records(limit int) ([]outbox.Record, error)
	Acknowledge(sequence uint64) error
	Health() (events int, full bool, err error)
}

type Publisher struct {
	producer interface {
		Publish(context.Context, outbox.Record) error
	}
	outbox      Outbox
	diagnostics *snapshot.Store
	logger      interface{ Error(string, ...any) }
}

func NewPublisher(producer interface {
	Publish(context.Context, outbox.Record) error
}, store Outbox, diagnostics *snapshot.Store, logger interface{ Error(string, ...any) }) *Publisher {
	return &Publisher{producer: producer, outbox: store, diagnostics: diagnostics, logger: logger}
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
		p.failed(err, 0)
		return
	}
	if len(records) == 0 {
		events, _, healthErr := p.outbox.Health()
		if healthErr != nil {
			p.failed(healthErr, events)
			return
		}
		p.diagnostics.SetKafkaBuffered(events)
		return
	}
	for _, record := range records {
		started := time.Now()
		if err := p.producer.Publish(ctx, record); err != nil {
			events, _, _ := p.outbox.Health()
			p.diagnostics.RecordKafkaPublish(false, time.Since(started), 0, observability.Classify(err), events)
			p.log(err)
			return
		}
		if err := p.outbox.Acknowledge(record.Sequence); err != nil {
			p.failed(err, len(records))
			return
		}
		events, _, _ := p.outbox.Health()
		p.diagnostics.RecordKafkaPublish(true, time.Since(started), 1, "", events)
	}
}

func (p *Publisher) failed(err error, buffered int) {
	p.diagnostics.RecordKafkaPublish(false, 0, 0, observability.Classify(err), buffered)
	p.log(err)
}

func (p *Publisher) log(err error) {
	if p.logger != nil {
		p.logger.Error("Kafka publisher failed", "component", "kafka", "failure_class", observability.Classify(err))
	}
}
