package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnvironmentIntervalPrecedenceOverYAML(t *testing.T) {
	document := `version: 1
monitoring:
  resources:
    enabled: true
    interval: 45s
    stale_after: 2m
  alerts:
    enabled: true
    interval: 50s
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
  dir: /tmp/ddae-exporter-test
`
	environment := map[string]string{
		"DDAE_COLLECTION_INTERVAL":          "40s",
		"DDAE_RESOURCE_COLLECTION_INTERVAL": "35s",
	}
	cfg, err := loadYAMLMap(document, environment, testSecrets())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ResourceCollectionInterval != 35*time.Second || cfg.AlertCollectionInterval != 40*time.Second {
		t.Fatalf("intervals resources=%s alerts=%s", cfg.ResourceCollectionInterval, cfg.AlertCollectionInterval)
	}
}

func TestEnvironmentOnlyRetainsDualPipelineDefaults(t *testing.T) {
	cfg, err := loadMap(validEnvironment(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ResourceMonitoringEnabled || !cfg.AlertMonitoringEnabled {
		t.Fatal("legacy environment did not enable both pipelines")
	}
	if cfg.ResourceCollectionInterval != 30*time.Second || cfg.AlertCollectionInterval != 30*time.Second {
		t.Fatal("legacy interval defaults changed")
	}
}

func TestCommandLineConfigPathWinsOverEnvironmentSelector(t *testing.T) {
	directory := t.TempDir()
	environmentFile := filepath.Join(directory, "environment.yaml")
	commandLineFile := filepath.Join(directory, "command.yaml")
	base := resourceOnlyYAML()
	if err := os.WriteFile(environmentFile, []byte(base+"server:\n  listen_address: 127.0.0.1:9101\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(commandLineFile, []byte(base+"server:\n  listen_address: 127.0.0.1:9102\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DDAE_EXPORTER_CONFIG_FILE", environmentFile)
	t.Setenv("DDAE_USERNAME", "monitor")
	t.Setenv("DDAE_PASSWORD", "password-canary")
	t.Setenv("DDAE_CLIENT_SECRET", "client-secret-canary")
	cfg, err := LoadFile(commandLineFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddress != "127.0.0.1:9102" {
		t.Fatalf("listen address = %s", cfg.ListenAddress)
	}
}

func TestEnvironmentConfigSelectorLoadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selected.yaml")
	if err := os.WriteFile(path, []byte(resourceOnlyYAML()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DDAE_EXPORTER_CONFIG_FILE", path)
	t.Setenv("DDAE_USERNAME", "monitor")
	t.Setenv("DDAE_PASSWORD", "password-canary")
	t.Setenv("DDAE_CLIENT_SECRET", "client-secret-canary")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ResourceMonitoringEnabled || cfg.AlertMonitoringEnabled {
		t.Fatal("environment-selected YAML was not loaded")
	}
}
