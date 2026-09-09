package contract

import (
	"os"
	"strings"
	"testing"
)

func TestPortableNormalExporterCaptureIsolation(t *testing.T) {
	for _, path := range []string{"../../cmd/ddae-exporter/main.go", "../app/app.go", "../config/yaml.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"NewDiagnosticClient", "internal/portable", "recipient_public_key_file", "ddaecap"} {
			if strings.Contains(string(data), forbidden) {
				t.Fatal("normal exporter gained a capture activation path")
			}
		}
	}
}
