//go:build !windows

package portable

import "os"

func platformPath(string) error { return nil }
func protect(path string, dir bool) error {
	mode := os.FileMode(0600)
	if dir {
		mode = 0700
	}
	return os.Chmod(path, mode)
}
func platformReady() error { return nil }
