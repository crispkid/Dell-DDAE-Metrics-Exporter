package ddae_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestTypedDecodeIgnoresUnknownAndSensitiveFields(t *testing.T) {
	const canary = "private-contact-canary@example.test"
	raw := `{
      "id":"alert-1","type":"WARNING","message":"approved operational text",
      "labels":{"credential":"secret-canary"},"links":["https://private.example.test"],
      "supportAssistContact":"` + canary + `","unknown":{"nested":true}
    }`
	var detail ddae.AlertDetail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		t.Fatal(err)
	}
	encoded, err := alerts.BuildEvent("site-a", "alert-1", detail, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	payload := string(encoded.Payload)
	for _, excluded := range []string{canary, "secret-canary", "private.example.test", "supportAssistContact", "labels", "links", "unknown"} {
		if strings.Contains(payload, excluded) {
			t.Fatalf("event contains excluded value %q: %s", excluded, payload)
		}
	}
}

func TestBoolStatusAcceptedRepresentationsAndUnknownRejection(t *testing.T) {
	for _, input := range []string{`true`, `false`, `"locked"`, `"UNLOCKED"`} {
		var status ddae.BoolStatus
		if err := json.Unmarshal([]byte(input), &status); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if _, valid := status.Value(); !valid {
			t.Fatalf("%s was not valid", input)
		}
	}
	for _, input := range []string{`"future"`, `1`, `{}`} {
		var status ddae.BoolStatus
		if err := json.Unmarshal([]byte(input), &status); err == nil {
			t.Fatalf("accepted invalid status %s", input)
		}
	}
}
