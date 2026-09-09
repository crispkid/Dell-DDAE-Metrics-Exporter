package ddae

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TEST-DDAE-10-002: fixed GET path and URL-encoded typed UTC bounds.
func TestBackfillLogWindowTransport(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			fmt.Fprint(w, `{"access_token":"synthetic","expires_in":3600}`)
			return
		}
		calls++
		if r.Method != "GET" || r.URL.Path != "/rest/v1/serviceability-events" || len(r.URL.Query()) != 1 {
			t.Error("unexpected operation")
		}
		want := `(updatetime ge "2026-01-01T00:00:00Z") and (updatetime le "2026-01-01T00:00:02Z")`
		if r.URL.Query().Get("filter") != want {
			t.Error(r.URL.Query())
		}
		fmt.Fprint(w, `{"threshold":500,"totalRecords":0,"results":[]}`)
	}))
	defer srv.Close()
	c, e := NewClient(clientConfig(t, srv.URL, trustedServerCA(t, srv), map[string]string{"DDAE_API_PATH_PREFIX": "/rest/v1"}))
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseIdleConnections()
	start, _ := time.Parse(time.RFC3339Nano, "2026-01-01T00:00:00.5Z")
	if _, e = c.ServiceabilityLogWindow(context.Background(), start, start.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	if _, e = c.ServiceabilityLogWindow(context.Background(), start, start); e == nil {
		t.Fatal("invalid bounds accepted")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
