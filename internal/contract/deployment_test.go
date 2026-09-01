package contract

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func repositoryFile(t *testing.T, parts ...string) string {
	t.Helper()
	items := append([]string{"..", ".."}, parts...)
	data, err := os.ReadFile(filepath.Join(items...))
	if err != nil {
		t.Fatalf("read %v: %v", parts, err)
	}
	return string(data)
}

func TestDeploymentProfilesRetainSecurityAndStateContracts(t *testing.T) {
	dockerfile := repositoryFile(t, "Dockerfile")
	for _, required := range []string{"@sha256:", "FROM scratch", "USER 65532:65532", "CGO_ENABLED=0", "-trimpath"} {
		if !strings.Contains(dockerfile, required) {
			t.Errorf("Dockerfile lacks %q", required)
		}
	}
	kubernetes := repositoryFile(t, "deploy", "kubernetes", "deployment.yaml")
	for _, required := range []string{
		"automountServiceAccountToken: false", "runAsNonRoot: true",
		"readOnlyRootFilesystem: true", "allowPrivilegeEscalation: false",
		"drop: [ALL]", "persistentVolumeClaim:", "strategy:\n    type: Recreate",
		"- --config", "- /etc/ddae-exporter/config.yaml", "subPath: config.yaml",
		"configMap:", "secretName: ddae-exporter-credentials", "defaultMode: 0440",
	} {
		if !strings.Contains(kubernetes, required) {
			t.Errorf("Kubernetes profile lacks %q", required)
		}
	}
	policy := repositoryFile(t, "deploy", "kubernetes", "networkpolicy.yaml")
	if !strings.Contains(policy, "ingress: []") || !strings.Contains(policy, "egress: []") {
		t.Fatal("committed Kubernetes policy is not default-deny")
	}
	systemd := repositoryFile(t, "deploy", "systemd", "ddae-exporter.service")
	for _, required := range []string{
		"LoadCredential=", "NoNewPrivileges=yes", "ProtectSystem=strict",
		"StateDirectoryMode=0700", "--config /etc/ddae-exporter/config.yaml",
	} {
		if !strings.Contains(systemd, required) {
			t.Errorf("systemd profile lacks %q", required)
		}
	}
}

func TestDeploymentExamplesContainNoLiteralSecretValues(t *testing.T) {
	for _, file := range [][]string{
		{"deploy", "kubernetes", "deployment.yaml"},
		{"deploy", "kubernetes", "configmap.yaml"},
		{"deploy", "systemd", "config.example.yaml"},
	} {
		content := repositoryFile(t, file...)
		for _, forbidden := range []string{"BEGIN PRIVATE KEY", "Bearer ", "password-test-canary", "client-secret-test-canary"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s contains forbidden secret material", filepath.Join(file...))
			}
		}
	}
}

func TestDeploymentProfilesUseStrictYAMLAndSeparateSecrets(t *testing.T) {
	for _, file := range [][]string{
		{"deploy", "kubernetes", "configmap.yaml"},
		{"deploy", "systemd", "config.example.yaml"},
	} {
		content := repositoryFile(t, file...)
		for _, required := range []string{
			"version: 1", "monitoring:", "resources:", "alerts:", "serviceability_logs:",
			"serviceability_logs_topic: ddae-serviceability-logs",
			"serviceability_logs_outbox_max_bytes:",
			"paths:", "ping_prefix: \"\"", "api_prefix: /v1",
			"allow_insecure_tls: false", "insecure_skip_verify: false",
		} {
			if !strings.Contains(content, required) {
				t.Errorf("%s lacks %q", filepath.Join(file...), required)
			}
		}
		for _, forbidden := range []string{"password:", "client_secret:", "private_key:"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s contains plaintext-secret YAML field %q", filepath.Join(file...), forbidden)
			}
		}
	}

	systemd := repositoryFile(t, "deploy", "systemd", "ddae-exporter.service")
	if strings.Contains(systemd, "EnvironmentFile=") {
		t.Fatal("systemd profile still uses the removed environment-file interface")
	}
	kubernetes := repositoryFile(t, "deploy", "kubernetes", "deployment.yaml")
	if strings.Contains(kubernetes, "envFrom:") {
		t.Fatal("Kubernetes profile still injects non-secret configuration through envFrom")
	}
}

