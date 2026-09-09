package ddae

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type diagnosticRecorder struct {
	values           []DiagnosticExchange
	beginErr, endErr bool
}

func (r *diagnosticRecorder) Begin(string) (int, error) {
	if r.beginErr {
		return 0, errors.New("budget")
	}
	return len(r.values) + 1, nil
}
func (r *diagnosticRecorder) Observe(e DiagnosticExchange) error {
	r.values = append(r.values, e)
	if r.endErr {
		return errors.New("storage")
	}
	return nil
}
func TestPortablePrivacyDiagnosticTransport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			w.Header().Set("Set-Cookie", "secret-cookie")
			w.Write([]byte(`{"access_token":"private-token","expires_in":3600}`))
			return
		}
		w.Header().Set("Set-Cookie", "secret-cookie")
		w.Header().Set("X-Echo", r.Header.Get("Authorization"))
		w.Header().Set("X-Safe", "retained")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	cfg := clientConfig(t, server.URL, trustedServerCA(t, server), nil)
	if _, err := NewDiagnosticClient(cfg, nil, 4); err == nil {
		t.Fatal("nil observer")
	}
	for _, limit := range []int64{1, 4 << 20} {
		recorder := &diagnosticRecorder{}
		client, err := NewDiagnosticClient(cfg, recorder, limit)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Ping(context.Background())
		if (err == nil) != (limit > 1) {
			t.Fatal("body bound behavior")
		}
		client.CloseIdleConnections()
		if len(recorder.values) != 2 {
			t.Fatal("attempt pairing")
		}
		token, business := recorder.values[0], recorder.values[1]
		if token.URL != "" || token.RequestHeaders != nil || token.ResponseHeaders != nil || len(token.Body) != 0 {
			t.Fatal("token capture leak")
		}
		if business.RequestHeaders.Get("Authorization") != "" || business.ResponseHeaders.Get("Set-Cookie") != "" || business.ResponseHeaders.Get("X-Echo") != "" || business.ResponseHeaders.Get("X-Safe") != "retained" {
			t.Fatal("header filter")
		}
		if limit == 1 && business.Complete {
			t.Fatal("partial body marked complete")
		}
	}
	for _, recorder := range []*diagnosticRecorder{{beginErr: true}, {endErr: true}} {
		c, err := NewDiagnosticClient(cfg, recorder, 8)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Ping(context.Background()); err == nil {
			t.Fatal("observer failure ignored")
		}
		c.CloseIdleConnections()
	}
}
func TestDiagnosticRecordedAllProductionOperations(t *testing.T) {
	values := map[string]string{
		"ping": `{"status":"ok"}`, "clusters": `[]`, "nodes": `{"results":[]}`, "lock": `{"status":false}`,
		"power":      `{"controlPlaneReady":true,"nodesReady":1,"totalNodes":1}`,
		"alert_list": `{"results":[],"totalRecords":0}`, "alert_detail": `{"id":"synthetic"}`,
		"serviceability_log_list": `{"results":[],"totalRecords":0}`, "serviceability_log_detail": `{"id":"synthetic"}`,
	}
	for op, body := range values {
		if _, err := DecodeRecorded(op, "synthetic", []byte(body)); err != nil {
			t.Fatal("recorded decode failed")
		}
	}
	if _, err := DecodeRecorded("unknown", "", nil); err == nil {
		t.Fatal("unknown operation")
	}
	rt := recordedTransport{}
	req, _ := http.NewRequest("POST", "https://invalid.invalid", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("offline method")
	}
}

type diagnosticRoundTripFunc func(*http.Request) (*http.Response, error)

func (f diagnosticRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestDiagnosticNoResponseAndReadFailure(t *testing.T) {
	for _, kind := range []string{"none", "read", "http-error", "observer"} {
		recorder := &diagnosticRecorder{endErr: kind == "observer"}
		rt := &diagnosticTransport{observer: recorder, routes: []Operation{{Collector: "nodes", Path: "/nodes"}}, limits: map[string]int64{"nodes": 100},
			next: diagnosticRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				if kind == "none" {
					return nil, errors.New("transport")
				}
				var body io.Reader = bytes.NewBufferString("{bad")
				if kind == "read" {
					body = io.MultiReader(body, diagnosticReadError{})
				}
				return &http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(body)}, nil
			})}
		req, _ := http.NewRequest("GET", "https://example.invalid/nodes", nil)
		response, _ := rt.RoundTrip(req)
		if response != nil {
			response.Body.Close()
		}
		if len(recorder.values) != 1 {
			t.Fatal("attempt missing")
		}
		if (kind == "none" || kind == "read") && recorder.values[0].Complete {
			t.Fatal("incomplete attempt passed")
		}
		req.URL.Path = "/unknown"
		if _, err := rt.RoundTrip(req); err == nil {
			t.Fatal("unknown route")
		}
	}
}
