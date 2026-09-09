package collector

import "github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"

// ValidateDiagnostic reuses resource normalization without publishing metric
// labels or snapshot identities into a diagnostic report.
func ValidateDiagnostic(value any) error {
	switch v := value.(type) {
	case []ddae.Cluster:
		_, _, err := normalizeClusters(v)
		return err
	case []ddae.InfrastructureNode:
		_, _, err := normalizeNodes(v)
		return err
	case ddae.PowerResponse:
		_, _, err := normalizePower(v)
		return err
	}
	return nil
}
