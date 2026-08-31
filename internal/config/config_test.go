package config

import (
	"errors"
	"strings"
	"testing"
)

func validEnvironment() map[string]string {
	return map[string]string{
		"DDAE_BASE_URL":        "https://ddae.example.test",
		"DDAE_SOURCE_INSTANCE": "site-a",
		"DDAE_USERNAME":        "monitor",
		"DDAE_PASSWORD":        "password-canary",
		"DDAE_CLIENT_SECRET":   "client-secret-canary",
		"KAFKA_BROKERS":        "kafka.example.test:9093",
		"KAFKA_TOPIC":          "ddae-alerts",
		"STATE_DIR":            "/tmp/ddae-exporter-test",
	}
}

func loadMap(values map[string]string, files map[string][]byte) (Config, error) {
	lookup := func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
	readFile := func(path string) ([]byte, error) {
		value, ok := files[path]
		if !ok {
			return nil, errors.New("missing")
		}
		return value, nil
	}
	return load(lookup, readFile)
}

func TestLoadDefaultsAndSecrets(t *testing.T) {
	cfg, err := loadMap(validEnvironment(), nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.DDAEBaseURL.String(); got != "https://ddae.example.test" {
		t.Fatalf("base URL = %q", got)
	}
	if cfg.DDAEPassword.Value() != "password-canary" {
		t.Fatal("password was not loaded")
	}
	if cfg.DDAEPassword.Empty() || TLSMinVersion() == 0 {
		t.Fatal("loaded secret or TLS minimum invariant is invalid")
	}
	if cfg.ListenAddress != "127.0.0.1:9469" || cfg.AlertDetailConcurrency != 4 {
		t.Fatalf("defaults not applied: %#v", cfg)
	}
	if cfg.DDAEPingPathPrefix != "" || cfg.DDAEAPIPathPrefix != "/v1" {
		t.Fatalf("path-prefix defaults ping=%q api=%q", cfg.DDAEPingPathPrefix, cfg.DDAEAPIPathPrefix)
	}
	if cfg.ServiceabilityLogMonitoringEnabled || cfg.KafkaServiceabilityLogTopic != "ddae-serviceability-logs" ||
		cfg.ServiceabilityLogDetailConcurrency != 4 || cfg.ServiceabilityLogListResponseMaxBytes != 8*1024*1024 {
		t.Fatalf("serviceability defaults not applied: %#v", cfg)
	}
}

func TestSecretFileRemovesOneLineEnding(t *testing.T) {
	env := validEnvironment()
	delete(env, "DDAE_PASSWORD")
	env["DDAE_PASSWORD_FILE"] = "/secret/password"
	cfg, err := loadMap(env, map[string][]byte{"/secret/password": []byte("value\n\n")})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.DDAEPassword.Value(); got != "value\n" {
		t.Fatalf("secret = %q", got)
	}
}

func TestSecretConflictDoesNotExposeValues(t *testing.T) {
	env := validEnvironment()
	env["DDAE_PASSWORD_FILE"] = "/secret/password"
	_, err := loadMap(env, map[string][]byte{"/secret/password": []byte("file-value")})
	if err == nil {
		t.Fatal("expected conflict")
	}
	for _, canary := range []string{"password-canary", "file-value"} {
		if strings.Contains(err.Error(), canary) {
			t.Fatalf("error exposed secret %q", canary)
		}
	}
}

func TestRejectsUnsafeBaseURLAndTiming(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"http":          func(env map[string]string) { env["DDAE_BASE_URL"] = "http://ddae.example.test" },
		"userinfo":      func(env map[string]string) { env["DDAE_BASE_URL"] = "https://user:pass@ddae.example.test" },
		"path":          func(env map[string]string) { env["DDAE_BASE_URL"] = "https://ddae.example.test/rest" },
		"timing":        func(env map[string]string) { env["DDAE_REQUEST_TIMEOUT"] = "25s" },
		"kafka-timeout": func(env map[string]string) { env["KAFKA_PUBLISH_TIMEOUT"] = "500ms" },
	} {
		t.Run(name, func(t *testing.T) {
			env := validEnvironment()
			mutate(env)
			if _, err := loadMap(env, nil); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestResponseBodyLimitsAcceptInclusiveBoundariesAndRejectOverflow(t *testing.T) {
	for _, name := range []string{"DDAE_RESPONSE_MAX_BYTES", "ALERT_LIST_RESPONSE_MAX_BYTES", "ALERT_DETAIL_RESPONSE_MAX_BYTES", "SERVICEABILITY_LOG_LIST_RESPONSE_MAX_BYTES", "SERVICEABILITY_LOG_DETAIL_RESPONSE_MAX_BYTES"} {
		for _, value := range []string{"1", "4194304", "67108864"} {
			env := validEnvironment()
			env[name] = value
			if _, err := loadMap(env, nil); err != nil {
				t.Fatalf("%s=%s rejected: %v", name, value, err)
			}
		}
		for _, value := range []string{"67108865", "9223372036854775807"} {
			env := validEnvironment()
			env[name] = value
			if _, err := loadMap(env, nil); err == nil {
				t.Fatalf("%s=%s accepted", name, value)
			}
		}
	}
}

func TestServiceabilityLogOnlyConfigurationAndIsolationValidation(t *testing.T) {
	env := validEnvironment()
	env["DDAE_RESOURCE_MONITORING_ENABLED"] = "false"
	env["DDAE_ALERT_MONITORING_ENABLED"] = "false"
	env["DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED"] = "true"
	env["KAFKA_SERVICEABILITY_LOG_TOPIC"] = "ddae-logs"
	delete(env, "KAFKA_TOPIC")
	cfg, err := loadMap(env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ServiceabilityLogMonitoringEnabled || cfg.AlertMonitoringEnabled || cfg.KafkaServiceabilityLogTopic != "ddae-logs" {
		t.Fatalf("log-only config = %#v", cfg)
	}

	env = validEnvironment()
	env["DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED"] = "true"
	env["KAFKA_SERVICEABILITY_LOG_TOPIC"] = env["KAFKA_TOPIC"]
	if _, err := loadMap(env, nil); err == nil {
		t.Fatal("shared alert/log topic was accepted")
	}
}
