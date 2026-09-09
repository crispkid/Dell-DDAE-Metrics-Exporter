package portable

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func allResponses(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ping":
		w.Write([]byte(`{"status":"ok"}`))
	case "/v1/ddae-clusters":
		w.Write([]byte(`[]`))
	case "/v1/infrastructure-nodes":
		w.Write([]byte(`{"results":[]}`))
	case "/v1/system-lock":
		w.Write([]byte(`{"status":false}`))
	case "/v1/system-shutdown":
		w.Write([]byte(`{"controlPlaneReady":true,"nodesReady":1,"totalNodes":1}`))
	case "/v1/serviceability-issues", "/v1/serviceability-events":
		w.Write([]byte(`{"results":[{"id":"synthetic-a"}],"totalRecords":1}`))
	default:
		if strings.HasSuffix(r.URL.Path, "/synthetic-a") {
			w.Write([]byte(`{"id":"synthetic-a"}`))
		} else {
			w.WriteHeader(404)
		}
	}
}

func TestPortableRunnerRetryAccountingAndDetailRotation(t *testing.T) {
	for _, status := range []int{401, 429, 503} {
		var attempts atomic.Int32
		c, _ := fieldConfig(t, func(w http.ResponseWriter, r *http.Request) {
			if attempts.Add(1) == 1 {
				w.WriteHeader(status)
				return
			}
			allResponses(w, r)
		})
		c.Checks.Resources = false
		c.Checks.Alerts = false
		c.Checks.Logs = false
		dir, code := Run(context.Background(), c, BuildInfo{})
		if code != 0 {
			t.Fatalf("retry status=%d code=%d", status, code)
		}
		data, _ := os.ReadFile(filepath.Join(dir, "report.json"))
		var r Report
		if json.Unmarshal(data, &r) != nil {
			t.Fatal("report")
		}
		want := 3
		if status == 401 {
			want = 4
		}
		if len(r.Steps) != want || attempts.Load() != 2 {
			t.Fatal("authentication or retry not counted")
		}
	}
	var details []string
	c, _ := fieldConfig(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/serviceability-issues" {
			w.Write([]byte(`{"results":[{"id":"b"},{"id":"a"}],"totalRecords":2}`))
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/serviceability-issues/")
		details = append(details, id)
		w.Write([]byte(`{"id":"` + id + `"}`))
	})
	c.Checks.Ping = false
	c.Checks.Resources = false
	c.Checks.Logs = false
	c.Run.MaxDetails = 1
	c.Duration = 1500 * time.Millisecond
	if _, code := Run(context.Background(), c, BuildInfo{}); code != 0 {
		t.Fatal("rotation failed")
	}
	if len(details) != 2 || details[0] != "a" || details[1] != "b" {
		t.Fatal("details not rotated")
	}
}
func TestPortableRunnerReadOnlyAllChecks(t *testing.T) {
	calls := []string{}
	c, _ := fieldConfig(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.RawQuery != "" {
			t.Error("unsafe business request")
		}
		calls = append(calls, r.URL.Path)
		allResponses(w, r)
	})
	path, code := Run(context.Background(), c, BuildInfo{Version: "synthetic"})
	if code != 0 {
		t.Fatalf("run code=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(path, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if json.Unmarshal(data, &report) != nil || !report.Complete || len(calls) != 9 {
		t.Fatalf("unexpected run: calls=%d", len(calls))
	}
}
func TestPortableDisabledAndFailure(t *testing.T) {
	c, _ := fieldConfig(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Error("disabled request")
		}
		w.WriteHeader(404)
	})
	c.Checks.Resources = false
	c.Checks.Alerts = false
	c.Checks.Logs = false
	_, code := Run(context.Background(), c, BuildInfo{})
	if code != 1 {
		t.Fatalf("404 exit=%d", code)
	}
}
