package ddae

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The documented fixture is structurally derived from Dell 1.5.0 PDF pages
// 38 and 64-66 and payload SHA-256
// aceaa7fbf6993c31deb570fe698800eb3cbb8ca332bf444b1634978e87106ceb.
// It contains synthetic values only.
func readNodeFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ddae-1.5.0", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestClientNodesAcceptsDocumentedEnvelopeAndLegacyArray(t *testing.T) {
	for _, test := range []struct {
		name      string
		fixture   string
		wantID    string
		wantCPU   string
		wantCount int
	}{
		{name: "documented-envelope", fixture: "nodes-documented-envelope.json", wantID: "node-synthetic-1", wantCPU: "12", wantCount: 1},
		{name: "legacy-array", fixture: "nodes.json", wantID: "node-1", wantCPU: "16", wantCount: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := readNodeFixture(t, test.fixture)
			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case tokenPath:
					fmt.Fprint(writer, `{"access_token":"token","expires_in":3600}`)
				case nodesPath:
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write(body)
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()

			client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), nil))
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			nodes, err := client.Nodes(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(nodes) != test.wantCount || nodes[0].ID != test.wantID || nodes[0].Capacity.CPU == nil || *nodes[0].Capacity.CPU != test.wantCPU {
				t.Fatalf("nodes = %#v", nodes)
			}
		})
	}
}

func TestInfrastructureNodeListRejectsUnsupportedTopLevels(t *testing.T) {
	for _, input := range []string{
		`{}`,
		`{"results":null}`,
		`{"results":{}}`,
		`{"results":[42]}`,
		`{"results":[null]}`,
		`true`,
		`"nodes"`,
	} {
		var nodes infrastructureNodeList
		if err := json.Unmarshal([]byte(input), &nodes); err == nil {
			t.Fatalf("accepted unsupported node response %s", input)
		}
	}
}

func TestInfrastructureNodeListRejectsTrailingJSON(t *testing.T) {
	var nodes infrastructureNodeList
	if err := decodeBounded(strings.NewReader(`{"results":[]} {"results":[]}`), 1024, &nodes); err == nil {
		t.Fatal("accepted trailing node JSON")
	}
}
