package metrics

import (
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

func TestAuditOverallHealthAgreesWithStalePingMetric(t *testing.T) {
	now := time.Now()
	s := snapshot.NewStore()
	s.RecordPing(true, true, true, now.Add(-40*time.Second), 0)
	s.RecordClusters(nil, true, true, now.Add(-21*time.Second), 0)
	s.RecordNodes(nil, true, true, now.Add(-21*time.Second), 0)
	s.RecordLock(false, true, true, now.Add(-21*time.Second), 0)
	s.RecordPower(snapshot.Power{}, true, true, now.Add(-21*time.Second), 0)
	s.CompleteRequiredCycle(now.Add(-21*time.Second), true)
	r, err := NewRegistry(s, 31*time.Second, BuildInfo{Version: "test", GoVersion: "test"}, PipelineMode{ResourcesEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	families, err := r.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range families {
		if f.GetName() == "ddae_up" || f.GetName() == "ddae_management_api_up" {
			found[f.GetName()] = true
			if f.Metric[0].Gauge.GetValue() != 0 {
				t.Error("stale metric healthy", f.GetName())
			}
		}
	}
	if len(found) != 2 || s.ReadyFor(now, 31*time.Second, true, false) {
		t.Fatal("stale health inconsistent")
	}
}
