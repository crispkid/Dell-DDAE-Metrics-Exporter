//go:build e2e && !windows

package integration

import "testing"

// The native implementation is in portable_windows_test.go. Do not silently
// omit its required gate when the inherited E2E stage runs on another OS.
func TestPortableNativeWindows(t *testing.T) {
	t.Fatal("DDAE-7 native Windows 11 evidence requires an authorized Windows host; cross-build is not native execution")
}
