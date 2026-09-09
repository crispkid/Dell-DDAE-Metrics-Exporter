package portable

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortablePathPackagePreparationAndSafety(t *testing.T) {
	root := privateTemp(t)
	var sums strings.Builder
	for _, name := range BundleFiles {
		path := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(path), 0700)
		data := []byte("synthetic-bundle-member")
		os.WriteFile(path, data, 0600)
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(data), name)
	}
	os.WriteFile(filepath.Join(root, "SHA256SUMS"), []byte(sums.String()), 0600)
	if VerifyBundle(root) != nil || Prepare(root) != nil || Prepare(root) != nil {
		t.Fatal("bundle prepare")
	}
	if _, err := os.Stat(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatal("config not prepared")
	}
	os.WriteFile(filepath.Join(root, "SHA256SUMS"), []byte("not-a-manifest"), 0600)
	if VerifyBundle(root) == nil {
		t.Fatal("invalid manifest")
	}
	link := filepath.Join(root, "unsafe")
	if os.Symlink(root, link) != nil {
		t.Skip("symlink unsupported")
	}
	if safePath(filepath.Join(link, "output")) == nil || privateDir(filepath.Join(link, "output")) == nil {
		t.Fatal("symlink accepted")
	}
}

func TestPortablePathRejectsBroadACLTargets(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.VolumeName(home) + string(filepath.Separator), home, filepath.Join(home, "Documents"), os.TempDir()} {
		if dedicatedDirectory(path) {
			t.Fatal("broad ACL target accepted")
		}
		if canonical, err := filepath.EvalSymlinks(path); err == nil && dedicatedDirectory(canonical) {
			t.Fatal("canonical broad ACL target accepted")
		}
	}
}
