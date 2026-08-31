package ddae

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
)

func trustedServerCA(t *testing.T, server *httptest.Server) string {
	t.Helper()
	certificate := server.Certificate()
	if certificate == nil {
		t.Fatal("TLS test server did not expose a certificate")
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write test CA: %v", err)
	}
	return path
}

func clientConfig(t *testing.T, baseURL, caFile string, extra map[string]string) config.Config {
	t.Helper()
	keys := []string{
		"DDAE_USERNAME_FILE", "DDAE_PASSWORD_FILE", "DDAE_CLIENT_SECRET_FILE",
		"DDAE_PING_PATH_PREFIX", "DDAE_API_PATH_PREFIX",
		"KAFKA_SASL_PASSWORD_FILE", "KAFKA_SASL_USERNAME", "KAFKA_SASL_PASSWORD",
		"KAFKA_SASL_MECHANISM", "KAFKA_CLIENT_CERT_FILE", "KAFKA_CLIENT_KEY_FILE",
	}
	for _, key := range keys {
		old, present := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		key, old, present := key, old, present
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	values := map[string]string{
		"DDAE_BASE_URL":        baseURL,
		"DDAE_SOURCE_INSTANCE": "test-appliance",
		"DDAE_USERNAME":        "monitor-user",
		"DDAE_PASSWORD":        "password-test-canary",
		"DDAE_CLIENT_SECRET":   "client-secret-test-canary",
		"DDAE_CA_FILE":         caFile,
		"DDAE_RETRY_MAX":       "0",
		"KAFKA_BROKERS":        "broker.example.test:9093",
		"KAFKA_TOPIC":          "ddae-test-alerts",
		"STATE_DIR":            filepath.Join(t.TempDir(), "state"),
	}
	for key, value := range extra {
		values[key] = value
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load test config: %v", err)
	}
	return cfg
}

func TestTokenFlowAndAllApprovedGETOperations(t *testing.T) {
	var tokenCalls atomic.Int32
	var mu sync.Mutex
	seen := make(map[string]int)
	total := int64(1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			tokenCalls.Add(1)
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("token request = %s %s", r.Method, r.Header.Get("Content-Type"))
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse token form: %v", err)
			}
			for key, want := range map[string]string{
				"grant_type": "password", "client_id": "dv-admin-rest",
				"username": "monitor-user", "password": "password-test-canary",
				"client_secret": "client-secret-test-canary",
			} {
				if got := r.Form.Get(key); got != want {
					t.Errorf("token field %s = %q", key, got)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"token-one","expires_in":3600}`)
			return
		}
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer token-one" {
			t.Errorf("product request = %s auth=%q", r.Method, r.Header.Get("Authorization"))
		}
		mu.Lock()
		seen[r.URL.Path]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case pingPath:
			fmt.Fprint(w, `{"status":"ok"}`)
		case clustersPath, nodesPath:
			fmt.Fprint(w, `[]`)
		case lockPath:
			fmt.Fprint(w, `{"status":"unlocked"}`)
		case powerPath:
			fmt.Fprint(w, `{"controlPlaneReady":true,"nodesReady":1,"totalNodes":1}`)
		case alertListPath:
			fmt.Fprint(w, `{"results":[{"id":"alert-1"}],"totalRecords":1}`)
		case alertDetailPath + "alert-1":
			fmt.Fprint(w, `{"id":"alert-1","type":"warning"}`)
		case serviceabilityLogListPath:
			fmt.Fprint(w, `{"results":[{"id":"log-1"}],"totalRecords":1}`)
		case serviceabilityLogDetailPath + "log-1":
			fmt.Fprint(w, `{"id":"log-1","type":"info"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.CloseIdleConnections()
	ctx := context.Background()
	if _, err := client.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Clusters(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Nodes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Lock(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Power(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AlertList(ctx); err != nil {
		t.Fatal(err)
	}
	if detail, err := client.AlertDetail(ctx, "alert-1"); err != nil || detail.ID != "alert-1" {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
	if _, err := client.ServiceabilityLogList(ctx); err != nil {
		t.Fatal(err)
	}
	if detail, err := client.ServiceabilityLogDetail(ctx, "log-1"); err != nil || detail.ID != "log-1" {
		t.Fatalf("serviceability detail=%#v err=%v", detail, err)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("token calls = %d", tokenCalls.Load())
	}
	for _, operation := range ApprovedOperations() {
		path := operation.Path
		if operation.Collector == "alert_detail" {
			path = alertDetailPath + "alert-1"
		}
		if operation.Collector == "serviceability_log_detail" {
			path = serviceabilityLogDetailPath + "log-1"
		}
		if operation.Method != http.MethodGet || seen[path] != 1 {
			t.Errorf("operation %#v seen=%d", operation, seen[path])
		}
	}
	copy := ApprovedOperations()
	copy[0].Path = "/mutated"
	if ApprovedOperations()[0].Path == copy[0].Path {
		t.Fatal("caller mutated compiled operation allowlist")
	}
	_ = total
}

func TestConcurrentRequestsCoalesceTokenRenewal(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			calls.Add(1)
			time.Sleep(20 * time.Millisecond)
			fmt.Fprint(w, `{"access_token":"shared-token","expires_in":3600}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer shared-token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()
	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := client.Ping(context.Background()); err != nil {
				t.Errorf("Ping: %v", err)
			}
		}()
	}
	group.Wait()
	if calls.Load() != 1 {
		t.Fatalf("token calls = %d", calls.Load())
	}
}

func TestTLSConfigRejectsInvalidCA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := tlsConfig(path); err == nil {
		t.Fatal("invalid custom CA was accepted")
	}
	if roots, err := x509.SystemCertPool(); err == nil && roots == nil {
		t.Fatal("unexpected nil system pool")
	}
}
