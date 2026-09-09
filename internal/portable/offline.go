package portable

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	_ "embed"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"
)

//go:embed testdata/selftest.json
var selfTestCases []byte

func SelfTest(root string, build BuildInfo) (string, int) {
	if platformReady() != nil {
		return "", 2
	}
	out, err := newReports(root, "self-test", build, 16<<20)
	if err != nil {
		return "", 2
	}
	code := 0
	check := func(op string, ok bool) {
		status := "PASS"
		if !ok {
			status = "FAIL"
			code = 1
		}
		if out.check(Check{Operation: op, Status: status}) != nil {
			code = 3
		}
	}
	var cases []struct {
		Operation, Body  string
		Decode, Contract bool
	}
	if json.Unmarshal(selfTestCases, &cases) != nil {
		code = 1
	}
	for _, tc := range cases {
		r, _ := Parse(tc.Operation, "", []byte(tc.Body))
		check(tc.Operation, r.DecodeOK == tc.Decode && r.ContractOK == tc.Contract)
	}
	key, e := rsa.GenerateKey(rand.Reader, 3072)
	if e != nil {
		check("encryption", false)
	} else {
		path := filepath.Join(out.dir, "selftest.ddaecap")
		w, e := NewCapture(path, &key.PublicKey, 1<<20)
		if e == nil {
			e = w.Append(Exchange{Operation: "nodes", Body: []byte("{\"results\":[]}"), Complete: true})
			if e == nil {
				e = w.Finish(true)
			} else {
				_ = w.Finish(false)
			}
		}
		count := 0
		if e == nil {
			complete, err := ReadCapture(path, key, func(x Exchange) error {
				count++
				if string(x.Body) != "{\"results\":[]}" {
					return ErrIntegrity
				}
				return nil
			})
			if err != nil || !complete || count != 1 {
				e = ErrIntegrity
			}
		}
		check("encryption", e == nil)
		// This scratch archive contains only synthetic data and a disposable key.
		if os.Remove(path) != nil {
			check("scratch_cleanup", false)
		}
	}
	check("loopback_tls_auth", selfTestTransport(out) == nil)
	if out.finish(code, true) != nil {
		code = 3
	}
	return out.dir, code
}
func selfTestTransport(out *reports) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return ErrInput
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/realms/ddae/protocol/openid-connect/token":
			_, _ = w.Write([]byte(`{"access_token":"synthetic-self-test","expires_in":3600}`))
		case "/ping":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(404)
		}
	})}}
	server.StartTLS()
	defer server.Close()
	ca := filepath.Join(out.dir, "synthetic-ca.pem")
	if privateWrite(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})) != nil {
		return ErrStorage
	}
	defer os.Remove(ca)
	values := map[string]string{"DDAE_BASE_URL": server.URL, "DDAE_USERNAME": "synthetic", "DDAE_PASSWORD": "synthetic", "DDAE_CLIENT_SECRET": "synthetic", "DDAE_ALERT_MONITORING_ENABLED": "false", "DDAE_CA_FILE": ca}
	cfg, err := config.LoadIsolated(values)
	if err != nil {
		return ErrInput
	}
	o := &runObserver{output: out, max: 2}
	c, err := ddae.NewDiagnosticClient(cfg, o, 4<<20)
	if err != nil {
		return err
	}
	defer c.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = c.Ping(ctx)
	return err
}

// Analyze never constructs an external network transport. Raw extraction is
// opt-in and protected; filenames come from a counter, never captured paths.
func Analyze(capture, privateKey, root string, extract bool, build BuildInfo) (string, int) {
	key, err := ReadPrivateKey(privateKey)
	if err != nil {
		return "", 2
	}
	mode := "replay"
	if extract {
		mode = "decrypt"
	}
	out, err := newReports(root, mode, build, 64<<20)
	if err != nil {
		return "", 2
	}
	code := 0
	index := 0
	var extractedBytes int64
	complete, readErr := ReadCapture(capture, key, func(e Exchange) error {
		index++
		if extract {
			data, err := json.MarshalIndent(e, "", "  ")
			if err != nil {
				return ErrIntegrity
			}
			extractedBytes += int64(len(data)) + int64(len(e.Body))
			if extractedBytes > 2<<30 {
				return ErrLimit
			}
			if privateWrite(filepath.Join(out.dir, fmt.Sprintf("%06d.exchange.json", index)), data) != nil {
				return ErrStorage
			}
			if privateWrite(filepath.Join(out.dir, fmt.Sprintf("%06d.body", index)), e.Body) != nil {
				return ErrStorage
			}
		}
		p, _ := Parse(e.Operation, e.RequestedID, e.Body)
		status := "PASS"
		if !e.Complete || e.Status < 200 || e.Status >= 300 || !p.DecodeOK || !p.ValidationOK || !p.ContractOK {
			status = "FAIL"
			code = 1
		}
		if err := out.step(Step{RequestID: index, Operation: e.Operation, HTTPStatus: e.Status, HTTP: e.Status >= 200 && e.Status < 300, CaptureComplete: e.Complete, Parse: &p}); err != nil {
			return err
		}
		return out.check(Check{Operation: e.Operation, Status: status, Reason: p.Reason})
	})
	if readErr != nil || !complete {
		code = 3
		complete = false
	}
	if out.finish(code, complete) != nil {
		code = 3
	}
	return out.dir, code
}
