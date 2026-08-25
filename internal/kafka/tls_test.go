package kafka

import (
	"crypto/tls"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
)

func TestProducerTLSConfigRetainsVerificationAndMinimumVersion(t *testing.T) {
	verified, err := producerTLSConfig(config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if verified.InsecureSkipVerify {
		t.Fatal("Kafka TLS verification is disabled by default")
	}
	if verified.MinVersion != tls.VersionTLS12 {
		t.Fatalf("Kafka TLS minimum = %d", verified.MinVersion)
	}

	insecure, err := producerTLSConfig(config.Config{KafkaTLSInsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if !insecure.InsecureSkipVerify || insecure.MinVersion != tls.VersionTLS12 {
		t.Fatal("Kafka target opt-out changed more than certificate verification")
	}
}

func TestNewProducerRejectsUnguardedInsecureTLS(t *testing.T) {
	if _, err := NewProducer(config.Config{KafkaTLSInsecureSkipVerify: true}); err == nil {
		t.Fatal("Kafka producer accepted unguarded insecure TLS")
	}
}
