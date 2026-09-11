package alerts

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

// TEST-DDAE-12-002: failed attempts leave lastFetched unchanged but must rotate.
func TestAuditFailedDetailsDoNotStarveWaitingIDs(t *testing.T) {
	for _, quota := range []int{1, 4, 200} {
		t.Run(fmt.Sprint(quota), func(t *testing.T) {
			p := &Pipeline{maxPerCycle: quota}
			tasks := []detailTask{{id: "z-healthy", priority: 0}, {id: "z-refresh", priority: 1}}
			for i := 0; i < quota; i++ {
				tasks = append(tasks, detailTask{id: fmt.Sprintf("a-failing-%03d", i)}, detailTask{id: fmt.Sprintf("a-refresh-%03d", i), priority: 1})
			}
			seen := map[string]bool{}
			cycles := 2*len(tasks) + 2
			for cycle := 0; cycle < cycles; cycle++ {
				selected := p.selectFair(tasks)
				if len(selected) > quota {
					t.Fatal("quota exceeded")
				}
				for _, task := range selected {
					seen[task.id] = true
				}
				// Continuous newcomers cannot jump ahead of already waiting IDs.
				tasks = append(tasks[:len(tasks):len(tasks)], detailTask{id: fmt.Sprintf("aaa-new-%04d", cycle)})
			}
			if !seen["z-healthy"] || !seen["z-refresh"] {
				t.Fatalf("healthy IDs starved: %v %v", seen["z-healthy"], seen["z-refresh"])
			}
		})
	}
}

func TestAuditPollingProgressDespitePermanentDetailFailure(t *testing.T) {
	total := int64(2)
	api := &fakeAlertAPI{list: ddae.AlertList{Results: []ddae.AlertListItem{{ID: "a-failing"}, {ID: "z-healthy"}}, TotalRecords: &total}, details: map[string]ddae.AlertDetail{"z-healthy": {ID: "z-healthy"}}, detailErr: map[string]error{"a-failing": errors.New("synthetic failure")}}
	state := &memoryState{checkpoints: make(map[string]struct {
		marker string
		at     time.Time
	})}
	p := NewPipeline(api, state, snapshot.NewStore(), Options{SourceInstance: "test", CycleTimeout: time.Second, RefreshInterval: time.Hour, MaxPerCycle: 1, Concurrency: 1}, nil)
	for range 10 {
		p.poll(context.Background())
	}
	if len(state.events) != 1 || state.events[0].Event.AlertID != "z-healthy" {
		t.Fatal("healthy event did not progress")
	}
	if _, _, at, err := state.FetchState("a-failing"); err != nil || !at.IsZero() {
		t.Fatal("failure recorded as successful fetch")
	}
	if len(p.waitingOrder) > 2 {
		t.Fatal("unbounded scheduling state")
	}
}

func TestAuditFairSelectionRemovalAndReentry(t *testing.T) {
	p := &Pipeline{maxPerCycle: 1}
	tasks := []detailTask{{id: "a"}, {id: "b"}, {id: "c", lastFetched: time.Unix(1, 0)}}
	if p.selectFair(tasks)[0].id != "a" {
		t.Fatal("initial ordering")
	}
	if p.selectFair(tasks)[0].id != "b" {
		t.Fatal("failed a did not rotate")
	}
	p.selectFair(nil)
	if len(p.waitingOrder) != 0 {
		t.Fatal("retained ineligible IDs")
	}
	if got := p.selectFair([]detailTask{{id: "new"}, {id: "a"}})[0].id; got != "a" {
		t.Fatalf("reseed: %s", got)
	}
}
