package serviceability

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type API interface {
	ServiceabilityLogList(context.Context) (ddae.ServiceabilityLogList, error)
	ServiceabilityLogDetail(context.Context, string) (ddae.ServiceabilityLogDetail, error)
}

type State interface {
	FetchState(logID string) (exists bool, marker string, lastFetched time.Time, err error)
	Enqueue(event EncodedEvent, marker string, observedAt time.Time) (bool, error)
	MarkSeen(logID, marker string, observedAt time.Time) error
	ReconcileListed(listed map[string]struct{}, now time.Time, complete bool) error
	Health() (events int, full bool, err error)
}

type Logger interface{ Error(string, ...any) }

type Options struct {
	SourceInstance  string
	Interval        time.Duration
	CycleTimeout    time.Duration
	RefreshInterval time.Duration
	MaxPerCycle     int
	Concurrency     int
}

type Pipeline struct {
	api             API
	state           State
	diagnostics     *snapshot.Store
	sourceInstance  string
	interval        time.Duration
	cycleTimeout    time.Duration
	refreshInterval time.Duration
	maxPerCycle     int
	concurrency     int
	nextRefreshTurn bool
	logger          Logger
}

type detailTask struct {
	id          string
	marker      string
	priority    int
	lastFetched time.Time
}

func NewPipeline(api API, state State, diagnostics *snapshot.Store, options Options, logger Logger) *Pipeline {
	return &Pipeline{
		api: api, state: state, diagnostics: diagnostics,
		sourceInstance: options.SourceInstance, interval: options.Interval,
		cycleTimeout: options.CycleTimeout, refreshInterval: options.RefreshInterval,
		maxPerCycle: options.MaxPerCycle, concurrency: options.Concurrency, logger: logger,
	}
}

func (p *Pipeline) Run(ctx context.Context) {
	p.poll(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Pipeline) poll(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, p.cycleTimeout)
	defer cancel()
	started := time.Now()
	list, err := p.api.ServiceabilityLogList(ctx)
	listDuration := time.Since(started)
	if err != nil {
		p.diagnostics.RecordServiceabilityLogList(false, false, listDuration)
		p.diagnostics.RecordServiceabilityLogDetail(false, 0, 0)
		p.diagnostics.SetServiceabilityLogCollectionReady(false)
		p.logFailure("serviceability log list collection failed", "serviceability_log_list", err)
		return
	}

	now := time.Now()
	listed := make(map[string]struct{}, len(list.Results))
	markers := make(map[string]string, len(list.Results))
	complete := !list.Malformed && list.TotalRecords != nil && *list.TotalRecords >= 0
	for _, item := range list.Results {
		if err := ddae.ValidateServiceabilityLogID(item.ID); err != nil {
			complete = false
			continue
		}
		if _, duplicate := listed[item.ID]; duplicate {
			continue
		}
		listed[item.ID] = struct{}{}
		markers[item.ID] = usableMarker(item.UpdatedOn)
	}
	if list.TotalRecords != nil && *list.TotalRecords > int64(len(listed)) {
		complete = false
	}
	p.diagnostics.RecordServiceabilityLogList(true, complete, listDuration)

	tasks := make([]detailTask, 0, len(listed))
	stateOK := true
	for id := range listed {
		exists, previousMarker, lastFetched, stateErr := p.state.FetchState(id)
		if stateErr != nil {
			stateOK = false
			p.diagnostics.SetServiceabilityLogPipelineStateHealthy(false)
			p.logFailure("serviceability log checkpoint read failed", "serviceability_log_detail", stateErr)
			continue
		}
		marker := markers[id]
		priority := 2
		switch {
		case !exists:
			priority = 0
		case marker != "" && marker != previousMarker:
			priority = 0
		case lastFetched.IsZero() || now.Sub(lastFetched) >= p.refreshInterval:
			priority = 1
		}
		if priority < 2 {
			tasks = append(tasks, detailTask{id: id, marker: marker, priority: priority, lastFetched: lastFetched})
			continue
		}
		if err := p.state.MarkSeen(id, marker, now); err != nil {
			stateOK = false
			p.diagnostics.SetServiceabilityLogPipelineStateHealthy(false)
			p.logFailure("serviceability log checkpoint update failed", "serviceability_log_detail", err)
		}
	}
	eligible := len(tasks)
	tasks = p.selectFair(tasks)
	deferred := eligible - len(tasks)

	detailStarted := time.Now()
	detailSuccess, detailStateOK := p.fetchDetails(ctx, tasks, now)
	if !detailStateOK {
		stateOK = false
		p.diagnostics.SetServiceabilityLogPipelineStateHealthy(false)
	}
	detailDuration := time.Since(detailStarted)
	if err := p.state.ReconcileListed(listed, now, complete); err != nil {
		stateOK = false
		p.diagnostics.SetServiceabilityLogPipelineStateHealthy(false)
		p.logFailure("serviceability log reconciliation failed", "serviceability_log_detail", err)
	}
	events, full, healthErr := p.state.Health()
	if healthErr != nil {
		stateOK = false
		p.diagnostics.SetServiceabilityLogPipelineStateHealthy(false)
		p.logFailure("serviceability log state health failed", "serviceability_log_detail", healthErr)
	}
	p.diagnostics.SetServiceabilityLogBuffered(events)
	p.diagnostics.SetServiceabilityLogStateFull(full)
	p.diagnostics.RecordServiceabilityLogDetail(detailSuccess && stateOK, detailDuration, deferred)
	p.diagnostics.SetServiceabilityLogCollectionReady(complete && detailSuccess)
	if stateOK && detailSuccess {
		p.diagnostics.SetServiceabilityLogPipelineStateHealthy(true)
	}
}

