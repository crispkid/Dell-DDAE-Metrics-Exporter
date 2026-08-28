package logpublisher

import (
	"context"
	"errors"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/logstate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type fakeProducer struct{ err error }

func (p fakeProducer) PublishServiceabilityLog(context.Context, []byte, []byte) error { return p.err }

type fakeOutbox struct {
	records   []logstate.Record
	acked     []uint64
	recordErr error
	ackErr    error
	healthErr error
	full      bool
}

func (s *fakeOutbox) Records(int) ([]logstate.Record, error) { return s.records, s.recordErr }
func (s *fakeOutbox) Acknowledge(sequence uint64) error {
	if s.ackErr != nil {
		return s.ackErr
	}
	s.acked = append(s.acked, sequence)
	s.records = s.records[1:]
	return nil
}
func (s *fakeOutbox) Health() (int, bool, error) { return len(s.records), s.full, s.healthErr }

type recordingProducer struct {
	keys     [][]byte
	payloads [][]byte
}

func (p *recordingProducer) PublishServiceabilityLog(_ context.Context, key, payload []byte) error {
	p.keys = append(p.keys, append([]byte(nil), key...))
	p.payloads = append(p.payloads, append([]byte(nil), payload...))
	return nil
}

type captureLogger struct{ calls int }

func (l *captureLogger) Error(string, ...any) { l.calls++ }

func TestBrokerFailureRetainsLogRecordWithoutMarkingStateCorrupt(t *testing.T) {
	state := &fakeOutbox{records: []logstate.Record{{Sequence: 1, RecordKey: []byte("key"), Payload: []byte("value")}}}
	diagnostics := snapshot.NewStore()
	diagnostics.SetServiceabilityLogCollectionReady(true)
	diagnostics.SetServiceabilityLogPipelineStateHealthy(true)
	publisher := New(fakeProducer{err: errors.New("broker-canary")}, state, diagnostics, nil)
	publisher.flush(context.Background())
	view := diagnostics.Load()
	if len(state.acked) != 0 || len(state.records) != 1 {
		t.Fatal("failed delivery removed durable record")
	}
	if !view.ServiceabilityLogPublisherStateOK || !view.ServiceabilityLogPipelineReady || view.ServiceabilityLogPublishSuccess {
		t.Fatalf("broker failure state = %#v", view)
	}
}

func TestStateFailureIsStickyUntilPublisherHealthSequenceSucceeds(t *testing.T) {
	state := &fakeOutbox{recordErr: errors.New("state-canary")}
	diagnostics := snapshot.NewStore()
	diagnostics.SetServiceabilityLogCollectionReady(true)
	diagnostics.SetServiceabilityLogPipelineStateHealthy(true)
	publisher := New(fakeProducer{}, state, diagnostics, nil)
	publisher.flush(context.Background())
	if diagnostics.Load().ServiceabilityLogPublisherStateOK {
		t.Fatal("state error reported publisher healthy")
	}
	state.recordErr = nil
	publisher.flush(context.Background())
	if !diagnostics.Load().ServiceabilityLogPublisherStateOK || !diagnostics.Load().ServiceabilityLogPipelineReady {
		t.Fatalf("publisher did not recover: %#v", diagnostics.Load())
	}
}

func TestSuccessfulFlushAcknowledgesInOrderAndUpdatesDiagnostics(t *testing.T) {
	state := &fakeOutbox{records: []logstate.Record{
		{Sequence: 1, RecordKey: []byte("key-1"), Payload: []byte("value-1")},
		{Sequence: 2, RecordKey: []byte("key-2"), Payload: []byte("value-2")},
	}}
	producer := &recordingProducer{}
	diagnostics := snapshot.NewStore()
	diagnostics.SetServiceabilityLogCollectionReady(true)
	diagnostics.SetServiceabilityLogPipelineStateHealthy(true)
	publisher := New(producer, state, diagnostics, nil)
	publisher.flush(context.Background())

	view := diagnostics.Load()
	if len(producer.keys) != 2 || len(state.acked) != 2 || state.acked[0] != 1 || state.acked[1] != 2 {
		t.Fatalf("publishes=%d acknowledgements=%v", len(producer.keys), state.acked)
	}
	if !view.ServiceabilityLogPublishSuccess || view.ServiceabilityLogPublishedTotal != 2 || view.ServiceabilityLogBuffered != 0 || !view.ServiceabilityLogPipelineReady {
		t.Fatalf("successful diagnostics = %#v", view)
	}
}

func TestPublisherStateFailuresFailClosedAndAreLogged(t *testing.T) {
	tests := []struct {
		name  string
		state *fakeOutbox
	}{
		{name: "empty health", state: &fakeOutbox{healthErr: errors.New("health failed")}},
		{name: "acknowledgement", state: &fakeOutbox{records: []logstate.Record{{Sequence: 1}}, ackErr: errors.New("ack failed")}},
		{name: "post publish health", state: &fakeOutbox{records: []logstate.Record{{Sequence: 1}}, healthErr: errors.New("health failed")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := snapshot.NewStore()
			logger := &captureLogger{}
			publisher := New(&recordingProducer{}, test.state, diagnostics, logger)
			publisher.flush(context.Background())
			if diagnostics.Load().ServiceabilityLogPublisherStateOK || logger.calls != 1 {
				t.Fatalf("state failure diagnostics=%#v logger calls=%d", diagnostics.Load(), logger.calls)
			}
		})
	}
}

func TestBrokerAndHealthFailureKeepsRecoveryStateUnhealthy(t *testing.T) {
	state := &fakeOutbox{
		records:   []logstate.Record{{Sequence: 1}},
		healthErr: errors.New("state health failed"),
	}
	diagnostics := snapshot.NewStore()
	logger := &captureLogger{}
	publisher := New(fakeProducer{err: errors.New("broker failed")}, state, diagnostics, logger)
	publisher.flush(context.Background())
	if diagnostics.Load().ServiceabilityLogPublisherStateOK || len(state.acked) != 0 || logger.calls != 1 {
		t.Fatalf("broker/state failure diagnostics=%#v acked=%v logs=%d", diagnostics.Load(), state.acked, logger.calls)
	}
}

func TestRunStopsAfterCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	diagnostics := snapshot.NewStore()
	New(&recordingProducer{}, &fakeOutbox{}, diagnostics, nil).Run(ctx)
	if !diagnostics.Load().ServiceabilityLogPublisherStateOK {
		t.Fatal("initial empty health check did not complete before cancellation")
	}
}
