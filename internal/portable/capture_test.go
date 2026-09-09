package portable

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableCaptureFailureAndCompression(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{200, 404} {
		c, _ := fieldConfig(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Encoding", "gzip")
			w.WriteHeader(status)
			z := gzip.NewWriter(w)
			z.Write([]byte("not-json-body"))
			z.Close()
		})
		c.Checks.Resources = false
		c.Checks.Alerts = false
		c.Checks.Logs = false
		c.Capture.Enabled = true
		pub, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
		c.Capture.PublicKey = filepath.Join(filepath.Dir(c.Output.Directory), "public.pem")
		os.WriteFile(c.Capture.PublicKey, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), 0600)
		dir, code := Run(context.Background(), c, BuildInfo{})
		if code != 1 {
			t.Fatalf("code=%d", code)
		}
		count := 0
		complete, err := ReadCapture(filepath.Join(dir, "http-capture.ddaecap"), key, func(e Exchange) error {
			count++
			if string(e.Body) != "not-json-body" || !e.ContentDecoded || e.Status != status || e.ResponseHeaders.Get("Content-Encoding") != "gzip" || e.RequestBody == nil || len(e.RequestBody) != 0 {
				t.Error("capture changed body/status")
			}
			return nil
		})
		if err != nil || !complete || count != 1 {
			t.Fatal("capture incomplete")
		}
	}
}