func TestDDAEPathPrefixDocumentationAndDeploymentContract(t *testing.T) {
	readme := repositoryFile(t, "README.md")
	for _, required := range []string{
		"[繁體中文](README.zh-TW.md)",
		"ddae.paths.ping_prefix", "ddae.paths.api_prefix",
		"DDAE_PING_PATH_PREFIX", "DDAE_API_PATH_PREFIX",
		"GET /ping", "GET /v1/ddae-clusters",
		"| RC2 compatibility | `/rest/v1` | `/rest/v1` |",
		"maximum length of 128 bytes", "runtime", "alternate-path fallback",
		"deploy/systemd/config.example.yaml", "docs/runbook.md",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README lacks path-prefix contract %q", required)
		}
	}
	translatedReadme := repositoryFile(t, "README.zh-TW.md")
	for _, required := range []string{
		"[English](README.md)", "ddae.paths.ping_prefix", "ddae.paths.api_prefix",
		"DDAE_PING_PATH_PREFIX", "DDAE_API_PATH_PREFIX",
		"GET /ping", "GET /v1/ddae-clusters",
		"| RC2 compatibility | `/rest/v1` | `/rest/v1` |",
		"最大長度是 128 bytes", "runtime discovery", "alternate-path fallback",
		"deploy/systemd/config.example.yaml", "docs/runbook.md",
	} {
		if !strings.Contains(translatedReadme, required) {
			t.Errorf("Traditional Chinese README lacks path-prefix contract %q", required)
		}
	}
	runbook := repositoryFile(t, "docs", "runbook.md")
	for _, required := range []string{
		"ddae.paths.ping_prefix", "ddae.paths.api_prefix",
		"Management API returns 404", "does not probe or retry an alternate namespace",
		"ping_prefix: /rest/v1", "api_prefix: /rest/v1",
	} {
		if !strings.Contains(runbook, required) {
			t.Errorf("runbook lacks path-prefix contract %q", required)
		}
	}
}

func TestREADMELanguageSwitchAndTechnicalExamplesMatch(t *testing.T) {
	english := repositoryFile(t, "README.md")
	traditionalChinese := repositoryFile(t, "README.zh-TW.md")
	if !strings.Contains(english, "\nEnglish | [繁體中文](README.zh-TW.md)\n") {
		t.Fatal("English README language switch does not match the public format")
	}
	if !strings.Contains(traditionalChinese, "\n[English](README.md) | 繁體中文\n") {
		t.Fatal("Traditional Chinese README language switch does not match the public format")
	}
	englishBlocks := technicalFencedBlocks(english)
	translatedBlocks := technicalFencedBlocks(traditionalChinese)
	if !reflect.DeepEqual(englishBlocks, translatedBlocks) {
		t.Fatal("README technical command and configuration examples differ by language")
	}
}

func TestREADMEArchitectureUsesStaticAsset(t *testing.T) {
	for _, file := range []string{"README.md", "README.zh-TW.md"} {
		document := repositoryFile(t, file)
		if !strings.Contains(document, "](docs/architecture.svg)") {
			t.Errorf("%s does not reference the static architecture diagram", file)
		}
		if strings.Contains(document, "```mermaid") {
			t.Errorf("%s still depends on Mermaid rendering", file)
		}
	}

	diagram := repositoryFile(t, "docs", "architecture.svg")
	for _, required := range []string{
		"DDAE Management API", "OAuth and read-only", "Resource", "Current",
		"Alert pipeline", "Serviceability Logs", "state.db",
		"serviceability-logs.db", "Prometheus", "Kafka topics", "Downstream",
	} {
		if !strings.Contains(diagram, required) {
			t.Errorf("architecture diagram lacks %q", required)
		}
	}
}

func technicalFencedBlocks(document string) []string {
	lines := strings.Split(document, "\n")
	blocks := make([]string, 0)
	var current []string
	capture := false
	inside := false
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if !inside {
				inside = true
				capture = line != "```text"
				if capture {
					current = []string{line}
				}
				continue
			}
			if capture {
				current = append(current, line)
				blocks = append(blocks, strings.Join(current, "\n"))
			}
			inside = false
			capture = false
			current = nil
			continue
		}
		if inside && capture {
			current = append(current, line)
		}
	}
	return blocks
}
