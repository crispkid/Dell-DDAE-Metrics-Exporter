package ddae

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

func TestDecodeBoundedRejectsOversizeAndTrailingJSON(t *testing.T) {
	var value PingResponse
	if err := decodeBounded(strings.NewReader(`{"status":"ok"}`), 4, &value); err == nil {
		t.Fatal("expected size error")
	}
	if err := decodeBounded(strings.NewReader(`{"status":"ok"} {}`), 64, &value); err == nil {
		t.Fatal("expected trailing JSON error")
	}
}

func TestDecodeBoundedAcceptsExactLimitAndRejectsInvalidLimits(t *testing.T) {
	payload := []byte(`{"status":"ok"}`)
	var value PingResponse
	if err := decodeBounded(bytes.NewReader(payload), int64(len(payload)), &value); err != nil || value.Status != "ok" {
		t.Fatalf("exact limit value=%#v err=%v", value, err)
	}
	for _, limit := range []int64{0, maxResponseBodyBytes + 1, int64(^uint64(0) >> 1)} {
		if err := decodeBounded(bytes.NewReader(payload), limit, &value); err == nil {
			t.Fatalf("invalid limit %d accepted", limit)
		}
	}
}

func TestTransportRejectsResponseHeadersAboveOneMiB(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			fmt.Fprint(w, `{"access_token":"token","expires_in":3600}`)
			return
		}
		w.Header().Set("X-Oversized", strings.Repeat("x", maxResponseHeaderBytes+1))
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()
	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	_, err = client.Ping(context.Background())
	if err == nil || observability.Classify(err) != observability.ClassTransport {
		t.Fatalf("oversized header error=%v class=%s", err, observability.Classify(err))
	}
}

func TestDecodeBoundedRejectsMalformedTruncatedAndDeepJSON(t *testing.T) {
	for _, input := range []string{``, `{"status":`, `{"status":"ok"} garbage`} {
		var value PingResponse
		if err := decodeBounded(strings.NewReader(input), 128, &value); err == nil {
			t.Fatalf("accepted malformed input %q", input)
		}
	}
	deep := bytes.Repeat([]byte("["), 10001)
	deep = append(deep, bytes.Repeat([]byte("]"), 10001)...)
	var destination any
	if err := decodeBounded(bytes.NewReader(deep), int64(len(deep)+1), &destination); err == nil {
		t.Fatal("accepted JSON beyond decoder nesting limit")
	}
}
