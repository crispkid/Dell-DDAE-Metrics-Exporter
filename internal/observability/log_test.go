package observability

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type classifiedError struct{ secret string }

func (e classifiedError) Error() string       { return e.secret }
func (e classifiedError) FailureClass() Class { return ClassAuth }

func TestLogFailureRedactsErrorText(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(&output, "debug", "json")
	LogFailure(logger, "collection failed", "clusters", classifiedError{secret: "password-canary"})
	got := output.String()
	if strings.Contains(got, "password-canary") {
		t.Fatal("log contains secret-derived error text")
	}
	if !strings.Contains(got, `"failure_class":"auth"`) {
		t.Fatalf("log lacks class: %s", got)
	}
}

func TestUnknownErrorIsInternal(t *testing.T) {
	if got := Classify(errors.New("anything")); got != ClassInternal {
		t.Fatalf("class = %q", got)
	}
}
