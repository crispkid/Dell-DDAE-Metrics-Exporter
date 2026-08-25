package kafka

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/outbox"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/kversion"
)

const (
	kafkaProduceAPI        int16 = 0
	kafkaMetadataAPI       int16 = 3
	kafkaAPIVersionsAPI    int16 = 18
	kafkaInitProducerIDAPI int16 = 22
)

type blackholeKafkaBroker struct {
	listener    net.Listener
	produceSeen chan struct{}
	stop        chan struct{}
	produceOnce sync.Once
	closeOnce   sync.Once
	wait        sync.WaitGroup
	requestMu   sync.Mutex
	requestKeys []int16
}

func newBlackholeKafkaBroker(t *testing.T) *blackholeKafkaBroker {
	t.Helper()
	certificateSource := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	tlsConfig := certificateSource.TLS.Clone()
	certificateSource.Close()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	broker := &blackholeKafkaBroker{
		listener: listener, produceSeen: make(chan struct{}), stop: make(chan struct{}),
	}
	broker.wait.Add(1)
	go broker.serve()
	t.Cleanup(broker.close)
	return broker
}

func (b *blackholeKafkaBroker) address() string { return b.listener.Addr().String() }

func (b *blackholeKafkaBroker) close() {
	b.closeOnce.Do(func() {
		close(b.stop)
		_ = b.listener.Close()
		b.wait.Wait()
	})
}

func (b *blackholeKafkaBroker) serve() {
	defer b.wait.Done()
	for {
		connection, err := b.listener.Accept()
		if err != nil {
			return
		}
		b.wait.Add(1)
		go func() {
			defer b.wait.Done()
			defer connection.Close()
			b.handle(connection)
		}()
	}
}

func (b *blackholeKafkaBroker) handle(connection net.Conn) {
	for {
		apiKey, version, correlationID, err := readKafkaRequest(connection)
		if err != nil {
			return
		}
		b.requestMu.Lock()
		b.requestKeys = append(b.requestKeys, apiKey)
		b.requestMu.Unlock()
		var body []byte
		switch apiKey {
		case kafkaAPIVersionsAPI:
			body = apiVersionsResponse(version)
		case kafkaMetadataAPI:
			body = metadataResponse(version, b.address(), "test-topic")
		case kafkaInitProducerIDAPI:
			body = initProducerIDResponse(version)
		case kafkaProduceAPI:
			b.produceOnce.Do(func() { close(b.produceSeen) })
			select {
			case <-b.stop:
			case <-time.After(10 * time.Second):
			}
			return
		default:
			return
		}
		if err := writeKafkaResponse(connection, correlationID, body); err != nil {
			return
		}
	}
}

func readKafkaRequest(connection net.Conn) (apiKey int16, version int16, correlationID int32, err error) {
	var sizeBytes [4]byte
	if _, err = io.ReadFull(connection, sizeBytes[:]); err != nil {
		return 0, 0, 0, err
	}
	size := binary.BigEndian.Uint32(sizeBytes[:])
	if size < 8 || size > 2*1024*1024 {
		return 0, 0, 0, errors.New("invalid Kafka request size")
	}
	body := make([]byte, size)
	if _, err = io.ReadFull(connection, body); err != nil {
		return 0, 0, 0, err
	}
	return int16(binary.BigEndian.Uint16(body[0:2])),
		int16(binary.BigEndian.Uint16(body[2:4])),
		int32(binary.BigEndian.Uint32(body[4:8])), nil
}

func writeKafkaResponse(connection net.Conn, correlationID int32, body []byte) error {
	frame := make([]byte, 8+len(body))
	binary.BigEndian.PutUint32(frame[0:4], uint32(4+len(body)))
	binary.BigEndian.PutUint32(frame[4:8], uint32(correlationID))
	copy(frame[8:], body)
	_, err := connection.Write(frame)
	return err
}

func apiVersionsResponse(version int16) []byte {
	response := kmsg.NewPtrApiVersionsResponse()
	response.SetVersion(version)
	for _, supported := range []struct {
		key, maximum int16
	}{
		{kafkaProduceAPI, 3},
		{kafkaMetadataAPI, 4},
		{kafkaAPIVersionsAPI, 1},
		{kafkaInitProducerIDAPI, 0},
	} {
		key := kmsg.NewApiVersionsResponseApiKey()
		key.ApiKey = supported.key
		key.MaxVersion = supported.maximum
		response.ApiKeys = append(response.ApiKeys, key)
	}
	return response.AppendTo(nil)
}

