package queryclient

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestQueryTypedBoundaryAndPrivacy(t *testing.T) {
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	sample, err := DecodeOverview([]byte(`{"history":[{"time":"`+stamp+`","metric":{"runningQueries":3,"queuedQueries":2}}]}`), now, time.Minute)
	if err != nil || sample.Running != 3 || sample.Queued != 2 {
		t.Fatal(sample, err)
	}
	for _, s := range []string{`null`, `{}`, `{"history":[]}`, `{"history":[{"time":"` + stamp + `","metric":{"runningQueries":-1,"queuedQueries":0}}]}`} {
		if _, err := DecodeOverview([]byte(s), now, time.Minute); err == nil {
			t.Fatal("invalid overview accepted")
		}
	}
	body := `{"queryId":"q_1","state":"FINISHED","user":"synthetic-user","source":"test","submissionTime":"` + stamp + `","completionTime":"` + stamp + `","elapsedTime":8,"queuedTime":1,"executionTime":7,"queryText":"must-never-export","principal":"private"}`
	e, err := DecodeDetail([]byte(body), "q_1", "test", now)
	if err != nil {
		t.Fatal(err)
	}
	if e.Elapsed == nil || *e.Elapsed != .008 {
		t.Fatal("milliseconds conversion")
	}
	raw, _ := json.Marshal(e)
	if strings.Contains(string(raw), "must-never") || strings.Contains(string(raw), "principal") {
		t.Fatal("privacy")
	}
	for _, field := range []string{`"elapsedTime":-1`, `"elapsedTime":1.5`, `"elapsedTime":"8"`} {
		if _, err := DecodeDetail([]byte(strings.Replace(body, `"elapsedTime":8`, field, 1)), "q_1", "test", now); err == nil {
			t.Fatal("bad duration")
		}
	}
}

func TestQueryHistoryAndMissingData(t *testing.T) {
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	item := `{"queryId":"q","state":"RUNNING","createTime":"` + stamp + `"}`
	if items, err := DecodeHistory([]byte(`[`+item+`]`), 1); err != nil || len(items) != 1 {
		t.Fatal(err)
	}
	for _, body := range []string{`null`, `{}`, `[null]`, `[` + item + `,` + item + `]`, `[{"queryId":"../bad","state":"RUNNING","createTime":"` + stamp + `"}]`, `[] null`} {
		if _, err := DecodeHistory([]byte(body), 1); err == nil {
			t.Fatal("bad history accepted")
		}
	}
	now := time.Now().UTC()
	old := now.Add(-2 * time.Minute).Format(time.RFC3339Nano)
	future := now.Add(time.Minute).Format(time.RFC3339Nano)
	for _, ts := range []string{old, future} {
		if _, err := DecodeOverview([]byte(`{"history":[{"time":"`+ts+`","metric":{"runningQueries":0,"queuedQueries":0}}]}`), now, time.Minute); err == nil {
			t.Fatal("bad sample time")
		}
	}
	body := `{"queryId":"q","state":"RUNNING","user":"synthetic","submissionTime":"` + stamp + `"}`
	e, err := DecodeDetail([]byte(body), "q", "test", now)
	if err != nil || e.Elapsed != nil || Terminal(e.State) {
		t.Fatal(e, err)
	}
	for _, b := range []string{strings.Replace(body, `"RUNNING"`, `"FINISHED"`, 1), strings.Replace(body, `"synthetic"`, `""`, 1), strings.Replace(body, `"q"`, `"wrong"`, 1)} {
		if _, err := DecodeDetail([]byte(b), "q", "test", now); err == nil {
			t.Fatal("invalid detail accepted")
		}
	}
	for _, state := range []string{"FINISHED", "FAILED", "CANCELED", "QUEUED", "RUNNING", "PLANNING", "STARTING", "FINISHING", "WAITING_FOR_RESOURCES", "DISPATCHING", "future"} {
		s := State(state)
		if s == "" {
			t.Fatal("empty state")
		}
	}
}
