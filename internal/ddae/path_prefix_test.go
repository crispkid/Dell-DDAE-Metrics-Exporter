package ddae

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConfiguredPathPrefixesDriveEveryOperation(t *testing.T) {
	for _, test := range []struct {
		name       string
		pingPrefix string
		apiPrefix  string
	}{
		{name: "new-default", pingPrefix: "", apiPrefix: "/v1"},
		{name: "rc2-compatible", pingPrefix: "/rest/v1", apiPrefix: "/rest/v1"},
		{name: "pdf-compatible", pingPrefix: "/rest", apiPrefix: "/rest/v1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertConfiguredOperationMatrix(t, test.pingPrefix, test.apiPrefix)
			exerciseConfiguredRoutes(t, test.pingPrefix, test.apiPrefix)
		})
	}
}

func assertConfiguredOperationMatrix(t *testing.T, pingPrefix, apiPrefix string) {
	t.Helper()
	operations, err := ApprovedOperationsForPrefixes(pingPrefix, apiPrefix)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"ping":                      pingPrefix + "/ping",
		"clusters":                  apiPrefix + "/ddae-clusters",
		"nodes":                     apiPrefix + "/infrastructure-nodes",
		"lock":                      apiPrefix + "/system-lock",
		"power":                     apiPrefix + "/system-shutdown",
		"alert_list":                apiPrefix + "/serviceability-issues",
		"alert_detail":              apiPrefix + "/serviceability-issues/{id}",
		"serviceability_log_list":   apiPrefix + "/serviceability-events",
		"serviceability_log_detail": apiPrefix + "/serviceability-events/{id}",
	}
	if len(operations) != len(want) {
		t.Fatalf("operation count = %d, want %d", len(operations), len(want))
	}
	for _, operation := range operations {
		if operation.Method != http.MethodGet || want[operation.Collector] != operation.Path {
			t.Fatalf("configured operation = %#v, want path %q", operation, want[operation.Collector])
		}
	}
}

func exerciseConfiguredRoutes(t *testing.T, pingPrefix, apiPrefix string) {
	t.Helper()
	operations, err := ApprovedOperationsForPrefixes(pingPrefix, apiPrefix)
	if err != nil {
		t.Fatal(err)
	}
	collectorByPath := make(map[string]string, len(operations))
	for _, operation := range operations {
		path := operation.Path
		switch operation.Collector {
		case "alert_detail":
			path = strings.Replace(path, "{id}", "alert-1", 1)
		case "serviceability_log_detail":
			path = strings.Replace(path, "{id}", "log-1", 1)
		}
		if operation.Method != http.MethodGet {
			t.Fatalf("operation method = %#v", operation)
		}
		collectorByPath[path] = operation.Collector
	}

	var tokenCalls atomic.Int32
	var mu sync.Mutex
	seen := make(map[string]int)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == tokenPath {
			tokenCalls.Add(1)
			if request.Method != http.MethodPost {
				t.Errorf("token method = %s", request.Method)
			}
			fmt.Fprint(writer, `{"access_token":"token","expires_in":3600}`)
			return
		}
		if request.Method != http.MethodGet {
			t.Errorf("Management API method = %s", request.Method)
		}
		mu.Lock()
		seen[request.URL.Path]++
		mu.Unlock()
		switch collectorByPath[request.URL.Path] {
		case "ping":
			fmt.Fprint(writer, `{"status":"ok"}`)
		case "clusters", "nodes":
			fmt.Fprint(writer, `[]`)
		case "lock":
			fmt.Fprint(writer, `{"status":"unlocked"}`)
		case "power":
			fmt.Fprint(writer, `{"controlPlaneReady":true,"nodesReady":1,"totalNodes":1}`)
		case "alert_list":
			fmt.Fprint(writer, `{"results":[{"id":"alert-1"}],"totalRecords":1}`)
		case "alert_detail":
			fmt.Fprint(writer, `{"id":"alert-1","type":"warning"}`)
		case "serviceability_log_list":
			fmt.Fprint(writer, `{"results":[{"id":"log-1"}],"totalRecords":1}`)
		case "serviceability_log_detail":
			fmt.Fprint(writer, `{"id":"log-1","type":"info"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), map[string]string{
		"DDAE_PING_PATH_PREFIX": pingPrefix,
		"DDAE_API_PATH_PREFIX":  apiPrefix,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	ctx := context.Background()
	if _, err := client.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Clusters(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Nodes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Lock(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Power(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AlertList(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AlertDetail(ctx, "alert-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ServiceabilityLogList(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ServiceabilityLogDetail(ctx, "log-1"); err != nil {
		t.Fatal(err)
	}

	if tokenCalls.Load() != 1 {
		t.Fatalf("token calls = %d", tokenCalls.Load())
	}
	for path := range collectorByPath {
		if seen[path] != 1 {
			t.Errorf("path %q seen %d times", path, seen[path])
		}
	}
	if len(seen) != len(collectorByPath) {
		t.Fatalf("unexpected paths = %v", seen)
	}
}
