package portable

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureCryptoRoundTrip(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(privateTemp(t), "capture.ddaecap")
	w, err := NewCapture(path, &key.PublicKey, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	original := Exchange{Operation: "nodes", Body: []byte("{malformed\xff"), Complete: true}
	if err := w.Append(original); err != nil {
		t.Fatal(err)
	}
	if err := w.Finish(true); err != nil {
		t.Fatal(err)
	}
	count := 0
	complete, err := ReadCapture(path, key, func(e Exchange) error {
		count++
		if string(e.Body) != string(original.Body) {
			t.Fatal("body changed")
		}
		return nil
	})
	if err != nil || !complete || count != 1 {
		t.Fatalf("roundtrip: complete=%v count=%d err=%v", complete, count, err)
	}
}

func TestPortableCryptoOrderRecipientAndPartialRecovery(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatal(err)
	}
	root := privateTemp(t)
	path := filepath.Join(root, "ordered.ddaecap")
	w, err := NewCapture(path, &key.PublicKey, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"first", "second"} {
		if w.Append(Exchange{Operation: "nodes", Body: []byte(body)}) != nil {
			t.Fatal("append")
		}
	}
	if w.Finish(true) != nil {
		t.Fatal("finish")
	}
	if complete, err := ReadCapture(path, other, func(Exchange) error { return nil }); err == nil || complete {
		t.Fatal("wrong key accepted")
	}
	b, _ := os.ReadFile(path)
	headerEnd := 12 + int(binary.BigEndian.Uint32(b[8:12]))
	firstEnd := headerEnd + 4 + int(binary.BigEndian.Uint32(b[headerEnd:headerEnd+4]))
	secondEnd := firstEnd + 4 + int(binary.BigEndian.Uint32(b[firstEnd:firstEnd+4]))
	for kind, body := range map[string][]byte{
		"reorder":       append(append(append(append([]byte{}, b[:headerEnd]...), b[firstEnd:secondEnd]...), b[headerEnd:firstEnd]...), b[secondEnd:]...),
		"duplicate":     append(append(append([]byte{}, b[:firstEnd]...), b[headerEnd:firstEnd]...), b[firstEnd:]...),
		"delete":        append(append([]byte{}, b[:headerEnd]...), b[firstEnd:]...),
		"missing-index": append([]byte{}, b[:secondEnd]...),
	} {
		p := filepath.Join(root, kind+".ddaecap")
		if os.WriteFile(p, body, 0600) != nil {
			t.Fatal("fixture")
		}
		count := 0
		complete, err := ReadCapture(p, key, func(Exchange) error { count++; return nil })
		if err == nil || complete {
			t.Fatal("order/integrity failure accepted")
		}
		if kind == "missing-index" && count != 2 {
			t.Fatal("authenticated partial records unavailable")
		}
	}
	second, err := NewCapture(filepath.Join(root, "fresh.ddaecap"), &key.PublicKey, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if second.hash == w.hash {
		t.Fatal("run identity reused")
	}
	_ = second.Finish(false)
}

func TestPortableCryptoTamperBoundsAndFaults(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatal(err)
	}
	root := privateTemp(t)
	path := filepath.Join(root, "valid.ddaecap")
	w, err := NewCapture(path, &key.PublicKey, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if w.Append(Exchange{Operation: "token"}) == nil {
		t.Fatal("token accepted")
	}
	if w.Append(Exchange{Operation: "nodes", Body: make([]byte, 1<<20)}) == nil {
		t.Fatal("size cap")
	}
	if w.Append(Exchange{Operation: "nodes", Body: []byte("[]")}) != nil || w.Finish(true) != nil {
		t.Fatal("write")
	}
	if w.Finish(true) == nil || w.Append(Exchange{}) == nil {
		t.Fatal("closed writer accepted")
	}
	good, _ := os.ReadFile(path)
	for _, kind := range []string{"magic", "header", "bitflip", "truncate", "append"} {
		data := append([]byte{}, good...)
		switch kind {
		case "magic":
			data[0] ^= 1
		case "header":
			data[8] = 255
		case "bitflip":
			data[len(data)-10] ^= 1
		case "truncate":
			data = data[:len(data)-5]
		case "append":
			data = append(data, 1)
		}
		damaged := filepath.Join(root, kind+".ddaecap")
		os.WriteFile(damaged, data, 0600)
		if complete, err := ReadCapture(damaged, key, func(Exchange) error { return nil }); err == nil || complete {
			t.Fatal("damaged capture accepted")
		}
	}
	bad := filepath.Join(root, "bad.pem")
	os.WriteFile(bad, []byte("not-a-key"), 0600)
	if _, err := ReadPublicKey(bad); err == nil {
		t.Fatal("bad public key")
	}
	if _, err := ReadPrivateKey(bad); err == nil {
		t.Fatal("bad private key")
	}
	if _, err := NewCapture(path, nil, 1<<20); err == nil {
		t.Fatal("nil key")
	}
	if _, err := readFrame(bytes.NewReader([]byte{255, 255, 255, 255}), 64); err == nil {
		t.Fatal("unbounded frame")
	}
	damaged := filepath.Join(root, "disk.ddaecap")
	f, err := NewCapture(damaged, &key.PublicKey, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.file.Close()
	if f.Append(Exchange{Operation: "nodes"}) == nil || f.Finish(false) == nil {
		t.Fatal("disk fault ignored")
	}
}

func privateTemp(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return path
}
