package portable

import (
	"encoding/pem"
	"gopkg.in/yaml.v3"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fieldConfig(t *testing.T, handler http.HandlerFunc) (Config, string) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/realms/ddae/protocol/openid-connect/token" {
			w.Write([]byte(`{"access_token":"synthetic-token","expires_in":3600}`))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	root := privateTemp(t)
	c := defaultConfig()
	c.DDAE.BaseURL = server.URL
	c.Run.Duration = "1s"
	c.Run.Interval = "1s"
	c.DDAE.TLS.CA = "ca.pem"
	c.DDAE.Credentials.Username = "user.txt"
	c.DDAE.Credentials.Password = "password.txt"
	c.DDAE.Credentials.Secret = "secret.txt"
	for name, value := range map[string]string{"user.txt": "synthetic-user", "password.txt": "synthetic-password", "secret.txt": "synthetic-secret"} {
		if os.WriteFile(filepath.Join(root, name), []byte(value+"\r\n"), 0600) != nil {
			t.Fatal("fixture write")
		}
	}
	if os.WriteFile(filepath.Join(root, "ca.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600) != nil {
		t.Fatal("CA write")
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.yaml")
	if os.WriteFile(path, data, 0600) != nil {
		t.Fatal("config write")
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	return loaded, path
}
func TestPortableConfigStrictAndIsolated(t *testing.T) {
	for _, bad := range []string{
		"ddae: {credentials: {username_file: 123}}", "run: {duration: true}",
		"checks: {ping: 'true'}", "run: {max_requests: 1.5}",
		"checks: {ping: false, resources: false, alerts: false, serviceability_logs: false}",
		"run: {shutdown_grace_period: 31s}", "run: {max_details_per_family_per_cycle: 101}",
		"capture: {max_total_bytes: 2147483649}", "capture: {max_body_bytes: 0}",
		"output: {directory: ''}",
	} {
		if _, err := parseConfig([]byte("version: 1\n" + bad)); err == nil {
			t.Fatal("malformed or out-of-bound field accepted")
		}
	}
	for _, bad := range []string{"", "version: 2", "version: 1\nunknown: true", "version: 1\nversion: 1", "version: 1\n---\nversion: 1", "version: 1\nchecks: {ping: null}", "version: 1\nchecks: &x {ping: true}", "version: 1\nrun: {duration: 0s}", "version: 1\nrun: {interval: 301s}", "version: 1\nrun: {max_requests: 10001}", "version: 1\ncapture: {max_body_bytes: 67108865}", "version: 1\noutput: {max_report_bytes: 1}"} {
		if _, err := parseConfig([]byte(bad)); err == nil {
			t.Errorf("invalid case accepted")
		}
	}
	c, path := fieldConfig(t, func(w http.ResponseWriter, r *http.Request) {})
	t.Setenv("DDAE_BASE_URL", "https://environment-canary.invalid")
	t.Setenv("DDAE_ALERT_MONITORING_ENABLED", "true")
	loaded, err := LoadConfig(path)
	if err != nil || loaded.Client.AlertMonitoringEnabled || loaded.Client.DDAEBaseURL.String() != c.Client.DDAEBaseURL.String() {
		t.Fatal("environment affected diagnostic config")
	}
	if loaded.Client.DDAEPassword.Value() != "synthetic-password" {
		t.Fatal("newline handling changed")
	}
	data, _ := os.ReadFile(path)
	for _, replacement := range []string{"username_file: missing-file", "username_file: ''"} {
		changed := strings.Replace(string(data), "username_file: user.txt", replacement, 1)
		if os.WriteFile(path, []byte(changed), 0600) != nil {
			t.Fatal("write")
		}
		if _, err := LoadConfig(path); err == nil {
			t.Fatal("missing secret accepted")
		}
	}
}
