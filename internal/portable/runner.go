package portable

import (
	"context"
	"errors"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"path/filepath"
	"time"
)

type runObserver struct {
	output        *reports
	capture       *CaptureWriter
	requests, max int
	latest        Step
	failure       error
	partial       bool
}

func (o *runObserver) Begin(string) (int, error) {
	if o.failure != nil {
		return 0, o.failure
	}
	if o.requests >= o.max {
		o.failure = ErrLimit
		return 0, ErrLimit
	}
	o.requests++
	return o.requests, nil
}
func (o *runObserver) Observe(e Exchange) error {
	s := Step{RequestID: e.ID, Operation: e.Operation, Started: e.Started, Milliseconds: e.Milliseconds, HTTPStatus: e.Status, HTTP: e.Status >= 200 && e.Status < 300, CaptureComplete: e.Complete, Reason: e.Reason}
	if e.Operation != "token" {
		if !e.Complete {
			o.partial = true
		}
		if s.HTTP && e.Complete {
			p, _ := Parse(e.Operation, e.RequestedID, e.Body)
			s.Parse = &p
		}
	}
	if e.Operation != "token" && o.capture != nil {
		if err := o.capture.Append(e); err != nil {
			o.failure = err
			s.CaptureComplete = false
			s.Reason = "capture_storage_or_limit"
		}
	}
	o.latest = s
	if err := o.output.step(s); err != nil {
		o.failure = err
		return err
	}
	return o.failure
}

// Run executes only the selected read-only API checks; no publisher or durable
// exporter state is initialized. Caller owns the cancellation context.
func Run(parent context.Context, c Config, build BuildInfo) (string, int) {
	if platformReady() != nil {
		return "", 2
	}
	out, err := newReports(c.Output.Directory, "field", build, c.Output.MaxBytes)
	if err != nil {
		return "", 2
	}
	out.report.InsecureTLS = c.DDAE.TLS.Insecure
	observer := &runObserver{output: out, max: c.Run.MaxRequests}
	if c.Capture.Enabled {
		key, e := ReadPublicKey(c.Capture.PublicKey)
		if e == nil {
			observer.capture, e = NewCapture(filepath.Join(out.dir, "http-capture.ddaecap"), key, c.Capture.TotalBytes)
		}
		if e != nil {
			_ = out.finish(2, false)
			return out.dir, 2
		}
	}
	client, err := ddae.NewDiagnosticClient(c.Client, observer, c.Capture.BodyBytes)
	if err != nil {
		if observer.capture != nil {
			_ = observer.capture.Finish(false)
		}
		_ = out.finish(2, false)
		return out.dir, 2
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(parent, c.Duration)
	defer cancel()
	failed := false
	add := func(check Check) {
		if err := out.check(check); err != nil {
			observer.failure = err
		}
	}
	enabled := map[string]bool{"ping": c.Checks.Ping, "clusters": c.Checks.Resources, "nodes": c.Checks.Resources, "lock": c.Checks.Resources, "power": c.Checks.Resources, "alert_list": c.Checks.Alerts, "serviceability_log_list": c.Checks.Logs}
	offsets := map[string]int{}
	inspect := func(op, id string) (ParseResult, bool) {
		value, err := client.Inspect(ctx, op, id)
		if err != nil && ctx.Err() != nil {
			observer.partial = true
		}
		_ = value
		good := err == nil && observer.latest.Operation == op && observer.latest.Parse != nil
		p := ParseResult{}
		if good {
			p = *observer.latest.Parse
			good = p.DecodeOK && p.ValidationOK && p.ContractOK
		}
		status, reason := "PASS", ""
		if !good {
			status = "FAIL"
			reason = "parser_or_contract"
			failed = true
			if err != nil {
				reason = string(observability.Classify(err))
			}
		}
		add(Check{Operation: op, Status: status, Reason: reason})
		return p, good
	}
	for _, op := range []string{"ping", "clusters", "nodes", "lock", "power", "alert_list", "serviceability_log_list"} {
		if !enabled[op] {
			add(Check{Operation: op, Status: "SKIP", Reason: "disabled"})
		}
	}
loop:
	for ctx.Err() == nil && observer.failure == nil {
		for _, op := range []string{"ping", "clusters", "nodes", "lock", "power", "alert_list", "serviceability_log_list"} {
			if !enabled[op] {
				continue
			}
			if ctx.Err() != nil || observer.failure != nil {
				break loop
			}
			p, _ := inspect(op, "")
			detail := ""
			if op == "alert_list" {
				detail = "alert_detail"
			}
			if op == "serviceability_log_list" {
				detail = "serviceability_log_detail"
			}
			if detail == "" {
				continue
			}
			n := min(len(p.IDs), c.Run.MaxDetails)
			okCount := 0
			for j := 0; j < n; j++ {
				if ctx.Err() != nil || observer.failure != nil {
					break loop
				}
				id := p.IDs[(offsets[op]+j)%len(p.IDs)]
				_, ok := inspect(detail, id)
				if ok {
					okCount++
				}
			}
			if len(p.IDs) > 0 {
				offsets[op] = (offsets[op] + n) % len(p.IDs)
			}
			reason := "selected_subset"
			status := "PASS"
			if n == 0 {
				status = "SKIP"
				reason = "no_selected_details"
			} else if okCount < n {
				status = "FAIL"
			}
			add(Check{Operation: detail, Status: status, Reason: reason, Available: len(p.IDs), Selected: n, Successful: okCount})
		}
		timer := time.NewTimer(c.Interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			break loop
		case <-timer.C:
		}
	}
	code := 0
	complete := true
	if failed {
		code = 1
	}
	if observer.partial || observer.failure != nil {
		code = 3
		complete = false
	}
	if errors.Is(parent.Err(), context.Canceled) {
		code = 130
		complete = false
	}
	out.deadline = time.Now().Add(c.Grace)
	if observer.capture != nil {
		if err := observer.capture.Finish(complete); err != nil {
			if code != 130 {
				code = 3
			}
			complete = false
		}
	}
	if out.finish(code, complete) != nil {
		if code != 130 {
			code = 3
		}
	}
	return out.dir, code
}
