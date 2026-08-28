package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	dto "github.com/prometheus/client_model/go"
)

type blockingGatherer struct {
	entered chan struct{}
	release chan struct{}
	active  atomic.Int32
	maximum atomic.Int32
}

func (g *blockingGatherer) Gather() ([]*dto.MetricFamily, error) {
	active := g.active.Add(1)
	defer g.active.Add(-1)
	for {
		maximum := g.maximum.Load()
		if active <= maximum || g.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	g.entered <- struct{}{}
	<-g.release
	return nil, nil
}

func TestMetricsHasFiveRequestBoundAndDoesNotBlockProbes(t *testing.T) {
	gatherer := &blockingGatherer{entered: make(chan struct{}, 8), release: make(chan struct{})}
	state := snapshot.NewStore()
	state.SetAlertPipelineReady(true)
	applicationServer := New("127.0.0.1:0", gatherer, state, time.Minute, PipelineMode{AlertsEnabled: true})

	var group sync.WaitGroup
	for range 5 {
		group.Add(1)
		go func() {
			defer group.Done()
			response := httptest.NewRecorder()
			applicationServer.http.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		}()
	}
	for range 5 {
		select {
		case <-gatherer.entered:
		case <-time.After(time.Second):
			t.Fatal("five metric gathers did not start")
		}
	}

	excess := httptest.NewRecorder()
	applicationServer.http.Handler.ServeHTTP(excess, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if excess.Code != http.StatusServiceUnavailable {
		t.Fatalf("excess metrics status=%d", excess.Code)
	}
	if gatherer.maximum.Load() != 5 {
		t.Fatalf("maximum concurrent gathers=%d", gatherer.maximum.Load())
	}
	for _, path := range []string{"/healthz", "/readyz"} {
		response := httptest.NewRecorder()
		applicationServer.http.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s blocked by metrics overload: %d", path, response.Code)
		}
	}
	close(gatherer.release)
	group.Wait()
}

func TestMetricsHandlerTimesOutAtNineSeconds(t *testing.T) {
	gatherer := &blockingGatherer{entered: make(chan struct{}, 1), release: make(chan struct{})}
	state := snapshot.NewStore()
	state.SetAlertPipelineReady(true)
	applicationServer := New("127.0.0.1:0", gatherer, state, time.Minute, PipelineMode{AlertsEnabled: true})
	response := httptest.NewRecorder()
	done := make(chan struct{})
	started := time.Now()
	go func() {
		applicationServer.http.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		close(done)
	}()
	select {
	case <-gatherer.entered:
	case <-time.After(time.Second):
		t.Fatal("metric gather did not start")
	}
	health := httptest.NewRecorder()
	applicationServer.http.Handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status during slow scrape=%d", health.Code)
	}
	select {
	case <-done:
		elapsed := time.Since(started)
		if elapsed < 8*time.Second || elapsed > 11*time.Second || response.Code != http.StatusServiceUnavailable {
			t.Fatalf("timeout elapsed=%v status=%d", elapsed, response.Code)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("metrics handler exceeded timeout tolerance")
	}
	close(gatherer.release)
	deadline := time.After(time.Second)
	for gatherer.active.Load() != 0 {
		select {
		case <-deadline:
			t.Fatal("timed-out gatherer goroutine did not return")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
