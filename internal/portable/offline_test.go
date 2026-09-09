package portable

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPortableOfflineSelfTestAndReplay(t *testing.T) {
	root := privateTemp(t)
	_, code := SelfTest(root, BuildInfo{})
	if code != 0 {
		t.Fatalf("selftest=%d", code)
	}
	priv := filepath.Join(root, "private.pem")
	pub := filepath.Join(root, "public.pem")
	if GenerateKeys(priv, pub) != nil {
		t.Fatal("keygen")
	}
	if GenerateKeys(priv, pub) == nil {
		t.Fatal("key overwrite")
	}
	key, err := ReadPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(root, "sample.ddaecap")
	w, err := NewCapture(capture, key, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if w.Append(Exchange{Operation: "nodes", Status: 200, Body: []byte(`{"results":[]}`), Complete: true}) != nil || w.Finish(true) != nil {
		t.Fatal("capture")
	}
	dir, code := Analyze(capture, priv, root, true, BuildInfo{})
	if code != 0 {
		t.Fatal("decrypt/replay")
	}
	body, err := os.ReadFile(filepath.Join(dir, "000001.body"))
	if err != nil || string(body) != `{"results":[]}` {
		t.Fatal("extracted bytes")
	}
	_, code = Analyze(capture, priv, root, false, BuildInfo{})
	if code != 0 {
		t.Fatal("offline replay")
	}
}
