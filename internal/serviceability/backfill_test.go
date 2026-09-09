package serviceability

import (
	"context"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"testing"
	"time"
)

// TEST-DDAE-10-007: valid truncated foreground stays usable without global completeness.
func TestBackfillTruncatedForegroundReadiness(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		threshold, total := int64(1), int64(2)
		state := &memoryState{}
		diag := snapshot.NewStore()
		p := NewPipeline(listAPI{list: ddae.ServiceabilityLogList{Results: []ddae.ServiceabilityLogListItem{{ID: "x"}}, Threshold: &threshold, TotalRecords: &total}}, state, diag, Options{BackfillEnabled: enabled, SourceInstance: "test", Interval: time.Minute, CycleTimeout: time.Second, RefreshInterval: time.Hour, MaxPerCycle: 1, Concurrency: 1}, nil)
		p.poll(context.Background())
		v := diag.Load()
		if v.ServiceabilityLogCollectionReady != enabled || v.ServiceabilityLogListComplete || state.reconciled {
			t.Fatal(enabled, v)
		}
	}
}