func (p *Pipeline) selectFair(tasks []detailTask) []detailTask {
	newTasks := make([]detailTask, 0, len(tasks))
	refreshTasks := make([]detailTask, 0, len(tasks))
	for _, task := range tasks {
		if task.priority == 0 {
			newTasks = append(newTasks, task)
		} else {
			refreshTasks = append(refreshTasks, task)
		}
	}
	order := func(values []detailTask) {
		sort.Slice(values, func(i, j int) bool {
			if !values[i].lastFetched.Equal(values[j].lastFetched) {
				return values[i].lastFetched.Before(values[j].lastFetched)
			}
			return values[i].id < values[j].id
		})
	}
	order(newTasks)
	order(refreshTasks)
	limit := min(p.maxPerCycle, len(tasks))
	if limit <= 0 {
		return nil
	}
	if len(newTasks) == 0 {
		return append([]detailTask(nil), refreshTasks[:min(limit, len(refreshTasks))]...)
	}
	if len(refreshTasks) == 0 {
		return append([]detailTask(nil), newTasks[:min(limit, len(newTasks))]...)
	}
	if limit == 1 {
		if p.nextRefreshTurn {
			p.nextRefreshTurn = false
			return []detailTask{refreshTasks[0]}
		}
		p.nextRefreshTurn = true
		return []detailTask{newTasks[0]}
	}
	refreshCount := min(max(1, limit/4), len(refreshTasks))
	newCount := min(limit-refreshCount, len(newTasks))
	remaining := limit - refreshCount - newCount
	if remaining > 0 {
		extraRefresh := min(remaining, len(refreshTasks)-refreshCount)
		refreshCount += extraRefresh
		remaining -= extraRefresh
	}
	if remaining > 0 {
		newCount += min(remaining, len(newTasks)-newCount)
	}
	selected := make([]detailTask, 0, newCount+refreshCount)
	selected = append(selected, newTasks[:newCount]...)
	selected = append(selected, refreshTasks[:refreshCount]...)
	return selected
}

func (p *Pipeline) fetchDetails(ctx context.Context, tasks []detailTask, observedAt time.Time) (bool, bool) {
	if len(tasks) == 0 {
		return true, true
	}
	workers := min(p.concurrency, len(tasks))
	work := make(chan detailTask)
	type result struct {
		err      error
		stateErr bool
	}
	results := make(chan result, len(tasks))
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for task := range work {
				detail, err := p.api.ServiceabilityLogDetail(ctx, task.id)
				stateErr := false
				if err == nil {
					var event EncodedEvent
					event, err = BuildEvent(p.sourceInstance, task.id, detail, observedAt)
					if err == nil {
						_, err = p.state.Enqueue(event, task.marker, observedAt)
						stateErr = err != nil
					}
				}
				results <- result{err: err, stateErr: stateErr}
			}
		}()
	}
	go func() {
		defer close(work)
		for _, task := range tasks {
			select {
			case <-ctx.Done():
				return
			case work <- task:
			}
		}
	}()
	group.Wait()
	close(results)
	success := true
	stateOK := true
	count := 0
	for current := range results {
		count++
		if current.err != nil {
			success = false
			p.logFailure("serviceability log detail processing failed", "serviceability_log_detail", current.err)
		}
		stateOK = stateOK && !current.stateErr
	}
	return success && count == len(tasks), stateOK
}

func usableMarker(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func (p *Pipeline) logFailure(message, component string, err error) {
	if p.logger != nil {
		p.logger.Error(message, "component", component, "failure_class", observability.Classify(err))
	}
}
