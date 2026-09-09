package config

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// AC-DDAE-9-001: query-only must not require Management credentials.
func TestQueryOnlyAndDisabledIsolation(t *testing.T) {
	values := map[string]string{"DDAE_RESOURCE_MONITORING_ENABLED": "false", "DDAE_ALERT_MONITORING_ENABLED": "false", "QUERY_ENABLED": "true", "QUERY_BASE_URL": "https://engine.invalid", "QUERY_AUTH_URL": "https://auth.invalid", "QUERY_REALM": "ddae", "QUERY_ROLE": "sysadmin", "QUERY_USERNAME_FILE": "u", "QUERY_PASSWORD_FILE": "p", "DDAE_SOURCE_INSTANCE": "test", "STATE_DIR": filepath.Join(t.TempDir(), "state")}
	lookup := func(k string) (string, bool) { v, ok := values[k]; return v, ok }
	read := func(path string) ([]byte, error) {
		if path == "missing" {
			return nil, errors.New("disabled query secret was read")
		}
		return []byte("synthetic"), nil
	}
	cfg, err := load(lookup, read)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Query.Enabled || cfg.DDAEBaseURL != nil {
		t.Fatal("query-only configuration not isolated")
	}
	values["QUERY_ENABLED"] = "false"
	values["DDAE_RESOURCE_MONITORING_ENABLED"] = "true"
	values["DDAE_BASE_URL"] = "https://management.invalid"
	values["DDAE_USERNAME"] = "u"
	values["DDAE_PASSWORD"] = "p"
	values["DDAE_CLIENT_SECRET"] = "s"
	values["QUERY_BASE_URL"] = "bad"
	values["QUERY_PASSWORD_FILE"] = "missing"
	if _, err := load(lookup, read); err != nil {
		t.Fatal(err)
	}
}

func TestQueryYAMLPrecedenceAndBounds(t *testing.T) {
	env := map[string]string{"QUERY_ENABLED": "true", "QUERY_BASE_URL": "https://engine.invalid", "QUERY_AUTH_URL": "https://auth.invalid", "QUERY_REALM": "ddae", "QUERY_ROLE": "sysadmin", "QUERY_USERNAME": "synthetic", "QUERY_PASSWORD": "synthetic", "DDAE_RESOURCE_MONITORING_ENABLED": "false", "DDAE_ALERT_MONITORING_ENABLED": "false", "DDAE_SOURCE_INSTANCE": "test", "STATE_DIR": t.TempDir()}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	read := func(string) ([]byte, error) { return nil, errors.New("secret file must not be read") }
	for _, tc := range []struct{ key, value string }{{"QUERY_BASE_URL", "http://bad"}, {"QUERY_AUTH_URL", "https://bad/path"}, {"QUERY_ROLE", "sysadmin}\r\n"}, {"QUERY_REALM", "../other"}, {"QUERY_DETAIL_CONCURRENCY", "0"}, {"QUERY_RESPONSE_MAX_BYTES", "67108865"}, {"QUERY_TLS_INSECURE_SKIP_VERIFY", "true"}, {"QUERY_EVENTS_ENABLED", "true"}} {
		old, exists := env[tc.key]
		env[tc.key] = tc.value
		if _, err := load(lookup, read); err == nil {
			t.Fatal("invalid accepted", tc.key)
		}
		if exists {
			env[tc.key] = old
		} else {
			delete(env, tc.key)
		}
	}
	yaml := map[string]string{"QUERY_PASSWORD_FILE": "missing", "QUERY_INTERVAL": "45s"}
	env["QUERY_INTERVAL"] = "50s"
	cfg, err := load(layeredLookup(yaml, lookup), read)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query.Interval != 50*time.Second || cfg.Query.Password.Value() != "synthetic" {
		t.Fatal("query precedence")
	}
	env["QUERY_EVENTS_ENABLED"] = "true"
	env["QUERY_KAFKA_TOPIC"] = "ddae-serviceability-logs"
	env["KAFKA_BROKERS"] = "broker.invalid:9093"
	if _, err := load(lookup, read); err == nil {
		t.Fatal("shared topic accepted")
	}
}
