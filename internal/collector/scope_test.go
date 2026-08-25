package collector

import (
	"reflect"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestCompiledCollectionScopeIsExactAndImmutable(t *testing.T) {
	wantCollectors := []string{"ping", "clusters", "nodes", "lock", "power"}
	if got := RequiredCollectors(); !reflect.DeepEqual(got, wantCollectors) {
		t.Fatalf("required collectors = %v", got)
	}
	wantOperations := map[string]bool{
		"ping": true, "clusters": true, "nodes": true, "lock": true,
		"power": true, "alert_list": true, "alert_detail": true,
	}
	operations := ddae.ApprovedOperations()
	if len(operations) != len(wantOperations) {
		t.Fatalf("operation count = %d", len(operations))
	}
	for _, operation := range operations {
		if operation.Method != "GET" || !wantOperations[operation.Collector] {
			t.Fatalf("out-of-scope operation: %#v", operation)
		}
	}
	copy := RequiredCollectors()
	copy[0] = "sql"
	if RequiredCollectors()[0] == "sql" {
		t.Fatal("caller changed compiled collector scope")
	}
}
