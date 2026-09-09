package historyscan

import (
	"context"
	"errors"
	"fmt"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/logstate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/querystate"
	"strings"
	"testing"
	"time"
)

// TEST-DDAE-10-002: strict source metadata and the actual server cap.
func TestBackfillPageContracts(t *testing.T) {
	threshold, total := int64(500), int64(720)
	l := ddae.ServiceabilityLogList{Threshold: &threshold, TotalRecords: &total}
	at := "2026-01-01T00:00:00Z"
	l.Results = []ddae.ServiceabilityLogListItem{{ID: "a", UpdatedOn: &at}}
	p, e := logPage(l)
	if e != nil || p.Complete {
		t.Fatal(p, e)
	}
	total = 1
	p, e = logPage(l)
	if e != nil || !p.Complete {
		t.Fatal(p, e)
	}
	for _, bad := range []ddae.ServiceabilityLogList{{}, {Malformed: true}, {Threshold: &threshold, TotalRecords: &total, Results: append(l.Results, l.Results...)}} {
		if _, e := logPage(bad); e == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
	rows := []string{}
	for n := 0; n < 1000; n++ {
		rows = append(rows, fmt.Sprintf(`{"queryId":"q%d","state":"FINISHED","createTime":"2026-01-01T00:00:00Z"}`, n))
	}
	p, e = queryPage([]byte("["+strings.Join(rows, ",")+"]"), 2000)
	if e != nil || p.Complete || len(p.Items) != 1000 {
		t.Fatal(p.Complete, e)
	}
	if _, e = queryPage([]byte("["+rows[0]+","+rows[0]+"]"), 1000); e == nil {
		t.Fatal("duplicate accepted")
	}
	if _, e = queryPage([]byte("["+strings.Join(rows, ",")+"]"), 999); e == nil {
		t.Fatal("local limit ignored")
	}
}

type fakeLogAPI struct {
	list   ddae.ServiceabilityLogList
	detail ddae.ServiceabilityLogDetail
	err    error
}

func (f *fakeLogAPI) ServiceabilityLogWindow(context.Context, time.Time, time.Time) (ddae.ServiceabilityLogList, error) {
	return f.list, f.err
}
func (f *fakeLogAPI) ServiceabilityLogDetail(context.Context, string) (ddae.ServiceabilityLogDetail, error) {
	return f.detail, f.err
}
func (f *fakeLogAPI) CloseIdleConnections() {}

type fakeQueryAPI struct {
	body []byte
	err  error
}

func (f *fakeQueryAPI) Scope(context.Context) error { return f.err }
func (f *fakeQueryAPI) HistoryWindow(context.Context, time.Time, time.Time) ([]byte, error) {
	return f.body, f.err
}
func (f *fakeQueryAPI) Get(context.Context, string) ([]byte, error) { return f.body, f.err }
func (f *fakeQueryAPI) CloseIdleConnections()                       {}
func TestBackfillLogAdapterRecordAndCapacity(t *testing.T) {
	st, e := logstate.Open(logstate.Options{StateDir: t.TempDir(), MaxBytes: 1 << 20, MaxEvents: 1, MaxCheckpoints: 100, Retention: time.Hour})
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	now := time.Now().UTC()
	at := now.Format(time.RFC3339Nano)
	api := &fakeLogAPI{detail: ddae.ServiceabilityLogDetail{ID: "a", UpdatedOn: &at}}
	src := &logSource{client: api, store: st, source: "test"}
	defer src.Close()
	ctx := context.Background()
	item := historystate.Item{ID: "a", When: now}
	if e = src.Record(ctx, item); e != nil {
		t.Fatal(e)
	}
	if e = src.Record(ctx, item); e != nil {
		t.Fatal("dedup", e)
	}
	item.ID = "b"
	api.detail.ID = "b"
	if e = src.Record(ctx, item); !errors.Is(e, historystate.ErrFull) {
		t.Fatal("capacity not reported", e)
	}
	api.detail.ID = "wrong"
	if e = src.Record(ctx, item); e == nil {
		t.Fatal("identity mismatch")
	}
	api.detail.UpdatedOn = nil
	if e = src.Record(ctx, item); e == nil {
		t.Fatal("missing timestamp")
	}
	api.detail.UpdatedOn = &at
	item.When = now.Add(time.Hour)
	if e = src.Record(ctx, item); e == nil {
		t.Fatal("old detail")
	}
	api.err = errors.New("synthetic transport")
	if _, e = src.List(ctx, historystate.Window{}); e == nil {
		t.Fatal("list failure ignored")
	}
	if e = src.Record(ctx, item); e == nil {
		t.Fatal("detail failure ignored")
	}
	api.err = nil
	threshold, total := int64(500), int64(0)
	api.list = ddae.ServiceabilityLogList{Threshold: &threshold, TotalRecords: &total}
	if p, e := src.List(ctx, historystate.Window{}); e != nil || !p.Complete {
		t.Fatal(p, e)
	}
}
func TestBackfillQueryAdapterRetentionAndCapacity(t *testing.T) {
	st, e := querystate.Open(querystate.Options{Dir: t.TempDir(), Source: "test", Retention: time.Hour, MaxEvents: 1, MaxBytes: 1 << 20, MaxCheckpoints: 100, Events: true})
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	now := time.Now().UTC()
	at := now.Format(time.RFC3339Nano)
	body := func(id string) []byte {
		return []byte(fmt.Sprintf(`{"queryId":%q,"state":"FINISHED","user":"synthetic","submissionTime":%q,"completionTime":%q,"elapsedTime":1}`, id, at, at))
	}
	api := &fakeQueryAPI{body: body("q1")}
	src := &querySource{client: api, store: st, source: "test", max: 1000}
	defer src.Close()
	ctx := context.Background()
	item := historystate.Item{ID: "q1", When: now}
	if e = src.Record(ctx, item); e != nil {
		t.Fatal(e)
	}
	if e = src.Record(ctx, item); e != nil {
		t.Fatal(e)
	}
	item.ID = "q2"
	api.body = body("q2")
	if e = src.Record(ctx, item); !errors.Is(e, historystate.ErrFull) {
		t.Fatal(e)
	}
	if e = st.Prune(now.Add(2 * time.Hour)); e != nil {
		t.Fatal(e)
	}
	if e = src.Record(ctx, item); !errors.Is(e, ErrExpired) {
		t.Fatal("retention", e)
	}
	api.body = []byte(`[]`)
	if p, e := src.List(ctx, historystate.Window{}); e != nil || !p.Complete {
		t.Fatal(p, e)
	}
	if e = src.Record(ctx, item); e == nil {
		t.Fatal("invalid detail accepted")
	}
	api.err = errors.New("synthetic forbidden")
	if _, e = src.List(ctx, historystate.Window{}); e == nil {
		t.Fatal("scope ignored")
	}
	if e = src.Record(ctx, item); e == nil {
		t.Fatal("detail failure ignored")
	}
}
