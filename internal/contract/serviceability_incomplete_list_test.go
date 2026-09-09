package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestObservedServiceabilityFixtureExpressesAnIncompleteList(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ddae-1.5.0", "serviceability-observed-structure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list ddae.ServiceabilityLogList
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}
	if list.TotalRecords == nil || *list.TotalRecords <= int64(len(list.Results)) {
		t.Fatalf("fixture does not preserve incomplete-list structure: %#v", list)
	}
}
