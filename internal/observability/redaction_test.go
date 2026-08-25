package observability

import (
	"bytes"
	"strings"
	"testing"
)

func TestFailureFieldsAllowOnlyComponentAndBoundedClass(t *testing.T) {
	canary := "bearer-token-and-private-message-canary"
	fields := FailureFields("alert_detail", classifiedError{secret: canary})
	if len(fields) != 4 || fields[0] != "component" || fields[1] != "alert_detail" || fields[2] != "failure_class" || fields[3] != ClassAuth {
		t.Fatalf("failure fields = %#v", fields)
	}
	var output bytes.Buffer
	LogFailure(NewLogger(&output, "info", "text"), "bounded failure", "alert_detail", classifiedError{secret: canary})
	if strings.Contains(output.String(), canary) {
		t.Fatalf("redaction canary leaked: %s", output.String())
	}
}
