package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	http                      *http.Server
	state                     *snapshot.Store
	staleAfter                time.Duration
	resourcesEnabled          bool
	alertsEnabled             bool
	serviceabilityLogsEnabled bool
}

type PipelineMode struct {
	ResourcesEnabled          bool
	AlertsEnabled             bool
	ServiceabilityLogsEnabled bool
}

func New(address string, registry prometheus.Gatherer, state *snapshot.Store, staleAfter time.Duration, modes ...PipelineMode) *Server {
	mux := http.NewServeMux()
	mode := PipelineMode{ResourcesEnabled: true, AlertsEnabled: true}
	if len(modes) > 0 {
		mode = modes[0]
	}
	server := &Server{
		state: state, staleAfter: staleAfter,
		resourcesEnabled: mode.ResourcesEnabled, alertsEnabled: mode.AlertsEnabled,
		serviceabilityLogsEnabled: mode.ServiceabilityLogsEnabled,
	}
	mux.Handle("/metrics", getOnly(promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		MaxRequestsInFlight: 5,
		Timeout:             9 * time.Second,
	})))
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("alive\n"))
	})
	mux.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !state.ReadyFor(time.Now(), staleAfter, server.resourcesEnabled, server.alertsEnabled, server.serviceabilityLogsEnabled) {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte("not ready\n"))
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ready\n"))
	})
	server.http = &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return server
}

func (s *Server) ListenAndServe() error {
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

func getOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
