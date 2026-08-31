package ddae

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestPathFailureDoesNotProbeAlternateRouteOrExposePrefix(t *testing.T) {
	const prefix = "/prefix-canary"
	var mu sync.Mutex
	var productPaths []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == tokenPath {
			fmt.Fprint(writer, `{"access_token":"token","expires_in":3600}`)
			return
		}
		mu.Lock()
		productPaths = append(productPaths, request.URL.Path)
		mu.Unlock()
		http.NotFound(writer, request)
	}))
	defer server.Close()

	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), map[string]string{
		"DDAE_PING_PATH_PREFIX": prefix,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	_, err = client.Ping(context.Background())
	if err == nil {
		t.Fatal("404 route unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), prefix) {
		t.Fatalf("runtime error exposed configured prefix: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(productPaths) != 1 || productPaths[0] != prefix+"/ping" {
		t.Fatalf("product paths = %v", productPaths)
	}
}

func TestNewClientRejectsInvalidPrefixWithoutEchoingIt(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	cfg := clientConfig(t, server.URL, trustedServerCA(t, server), nil)
	cfg.DDAEAPIPathPrefix = "/invalid?prefix-canary"
	_, err := NewClient(cfg)
	if err == nil {
		t.Fatal("invalid direct configuration was accepted")
	}
	if strings.Contains(err.Error(), "prefix-canary") {
		t.Fatalf("client error exposed configured prefix: %v", err)
	}
}
