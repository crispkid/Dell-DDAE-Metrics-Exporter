// portable-package is a build-host tool. It is not a field runtime dependency.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/portable"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "portable packaging failed:", err)
		os.Exit(1)
	}
}
func identity() (string, map[string]string, error) {
	files := []string{"go.mod", "go.sum", "Dockerfile"}
	for _, dir := range []string{"cmd", "internal", "scripts", "testdata"} {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("source symlink rejected")
			}
			switch filepath.Ext(path) {
			case ".go", ".sh", ".json", ".yaml":
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return "", nil, err
		}
	}
	for _, name := range portable.BundleFiles {
		if strings.HasSuffix(name, ".cmd") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".md") {
			files = append(files, filepath.Join("Portable", name))
		}
	}
	sort.Strings(files)
	hashes := map[string]string{}
	h := sha256.New()
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		sum := sha256.Sum256(data)
		name := filepath.ToSlash(path)
		hashes[name] = hex.EncodeToString(sum[:])
		fmt.Fprintf(h, "%d:%s:%s\n", len(name), name, hashes[name])
	}
	return hex.EncodeToString(h.Sum(nil)), hashes, nil
}
func run() error {
	return runArgs(os.Args[1:])
}

var commandOutput = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func runArgs(args []string) error {
	flags := flag.NewFlagSet("portable-package", flag.ContinueOnError)
	only := flags.Bool("identity", false, "print source identity")
	stage := flags.String("stage", "", "staging directory")
	source := flags.String("source", "", "expected source identity")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected package argument")
	}
	sum, inputs, err := identity()
	if err != nil {
		return err
	}
	if *only {
		fmt.Println(sum)
		return nil
	}
	if *stage == "" || sum != *source {
		return fmt.Errorf("source changed during build")
	}
	revision, err := commandOutput("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	status, err := commandOutput("git", "status", "--porcelain")
	if err != nil {
		return err
	}
	manifest, err := json.MarshalIndent(struct {
		Schema         int               `json:"schema_version"`
		Version        string            `json:"version"`
		Go             string            `json:"go_version"`
		Revision       string            `json:"revision"`
		Dirty          bool              `json:"working_tree_dirty"`
		Source         string            `json:"source_sha256"`
		Inputs         map[string]string `json:"source_inputs"`
		WindowsRuntime string            `json:"windows_runtime_validation"`
		DDAE           string            `json:"authenticated_ddae_validation"`
	}{1, "ddae7-local", runtime.Version(), strings.TrimSpace(string(revision)), len(status) > 0, sum, inputs, "not-executed", "not-executed"}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*stage, "build-manifest.json"), manifest, 0600); err != nil {
		return err
	}
	notices, err := dependencyNotices()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*stage, "THIRD-PARTY-NOTICES.txt"), notices, 0600); err != nil {
		return err
	}
	var sums strings.Builder
	for _, name := range portable.BundleFiles {
		target := filepath.Join(*stage, filepath.FromSlash(name))
		if !strings.HasPrefix(name, "bin/") && !strings.HasPrefix(name, "tools/") && name != "build-manifest.json" && name != "THIRD-PARTY-NOTICES.txt" {
			if err := copyFile(filepath.Join("Portable", name), target); err != nil {
				return err
			}
		}
		if strings.HasSuffix(name, ".exe") {
			f, err := pe.Open(target)
			if err != nil {
				return err
			}
			expected := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
			if strings.Contains(name, "windows-arm64") {
				expected = pe.IMAGE_FILE_MACHINE_ARM64
			}
			machine := f.Machine
			f.Close()
			if machine != expected {
				return fmt.Errorf("PE architecture mismatch")
			}
		}
		b, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(b)
		fmt.Fprintf(&sums, "%x  %s\n", hash, name)
	}
	if err := os.WriteFile(filepath.Join(*stage, "SHA256SUMS"), []byte(sums.String()), 0600); err != nil {
		return err
	}
	if err := portable.VerifyBundle(*stage); err != nil {
		return err
	}
	members := append(append([]string{}, portable.BundleFiles...), "SHA256SUMS")
	// Publish only generated artifacts. Runtime YAML/keys/results are untouched.
	for _, name := range members {
		if strings.HasPrefix(name, "bin/") || strings.HasPrefix(name, "tools/") || name == "build-manifest.json" || name == "SHA256SUMS" || name == "THIRD-PARTY-NOTICES.txt" {
			if err := copyFile(filepath.Join(*stage, name), filepath.Join("Portable", name)); err != nil {
				return err
			}
		}
	}
	f, err := os.Create(filepath.Join(*stage, "Portable-Windows11.zip"))
	if err != nil {
		return err
	}
	z := zip.NewWriter(f)
	for _, name := range members {
		info, err := os.Stat(filepath.Join(*stage, name))
		if err != nil {
			f.Close()
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			f.Close()
			return err
		}
		header.Name = "Portable/" + name
		header.Method = zip.Deflate
		entry, err := z.CreateHeader(header)
		if err != nil {
			f.Close()
			return err
		}
		src, err := os.Open(filepath.Join(*stage, name))
		if err != nil {
			f.Close()
			return err
		}
		_, err = io.Copy(entry, src)
		src.Close()
		if err != nil {
			f.Close()
			return err
		}
	}
	if err := z.Close(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return copyFile(filepath.Join(*stage, "Portable-Windows11.zip"), filepath.Join("Portable", "Portable-Windows11.zip"))
}
func copyFile(src, dest string) error {
	abs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for p := abs; ; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err == nil && st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination symlink rejected")
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	if st, err := os.Lstat(src); err != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("invalid package source")
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	st, err := input.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("invalid package member")
	}
	output, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, st.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func dependencyNotices() ([]byte, error) {
	data, err := commandOutput("go", "list", "-deps", "-json", "./cmd/ddae-exporter", "./cmd/ddae-diagnose")
	if err != nil {
		return nil, err
	}
	type module struct{ Path, Version, Dir string }
	modules := map[string]module{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var p struct{ Module *module }
		err := decoder.Decode(&p)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if p.Module != nil && p.Module.Version != "" {
			modules[p.Module.Path] = *p.Module
		}
	}
	keys := []string{}
	for k := range modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var result bytes.Buffer
	result.WriteString("Bundled third-party notices. Project licensing is not assigned by this file.\n\nGo runtime\n")
	license, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "LICENSE"))
	if err != nil {
		return nil, err
	}
	result.Write(license)
	for _, k := range keys {
		m := modules[k]
		fmt.Fprintf(&result, "\n\n%s %s\n", m.Path, m.Version)
		found := false
		entries, err := os.ReadDir(m.Dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			name := strings.ToUpper(entry.Name())
			if entry.IsDir() || !(strings.HasPrefix(name, "LICENSE") || name == "COPYING" || strings.HasPrefix(name, "NOTICE") || name == "COPYRIGHT") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(m.Dir, entry.Name()))
			if err != nil {
				return nil, err
			}
			result.Write(content)
			result.WriteByte('\n')
			found = true
		}
		if !found {
			return nil, fmt.Errorf("dependency license missing: %s", m.Path)
		}
	}
	return result.Bytes(), nil
}
