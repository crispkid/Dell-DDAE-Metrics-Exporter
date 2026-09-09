package queryclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// TEST-DDAE-10-002: strict operation and typed JSON time filter.
func TestBackfillQueryWindowTransport(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/ui/api/insights/history/queries" || len(r.URL.Query()) != 3 || r.URL.Query().Get("sortBy") != "createDate" || r.URL.Query().Get("sortOrder") != "desc" {
			t.Error("unexpected operation")
		}
		var f map[string]string
		if json.Unmarshal([]byte(r.URL.Query().Get("filter")), &f) != nil || len(f) != 2 || f["startDate"] != "2026-01-01T00:00:00Z" || f["endDate"] != "2026-01-01T00:00:02Z" {
			t.Error(f)
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	c := &Client{base: u, http: srv.Client(), cfg: config.QueryConfig{ResponseMaxBytes: 1 << 20}}
	start, _ := time.Parse(time.RFC3339Nano, "2026-01-01T00:00:00.5Z")
	if _, e := c.HistoryWindow(context.Background(), start, start.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	if _, e := c.HistoryWindow(context.Background(), start, start); e == nil {
		t.Fatal("invalid bounds")
	}
	if _, e := c.Get(context.Background(), "/ui/api/insights/history/queries?filter=unsafe"); e == nil {
		t.Fatal("arbitrary filter accepted")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
