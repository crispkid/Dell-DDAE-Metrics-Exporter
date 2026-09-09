//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/portable"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableAuthorizedDDAE(t *testing.T) {
	if os.Getenv("DDAE_PORTABLE_FIELD_ENABLED") != "1" || os.Getenv("DDAE_TEST_SOFTWARE_VERSION") != "1.5.0" {
		t.Fatal("separate authorized non-production field boundary required")
	}
	cfg, err := portable.LoadConfig(os.Getenv("DDAE_PORTABLE_CONFIG"))
	if err != nil {
		t.Fatal("field configuration invalid")
	}
	if cfg.DDAE.TLS.Insecure || !cfg.Checks.Ping || !cfg.Checks.Resources || !cfg.Checks.Alerts || !cfg.Checks.Logs || cfg.Run.MaxDetails < 1 {
		t.Fatal("verified TLS and all API families required for compatibility evidence")
	}
	dir, code := portable.Run(context.Background(), cfg, portable.BuildInfo{Version: "external-validation"})
	if code != 0 {
		t.Fatal("actual API/parser checks failed")
	}
	data, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal("field report absent")
	}
	var report portable.Report
	if json.Unmarshal(data, &report) != nil || !report.Complete {
		t.Fatal("field evidence incomplete")
	}
	details := map[string]bool{}
	for _, check := range report.Checks {
		if check.Status == "PASS" && check.Selected > 0 && check.Selected == check.Successful {
			details[check.Operation] = true
		}
	}
	if !details["alert_detail"] || !details["serviceability_log_detail"] {
		t.Fatal("both actual list/detail relationships must be proved")
	}
}
