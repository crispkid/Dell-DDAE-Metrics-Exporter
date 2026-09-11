package queryclient

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type auditGetter func(context.Context, string) ([]byte, error)

func (f auditGetter) Get(ctx context.Context, path string) ([]byte, error) { return f(ctx, path) }

// TEST-DDAE-12-006: deterministic elapsed-request clock, no wall-clock sleeps.
func TestAuditDetailUsesResponseTimeAndPreservesObservation(t *testing.T) {
	start := time.Unix(1000, 0).UTC()
	for _, future := range []time.Duration{0, 5 * time.Second, 5*time.Second + time.Nanosecond} {
		now := start
		api := auditGetter(func(_ context.Context, path string) ([]byte, error) {
			if path != "/ui/api/insights/history/queries/q1" {
				t.Fatal("unexpected detail route")
			}
			now = start.Add(10 * time.Second)
			return []byte(fmt.Sprintf(`{"queryId":"q1","state":"FINISHED","user":"synthetic","submissionTime":%q,"completionTime":%q,"elapsedTime":10000}`, start.Format(time.RFC3339Nano), now.Add(future).Format(time.RFC3339Nano))), nil
		})
		event, err := fetchDetail(context.Background(), api, "q1", "test", func() time.Time { return now })
		if future > 5*time.Second {
			if !errors.Is(err, ErrSchema) {
				t.Fatalf("future accepted: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !event.Observed.Equal(start) {
			t.Fatal("response arrival replaced observation ordering")
		}
	}
	want := errors.New("synthetic transport")
	if _, err := fetchDetail(context.Background(), auditGetter(func(context.Context, string) ([]byte, error) { return nil, want }), "q1", "test", func() time.Time { return start }); !errors.Is(err, want) {
		t.Fatal("transport error lost")
	}
}
