//go:build e2e

package integration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDeploymentRunbook(t *testing.T) {
	if os.Getenv("DDAE_E2E_ENABLED") != "1" {
		t.Fatal("authorized deployment environment is not enabled")
	}
	client := authorizedE2EClient(t)
	type deploymentCase struct {
		variable         string
		resourcesEnabled bool
		alertsEnabled    bool
	}
	for _, profile := range []string{"CONTAINER", "KUBERNETES", "SYSTEMD"} {
		for _, mode := range []deploymentCase{
			{variable: "RESOURCE", resourcesEnabled: true},
			{variable: "ALERT", alertsEnabled: true},
			{variable: "DUAL", resourcesEnabled: true, alertsEnabled: true},
		} {
			mode.variable = "DDAE_E2E_" + profile + "_" + mode.variable + "_URL"
			t.Run(profile+"/"+mode.variable, func(t *testing.T) {
				verifyDeployment(t, client, mode)
			})
		}
	}
}

func verifyDeployment(t *testing.T, client *http.Client, deployment struct {
	variable         string
	resourcesEnabled bool
	alertsEnabled    bool
}) {
	t.Helper()
	origin := os.Getenv(deployment.variable)
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		t.Fatalf("%s must be an authorized HTTPS origin", deployment.variable)
	}
	for path, marker := range map[string]string{"/healthz": "alive", "/readyz": "ready", "/metrics": "ddae_build_info"} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(origin, "/")+path, nil)
		if err != nil {
			cancel()
			t.Fatalf("construct %s request", path)
		}
		response, err := client.Do(request)
		if err != nil {
			cancel()
			t.Fatalf("%s request failed without logging its private origin", path)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
		_ = response.Body.Close()
		cancel()
		if readErr != nil || len(body) > 2*1024*1024 || response.StatusCode != http.StatusOK || !strings.Contains(string(body), marker) {
			t.Fatalf("%s response failed bounded status/content checks", path)
		}
		if path == "/metrics" {
			metrics := string(body)
			assertEnableMetric(t, metrics, "resources", deployment.resourcesEnabled)
			assertEnableMetric(t, metrics, "alerts", deployment.alertsEnabled)
			if strings.Contains(metrics, "\nddae_up ") != deployment.resourcesEnabled {
				t.Fatal("resource metric presence does not match configured mode")
			}
			if strings.Contains(metrics, "\nddae_alert_pipeline_ready ") != deployment.alertsEnabled {
				t.Fatal("alert metric presence does not match configured mode")
			}
		}
	}
}

func assertEnableMetric(t *testing.T, metrics, pipeline string, enabled bool) {
	t.Helper()
	value := "0"
	if enabled {
		value = "1"
	}
	needle := `ddae_monitoring_enabled{pipeline="` + pipeline + `"} ` + value
	if !strings.Contains(metrics, needle) {
		t.Fatalf("metrics lack %s", needle)
	}
}

func authorizedE2EClient(t *testing.T) *http.Client {
	t.Helper()
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if path := os.Getenv("DDAE_E2E_CA_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil || !roots.AppendCertsFromPEM(data) {
			t.Fatal("DDAE_E2E_CA_FILE is invalid")
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	certFile, keyFile := os.Getenv("DDAE_E2E_CLIENT_CERT_FILE"), os.Getenv("DDAE_E2E_CLIENT_KEY_FILE")
	if (certFile == "") != (keyFile == "") {
		t.Fatal("E2E mTLS certificate and key must be configured together")
	}
	if certFile != "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			t.Fatal("E2E mTLS certificate pair is invalid")
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return &http.Client{
		Transport:     &http.Transport{Proxy: nil, TLSClientConfig: tlsConfig},
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects disabled") },
	}
}
