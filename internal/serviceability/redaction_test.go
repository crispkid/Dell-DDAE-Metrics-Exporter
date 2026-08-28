package serviceability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/metrics"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type errorAPI struct{ canary string }

func (a errorAPI) ServiceabilityLogList(context.Context) (ddae.ServiceabilityLogList, error) {
	return ddae.ServiceabilityLogList{}, errors.New(a.canary)
}
func (a errorAPI) ServiceabilityLogDetail(context.Context, string) (ddae.ServiceabilityLogDetail, error) {
	return ddae.ServiceabilityLogDetail{}, errors.New(a.canary)
}

type captureLogger struct{ text string }

func (l *captureLogger) Error(message string, args ...any) { l.text += message + fmt.Sprint(args...) }

func TestDiagnosticsNeverExposeServiceabilityContent(t *testing.T) {
	canary := "message-topic-endpoint-secret-canary"
	logger := &captureLogger{}
	state := &memoryState{}
	diagnostics := snapshot.NewStore()
	pipeline := NewPipeline(errorAPI{canary: canary}, state, diagnostics, Options{
		SourceInstance: "site-a", Interval: time.Minute, CycleTimeout: time.Second,
		RefreshInterval: time.Hour, MaxPerCycle: 1, Concurrency: 1,
	}, logger)
	pipeline.poll(context.Background())
	if strings.Contains(logger.text, canary) {
		t.Fatalf("log exposed canary: %s", logger.text)
	}
	registry, err := metrics.NewRegistry(diagnostics, time.Minute, metrics.BuildInfo{Version: "test", GoVersion: "go-test"}, metrics.PipelineMode{ServiceabilityLogsEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	text := testutil.CollectAndCount(registry)
	if text == 0 {
		t.Fatal("no metrics gathered")
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprint(families), canary) {
		t.Fatal("metrics exposed canary")
	}
}
