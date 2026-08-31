package contract

import (
	"os"
	"path/filepath"
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
		"ddae.paths.ping_prefix", "ddae.paths.api_prefix",
		"DDAE_PING_PATH_PREFIX", "DDAE_API_PATH_PREFIX",
		"GET /ping", "GET /v1/ddae-clusters",
		"| Preserve v1.0.0-rc2 routes | `/rest/v1` | `/rest/v1` |",
		"Maximum length: 128 bytes", "runtime discovery", "alternate-path",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README lacks path-prefix contract %q", required)
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
