package main

import (
	"archive/zip"
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/portable"
)

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func packageFixture(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for _, dir := range []string{"cmd", "internal", "scripts", "testdata"} {
		writeFixture(t, filepath.Join(dir, "fixture.go"), []byte("synthetic source"))
	}
	for _, name := range []string{"go.mod", "go.sum", "Dockerfile"} {
		writeFixture(t, name, []byte("synthetic"))
	}
	stage := filepath.Join(root, "stage")
	for _, name := range portable.BundleFiles {
		if strings.HasPrefix(name, "bin/") {
			// Minimal COFF/PE header for the architecture validator; never executed.
			var b bytes.Buffer
			header := make([]byte, 128)
			header[0] = 'M'
			header[1] = 'Z'
			binary.LittleEndian.PutUint32(header[60:], 128)
			b.Write(header)
			b.Write([]byte{'P', 'E', 0, 0})
			machine := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
			if strings.Contains(name, "arm64") {
				machine = pe.IMAGE_FILE_MACHINE_ARM64
			}
			if err := binary.Write(&b, binary.LittleEndian, pe.FileHeader{Machine: machine}); err != nil {
				t.Fatal(err)
			}
			writeFixture(t, filepath.Join(stage, name), b.Bytes())
		} else if strings.HasPrefix(name, "tools/") {
			writeFixture(t, filepath.Join(stage, name), []byte("synthetic helper"))
		} else {
			writeFixture(t, filepath.Join("Portable", name), []byte("synthetic static asset"))
		}
	}
	moduleDir := filepath.Join(root, "dependency")
	writeFixture(t, filepath.Join(moduleDir, "LICENSE"), []byte("synthetic license"))
	original := commandOutput
	t.Cleanup(func() { commandOutput = original })
	commandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "git" {
			return []byte("synthetic dirty revision"), nil
		}
		return json.Marshal(map[string]any{"Module": map[string]string{"Path": "synthetic/module", "Version": "v1.0.0", "Dir": moduleDir}})
	}
	sum, _, err := identity()
	if err != nil {
		t.Fatal(err)
	}
	return stage, sum
}

func TestPortablePackageAllowlistIdentityAndNotices(t *testing.T) {
	stage, sum := packageFixture(t)
	for _, name := range []string{"Portable/config.yaml", "Portable/secrets/password", "Portable/keys/private.pem", "Portable/results/old.log", "unrelated.docx"} {
		writeFixture(t, name, []byte("excluded-private-canary"))
	}
	if after, _, err := identity(); err != nil || after != sum {
		t.Fatal("runtime data entered source identity", err)
	}
	if err := runArgs([]string{"-stage", stage, "-source", sum}); err != nil {
		t.Fatal(err)
	}
	if err := portable.VerifyBundle("Portable"); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader("Portable/Portable-Windows11.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if len(z.File) != len(portable.BundleFiles)+1 {
		t.Fatal("unexpected ZIP member count")
	}
	for _, member := range z.File {
		if strings.Contains(member.Name, "secrets/") || strings.Contains(member.Name, "results/") || member.Name == "Portable/config.yaml" || strings.Contains(member.Name, "private.pem") {
			t.Fatal("unsafe distribution")
		}
	}
	b, err := os.ReadFile("Portable/THIRD-PARTY-NOTICES.txt")
	if err != nil || !bytes.Contains(b, []byte("synthetic license")) {
		t.Fatal("license missing", err)
	}
	if err := runArgs([]string{"-identity"}); err != nil {
		t.Fatal(err)
	}
	if err := runArgs([]string{"-stage", stage, "-source", "wrong"}); err == nil {
		t.Fatal("identity mismatch accepted")
	}
	if err := runArgs([]string{"-unknown"}); err == nil {
		t.Fatal("unknown flag accepted")
	}
	if err := runArgs([]string{"extra"}); err == nil {
		t.Fatal("positional flag accepted")
	}
	writeFixture(t, "cmd/fixture.go", []byte("modified source"))
	if after, _, _ := identity(); after == sum {
		t.Fatal("source change not detected")
	}
}

func TestPortablePackageRejectsMalformedSources(t *testing.T) {
	stage, sum := packageFixture(t)
	writeFixture(t, filepath.Join(stage, "bin/windows-amd64/ddae-diagnose.exe"), []byte("not PE"))
	if err := runArgs([]string{"-stage", stage, "-source", sum}); err == nil {
		t.Fatal("malformed PE accepted")
	}
	commandOutput = func(string, ...string) ([]byte, error) { return nil, errors.New("synthetic command failure") }
	if _, err := dependencyNotices(); err == nil {
		t.Fatal("command failure ignored")
	}
	commandOutput = func(string, ...string) ([]byte, error) { return []byte("{"), nil }
	if _, err := dependencyNotices(); err == nil {
		t.Fatal("malformed module metadata accepted")
	}
	if err := copyFile("missing", "target"); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := os.Symlink("cmd", "linked"); err == nil {
		if copyFile("go.mod", "linked/created/target") == nil {
			t.Fatal("destination symlink accepted")
		}
		if _, err := os.Stat("cmd/created"); !os.IsNotExist(err) {
			t.Fatal("unsafe directory created before validation")
		}
		if err := os.Symlink("../go.mod", "cmd/linked.go"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := identity(); err == nil {
			t.Fatal("source symlink accepted")
		}
	}
}
