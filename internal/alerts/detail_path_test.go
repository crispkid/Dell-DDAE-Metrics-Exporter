package alerts

import (
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestDetailIDMustBeOneValidatedSegmentAndMatchResponse(t *testing.T) {
	for _, id := range []string{"", ".", "..", "a/b", `a\b`, "a%2Fb", "a?b", "a#b", strings.Repeat("x", 257)} {
		if err := ddae.ValidateAlertID(id); err == nil {
			t.Fatalf("accepted unsafe alert ID %q", id)
		}
	}
	if _, err := BuildEvent("site-a", "requested", ddae.AlertDetail{ID: "returned"}, time.Now()); err == nil {
		t.Fatal("mismatched detail response ID was accepted")
	}
}

func TestUsableMarkerNormalizesOnlyValidTimestamps(t *testing.T) {
	valid := "2026-08-24T11:00:00+08:00"
	if got := usableMarker(&valid); got != "2026-08-24T03:00:00Z" {
		t.Fatalf("marker = %q", got)
	}
	for _, value := range []*string{nil, pointer(""), pointer("not-a-time")} {
		if got := usableMarker(value); got != "" {
			t.Fatalf("invalid marker became %q", got)
		}
	}
}
