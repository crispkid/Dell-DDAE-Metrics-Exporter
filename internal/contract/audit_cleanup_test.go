package contract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TEST-DDAE-12-009: remove unused fields, retaining the actual runtime/build contracts.
func TestAuditUnusedFieldsAndDuplicateConfigurationRemoved(t *testing.T) {
	for _, tc := range []struct{ path, typ, field string }{
		{"internal/ddae/auth.go", "tokenRefreshResult", "refreshAt"},
		{"internal/server/server.go", "Server", "state"},
		{"internal/server/server.go", "Server", "staleAfter"},
		{"internal/app/app.go", "BuildInfo", "Revision"},
		{"internal/app/app.go", "BuildInfo", "BuildDate"},
		{"internal/historyscan/scan_test.go", "testSource", "fail"},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "../../"+tc.path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			s, ok := n.(*ast.TypeSpec)
			if !ok || s.Name.Name != tc.typ {
				return true
			}
			st, ok := s.Type.(*ast.StructType)
			if !ok {
				t.Fatal("expected struct")
			}
			for _, f := range st.Fields.List {
				for _, name := range f.Names {
					if name.Name == tc.field {
						t.Errorf("unused %s.%s remains", tc.typ, tc.field)
					}
				}
			}
			return false
		})
	}
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile("../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if strings.Contains(read("internal/alerts/event.go"), "var _ = fmt.Sprintf") {
		t.Error("dummy fmt reference remains")
	}
	if strings.Count(read("internal/config/config.go"), `boolean(lookup, "ALLOW_INSECURE_TLS"`) != 1 {
		t.Error("TLS opt-in must be loaded once")
	}
	if !strings.Contains(read("internal/ddae/auth.go"), "m.refreshAt = refreshAt") {
		t.Fatal("real token refresh removed")
	}
	for _, path := range []string{"scripts/build.sh", "Dockerfile", "scripts/reproducible-build.sh"} {
		for _, symbol := range []string{"main.version", "main.revision", "main.buildDate"} {
			if !strings.Contains(read(path), symbol) {
				t.Errorf("missing linker input %s in %s", symbol, path)
			}
		}
	}
}
