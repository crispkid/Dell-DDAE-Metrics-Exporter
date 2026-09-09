package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestServiceabilityListsRemainIndexesForCompiledDetailOperations(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ddae-1.5.0", "serviceability-observed-structure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var logs ddae.ServiceabilityLogList
	if err := json.Unmarshal(data, &logs); err != nil {
		t.Fatal(err)
	}
	var alertsList ddae.AlertList
	if err := json.Unmarshal(data, &alertsList); err != nil {
		t.Fatal(err)
	}
	if logs.Malformed || len(logs.Results) != 1 || logs.Results[0].ID != "log-synthetic-1" || logs.TotalRecords == nil || *logs.TotalRecords != 2 ||
		len(alertsList.Results) != 1 || alertsList.Results[0].ID != "log-synthetic-1" {
		t.Fatalf("decoded lists: logs=%#v alerts=%#v", logs, alertsList)
	}
	encodedLogs, err := json.Marshal(logs)
	if err != nil {
		t.Fatal(err)
	}
	encodedAlerts, err := json.Marshal(alertsList)
	if err != nil {
		t.Fatal(err)
	}
	for _, excluded := range [][]byte{[]byte("excluded-list-message-canary"), []byte("excluded-list-label-canary"), []byte(`"message"`), []byte(`"labels"`), []byte(`"type"`)} {
		if bytes.Contains(encodedLogs, excluded) || bytes.Contains(encodedAlerts, excluded) {
			t.Fatalf("list DTO retained detail content %q", excluded)
		}
	}

	paths := make(map[string]string)
	for _, operation := range ddae.ApprovedOperations() {
		paths[operation.Collector] = operation.Path
	}
	for _, pair := range [][2]string{
		{"alert_list", "/v1/serviceability-issues"},
		{"alert_detail", "/v1/serviceability-issues/{id}"},
		{"serviceability_log_list", "/v1/serviceability-events"},
		{"serviceability_log_detail", "/v1/serviceability-events/{id}"},
	} {
		if paths[pair[0]] != pair[1] {
			t.Fatalf("operation %s path=%q want=%q", pair[0], paths[pair[0]], pair[1])
		}
	}
}
