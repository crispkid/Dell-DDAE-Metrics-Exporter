package ddae

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Synthetic fixtures exercise AC-DDAE-8-001/002 without retaining field data.
func TestClientClusterResponseCompatibility(t *testing.T) {
	const item = `{"id":"cluster-synthetic","clusterStatus":"Available","coordinator":{"cpu":"2","memory":"4Gi"},"worker":{"cpu":"8","memory":"16Gi"}}`
	for _, tc := range []struct {
		name, body string
		count      int
		bad        bool
	}{
		{"array", `[` + item + `]`, 1, false},
		{"envelope", `{"results":[` + item + `]}`, 1, false},
		{"metadata", " \n" + `{"unknown":{"private":"unused"},"results":[` + item + `]}` + "\n", 1, false},

		{"observed-fields", `{"results":[{"id":"cluster-synthetic","clusterStatus":{"status":"Available","message":"ignored","reason":"ignored"},"coordinator":{"resources":{"cpu":2,"memory":"4Gi"}},"worker":{"resources":{"cpu":8,"memory":"16Gi"}}}]}`, 1, false},
		{"missing-object-status", `[{"id":"c","clusterStatus":{}}]`, 0, true},
		{"null-object-status", `[{"id":"c","clusterStatus":{"status":null}}]`, 0, true},
		{"numeric-object-status", `[{"id":"c","clusterStatus":{"status":1}}]`, 0, true},
		{"boolean-status", `[{"id":"c","clusterStatus":true}]`, 0, true},
		{"array-status", `[{"id":"c","clusterStatus":[]}]`, 0, true},
		{"null-resources", `[{"id":"c","coordinator":{"resources":null}}]`, 0, true},
		{"array-resources", `[{"id":"c","worker":{"resources":[]}}]`, 0, true},
		{"scalar-resources", `[{"id":"c","worker":{"resources":4}}]`, 0, true},
		{"mixed-equal", `[{"id":"c","coordinator":{"cpu":"2","resources":{"cpu":2}}}]`, 0, true},
		{"mixed-conflict", `[{"id":"c","worker":{"memory":"1Gi","resources":{"memory":"2Gi"}}}]`, 0, true},
		{"mixed-null", `[{"id":"c","worker":{"cpu":null,"resources":{}}}]`, 0, true},
		{"nested-fraction-cpu", `[{"id":"c","coordinator":{"resources":{"cpu":2.5}}}]`, 0, true},
		{"nested-exponent-cpu", `[{"id":"c","coordinator":{"resources":{"cpu":2e3}}}]`, 0, true},
		{"negative-cpu", `[{"id":"c","coordinator":{"cpu":-2}}]`, 0, true},
		{"boolean-cpu", `[{"id":"c","coordinator":{"resources":{"cpu":true}}}]`, 0, true},
		{"numeric-memory", `[{"id":"c","worker":{"resources":{"memory":1024}}}]`, 0, true},
		{"empty-array", `[]`, 0, false},
		{"empty-envelope", `{"results":[]}`, 0, false},
		{"missing-results", `{}`, 0, true},
		{"null-results", `{"results":null}`, 0, true},
		{"object-results", `{"results":{}}`, 0, true},
		{"string-results", `{"results":"[]"}`, 0, true},
		{"null", `null`, 0, true},
		{"boolean", `true`, 0, true},
		{"number", `42`, 0, true},
		{"string", `"clusters"`, 0, true},
		{"null-item", `[null]`, 0, true},
		{"envelope-null-item", `{"results":[null]}`, 0, true},
		{"scalar-item", `[42]`, 0, true},
		{"nested-array-item", `{"results":[[]]}`, 0, true},
		{"invalid-item-after-valid", `{"results":[` + item + `,null]}`, 0, true},
		{"wrong-field-type", `{"results":[{"id":42}]}`, 0, true},
		{"integer-cpu", `[` + strings.ReplaceAll(item, `"cpu":"2"`, `"cpu":2`) + `]`, 1, false},
		{"truncated", `{"results":[`, 0, true},
		{"empty-body", ``, 0, true},
		{"html", `<html>not an API</html>`, 0, true},
		{"trailing-object", `{"results":[]} {}`, 0, true},
		{"trailing-null", `[] null`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case tokenPath:
					fmt.Fprint(w, `{"access_token":"synthetic-token","expires_in":3600}`)
				case clustersPath:
					if r.Method != http.MethodGet {
						t.Error("cluster operation must remain GET")
					}
					fmt.Fprint(w, tc.body)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			got, err := client.Clusters(context.Background())
			if (err != nil) != tc.bad {
				t.Fatalf("client error = %v; want failure %v", err, tc.bad)
			}
			recorded, replayErr := decodeClusterResponse([]byte(tc.body))
			if (replayErr != nil) != tc.bad {
				t.Fatalf("replay error = %v; want failure %v", replayErr, tc.bad)
			}
			if tc.bad {
				return
			}
			if !reflect.DeepEqual(got, recorded) || len(got) != tc.count {
				t.Fatalf("live/replay mismatch: live=%#v replay=%#v", got, recorded)
			}
			if tc.count > 0 && (got[0].ID != "cluster-synthetic" || got[0].ClusterStatus != "Available" ||
				got[0].Coordinator.CPU == nil || *got[0].Coordinator.CPU != "2" ||
				got[0].Coordinator.Memory == nil || *got[0].Coordinator.Memory != "4Gi" ||
				got[0].Worker.CPU == nil || *got[0].Worker.CPU != "8" ||
				got[0].Worker.Memory == nil || *got[0].Worker.Memory != "16Gi") {
				t.Fatalf("cluster fields changed: %#v", got)
			}
		})
	}
}

