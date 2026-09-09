package portable

import (
	"bufio"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// BundleFiles is the complete immutable distribution allowlist. Runtime values
// and private/analysis directories cannot enter a distribution via traversal.
var BundleFiles = []string{
	"README.zh-TW.md", "Prepare.cmd", "Run-SelfTest.cmd", "Run-Diagnostics.cmd",
	"Create-Analysis-Key.cmd", "Review-Results.cmd", "Run-Exporter.cmd", "Launch.cmd",
	"config.example.yaml", "exporter.example.yaml", "THIRD-PARTY-NOTICES.txt", "build-manifest.json",
	"bin/windows-amd64/ddae-diagnose.exe", "bin/windows-amd64/ddae-exporter.exe",
	"bin/windows-arm64/ddae-diagnose.exe", "bin/windows-arm64/ddae-exporter.exe",
	"tools/darwin-arm64/ddae-diagnose",
}

func VerifyBundle(root string) error {
	data, err := readBounded(filepath.Join(root, "SHA256SUMS"), 1<<20)
	if err != nil {
		return ErrInput
	}
	expected := map[string]bool{}
	for _, name := range BundleFiles {
		expected[name] = true
	}
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	seen := map[string]bool{}
	for scan.Scan() {
		fields := strings.SplitN(scan.Text(), "  ", 2)
		if len(fields) != 2 || len(fields[0]) != 64 || !expected[fields[1]] || seen[fields[1]] {
			return ErrIntegrity
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return ErrIntegrity
		}
		got, err := fileHash(filepath.Join(root, filepath.FromSlash(fields[1])))
		if err != nil || got != fields[0] {
			return ErrIntegrity
		}
		seen[fields[1]] = true
	}
	if scan.Err() != nil || len(seen) != len(expected) {
		return ErrIntegrity
	}
	return nil
}
func Prepare(root string) error {
	if platformReady() != nil || VerifyBundle(root) != nil {
		return ErrInput
	}
	for _, name := range []string{"secrets", "trust", "keys", "results"} {
		if privateDir(filepath.Join(root, name)) != nil {
			return ErrStorage
		}
	}
	for _, pair := range [][2]string{{"config.example.yaml", "config.yaml"}, {"exporter.example.yaml", "exporter.yaml"}} {
		dest := filepath.Join(root, pair[1])
		if safePath(dest) != nil {
			return ErrInput
		}
		if st, err := os.Lstat(dest); err == nil {
			if !st.Mode().IsRegular() {
				return ErrInput
			}
			continue
		} else if !os.IsNotExist(err) {
			return ErrInput
		}
		data, err := readBounded(filepath.Join(root, pair[0]), 1<<20)
		if err != nil {
			return err
		}
		if privateWrite(dest, data) != nil {
			return ErrStorage
		}
	}
	return nil
}
