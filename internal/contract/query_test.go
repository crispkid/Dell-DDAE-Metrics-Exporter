package contract

import (
	"os"
	"strings"
	"testing"
)

func TestQueryDocumentedIsolation(t *testing.T) {
	b, e := os.ReadFile("../../docs/query-monitoring.md")
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"QUERY_ENABLED", "QUERY_PASSWORD_FILE", "query-events.db", "QUERY_KAFKA_TOPIC", "history_complete"} {
		if !strings.Contains(string(b), v) {
			t.Fatalf("missing %s", v)
		}
	}
}
