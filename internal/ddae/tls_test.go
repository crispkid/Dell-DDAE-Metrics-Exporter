package ddae

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

func TestTLSFailsClosedAndCustomCAAddsTrust(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			fmt.Fprint(w, `{"access_token":"tls-token","expires_in":3600}`)
			return
		}
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()

	untrusted, err := NewClient(clientConfig(t, server.URL, "", nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := untrusted.Ping(context.Background()); err == nil || observability.Classify(err) != observability.ClassTLS {
		t.Fatalf("untrusted error=%v class=%s", err, observability.Classify(err))
	}

	trusted, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := trusted.Ping(context.Background()); err != nil {
		t.Fatalf("trusted request: %v", err)
	}
}

func TestTLSHostnameMismatchFailsClosed(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"access_token":"tls-token","expires_in":3600}`)
	}))
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Host = "localhost:" + strings.Split(parsed.Host, ":")[1]
	client, err := NewClient(clientConfig(t, parsed.String(), trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Ping(context.Background()); err == nil || observability.Classify(err) != observability.ClassTLS {
		t.Fatalf("hostname mismatch error=%v class=%s", err, observability.Classify(err))
	}
}
