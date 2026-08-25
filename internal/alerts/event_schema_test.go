package alerts

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func pointer[T any](value T) *T { return &value }

func TestBuildEventExactAllowlistAndStableIdentity(t *testing.T) {
	detail := ddae.AlertDetail{
		ID: "alert-1", Type: pointer("WARNING"), Acknowledged: pointer("false"),
		Count: pointer[int64](2), CreatedOn: pointer("2026-08-24T10:00:00+08:00"),
		Message: pointer("confidential detail"), Remedies: []string{"inspect node"},
	}
	observed := time.Date(2026, 8, 24, 3, 0, 0, 0, time.UTC)
	first, err := BuildEvent("site-a", "alert-1", detail, observed)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	second, err := BuildEvent("site-a", "alert-1", detail, observed.Add(time.Minute))
	if err != nil {
		t.Fatalf("build second: %v", err)
	}
	if !bytes.Equal(first.RecordKey, second.RecordKey) || first.ContentHash != second.ContentHash {
		t.Fatal("stable identity/hash changed")
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(first.Payload, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"schema_version", "event_type", "source_system", "source_instance", "alert_id", "content_hash_sha256", "observed_at", "alert"} {
		if _, ok := decoded[required]; !ok {
			t.Fatalf("missing field %s", required)
		}
	}
	if strings.Contains(string(first.Payload), "labels") || strings.Contains(string(first.Payload), "links") {
		t.Fatal("payload contains excluded field")
	}
}

func TestBuildEventRejectsMismatchAndInvalidFields(t *testing.T) {
	if _, err := BuildEvent("site-a", "alert-1", ddae.AlertDetail{ID: "other"}, time.Now()); err == nil {
		t.Fatal("expected ID mismatch")
	}
	invalidBool := "sometimes"
	if _, err := BuildEvent("site-a", "alert-1", ddae.AlertDetail{ID: "alert-1", Acknowledged: &invalidBool}, time.Now()); err == nil {
		t.Fatal("expected boolean validation error")
	}
	large := strings.Repeat("x", 8193)
	if _, err := BuildEvent("site-a", "alert-1", ddae.AlertDetail{ID: "alert-1", Message: &large}, time.Now()); err == nil {
		t.Fatal("expected message size error")
	}
}
