package serviceability

import (
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestServiceabilityAllowlistContainsExactlyDocumentedGETRoutes(t *testing.T) {
	want := map[string]string{
		"serviceability_log_list":   "/rest/v1/serviceability-events",
		"serviceability_log_detail": "/rest/v1/serviceability-events/{id}",
	}
	seen := make(map[string]string)
	for _, operation := range ddae.ApprovedOperations() {
		if path, relevant := want[operation.Collector]; relevant {
			if operation.Method != "GET" || operation.Path != path {
				t.Fatalf("operation = %#v", operation)
			}
			seen[operation.Collector] = operation.Path
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("serviceability operations = %v", seen)
	}
	for _, id := range []string{"log-1", "日誌/one segment", "a:b_c.d"} {
		if err := ddae.ValidateServiceabilityLogID(id); err != nil {
			t.Fatalf("safe ID %q: %v", id, err)
		}
	}
	for _, id := range []string{"", ".", "..", "nul\x00id", string(make([]byte, 257))} {
		if err := ddae.ValidateServiceabilityLogID(id); err == nil {
			t.Fatalf("unsafe ID %q accepted", id)
		}
	}
}
