package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/metrics"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

func TestHealthAndReadinessSemantics(t *testing.T) {
	state := snapshot.NewStore()
	registry, err := metrics.NewRegistry(state, time.Minute, metrics.BuildInfo{Version: "test", GoVersion: "go-test"})
	if err != nil {
		t.Fatal(err)
	}
	server := New("127.0.0.1:0", registry, state, time.Minute)
	for path, status := range map[string]int{"/healthz": http.StatusOK, "/readyz": http.StatusServiceUnavailable, "/unknown": http.StatusNotFound} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		server.http.Handler.ServeHTTP(response, request)
		if response.Code != status {
			t.Fatalf("%s status=%d want=%d", path, response.Code, status)
		}
	}
}

func TestMetricsRejectsNonGET(t *testing.T) {
	state := snapshot.NewStore()
	registry, _ := metrics.NewRegistry(state, time.Minute, metrics.BuildInfo{})
	server := New("127.0.0.1:0", registry, state, time.Minute)
	request := httptest.NewRequest(http.MethodPost, "/metrics", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
}
