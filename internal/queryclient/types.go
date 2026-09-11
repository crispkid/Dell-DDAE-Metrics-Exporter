package queryclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrSchema = errors.New("query response schema invalid")
var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

func ValidID(id string) bool { return safeID.MatchString(id) }
func decode(body []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(body))
	if err := d.Decode(v); err != nil {
		return ErrSchema
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return ErrSchema
	}
	return nil
}

type Sample struct {
	At              time.Time
	Running, Queued int64
}

func DecodeOverview(body []byte, now time.Time, stale time.Duration) (Sample, error) {
	var data struct {
		History []struct {
			Time   time.Time `json:"time"`
			Metric *struct {
				Running *int64 `json:"runningQueries"`
				Queued  *int64 `json:"queuedQueries"`
			} `json:"metric"`
		} `json:"history"`
	}
	if decode(body, &data) != nil || len(data.History) == 0 {
		return Sample{}, ErrSchema
	}
	var result Sample
	for _, s := range data.History {
		if s.Time.IsZero() || s.Time.After(now.Add(5*time.Second)) || s.Metric == nil || s.Metric.Running == nil || s.Metric.Queued == nil || *s.Metric.Running < 0 || *s.Metric.Queued < 0 {
			return Sample{}, ErrSchema
		}
		if s.Time.After(result.At) {
			result = Sample{s.Time, *s.Metric.Running, *s.Metric.Queued}
		}
	}
	if now.Sub(result.At) > stale {
		return Sample{}, errors.New("query sample stale")
	}
	return result, nil
}

type HistoryItem struct {
	ID      string    `json:"queryId"`
	State   string    `json:"state"`
	Created time.Time `json:"createTime"`
}

func DecodeHistory(body []byte, max int) ([]HistoryItem, error) {
	var items []HistoryItem
	if decode(body, &items) != nil || items == nil || len(items) > max {
		return nil, ErrSchema
	}
	seen := map[string]bool{}
	for _, item := range items {
		if !ValidID(item.ID) || seen[item.ID] || item.State == "" || item.Created.IsZero() {
			return nil, ErrSchema
		}
		seen[item.ID] = true
	}
	return items, nil
}

type Event struct {
	SourceInstance string     `json:"source_instance"`
	QueryID        string     `json:"query_id"`
	User           string     `json:"user"`
	Source         string     `json:"source,omitempty"`
	State          string     `json:"state"`
	Submitted      time.Time  `json:"submitted_at"`
	Completed      *time.Time `json:"completed_at,omitempty"`
	Observed       time.Time  `json:"observed_at"`
	Elapsed        *float64   `json:"elapsed_seconds,omitempty"`
	Queued         *float64   `json:"queued_seconds,omitempty"`
	Execution      *float64   `json:"execution_seconds,omitempty"`
	CPU            *float64   `json:"cpu_seconds,omitempty"`
}

func Terminal(s string) bool { return s == "finished" || s == "failed" || s == "canceled" }
func State(s string) string {
	switch strings.ToUpper(s) {
	case "FINISHED":
		return "finished"
	case "FAILED":
		return "failed"
	case "CANCELED", "CANCELLED":
		return "canceled"
	case "QUEUED":
		return "queued"
	case "RUNNING":
		return "running"
	case "PLANNING":
		return "planning"
	case "STARTING":
		return "starting"
	case "FINISHING":
		return "finishing"
	case "WAITING_FOR_RESOURCES":
		return "waiting_for_resources"
	case "DISPATCHING":
		return "dispatching"
	default:
		return "unknown"
	}
}
func textValid(s string, required bool) bool {
	return (!required || s != "") && len(s) <= 1024 && utf8.ValidString(s) && !strings.ContainsAny(s, "\x00\r\n")
}
func DecodeDetail(body []byte, id, source string, now time.Time) (Event, error) {
	return decodeDetail(body, id, source, now, now)
}

func decodeDetail(body []byte, id, source string, observed, now time.Time) (Event, error) {
	var wire struct {
		ID        string     `json:"queryId"`
		State     string     `json:"state"`
		User      string     `json:"user"`
		Source    string     `json:"source"`
		Submitted time.Time  `json:"submissionTime"`
		Completed *time.Time `json:"completionTime"`
		Elapsed   *int64     `json:"elapsedTime"`
		Queued    *int64     `json:"queuedTime"`
		Execution *int64     `json:"executionTime"`
		CPU       *int64     `json:"cpuTime"`
	}
	if decode(body, &wire) != nil || wire.ID != id || !ValidID(id) || !textValid(wire.User, true) || !textValid(wire.Source, false) || wire.State == "" || wire.Submitted.IsZero() || wire.Submitted.After(now.Add(5*time.Second)) {
		return Event{}, ErrSchema
	}
	if wire.Completed != nil && (wire.Completed.Before(wire.Submitted) || wire.Completed.After(now.Add(5*time.Second))) {
		return Event{}, ErrSchema
	}
	e := Event{SourceInstance: source, QueryID: id, User: wire.User, Source: wire.Source, State: State(wire.State), Submitted: wire.Submitted, Completed: wire.Completed, Observed: observed}
	if Terminal(e.State) && (e.Completed == nil || wire.Elapsed == nil) {
		return Event{}, ErrSchema
	}
	for _, v := range []struct {
		in  *int64
		out **float64
	}{{wire.Elapsed, &e.Elapsed}, {wire.Queued, &e.Queued}, {wire.Execution, &e.Execution}, {wire.CPU, &e.CPU}} {
		if v.in != nil {
			if *v.in < 0 || *v.in > 9007199254740991 {
				return Event{}, ErrSchema
			}
			n := float64(*v.in) / 1000
			*v.out = &n
		}
	}
	return e, nil
}
