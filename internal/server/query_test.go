package server

import (
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQueryReadinessAndLegacyPartialFailure(t *testing.T) {
	ready := false
	s := New("127.0.0.1:0", prometheus.NewRegistry(), snapshot.NewStore(), time.Minute, PipelineMode{QueryReady: func() bool { return ready }})
	check := func(server *Server, path string, want int) {
		t.Helper()
		w := httptest.NewRecorder()
		server.http.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Fatal(path, w.Code, want)
		}
	}
	check(s, "/readyz", 503)
	check(s, "/healthz", 200)
	ready = true
	check(s, "/readyz", 200)
	mixed := New("127.0.0.1:0", prometheus.NewRegistry(), snapshot.NewStore(), time.Minute, PipelineMode{ResourcesEnabled: true, QueryReady: func() bool { return true }})
	check(mixed, "/readyz", 503)
}
