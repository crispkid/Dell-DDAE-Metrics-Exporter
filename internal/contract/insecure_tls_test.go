package contract

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestInsecureTLSRequiresBothOptInsAndIsTargetScoped(t *testing.T) {
	server := tlsDDAEServer()
	defer server.Close()

	if _, err := loadTLSYAML(t, ddaeTLSYAML(server.URL, "", false, true)); err == nil {
		t.Fatal("target-only insecure TLS was accepted")
	}

	globalOnly, err := loadTLSYAML(t, ddaeTLSYAML(server.URL, "", true, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(globalOnly.InsecureTLSTargets()) != 0 {
		t.Fatal("global acknowledgement disabled verification by itself")
	}

	insecureConfig, err := loadTLSYAML(t, ddaeTLSYAML(server.URL, "", true, true))
	if err != nil {
		t.Fatal(err)
	}
	if targets := insecureConfig.InsecureTLSTargets(); len(targets) != 1 || targets[0] != "ddae" {
		t.Fatalf("insecure targets = %v", targets)
	}
	client, err := ddae.NewClient(insecureConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	if _, err := client.Ping(context.Background()); err != nil {
		t.Fatalf("guarded insecure request: %v", err)
	}
}

func TestInsecureTLSStillRejectsTLS11(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "openid-connect") {
			_, _ = writer.Write([]byte(`{"access_token":"contract-token","expires_in":3600}`))
			return
		}
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	}))
	server.TLS = &tls.Config{MaxVersion: tls.VersionTLS11}
	server.StartTLS()
	defer server.Close()

	cfg, err := loadTLSYAML(t, ddaeTLSYAML(server.URL, "", true, true))
	if err != nil {
		t.Fatal(err)
	}
	client, err := ddae.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	if _, err := client.Ping(context.Background()); err == nil {
		t.Fatal("TLS 1.1 succeeded while insecure verification was enabled")
	}
}

func TestKafkaInsecureTargetDoesNotDisableDDAE(t *testing.T) {
	server := tlsDDAEServer()
	defer server.Close()
	document := `version: 1
monitoring:
  resources:
    enabled: false
  alerts:
    enabled: true
security:
  allow_insecure_tls: true
ddae:
  base_url: ` + server.URL + `
  source_instance: site-a
kafka:
  brokers: [kafka.example.test:9093]
  topic: alerts
  tls:
    insecure_skip_verify: true
`
	cfg, err := loadTLSYAML(t, document)
	if err != nil {
		t.Fatal(err)
	}
	if targets := cfg.InsecureTLSTargets(); len(targets) != 1 || targets[0] != "kafka" || cfg.DDAETLSInsecureSkipVerify {
		t.Fatalf("target isolation failed: %v", targets)
	}
}

func TestInsecureTLSConflictsWithCustomCAForSameTarget(t *testing.T) {
	server := tlsDDAEServer()
	defer server.Close()
	if _, err := loadTLSYAML(t, ddaeTLSYAML(server.URL, "/unused/ddae-ca.pem", true, true)); err == nil {
		t.Fatal("DDAE custom CA and insecure mode were accepted together")
	}

	document := `version: 1
monitoring:
  resources:
    enabled: false
  alerts:
    enabled: true
security:
  allow_insecure_tls: true
ddae:
  base_url: https://ddae.example.test
  source_instance: site-a
kafka:
  brokers: [kafka.example.test:9093]
  topic: alerts
  tls:
    ca_file: /unused/kafka-ca.pem
    insecure_skip_verify: true
`
	if _, err := loadTLSYAML(t, document); err == nil {
		t.Fatal("Kafka custom CA and insecure mode were accepted together")
	}
}

func TestDDAEClientRejectsUnguardedInsecureTLS(t *testing.T) {
	if _, err := ddae.NewClient(config.Config{DDAETLSInsecureSkipVerify: true}); err == nil {
		t.Fatal("DDAE client accepted unguarded insecure TLS")
	}
}
