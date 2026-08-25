package snapshot

import (
	"testing"
	"time"
)

func TestRequiredCurrentAndReady(t *testing.T) {
	store := NewStore()
	now := time.Unix(1000, 0)
	store.RecordPing(true, true, true, now, time.Millisecond)
	store.RecordClusters(nil, true, true, now, time.Millisecond)
	store.RecordNodes(nil, true, true, now, time.Millisecond)
	store.RecordLock(false, true, true, now, time.Millisecond)
	store.RecordPower(Power{}, true, true, now, time.Millisecond)
	store.CompleteRequiredCycle(now, true)
	if !RequiredCurrent(store.Load(), now.Add(time.Minute), 2*time.Minute) {
		t.Fatal("current complete snapshot reported stale")
	}
	if RequiredCurrent(store.Load(), now.Add(3*time.Minute), 2*time.Minute) {
		t.Fatal("stale snapshot reported current")
	}
	if store.Ready(now.Add(time.Minute), 2*time.Minute) {
		t.Fatal("alert pipeline has not become ready")
	}
	store.SetAlertPipelineReady(true)
	if !store.Ready(now.Add(time.Minute), 2*time.Minute) {
		t.Fatal("ready state was not aggregated")
	}
}
