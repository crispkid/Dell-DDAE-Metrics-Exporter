package server

import (
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus"
	"net/http/httptest"
	"testing"
	"time"
)

// TEST-DDAE-10-007: recovery readiness is independent of process liveness.
func TestBackfillReadiness(t *testing.T) {
	ready := false
	s := New("127.0.0.1:0", prometheus.NewRegistry(), snapshot.NewStore(), time.Minute, PipelineMode{QueryReady: func() bool { return true }, HistoryReady: func() bool { return ready }})
	for _, tc := range []struct {
		ready  bool
		path   string
		status int
	}{{false, "/readyz", 503}, {false, "/healthz", 200}, {true, "/readyz", 200}, {true, "/metrics", 200}} {
		ready = tc.ready
		w := httptest.NewRecorder()
		s.http.Handler.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status {
			t.Fatal(tc, w.Code)
		}
	}
}