func metadataResponse(version int16, address, topic string) []byte {
	host, portText, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portText)
	response := kmsg.NewPtrMetadataResponse()
	response.SetVersion(version)
	response.ControllerID = 1
	broker := kmsg.NewMetadataResponseBroker()
	broker.NodeID = 1
	broker.Host = host
	broker.Port = int32(port)
	response.Brokers = append(response.Brokers, broker)
	partition := kmsg.NewMetadataResponseTopicPartition()
	partition.Leader = 1
	partition.Replicas = []int32{1}
	partition.ISR = []int32{1}
	topicRow := kmsg.NewMetadataResponseTopic()
	topicRow.Topic = &topic
	topicRow.Partitions = append(topicRow.Partitions, partition)
	response.Topics = append(response.Topics, topicRow)
	return response.AppendTo(nil)
}

func initProducerIDResponse(version int16) []byte {
	response := kmsg.NewPtrInitProducerIDResponse()
	response.SetVersion(version)
	response.ProducerID = 1
	return response.AppendTo(nil)
}

func (b *blackholeKafkaBroker) requests() []int16 {
	b.requestMu.Lock()
	defer b.requestMu.Unlock()
	return append([]int16(nil), b.requestKeys...)
}

func TestPublishHardTimeoutAfterProduceIsInFlight(t *testing.T) {
	broker := newBlackholeKafkaBroker(t)
	timeout := time.Second
	producer, err := newProducer(config.Config{
		KafkaBrokers: []string{broker.address()}, KafkaTopic: "test-topic",
		KafkaClientID: "hard-timeout-test", KafkaPublishTimeout: timeout,
		AllowInsecureTLS: true, KafkaTLSInsecureSkipVerify: true,
	}, kgo.MaxVersions(kversion.V0_11_0()))
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()

	started := time.Now()
	result := make(chan error, 1)
	go func() {
		result <- producer.Publish(context.Background(), outbox.Record{
			Sequence: 1, RecordKey: []byte("stable-key"), Payload: []byte(`{"event":"test"}`),
		})
	}()
	select {
	case <-broker.produceSeen:
	case err := <-result:
		t.Fatalf("publish returned before produce was in flight: %v", err)
	case <-time.After(timeout):
		t.Fatalf("produce request did not become in flight; requests=%v", broker.requests())
	}

	select {
	case err := <-result:
		if observability.Classify(err) != observability.ClassKafkaTimeout {
			t.Fatalf("publish error class=%s err=%v", observability.Classify(err), err)
		}
		if elapsed := time.Since(started); elapsed > timeout+time.Second {
			t.Fatalf("publish exceeded hard timeout tolerance: %v", elapsed)
		}
	case <-time.After(timeout + time.Second):
		t.Fatal("publish did not honor hard timeout")
	}

	closed := make(chan struct{})
	go func() {
		producer.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("producer close blocked after timed-out in-flight request")
	}
}

func TestPublisherRetainsOutboxRecordAfterHardTimeout(t *testing.T) {
	broker := newBlackholeKafkaBroker(t)
	producer, err := newProducer(config.Config{
		KafkaBrokers: []string{broker.address()}, KafkaTopic: "test-topic",
		KafkaClientID: "hard-timeout-publisher-test", KafkaPublishTimeout: time.Second,
		AllowInsecureTLS: true, KafkaTLSInsecureSkipVerify: true,
	}, kgo.MaxVersions(kversion.V0_11_0()))
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()

	record := outbox.Record{
		Sequence: 7, RecordKey: []byte("stable-key"), Payload: []byte(`{"event":"retained"}`),
	}
	store := &fakeOutbox{records: []outbox.Record{record}}
	diagnostics := snapshot.NewStore()
	publisher := NewPublisher(producer, store, diagnostics, nil)
	finished := make(chan struct{})
	go func() {
		publisher.flush(context.Background())
		close(finished)
	}()
	select {
	case <-broker.produceSeen:
	case <-finished:
		t.Fatal("publisher returned before produce was in flight")
	case <-time.After(time.Second):
		t.Fatal("publisher produce request did not become in flight")
	}
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher did not return after hard timeout")
	}
	if len(store.acked) != 0 || len(store.records) != 1 || store.records[0].Sequence != record.Sequence {
		t.Fatalf("timed-out record changed: acked=%v records=%#v", store.acked, store.records)
	}
	view := diagnostics.Load()
	if view.KafkaFailedTotal[observability.ClassKafkaTimeout] != 1 || view.KafkaPublishedTotal != 0 {
		t.Fatalf("timeout diagnostics=%#v", view)
	}
}
