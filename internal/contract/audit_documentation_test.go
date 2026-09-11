package contract

import (
	"os"
	"strings"
	"testing"
)

// TEST-DDAE-12-010: bilingual operator facts for the corrected boundaries.
func TestAuditDocumentationPreservesOperatorFacts(t *testing.T) {
	for _, path := range []string{"README.md", "README.zh-TW.md", "docs/runbook.md"} {
		b, err := os.ReadFile("../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{"[A-Za-z0-9._-]", "query-events.db", "CHECKPOINT_MAX_ALERTS", "CollectedAt"} {
			if !strings.Contains(string(b), value) {
				t.Errorf("%s missing operator fact %s", path, value)
			}
		}
	}
}
