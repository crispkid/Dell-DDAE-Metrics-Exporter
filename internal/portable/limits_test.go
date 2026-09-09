package portable

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestPortableLimitsAndCancellation(t *testing.T) {
	for _, kind := range []string{"requests", "body", "cancel"} {
		c, _ := fieldConfig(t, allResponses)
		switch kind {
		case "requests":
			c.Run.MaxRequests = 1
		case "body":
			c.Capture.BodyBytes = 1
		}
		ctx, cancel := context.WithCancel(context.Background())
		if kind == "cancel" {
			cancel()
		}
		_, code := Run(ctx, c, BuildInfo{})
		cancel()
		if kind == "cancel" && code != 130 {
			t.Fatalf("interrupt=%d", code)
		}
		if kind != "cancel" && code != 3 {
			t.Fatalf("limit=%d", code)
		}
	}
	c, _ := fieldConfig(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	c.Checks.Resources = false
	c.Checks.Alerts = false
	c.Checks.Logs = false
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, code := Run(ctx, c, BuildInfo{})
	if code != 130 {
		t.Fatal("slow cancellation")
	}
}
