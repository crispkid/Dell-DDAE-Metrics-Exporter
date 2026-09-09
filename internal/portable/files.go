package portable

import (
	"os"
	"path/filepath"
	"strings"
)

// Check every existing component, not just the final file, before accessing
// protected data. Output directories must be trusted against concurrent changes.
func safePath(path string) error {
	if path == "" {
		return ErrInput
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return ErrInput
	}
	for {
		st, e := os.Lstat(p)
		if e == nil {
			if st.Mode()&os.ModeSymlink != 0 || platformPath(p) != nil {
				return ErrInput
			}
		} else if !os.IsNotExist(e) {
			return ErrInput
		}
		next := filepath.Dir(p)
		if next == p {
			break
		}
		p = next
	}
	return nil
}
func privateDir(path string) error {
	if safePath(path) != nil || !dedicatedDirectory(path) {
		return ErrStorage
	}
	if os.MkdirAll(path, 0700) != nil {
		return ErrStorage
	}
	if protect(path, true) != nil {
		return ErrStorage
	}
	return nil
}

func dedicatedDirectory(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil || filepath.Dir(abs) == abs {
		return false
	}
	// Applying an inheritable protected ACL to an existing broad directory can
	// affect unrelated user files. Require dedicated output/key directories.
	home, _ := os.UserHomeDir()
	for _, broad := range []string{home, os.TempDir(), filepath.Join(home, "Desktop"), filepath.Join(home, "Documents"), filepath.Join(home, "Downloads")} {
		if broad == "" {
			continue
		}
		if strings.EqualFold(filepath.Clean(abs), filepath.Clean(broad)) {
			return false
		}
		canonical, err := filepath.EvalSymlinks(broad)
		if err == nil && strings.EqualFold(filepath.Clean(abs), filepath.Clean(canonical)) {
			return false
		}
	}
	return true
}
func privateCreate(path string) (*os.File, error) {
	if safePath(path) != nil {
		return nil, ErrStorage
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, ErrStorage
	}
	if protect(path, false) != nil {
		f.Close()
		return nil, ErrStorage
	}
	return f, nil
}
func privateWrite(path string, data []byte) error {
	f, err := privateCreate(path)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err != nil || syncErr != nil || closeErr != nil {
		return ErrStorage
	}
	return nil
}
