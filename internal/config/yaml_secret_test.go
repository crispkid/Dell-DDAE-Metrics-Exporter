package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestYAMLRejectsPlaintextSecretFields(t *testing.T) {
	document := `version: 1
ddae:
  credentials:
    password: plaintext-canary
`
	_, err := decodeYAML([]byte(document))
	if err == nil || strings.Contains(err.Error(), "plaintext-canary") {
		t.Fatalf("plaintext secret handling error = %v", err)
	}
}

func TestDirectEnvironmentSecretOverridesYAMLFilePath(t *testing.T) {
	environment := map[string]string{"DDAE_PASSWORD": "direct-canary"}
	cfg, err := loadYAMLMap(resourceOnlyYAML(), environment, testSecrets())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DDAEPassword.Value() != "direct-canary" {
		t.Fatal("direct environment secret did not override YAML file path")
	}
}

func TestEnvironmentSecretFileOverridesYAMLFilePath(t *testing.T) {
	environment := map[string]string{"DDAE_PASSWORD_FILE": "/runtime/password"}
	files := testSecrets()
	files["/runtime/password"] = []byte("runtime-file-canary")
	cfg, err := loadYAMLMap(resourceOnlyYAML(), environment, files)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DDAEPassword.Value() != "runtime-file-canary" {
		t.Fatal("environment secret file did not override YAML file path")
	}
}

func TestExplicitEnvironmentSecretPairStillConflicts(t *testing.T) {
	environment := map[string]string{
		"DDAE_PASSWORD":      "direct-canary",
		"DDAE_PASSWORD_FILE": "/secrets/password",
	}
	_, err := loadYAMLMap(resourceOnlyYAML(), environment, testSecrets())
	if err == nil {
		t.Fatal("explicit direct and file pair was accepted")
	}
	if strings.Contains(err.Error(), "direct-canary") || strings.Contains(err.Error(), "password-canary") {
		t.Fatal("secret value appeared in error")
	}
}

func TestDisabledKafkaExplicitEnvironmentSecretPairStillConflicts(t *testing.T) {
	environment := map[string]string{
		"KAFKA_SASL_PASSWORD":      "direct-kafka-canary",
		"KAFKA_SASL_PASSWORD_FILE": "/secrets/kafka-password",
	}
	_, err := loadYAMLMap(resourceOnlyYAML(), environment, testSecrets())
	if err == nil {
		t.Fatal("disabled Kafka direct and file pair was accepted")
	}
	if strings.Contains(err.Error(), "direct-kafka-canary") {
		t.Fatal("Kafka secret value appeared in error")
	}
}

func TestYAMLSecretFilesRetainBoundsAndContentValidation(t *testing.T) {
	for name, password := range map[string][]byte{
		"missing":   nil,
		"oversized": bytes.Repeat([]byte("x"), secretFileMaxBytes+1),
		"nul":       []byte("canary\x00value"),
		"utf8":      {0xff},
	} {
		t.Run(name, func(t *testing.T) {
			files := testSecrets()
			if password == nil {
				delete(files, "/secrets/password")
			} else {
				files["/secrets/password"] = password
			}
			_, err := loadYAMLMap(resourceOnlyYAML(), nil, files)
			if err == nil {
				t.Fatal("invalid secret file was accepted")
			}
			if strings.Contains(err.Error(), "canary") {
				t.Fatal("secret content appeared in error")
			}
		})
	}
}
