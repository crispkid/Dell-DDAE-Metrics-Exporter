package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/outbox"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type fakeProducer struct{ err error }

func (f fakeProducer) Publish(context.Context, outbox.Record) error { return f.err }

type fakeOutbox struct {
	records    []outbox.Record
	acked      []uint64
	recordsErr error
	ackErr     error
	healthErr  error
}

func (f *fakeOutbox) Records(int) ([]outbox.Record, error) { return f.records, f.recordsErr }
func (f *fakeOutbox) Acknowledge(sequence uint64) error {
	if f.ackErr != nil {
		return f.ackErr
	}
	f.acked = append(f.acked, sequence)
	f.records = f.records[1:]
	return nil
}
func (f *fakeOutbox) Health() (int, bool, error) { return len(f.records), false, f.healthErr }

func TestPublisherAcknowledgesOnlySuccessfulPublish(t *testing.T) {
	store := &fakeOutbox{records: []outbox.Record{{Sequence: 1}}}
	diagnostics := snapshot.NewStore()
	publisher := NewPublisher(fakeProducer{}, store, diagnostics, nil)
	publisher.flush(context.Background())
	if len(store.acked) != 1 || diagnostics.Load().KafkaPublishedTotal != 1 {
		t.Fatalf("ack=%v diagnostics=%#v", store.acked, diagnostics.Load())
	}

	store = &fakeOutbox{records: []outbox.Record{{Sequence: 2}}}
	publisher = NewPublisher(fakeProducer{err: errors.New("broker details")}, store, diagnostics, nil)
	publisher.flush(context.Background())
	if len(store.acked) != 0 || len(store.records) != 1 {
		t.Fatal("failed publish removed outbox record")
	}
}
