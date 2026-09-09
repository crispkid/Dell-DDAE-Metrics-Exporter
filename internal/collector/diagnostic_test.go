package collector

import (
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"testing"
)

func TestDiagnosticUsesResourceValidators(t *testing.T) {
	for _, value := range []any{[]ddae.Cluster{{}}, []ddae.InfrastructureNode{{}}, ddae.PowerResponse{}} {
		if ValidateDiagnostic(value) == nil {
			t.Fatal("missing required resource data accepted")
		}
	}
	if ValidateDiagnostic(ddae.PingResponse{}) != nil {
		t.Fatal("unrelated validation")
	}
}
