//go:build e2e

package integration

import (
	"context"
	"encoding/json"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/portable"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPortableNativeWindows(t *testing.T) {
	if os.Getenv("DDAE_PORTABLE_WINDOWS_ENABLED") != "1" || runtime.GOOS != "windows" {
		t.Fatal("native authorized Windows 11 execution required")
	}
	root := os.Getenv("DDAE_PORTABLE_ROOT")
	if root == "" || portable.VerifyBundle(root) != nil {
		t.Fatal("complete authorized Portable bundle required")
	}
	native := os.Getenv("PROCESSOR_ARCHITEW6432")
	if native == "" {
		native = os.Getenv("PROCESSOR_ARCHITECTURE")
	}
	if (runtime.GOARCH == "amd64" && !strings.EqualFold(native, "AMD64")) || (runtime.GOARCH == "arm64" && !strings.EqualFold(native, "ARM64")) {
		t.Fatal("native architecture required; emulation is not evidence")
	}
	dest := filepath.Join(t.TempDir(), "Portable Unicode 測試")
	for _, name := range append(append([]string{}, portable.BundleFiles...), "SHA256SUMS") {
		path := filepath.Join(dest, name)
		if os.MkdirAll(filepath.Dir(path), 0700) != nil {
			t.Fatal("fixture directory")
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || os.WriteFile(path, data, 0700) != nil {
			t.Fatal("fixture copy")
		}
	}
	for _, script := range []string{"Prepare.cmd", "Run-SelfTest.cmd"} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		command := exec.CommandContext(ctx, "cmd.exe", "/d", "/c", filepath.Join(dest, script))
		command.Dir = dest
		command.Stdin = strings.NewReader("\r\n\r\n")
		err := command.Run()
		cancel()
		if err != nil {
			t.Fatal("native launcher failed")
		}
	}
	entries, err := os.ReadDir(filepath.Join(dest, "results"))
	if err != nil || len(entries) == 0 {
		t.Fatal("native report absent")
	}
	validated := false
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dest, "results", entry.Name(), "report.json"))
		if err != nil {
			continue
		}
		var r portable.Report
		if json.Unmarshal(data, &r) == nil && r.OS == "windows" && r.Architecture == runtime.GOARCH && r.Mode == "self-test" && r.Complete && r.ExitCode == 0 {
			validated = true
		}
	}
	if !validated {
		t.Fatal("native self-test evidence missing")
	}
	// Actual key/capture/replay and Ctrl+C checks are also required. The operator
	// must supply their separate completed checklist; never infer it from startup.
	if os.Getenv("DDAE_PORTABLE_OPERATOR_CHECKLIST") != "completed" {
		t.Fatal("native ACL, key roundtrip/replay and Ctrl+C operator checklist incomplete")
	}
}
