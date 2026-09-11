package snapshot

import (
	"testing"
	"time"
)

// TEST-DDAE-12-003: the cycle timestamp cannot renew an older family.
func TestAuditRequiredFamiliesHaveIndependentFreshness(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, name := range []string{"ping", "clusters", "nodes", "lock", "power"} {
		for _, kind := range []string{"boundary", "stale", "absent", "zero", "failure"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				s := NewStore()
				s.RecordPing(true, true, true, now, 0)
				s.RecordClusters(nil, true, true, now, 0)
				s.RecordNodes(nil, true, true, now, 0)
				s.RecordLock(false, true, true, now, 0)
				s.RecordPower(Power{}, true, true, now, 0)
				s.CompleteRequiredCycle(now, true)
				s.SetAlertPipelineReady(true)
				var at *time.Time
				var present *bool
				switch name {
				case "ping":
					at, present = &s.view.Ping.CollectedAt, &s.view.Ping.Present
				case "clusters":
					at, present = &s.view.Clusters.CollectedAt, &s.view.Clusters.Present
				case "nodes":
					at, present = &s.view.Nodes.CollectedAt, &s.view.Nodes.Present
				case "lock":
					at, present = &s.view.Lock.CollectedAt, &s.view.Lock.Present
				case "power":
					at, present = &s.view.Power.CollectedAt, &s.view.Power.Present
				}
				*at = now.Add(-time.Minute)
				switch kind {
				case "stale":
					*at = at.Add(-time.Nanosecond)
				case "absent":
					*present = false
				case "zero":
					*at = time.Time{}
				case "failure":
					status := s.view.Collectors[name]
					status.Success = false
					s.view.Collectors[name] = status
				}
				want := kind == "boundary"
				if got := RequiredCurrent(s.Load(), now, time.Minute); got != want {
					t.Errorf("RequiredCurrent=%v want %v", got, want)
				}
				if got := s.ReadyFor(now, time.Minute, true, false); got != want {
					t.Errorf("ReadyFor=%v want %v", got, want)
				}
				if !s.ReadyFor(now, time.Minute, false, true) {
					t.Fatal("disabled resources affected alert readiness")
				}
			})
		}
	}
}
