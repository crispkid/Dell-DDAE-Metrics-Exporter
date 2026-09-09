package historyscan

import (
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"github.com/prometheus/client_golang/prometheus"
	"testing"
	"time"
)

// TEST-DDAE-10-007: constant labels, cache-only scrape, omitted initial completion, stale health.
func TestBackfillMetrics(t *testing.T) {
	st, e := historystate.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	registry := prometheus.NewRegistry()
	for _, key := range []string{"queries", "serviceability_logs"} {
		w, _ := New(st, key, "test", testConfig(), 2*time.Hour, callbackSource{})
		if e = registry.Register(w); e != nil {
			t.Fatal(e)
		}
		w.success = true
		w.at = time.Now().Add(-4 * w.cfg.Interval)
		if w.Ready() {
			t.Fatal("stale ready")
		}
	}
	families, e := registry.Gather()
	if e != nil {
		t.Fatal(e)
	}
	series := 0
	for _, f := range families {
		if f.GetName() == "ddae_history_backfill_last_completed_timestamp_seconds" {
			t.Fatal("invented completion")
		}
		for _, m := range f.Metric {
			series++
			if len(m.Label) != 1 || m.Label[0].GetName() != "pipeline" {
				t.Fatal("unbounded labels")
			}
		}
	}
	if series != 12 {
		t.Fatal(series)
	}
}
