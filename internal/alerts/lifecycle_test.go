package alerts

import (
	"bytes"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestEventLifecycleIdentityUpdateAndNoSyntheticClear(t *testing.T) {
	now := time.Unix(1000, 0)
	firstMessage := "first"
	secondMessage := "second"
	first, err := BuildEvent("site-a", "alert-1", ddae.AlertDetail{ID: "alert-1", Message: &firstMessage}, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, _ := BuildEvent("site-a", "alert-1", ddae.AlertDetail{ID: "alert-1", Message: &firstMessage}, now.Add(time.Minute))
	updated, _ := BuildEvent("site-a", "alert-1", ddae.AlertDetail{ID: "alert-1", Message: &secondMessage}, now.Add(2*time.Minute))
	if !bytes.Equal(first.RecordKey, replay.RecordKey) || first.ContentHash != replay.ContentHash {
		t.Fatal("unchanged alert did not retain key and content hash")
	}
	if !bytes.Equal(first.RecordKey, updated.RecordKey) || first.ContentHash == updated.ContentHash {
		t.Fatal("updated alert key/hash semantics are incorrect")
	}
	if first.Event.EventType != EventType || EventType != "ddae.serviceability_alert.upsert" {
		t.Fatalf("event type = %q", first.Event.EventType)
	}
}

func TestRelatedEventAndFieldBounds(t *testing.T) {
	message := "nested"
	detail := ddae.AlertDetail{ID: "alert-1", Events: []ddae.RelatedAlertRaw{{Message: &message}}}
	event, err := BuildEvent("site-a", "alert-1", detail, time.Now())
	if err != nil || len(event.Event.Alert.RelatedEvents) != 1 {
		t.Fatalf("related event=%#v err=%v", event.Event.Alert.RelatedEvents, err)
	}
	invalidTimestamp := "yesterday"
	negative := int64(-1)
	for name, candidate := range map[string]ddae.AlertDetail{
		"timestamp": {ID: "alert-1", UpdatedOn: &invalidTimestamp},
		"count":     {ID: "alert-1", Count: &negative},
		"timeout":   {ID: "alert-1", AutoClearTimeout: &negative},
		"remedies":  {ID: "alert-1", Remedies: make([]string, 33)},
		"events":    {ID: "alert-1", Events: make([]ddae.RelatedAlertRaw, 101)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildEvent("site-a", "alert-1", candidate, time.Now()); err == nil {
				t.Fatal("invalid event field was accepted")
			}
		})
	}
	if _, err := BuildEvent("https://private.example.test", "alert-1", ddae.AlertDetail{ID: "alert-1"}, time.Now()); err == nil {
		t.Fatal("URL-like source identity was accepted")
	}
	classified := ValidationError{field: "test"}
	if classified.Error() == "" || classified.FailureClass() == "" {
		t.Fatal("validation error lacks bounded classification")
	}
}