func TestClusterResponseRetainsBodyLimit(t *testing.T) {
	for _, body := range []string{`[]`, `{"results":[]}`} {
		if _, err := decodeClusterResponse([]byte(body + strings.Repeat(" ", int(maxResponseBodyBytes)))); err == nil {
			t.Fatal("oversized cluster response accepted")
		}
	}
}

// AC-DDAE-8-004/005/006: conversion preserves exact integers and optional values.
func TestClusterFieldQuantitiesAndOptionalValues(t *testing.T) {
	for _, tc := range []struct{ cpu, want string }{
		{`0`, "0"}, {`2`, "2"}, {`1234567890123456789`, "1234567890123456789"},
		{`"2500m"`, "2500m"},
	} {
		for _, nested := range []bool{false, true} {
			resources := `{"cpu":` + tc.cpu + `,"memory":"4Gi"}`
			if nested {
				resources = `{"resources":` + resources + `}`
			}
			value, err := decodeClusterResponse([]byte(`[{"id":"c","coordinator":` + resources + `}]`))
			if err != nil {
				t.Fatal(err)
			}
			got := value.([]Cluster)[0].Coordinator
			if got.CPU == nil || *got.CPU != tc.want || got.Memory == nil || *got.Memory != "4Gi" {
				t.Fatalf("resource conversion lost precision or unit: %#v", got)
			}
		}
	}
	for _, body := range []string{
		`[{"id":"c"}]`,
		`[{"id":"c","clusterStatus":null,"coordinator":null}]`,
		`[{"id":"c","coordinator":{"cpu":null,"memory":null}}]`,
		`[{"id":"c","coordinator":{"resources":{}}}]`,
		`[{"id":"c","coordinator":{"resources":{"cpu":null,"memory":null}}}]`,
	} {
		value, err := decodeClusterResponse([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		got := value.([]Cluster)[0]
		if got.ClusterStatus != "" || got.Coordinator.CPU != nil || got.Coordinator.Memory != nil {
			t.Fatalf("missing optional values fabricated: %#v", got)
		}
	}
}

func TestClusterFieldDecodeFailureDoesNotPartiallyOverwrite(t *testing.T) {
	value := Cluster{ID: "original", ClusterStatus: "Available"}
	err := json.Unmarshal([]byte(`{"id":"replacement","clusterStatus":{"status":12}}`), &value)
	if err == nil || value.ID != "original" || value.ClusterStatus != "Available" {
		t.Fatalf("failed decoding changed receiver: %#v, %v", value, err)
	}
}
