package contract

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func loadTLSYAML(t *testing.T, body string) (config.Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DDAE_USERNAME", "monitor")
	t.Setenv("DDAE_PASSWORD", "password-canary")
	t.Setenv("DDAE_CLIENT_SECRET", "client-secret-canary")
	return config.LoadFile(path)
}

func ddaeTLSYAML(baseURL, caFile string, allowInsecure, skipVerify bool) string {
	caLine := ""
	if caFile != "" {
		caLine = fmt.Sprintf("    ca_file: %s\n", caFile)
	}
	return fmt.Sprintf(`version: 1
monitoring:
  resources:
    enabled: true
  alerts:
    enabled: false
security:
  allow_insecure_tls: %t
ddae:
  base_url: %s
  tls:
%s    insecure_skip_verify: %t
`, allowInsecure, baseURL, caLine, skipVerify)
}

func tlsDDAEServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/auth/realms/ddae/protocol/openid-connect/token" {
			_, _ = writer.Write([]byte(`{"access_token":"contract-token","expires_in":3600}`))
			return
		}
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	}))
}

func writeServerCA(t *testing.T, server *httptest.Server) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server-ca.pem")
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultTLSFailsClosedAndCustomCASucceeds(t *testing.T) {
	server := tlsDDAEServer()
	defer server.Close()

	untrustedConfig, err := loadTLSYAML(t, ddaeTLSYAML(server.URL, "", false, false))
	if err != nil {
		t.Fatal(err)
	}
	untrusted, err := ddae.NewClient(untrustedConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer untrusted.CloseIdleConnections()
	if _, err := untrusted.Ping(context.Background()); err == nil {
		t.Fatal("default TLS accepted an untrusted certificate")
	}

	trustedConfig, err := loadTLSYAML(t, ddaeTLSYAML(server.URL, writeServerCA(t, server), false, false))
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := ddae.NewClient(trustedConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer trusted.CloseIdleConnections()
	if _, err := trusted.Ping(context.Background()); err != nil {
		t.Fatalf("custom CA request: %v", err)
	}
}
