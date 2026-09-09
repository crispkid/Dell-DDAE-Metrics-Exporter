package queries

import (
	"context"
	"errors"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/queryclient"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/querystate"
	"github.com/prometheus/client_golang/prometheus"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQueryMetricsStalenessAndScope(t *testing.T) {
	p := &Pipeline{cfg: config.QueryConfig{StaleAfter: time.Minute}, sample: queryclient.Sample{At: time.Now(), Running: 3, Queued: 2}, scope: true, overviewOK: true, detailOK: true, stateOK: true}
	reg := prometheus.NewRegistry()
	reg.MustRegister(p)
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range families {
		if f.GetName() == "ddae_queries_queued" {
			found = true
			if f.Metric[0].Gauge.GetValue() != 2 {
				t.Fatal("queue")
			}
		}
	}
	if !found {
		t.Fatal("missing queue")
	}
	p.sample.At = time.Now().Add(-2 * time.Minute)
	if p.Ready() {
		t.Fatal("stale ready")
	}
	families, _ = reg.Gather()
	for _, f := range families {
		if f.GetName() == "ddae_queries_running" {
			t.Fatal("stale gauge emitted")
		}
	}
}

type fakeAPI struct {
	now        time.Time
	failDetail bool
	scopeErr   bool
	calls      int
}

func (f *fakeAPI) CloseIdleConnections() {}
func (f *fakeAPI) Scope(context.Context) error {
	if f.scopeErr {
		return errors.New("scope denied")
	}
	return nil
}
func (f *fakeAPI) Get(_ context.Context, path string) ([]byte, error) {
	f.calls++
	stamp := f.now.Format(time.RFC3339Nano)
	switch path {
	case "/ui/api/insights/overview/queries":
		return []byte(`{"history":[{"time":"` + stamp + `","metric":{"runningQueries":4,"queuedQueries":2}}]}`), nil
	case "/ui/api/insights/history/queries":
		return []byte(`[{"queryId":"q1","state":"FINISHED","createTime":"` + stamp + `"}]`), nil
	default:
		if f.failDetail {
			return nil, errors.New("detail unavailable")
		}
		return []byte(`{"queryId":"q1","state":"FINISHED","user":"synthetic-user","submissionTime":"` + stamp + `","completionTime":"` + stamp + `","elapsedTime":8,"queuedTime":1,"executionTime":7,"queryText":"private-sql"}`), nil
	}
}

type fakeSender struct {
	fail    bool
	payload []byte
}

func (f *fakeSender) Close() {}
func (f *fakeSender) PublishQuery(_ context.Context, key, payload []byte) error {
	if len(key) != 32 {
		return errors.New("invalid key")
	}
	if f.fail {
		return errors.New("broker unavailable")
	}
	f.payload = append([]byte(nil), payload...)
	return nil
}
func TestQueryPollPublishFailureAndRecovery(t *testing.T) {
	now := time.Now().UTC()
	store, err := querystate.Open(querystate.Options{Dir: filepath.Join(t.TempDir(), "state"), Source: "test", MaxEvents: 10, MaxBytes: 1 << 20, MaxCheckpoints: 100, Retention: time.Hour, Events: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	client := &fakeAPI{now: now}
	send := &fakeSender{fail: true}
	p := &Pipeline{cfg: config.QueryConfig{StaleAfter: time.Minute, CycleTimeout: time.Second, MaxHistory: 10, MaxPerCycle: 10, Concurrency: 1, Events: true}, source: "test", client: client, store: store, producer: send}
	p.Poll(context.Background())
	p.flush(context.Background())
	if p.Ready() {
		t.Fatal("failed publish was ready")
	}
	a, _ := store.Stats()
	if a.Completed["finished"] != 1 || a.Pending != 1 {
		t.Fatal(a)
	}
	p.Poll(context.Background())
	a, _ = store.Stats()
	if a.Completed["finished"] != 1 {
		t.Fatal("duplicate counted")
	}
	send.fail = false
	p.flush(context.Background())
	if !p.Ready() {
		t.Fatal("recovery not ready")
	}
	if strings.Contains(string(send.payload), "private-sql") || !strings.Contains(string(send.payload), "synthetic-user") {
		t.Fatal("Kafka field contract")
	}
	calls := client.calls
	reg := prometheus.NewRegistry()
	reg.MustRegister(p)
	reg.Gather()
	if client.calls != calls {
		t.Fatal("scrape made API call")
	}
	client.scopeErr = true
	p.Poll(context.Background())
	if p.Ready() {
		t.Fatal("scope failure ready")
	}
	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "ddae_queries_running" {
			t.Fatal("scope failure exposed cluster gauge")
		}
	}
}

func TestQueryRuntimeLifecycleAndDetailFailure(t *testing.T) {
	for k, v := range map[string]string{"QUERY_ENABLED": "true", "QUERY_BASE_URL": "https://engine.invalid", "QUERY_AUTH_URL": "https://auth.invalid", "QUERY_REALM": "ddae", "QUERY_ROLE": "monitoring", "QUERY_USERNAME": "synthetic", "QUERY_PASSWORD": "synthetic", "DDAE_RESOURCE_MONITORING_ENABLED": "false", "DDAE_ALERT_MONITORING_ENABLED": "false", "DDAE_SOURCE_INSTANCE": "test", "STATE_DIR": t.TempDir()} {
		t.Setenv(k, v)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := p.client
	p.client = &fakeAPI{now: time.Now().UTC(), failDetail: true}
	original.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	p.Run(ctx)
	if p.Ready() || !p.overviewOK || p.detailOK {
		t.Fatal("detail failure masked overview or readiness")
	}
	p.client = &fakeAPI{now: time.Now().UTC()}
	p.Poll(context.Background())
	if !p.Ready() {
		t.Fatal("detail recovery failed")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(cfg, nil)
	if err != nil {
		t.Fatal("state not closed", err)
	}
	reopened.Close()
	bad := cfg
	bad.Query.BaseURL = nil
	if _, err := New(bad, nil); err == nil {
		t.Fatal("bad client accepted")
	}
	bad = cfg
	bad.StateDir = "relative"
	if _, err := New(bad, nil); err == nil {
		t.Fatal("bad state accepted")
	}
}
