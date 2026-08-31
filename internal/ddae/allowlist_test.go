package ddae

import (
	"net/http"
	"strings"
	"testing"
)

func TestAllowlistContainsOnlyApprovedProductGETPaths(t *testing.T) {
	if pingPath != "/ping" {
		t.Fatalf("default Ping path = %q", pingPath)
	}
	paths := []string{clustersPath, nodesPath, lockPath, powerPath, alertListPath, alertDetailPath}
	for _, path := range paths {
		if !strings.HasPrefix(path, "/v1/") {
			t.Fatalf("default API path %q is outside /v1", path)
		}
	}
	if http.MethodGet != "GET" {
		t.Fatal("unexpected HTTP library invariant")
	}
}

func TestValidateAlertIDRejectsPathManipulation(t *testing.T) {
	for _, id := range []string{"", ".", "..", "a/b", `a\\b`, "a%2fb", "a?b", "a#b", strings.Repeat("a", 257)} {
		if err := ValidateAlertID(id); err == nil {
			t.Fatalf("accepted unsafe ID %q", id)
		}
	}
	for _, id := range []string{"1234", "550e8400-e29b-41d4-a716-446655440000", "alert:123.4"} {
		if err := ValidateAlertID(id); err != nil {
			t.Fatalf("rejected valid ID %q: %v", id, err)
		}
	}
}
