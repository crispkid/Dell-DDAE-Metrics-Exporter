package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadYAMLMap(document string, environment map[string]string, files map[string][]byte) (Config, error) {
	values, err := decodeYAML([]byte(document))
	if err != nil {
		return Config{}, err
	}
	lookup := func(name string) (string, bool) {
		value, ok := environment[name]
		return value, ok
	}
	readFile := func(path string) ([]byte, error) {
		value, ok := files[path]
		if !ok {
			return nil, errors.New("missing")
		}
		return value, nil
	}
	return load(layeredLookup(values, lookup), readFile)
}

func resourceOnlyYAML() string {
	return `version: 1
monitoring:
  resources:
    enabled: true
  alerts:
    enabled: false
ddae:
  base_url: https://ddae.example.test
  credentials:
    username_file: /secrets/username
    password_file: /secrets/password
    client_secret_file: /secrets/client-secret
`
}

func testSecrets() map[string][]byte {
	return map[string][]byte{
		"/secrets/username":      []byte("monitor"),
		"/secrets/password":      []byte("password-canary"),
		"/secrets/client-secret": []byte("client-secret-canary"),
	}
}

func TestYAMLLoadsResourceOnlyConfiguration(t *testing.T) {
	cfg, err := loadYAMLMap(resourceOnlyYAML(), nil, testSecrets())
	if err != nil {
		t.Fatalf("load YAML: %v", err)
	}
	if !cfg.ResourceMonitoringEnabled || cfg.AlertMonitoringEnabled {
		t.Fatalf("pipeline flags resources=%v alerts=%v", cfg.ResourceMonitoringEnabled, cfg.AlertMonitoringEnabled)
	}
	if cfg.DDAEBaseURL.String() != "https://ddae.example.test" || cfg.DDAEPassword.Value() != "password-canary" {
		t.Fatal("YAML values or secret files were not loaded")
	}
	if len(cfg.KafkaBrokers) != 0 || cfg.KafkaTopic != "" {
		t.Fatal("resource-only configuration unexpectedly required Kafka")
	}
}

func TestYAMLLoadsAlertOnlyConfigurationWithoutResourceFreshness(t *testing.T) {
	document := `version: 1
monitoring:
  resources:
    enabled: false
  alerts:
    enabled: true
    interval: 30s
ddae:
  base_url: https://ddae.example.test
  source_instance: site-a
  credentials:
    username_file: /secrets/username
    password_file: /secrets/password
    client_secret_file: /secrets/client-secret
kafka:
  brokers: [kafka.example.test:9093]
  topic: ddae-alerts
state:
  dir: /var/lib/ddae-exporter
`
	cfg, err := loadYAMLMap(document, nil, testSecrets())
	if err != nil {
		t.Fatalf("load alert-only YAML: %v", err)
	}
	if cfg.ResourceMonitoringEnabled || !cfg.AlertMonitoringEnabled {
		t.Fatalf("pipeline flags resources=%v alerts=%v", cfg.ResourceMonitoringEnabled, cfg.AlertMonitoringEnabled)
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.SourceInstance != "site-a" {
		t.Fatal("alert-only dependencies were not loaded")
	}
}

func TestYAMLLoadsServiceabilityLogOnlyConfiguration(t *testing.T) {
	document := `version: 1
monitoring:
  resources:
    enabled: false
  alerts:
    enabled: false
  serviceability_logs:
    enabled: true
    interval: 30s
    list_response_max_bytes: 8388608
    detail:
      response_max_bytes: 1048576
      refresh_interval: 10m
      max_per_cycle: 200
      concurrency: 4
ddae:
  base_url: https://ddae.example.test
  source_instance: site-a
  credentials:
    username_file: /secrets/username
    password_file: /secrets/password
    client_secret_file: /secrets/client-secret
kafka:
  brokers: [kafka.example.test:9093]
  serviceability_logs_topic: ddae-serviceability-logs
state:
  dir: /var/lib/ddae-exporter
  serviceability_logs_outbox_max_bytes: 1073741824
  serviceability_logs_outbox_max_events: 100000
  serviceability_logs_checkpoint_retention: 720h
  serviceability_logs_checkpoint_max_records: 100000
`
	cfg, err := loadYAMLMap(document, nil, testSecrets())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ResourceMonitoringEnabled || cfg.AlertMonitoringEnabled || !cfg.ServiceabilityLogMonitoringEnabled || cfg.KafkaTopic != "" || cfg.KafkaServiceabilityLogTopic != "ddae-serviceability-logs" {
		t.Fatalf("log-only YAML = %#v", cfg)
	}
}

func TestYAMLRejectsBothPipelinesDisabled(t *testing.T) {
	document := strings.Replace(resourceOnlyYAML(), "enabled: true", "enabled: false", 1)
	if _, err := loadYAMLMap(document, nil, testSecrets()); err == nil {
		t.Fatal("both-disabled YAML was accepted")
	}
}

func TestYAMLResponseBodyLimitsEnforce64MiBCeiling(t *testing.T) {
	for _, value := range []string{"1", "67108864"} {
		document := strings.Replace(resourceOnlyYAML(), "  alerts:\n    enabled: false\n", "  alerts:\n    enabled: false\n    list_response_max_bytes: "+value+"\n    detail:\n      response_max_bytes: "+value+"\n", 1)
		document = strings.Replace(document, "ddae:\n", "ddae:\n  response_max_bytes: "+value+"\n", 1)
		document = strings.Replace(document, "ddae:\n", "  serviceability_logs:\n    enabled: false\n    list_response_max_bytes: "+value+"\n    detail:\n      response_max_bytes: "+value+"\nddae:\n", 1)
		if _, err := loadYAMLMap(document, nil, testSecrets()); err != nil {
			t.Fatalf("YAML response_max_bytes=%s rejected: %v", value, err)
		}
	}
	for _, value := range []string{"67108865", "9223372036854775807"} {
		document := strings.Replace(resourceOnlyYAML(), "  alerts:\n    enabled: false\n", "  alerts:\n    enabled: false\n    list_response_max_bytes: "+value+"\n    detail:\n      response_max_bytes: "+value+"\n", 1)
		document = strings.Replace(document, "ddae:\n", "ddae:\n  response_max_bytes: "+value+"\n", 1)
		document = strings.Replace(document, "ddae:\n", "  serviceability_logs:\n    enabled: false\n    list_response_max_bytes: "+value+"\n    detail:\n      response_max_bytes: "+value+"\nddae:\n", 1)
		if _, err := loadYAMLMap(document, nil, testSecrets()); err == nil {
			t.Fatalf("YAML response_max_bytes=%s accepted", value)
		}
	}
}

func TestYAMLRejectsUnknownDuplicateAndMultipleDocuments(t *testing.T) {
	for name, document := range map[string]string{
		"unknown":   "version: 1\nplaintext_password: canary\n",
		"duplicate": "version: 1\nversion: 1\n",
		"multiple":  "version: 1\n---\nversion: 1\n",
		"version":   "version: 2\n",
		"type":      "version: 1\nmonitoring:\n  resources:\n    enabled: maybe\n",
		"ddae_tls":  "version: 1\nddae:\n  tls:\n    client_cert_file: /not-supported\n",
		"alias":     "version: 1\nmonitoring:\n  resources:\n    enabled: &enabled true\n  alerts:\n    enabled: *enabled\n",
		"merge":     "version: 1\nmonitoring:\n  resources: &pipeline\n    enabled: true\n  alerts:\n    <<: *pipeline\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeYAML([]byte(document)); err == nil {
				t.Fatal("invalid YAML was accepted")
			}
		})
	}
}

