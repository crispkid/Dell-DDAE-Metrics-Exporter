package ddae

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

func TestUnauthorizedRequestRenewsOnceThenFailsAuth(t *testing.T) {
	var tokenCalls atomic.Int32
	var getCalls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			call := tokenCalls.Add(1)
			fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":3600}`, call)
			return
		}
		getCalls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "credential-body-canary")
	}))
	defer server.Close()
	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Ping(context.Background())
	if err == nil || observability.Classify(err) != observability.ClassAuth {
		t.Fatalf("error = %v class=%s", err, observability.Classify(err))
	}
	if strings.Contains(err.Error(), "canary") {
		t.Fatalf("error exposed response body: %v", err)
	}
	if tokenCalls.Load() != 2 || getCalls.Load() != 2 {
		t.Fatalf("token=%d GET=%d", tokenCalls.Load(), getCalls.Load())
	}
}

func TestFirstUnauthorizedRenewsAndSucceeds(t *testing.T) {
	var tokenCalls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			call := tokenCalls.Add(1)
			fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":3600}`, call)
			return
		}
		if r.Header.Get("Authorization") == "Bearer token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()
	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping after renewal: %v", err)
	}
	if tokenCalls.Load() != 2 {
		t.Fatalf("token calls = %d", tokenCalls.Load())
	}
}

func TestRejectedTokenResponseIsBoundedAuthFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "password-test-canary client-secret-test-canary")
	}))
	defer server.Close()
	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Ping(context.Background())
	if err == nil || observability.Classify(err) != observability.ClassAuth {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "canary") {
		t.Fatal("token error included server body or secret")
	}
}
