package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func queryEventsSASLEnvironment(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"DDAE_RESOURCE_MONITORING_ENABLED":           "false",
		"DDAE_ALERT_MONITORING_ENABLED":              "false",
		"DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED": "false",
		"DDAE_SOURCE_INSTANCE":                       "sasl-regression",
		"QUERY_ENABLED":                              "true", "QUERY_EVENTS_ENABLED": "true",
		"QUERY_BASE_URL": "https://engine.invalid", "QUERY_AUTH_URL": "https://auth.invalid",
		"QUERY_REALM": "ddae", "QUERY_ROLE": "sysadmin",
		"QUERY_USERNAME": "query-user-canary", "QUERY_PASSWORD": "query-password-canary",
		"QUERY_KAFKA_TOPIC": "query-events", "KAFKA_BROKERS": "broker.invalid:9093",
		"KAFKA_SASL_MECHANISM": "PLAIN", "KAFKA_SASL_USERNAME": "kafka-user-canary",
		"KAFKA_SASL_PASSWORD": "kafka-password-canary", "STATE_DIR": t.TempDir(),
	}
}

// TEST-DDAE-11-001 / AC-DDAE-11-001: inspect the loaded credentials, not just the error.
func TestQueryEventsKafkaSASLCredentials(t *testing.T) {
	for _, mechanism := range []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"} {
		for _, tc := range []struct {
			name, direct, file, want string
		}{
			{name: "direct", direct: "kafka-password-canary", want: "kafka-password-canary"},
			{name: "direct-whitespace-preserved", direct: " password-canary \n", want: " password-canary \n"},
			{name: "file", file: "kafka-password-canary", want: "kafka-password-canary"},
			{name: "file-crlf", file: "kafka-password-canary\r\n", want: "kafka-password-canary"},
			{name: "file-one-newline-only", file: "kafka-password-canary\n\n", want: "kafka-password-canary\n"},
			{name: "file-minimum", file: "x", want: "x"},
			{name: "file-maximum", file: strings.Repeat("x", secretFileMaxBytes), want: strings.Repeat("x", secretFileMaxBytes)},
			{name: "file-utf8", file: "合成密碼-canary\n", want: "合成密碼-canary"},
		} {
			t.Run(mechanism+"/"+tc.name, func(t *testing.T) {
				env := queryEventsSASLEnvironment(t)
				env["KAFKA_SASL_MECHANISM"] = strings.ToLower(mechanism)
				files := map[string][]byte{}
				if tc.file != "" {
					delete(env, "KAFKA_SASL_PASSWORD")
					env["KAFKA_SASL_PASSWORD_FILE"] = "/synthetic/kafka-password"
					files[env["KAFKA_SASL_PASSWORD_FILE"]] = []byte(tc.file)
				} else {
					env["KAFKA_SASL_PASSWORD"] = tc.direct
				}
				cfg, err := loadMap(env, files)
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				if cfg.KafkaSASLMechanism != mechanism || cfg.KafkaSASLUsername.Value() != env["KAFKA_SASL_USERNAME"] || cfg.KafkaSASLPassword.Value() != tc.want {
					t.Fatal("query-only SASL mechanism or credentials were not loaded correctly")
				}
				if !cfg.Query.Events || cfg.DDAEBaseURL != nil || !cfg.DDAEPassword.Empty() {
					t.Fatal("query-only profile unexpectedly initialized Management credentials")
				}
			})
		}
	}
}

// TEST-DDAE-11-001 / AC-DDAE-11-001: actual YAML decoding plus field-level env precedence.
func TestQueryEventsKafkaSASLYAMLPrecedence(t *testing.T) {
	for _, mechanism := range []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"} {
		for _, source := range []string{"yaml", "env-direct", "env-file"} {
			t.Run(mechanism+"/"+source, func(t *testing.T) {
				env := queryEventsSASLEnvironment(t)
				for _, key := range []string{"KAFKA_SASL_MECHANISM", "KAFKA_SASL_USERNAME", "KAFKA_SASL_PASSWORD"} {
					delete(env, key)
				}
				yamlMechanism := mechanism
				wantUser, wantPassword := "yaml-user-canary", "yaml-password-canary"
				files := map[string][]byte{"/synthetic/yaml-password": []byte(wantPassword)}
				if source != "yaml" {
					// Invalid YAML input must be superseded by the corresponding env field.
					yamlMechanism = "invalid-when-not-overridden"
					env["KAFKA_SASL_MECHANISM"] = mechanism
					wantUser, wantPassword = "env-user-canary", "env-password-canary"
					env["KAFKA_SASL_USERNAME"] = wantUser
					delete(files, "/synthetic/yaml-password")
					if source == "env-direct" {
						env["KAFKA_SASL_PASSWORD"] = wantPassword
					} else {
						env["KAFKA_SASL_PASSWORD_FILE"] = "/synthetic/env-password"
						files[env["KAFKA_SASL_PASSWORD_FILE"]] = []byte(wantPassword)
					}
				}
				document := fmt.Sprintf("version: 1\nkafka:\n  sasl:\n    mechanism: %s\n    username: yaml-user-canary\n    password_file: /synthetic/yaml-password\n", yamlMechanism)
				cfg, err := loadYAMLMap(document, env, files)
				if err != nil {
					t.Fatalf("load YAML: %v", err)
				}
				if cfg.KafkaSASLMechanism != mechanism || cfg.KafkaSASLUsername.Value() != wantUser || cfg.KafkaSASLPassword.Value() != wantPassword {
					t.Fatal("YAML/env SASL precedence did not yield the expected credentials")
				}
			})
		}
	}
}