func TestYAMLTypeErrorsDoNotEchoScalarValues(t *testing.T) {
	canary := "secret-type-error-canary"
	_, err := decodeYAML([]byte("version: 1\nmonitoring:\n  resources:\n    enabled: " + canary + "\n"))
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Fatalf("type error handling = %v", err)
	}
}

func TestYAMLFileBoundAndUTF8(t *testing.T) {
	directory := t.TempDir()
	exact := filepath.Join(directory, "exact.yaml")
	prefix := []byte("version: 1\n#")
	contents := append(prefix, []byte(strings.Repeat("x", yamlFileMaxBytes-len(prefix)))...)
	if err := os.WriteFile(exact, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := readYAMLFile(exact); err != nil || len(data) != yamlFileMaxBytes {
		t.Fatalf("exact-limit file length=%d error=%v", len(data), err)
	}
	oversized := filepath.Join(directory, "oversized.yaml")
	if err := os.WriteFile(oversized, []byte(strings.Repeat("x", yamlFileMaxBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readYAMLFile(oversized); err == nil {
		t.Fatal("oversized configuration was accepted")
	}
	invalidUTF8 := filepath.Join(directory, "invalid.yaml")
	if err := os.WriteFile(invalidUTF8, []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readYAMLFile(invalidUTF8); err == nil {
		t.Fatal("invalid UTF-8 was accepted")
	}
	if _, err := readYAMLFile(filepath.Join(directory, "missing.yaml")); err == nil {
		t.Fatal("missing configuration was accepted")
	}
	if _, err := readYAMLFile(directory); err == nil {
		t.Fatal("directory configuration was accepted")
	}
}

func TestCommittedSystemdExampleUsesStrictYAMLSchema(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "deploy", "systemd", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	values, err := decodeYAML(data)
	if err != nil {
		t.Fatalf("committed systemd example: %v", err)
	}
	for key, want := range map[string]string{
		"DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED": "false",
		"KAFKA_SERVICEABILITY_LOG_TOPIC":             "ddae-serviceability-logs",
		"SERVICEABILITY_LOG_OUTBOX_MAX_EVENTS":       "100000",
	} {
		if values[key] != want {
			t.Fatalf("%s = %q, want %q", key, values[key], want)
		}
	}
}
