package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestPortableCLIInvalidAndKeyLifecycle(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"run", "--unknown", "sensitive-canary"}, {"decrypt"}, {"keygen"}, {"verify-bundle", "--root", "missing"}, {"run", "--config", "missing"}} {
		var out bytes.Buffer
		if run(args, &out) != 2 {
			t.Fatal("invalid CLI accepted")
		}
		if bytes.Contains(out.Bytes(), []byte("sensitive-canary")) {
			t.Fatal("argument echoed")
		}
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"keygen", "--private-key", filepath.Join(root, "private.pem"), "--public-key", filepath.Join(root, "public.pem")}
	var out bytes.Buffer
	if run(args, &out) != 0 {
		t.Fatal("keygen")
	}
	if run(args, &out) != 2 {
		t.Fatal("key overwrite")
	}
	if run([]string{"self-test", "--output", root}, &out) != 0 {
		t.Fatal("self-test CLI")
	}
}