// TEST-DDAE-11-002 / AC-DDAE-11-002: fail at configuration loading, with redacted errors.
func TestQueryEventsKafkaSASLRejectsInvalidCredentials(t *testing.T) {
	const passwordPath = "/synthetic/private-path-canary"
	for _, mechanism := range []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"} {
		for _, tc := range []struct {
			name, key, value, file, want  string
			absent, fromFile, readFailure bool
		}{
			{name: "missing-username", key: "KAFKA_SASL_USERNAME", absent: true, want: "KAFKA_SASL_USERNAME is required"},
			{name: "empty-username", key: "KAFKA_SASL_USERNAME", want: "KAFKA_SASL_USERNAME is required"},
			{name: "blank-username", key: "KAFKA_SASL_USERNAME", value: " \t", want: "KAFKA_SASL_USERNAME is required"},
			{name: "nul-username", key: "KAFKA_SASL_USERNAME", value: "user-canary\x00", want: "KAFKA_SASL_USERNAME is invalid"},
			{name: "missing-password", key: "KAFKA_SASL_PASSWORD", absent: true, want: "KAFKA_SASL_PASSWORD or KAFKA_SASL_PASSWORD_FILE is required"},
			{name: "empty-password", key: "KAFKA_SASL_PASSWORD", want: "KAFKA_SASL_PASSWORD is invalid"},
			{name: "nul-password", key: "KAFKA_SASL_PASSWORD", value: "password-canary\x00", want: "KAFKA_SASL_PASSWORD is invalid"},
			{name: "invalid-utf8-password", key: "KAFKA_SASL_PASSWORD", value: "password-canary\xff", want: "KAFKA_SASL_PASSWORD is invalid"},
			{name: "empty-file-path", fromFile: true, key: "KAFKA_SASL_PASSWORD_FILE", want: "KAFKA_SASL_PASSWORD_FILE is invalid"},
			{name: "read-failure", fromFile: true, readFailure: true, want: "cannot read KAFKA_SASL_PASSWORD_FILE"},
			{name: "empty-file", fromFile: true, want: "KAFKA_SASL_PASSWORD is invalid"},
			{name: "newline-only-file", fromFile: true, file: "\r\n", want: "KAFKA_SASL_PASSWORD is invalid"},
			{name: "nul-file", fromFile: true, file: "file-content-canary\x00", want: "KAFKA_SASL_PASSWORD is invalid"},
			{name: "invalid-utf8-file", fromFile: true, file: "file-content-canary\xff", want: "KAFKA_SASL_PASSWORD is invalid"},
			{name: "oversized-file", fromFile: true, file: strings.Repeat("x", secretFileMaxBytes+1), want: "KAFKA_SASL_PASSWORD_FILE exceeds the size limit"},
			{name: "oversized-before-trimming", fromFile: true, file: strings.Repeat("x", secretFileMaxBytes) + "\n", want: "KAFKA_SASL_PASSWORD_FILE exceeds the size limit"},
			{name: "direct-file-conflict", key: "KAFKA_SASL_PASSWORD_FILE", value: passwordPath, want: "KAFKA_SASL_PASSWORD and KAFKA_SASL_PASSWORD_FILE conflict"},
		} {
			t.Run(mechanism+"/"+tc.name, func(t *testing.T) {
				env := queryEventsSASLEnvironment(t)
				env["KAFKA_SASL_MECHANISM"] = mechanism
				if tc.fromFile {
					delete(env, "KAFKA_SASL_PASSWORD")
					env["KAFKA_SASL_PASSWORD_FILE"] = passwordPath
				}
				if tc.key != "" {
					if tc.absent {
						delete(env, tc.key)
					} else {
						env[tc.key] = tc.value
					}
				}
				lookup := func(key string) (string, bool) { value, ok := env[key]; return value, ok }
				read := func(path string) ([]byte, error) {
					if path != passwordPath {
						t.Fatal("unexpected credential file read")
					}
					if tc.readFailure {
						return nil, errors.New("reader-error-canary with private-path-canary")
					}
					return []byte(tc.file), nil
				}
				_, err := load(lookup, read)
				if err == nil {
					t.Fatal("invalid query-only Kafka credentials were accepted")
				}
				for _, canary := range []string{"user-canary", "password-canary", "file-content-canary", "reader-error-canary", "private-path-canary"} {
					if strings.Contains(err.Error(), canary) {
						t.Fatal("configuration error exposed a sensitive canary")
					}
				}
				if err.Error() != tc.want {
					t.Fatalf("unexpected validation error, expected %q", tc.want)
				}
			})
		}
	}
}

