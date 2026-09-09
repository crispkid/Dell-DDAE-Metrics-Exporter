package portable

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortablePrivacySafeReports(t *testing.T) {
	canary := "synthetic-private-body-canary"
	c, _ := fieldConfig(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "synthetic-cookie-canary")
		w.Header().Set("X-Echo-Authorization", r.Header.Get("Authorization"))
		w.Write([]byte(`{"status":"ok","unknown":"` + canary + `"}`))
	})
	c.Checks.Resources = false
	c.Checks.Alerts = false
	c.Checks.Logs = false
	dir, code := Run(context.Background(), c, BuildInfo{})
	if code != 0 {
		t.Fatal("run failed")
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{canary, "synthetic-cookie-canary", "synthetic-token", "synthetic-password", c.DDAE.BaseURL} {
			if strings.Contains(string(data), secret) {
				t.Fatal("private canary leaked")
			}
		}
	}
}
