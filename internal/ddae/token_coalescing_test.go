package ddae

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

func TestFailedTokenRefreshResultIsSharedByConcurrentWaiters(t *testing.T) {
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()

	const waiters = 32
	start := make(chan struct{})
	errs := make([]error, waiters)
	var group sync.WaitGroup
	for i := range waiters {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			_, errs[index] = client.Ping(context.Background())
		}(i)
	}
	close(start)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("token request did not start")
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	group.Wait()
	if calls.Load() != 1 {
		t.Fatalf("token calls=%d", calls.Load())
	}
	for i, err := range errs {
		if err == nil || observability.Classify(err) != observability.ClassAuth || err != errs[0] {
			t.Fatalf("waiter %d did not receive shared auth error: %v", i, err)
		}
	}
}

func TestRejectedOldGenerationReusesNewerUsableToken(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)
		fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":3600}`, call)
	}))
	defer server.Close()
	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.tokens.get(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.tokens.get(context.Background(), first.generation)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := client.tokens.get(context.Background(), first.generation)
	if err != nil {
		t.Fatal(err)
	}
	if first.token != "token-1" || second.token != "token-2" || reused != second || calls.Load() != 2 {
		t.Fatalf("first=%#v second=%#v reused=%#v calls=%d", first, second, reused, calls.Load())
	}
}

func TestConcurrentUnauthorizedRequestsShareOneRenewal(t *testing.T) {
	var tokenCalls atomic.Int32
	var oldTokenGets atomic.Int32
	allOldRequests := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			call := tokenCalls.Add(1)
			fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":3600}`, call)
			return
		}
		if r.Header.Get("Authorization") == "Bearer token-1" {
			if oldTokenGets.Add(1) == 32 {
				close(allOldRequests)
			}
			<-allOldRequests
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
	if _, err := client.tokens.get(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := client.Ping(context.Background()); err != nil {
				t.Errorf("Ping: %v", err)
			}
		}()
	}
	group.Wait()
	if tokenCalls.Load() != 2 || oldTokenGets.Load() != 32 {
		t.Fatalf("token calls=%d old-token GETs=%d", tokenCalls.Load(), oldTokenGets.Load())
	}
}

func TestTokenTransientStatusesRetryButCredentialRejectionDoesNot(t *testing.T) {
	for _, test := range []struct {
		name      string
		first     int
		retryMax  string
		wantCalls int32
		wantErr   bool
	}{
		{name: "transient", first: http.StatusServiceUnavailable, retryMax: "1", wantCalls: 2},
		{name: "credentials", first: http.StatusForbidden, retryMax: "2", wantCalls: 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				call := calls.Add(1)
				if call == 1 {
					w.WriteHeader(test.first)
					return
				}
				fmt.Fprint(w, `{"access_token":"token","expires_in":3600}`)
			}))
			defer server.Close()
			client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), map[string]string{"DDAE_RETRY_MAX": test.retryMax}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.tokens.get(context.Background(), 0)
			if (err != nil) != test.wantErr || calls.Load() != test.wantCalls {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
		})
	}
}