// TEST-DDAE-11-003 / AC-DDAE-11-003: all publisher combinations share the same secret boundary.
func TestKafkaSASLPipelineSelection(t *testing.T) {
	for _, mechanism := range []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"} {
		for mask := 0; mask < 8; mask++ {
			for _, resources := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/publishers-%03b/resources-%t", mechanism, mask, resources), func(t *testing.T) {
					env := queryEventsSASLEnvironment(t)
					alerts, logs, events := mask&1 != 0, mask&2 != 0, mask&4 != 0
					env["DDAE_RESOURCE_MONITORING_ENABLED"] = strconv.FormatBool(resources)
					env["DDAE_ALERT_MONITORING_ENABLED"] = strconv.FormatBool(alerts)
					env["DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED"] = strconv.FormatBool(logs)
					env["QUERY_EVENTS_ENABLED"] = strconv.FormatBool(events)
					env["KAFKA_SASL_MECHANISM"] = mechanism
					if resources || alerts || logs {
						env["DDAE_BASE_URL"] = "https://management.invalid"
						env["DDAE_USERNAME"] = "management-user-canary"
						env["DDAE_PASSWORD"] = "management-password-canary"
						env["DDAE_CLIENT_SECRET"] = "management-secret-canary"
					}
					if alerts {
						env["KAFKA_TOPIC"] = "alerts"
					}
					delete(env, "KAFKA_SASL_PASSWORD")
					env["KAFKA_SASL_PASSWORD_FILE"] = "/synthetic/kafka-password"
					if mask == 0 {
						delete(env, "KAFKA_BROKERS")
						delete(env, "KAFKA_SASL_USERNAME")
					}
					reads := 0
					lookup := func(key string) (string, bool) { value, ok := env[key]; return value, ok }
					read := func(path string) ([]byte, error) {
						reads++
						if path != "/synthetic/kafka-password" || mask == 0 {
							return nil, errors.New("unexpected Kafka credential read")
						}
						return []byte("kafka-password-canary"), nil
					}
					cfg, err := load(lookup, read)
					if err != nil {
						t.Fatalf("load: %v", err)
					}
					if mask == 0 {
						if reads != 0 || !cfg.KafkaSASLUsername.Empty() || !cfg.KafkaSASLPassword.Empty() {
							t.Fatal("metrics-only profile read Kafka credentials")
						}
					} else if reads != 1 || cfg.KafkaSASLUsername.Value() != "kafka-user-canary" || cfg.KafkaSASLPassword.Value() != "kafka-password-canary" {
						t.Fatal("enabled publishers did not share one correctly loaded Kafka credential pair")
					}
				})
			}
		}
	}

	for _, profile := range []string{"resources-only", "disabled-query-with-events-flag", "query-events-without-sasl"} {
		t.Run(profile, func(t *testing.T) {
			env := queryEventsSASLEnvironment(t)
			delete(env, "KAFKA_SASL_USERNAME")
			delete(env, "KAFKA_SASL_PASSWORD")
			env["KAFKA_SASL_PASSWORD_FILE"] = "/synthetic/unused-password"
			if profile == "query-events-without-sasl" {
				delete(env, "KAFKA_SASL_MECHANISM")
			} else {
				env["DDAE_RESOURCE_MONITORING_ENABLED"] = "true"
				env["QUERY_ENABLED"] = "false"
				if profile == "resources-only" {
					env["QUERY_EVENTS_ENABLED"] = "false"
				}
				env["DDAE_BASE_URL"] = "https://management.invalid"
				env["DDAE_USERNAME"] = "management-user-canary"
				env["DDAE_PASSWORD"] = "management-password-canary"
				env["DDAE_CLIENT_SECRET"] = "management-secret-canary"
				delete(env, "KAFKA_BROKERS")
			}
			reads := 0
			lookup := func(key string) (string, bool) { value, ok := env[key]; return value, ok }
			read := func(string) ([]byte, error) {
				reads++
				return nil, errors.New("unused secret must not be read")
			}
			cfg, err := load(lookup, read)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if reads != 0 || !cfg.KafkaSASLUsername.Empty() || !cfg.KafkaSASLPassword.Empty() {
				t.Fatal("unused Kafka credentials were read")
			}
			if cfg.Query.Events != (profile == "query-events-without-sasl") {
				t.Fatal("query event enable/disable semantics changed")
			}
		})
	}
}
