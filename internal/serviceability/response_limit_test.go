package serviceability

import (
	"strings"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestEventFieldAndPayloadBoundsFailWithoutTruncation(t *testing.T) {
	boundary := strings.Repeat("m", 8192)
	encoded, err := BuildEvent("site-a", "log-1", ddae.ServiceabilityLogDetail{ID: "log-1", Message: &boundary}, time.Now())
	if err != nil || encoded.Event.Log.Message == nil || len(*encoded.Event.Log.Message) != len(boundary) {
		t.Fatalf("boundary event=%#v err=%v", encoded, err)
	}
	over := boundary + "x"
	if _, err := BuildEvent("site-a", "log-1", ddae.ServiceabilityLogDetail{ID: "log-1", Message: &over}, time.Now()); err == nil {
		t.Fatal("oversized message was truncated or accepted")
	}
	remedies := make([]string, 33)
	if _, err := BuildEvent("site-a", "log-1", ddae.ServiceabilityLogDetail{ID: "log-1", Remedies: remedies}, time.Now()); err == nil {
		t.Fatal("oversized remedies were accepted")
	}
}
