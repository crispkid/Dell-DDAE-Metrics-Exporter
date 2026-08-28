package serviceability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func stringPointer(value string) *string { return &value }
func int64Pointer(value int64) *int64    { return &value }

func TestTypedEventUsesExactAllowlistAndValidatesPresentValues(t *testing.T) {
	raw := []byte(`{"id":"log-1","type":"WARNING","acknowledged":"true","count":2,"createdon":"2026-08-28T01:00:00+08:00","updatedon":"2026-08-28T02:00:00+08:00","appname":"app","component":"component","namespace":"namespace","message":"message","reason":"reason","remedies":["remedy"],"resourceID":"resource","symptomid":"symptom","related":"related","labels":{"secret":"labels-canary"},"links":{"href":"links-canary"},"unknown":"unknown-canary"}`)
	var detail ddae.ServiceabilityLogDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	encoded, err := BuildEvent("site-a", "log-1", detail, time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"labels", "links", "unknown", "labels-canary", "links-canary", "unknown-canary"} {
		if bytes.Contains(encoded.Payload, []byte(forbidden)) {
			t.Fatalf("payload contains %q: %s", forbidden, encoded.Payload)
		}
	}
	if encoded.Event.Log.Severity != "warning" || encoded.Event.Log.Acknowledged == nil || !*encoded.Event.Log.Acknowledged ||
		encoded.Event.Log.CreatedAt == nil || *encoded.Event.Log.CreatedAt != "2026-08-27T17:00:00Z" {
		t.Fatalf("normalized log = %#v", encoded.Event.Log)
	}
	if _, err := DecodeStoredEvent(encoded.Payload); err != nil {
		t.Fatalf("stored event: %v", err)
	}

	invalid := []ddae.ServiceabilityLogDetail{
		{ID: "log-1", Acknowledged: stringPointer("yes")},
		{ID: "log-1", Count: int64Pointer(-1)},
		{ID: "log-1", CreatedOn: stringPointer("not-time")},
		{ID: "log-1", Message: stringPointer(strings.Repeat("x", 8193))},
		{ID: "different"},
	}
	for index, detail := range invalid {
		if _, err := BuildEvent("site-a", "log-1", detail, time.Now()); err == nil {
			t.Fatalf("invalid fixture %d accepted", index)
		}
	}
}

func TestStoredEventRejectsUnknownEnvelopeField(t *testing.T) {
	encoded, err := BuildEvent("site-a", "log-1", ddae.ServiceabilityLogDetail{ID: "log-1"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	payload := append([]byte(nil), encoded.Payload[:len(encoded.Payload)-1]...)
	payload = append(payload, []byte(`,"extra":"canary"}`)...)
	if _, err := DecodeStoredEvent(payload); err == nil {
		t.Fatal("unknown stored field was accepted")
	}
}
