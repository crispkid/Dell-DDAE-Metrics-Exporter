package alerts

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
)

type AlertAPI interface {
	AlertList(context.Context) (ddae.AlertList, error)
	AlertDetail(context.Context, string) (ddae.AlertDetail, error)
}

type State interface {
	FetchState(alertID string) (exists bool, marker string, lastFetched time.Time, err error)
	Enqueue(event EncodedEvent, marker string, observedAt time.Time) (bool, error)
	MarkSeen(alertID, marker string, observedAt time.Time) error
	ReconcileListed(listed map[string]struct{}, now time.Time, complete bool) error
	Health() (events int, full bool, err error)
}

type Pipeline struct {
	api             AlertAPI
	state           State
	diagnostics     *snapshot.Store
	sourceInstance  string
	interval        time.Duration
	cycleTimeout    time.Duration
	refreshInterval time.Duration
	maxPerCycle     int
	concurrency     int
	nextRefreshTurn bool
	waitingOrder    map[string]int
	logger          Logger
}

type Logger interface {
	Error(msg string, args ...any)
}

type Options struct {
	SourceInstance  string
	Interval        time.Duration
	CycleTimeout    time.Duration
	RefreshInterval time.Duration
	MaxPerCycle     int
	Concurrency     int
}

func NewPipeline(api AlertAPI, state State, diagnostics *snapshot.Store, options Options, logger Logger) *Pipeline {
	return &Pipeline{
		api: api, state: state, diagnostics: diagnostics,
		sourceInstance: options.SourceInstance, interval: options.Interval,
		cycleTimeout: options.CycleTimeout, refreshInterval: options.RefreshInterval,
		maxPerCycle: options.MaxPerCycle, concurrency: options.Concurrency,
		logger: logger,
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
	list, err := p.api.AlertList(ctx)
	listDuration := time.Since(started)
	if err != nil {
		p.diagnostics.RecordAlertList(false, false, listDuration)
		p.diagnostics.RecordAlertDetail(false, 0, 0)
		p.diagnostics.SetAlertCollectionReady(false)
		p.logFailure("alert list collection failed", "alert_list", err)
		return
	}

	now := time.Now()
	listed := make(map[string]struct{}, len(list.Results))
	markers := make(map[string]string, len(list.Results))
	complete := list.TotalRecords != nil && *list.TotalRecords >= 0
	for _, item := range list.Results {
		if err := ddae.ValidateAlertID(item.ID); err != nil {
			complete = false
			continue
		}
		if _, duplicate := listed[item.ID]; duplicate {
			complete = false
			continue
		}
		listed[item.ID] = struct{}{}
		markers[item.ID] = usableMarker(item.UpdatedOn)
	}
	if list.TotalRecords != nil && *list.TotalRecords > int64(len(listed)) {
		complete = false
	}
	p.diagnostics.RecordAlertList(true, complete, listDuration)

	tasks := make([]detailTask, 0, len(listed))
	stateOK := true
	for id := range listed {
		exists, previousMarker, lastFetched, stateErr := p.state.FetchState(id)
		if stateErr != nil {
			stateOK = false
			p.diagnostics.SetAlertPipelineStateHealthy(false)
			p.logFailure("alert checkpoint read failed", "alert_detail", stateErr)
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
			p.diagnostics.SetAlertPipelineStateHealthy(false)
			p.logFailure("alert checkpoint update failed", "alert_detail", err)
		}
	}
	eligible := len(tasks)
	tasks = p.selectFair(tasks)
	deferred := eligible - len(tasks)

	detailStarted := time.Now()
	detailSuccess, detailStateOK := p.fetchDetails(ctx, tasks, now)
	if !detailStateOK {
		stateOK = false
		p.diagnostics.SetAlertPipelineStateHealthy(false)
	}
	detailDuration := time.Since(detailStarted)
	if err := p.state.ReconcileListed(listed, now, complete); err != nil {
		stateOK = false
		p.diagnostics.SetAlertPipelineStateHealthy(false)
		p.logFailure("alert checkpoint reconciliation failed", "alert_detail", err)
	}
	events, full, healthErr := p.state.Health()
	if healthErr != nil {
		stateOK = false
		p.diagnostics.SetAlertPipelineStateHealthy(false)
		p.logFailure("alert persistent state health failed", "alert_detail", healthErr)
	}
	p.diagnostics.SetKafkaBuffered(events)
	p.diagnostics.SetAlertStateFull(full)
	p.diagnostics.RecordAlertDetail(detailSuccess && stateOK, detailDuration, deferred)
	p.diagnostics.SetAlertCollectionReady(complete && detailSuccess)
	if stateOK && detailSuccess {
		p.diagnostics.SetAlertPipelineStateHealthy(true)
	}
}

func (p *Pipeline) selectFair(tasks []detailTask) (selected []detailTask) {
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
			left, leftWaiting := p.waitingOrder[values[i].id]
			right, rightWaiting := p.waitingOrder[values[j].id]
			if leftWaiting != rightWaiting {
				return leftWaiting
			}
			if leftWaiting {
				return left < right
			}
			if !values[i].lastFetched.Equal(values[j].lastFetched) {
				return values[i].lastFetched.Before(values[j].lastFetched)
			}
			return values[i].id < values[j].id
		})
	}
	order(newTasks)
	order(refreshTasks)
	defer func() {
		// Attempts consume a turn even when they fail. Keep only this bounded
		// eligible set; newcomers join behind IDs already waiting.
		order(tasks)
		attempted := make(map[string]bool, len(selected))
		for _, task := range selected {
			attempted[task.id] = true
		}
		next := make(map[string]int, len(tasks))
		for _, task := range tasks {
			if !attempted[task.id] {
				next[task.id] = len(next)
			}
		}
		for _, task := range selected {
			next[task.id] = len(next)
		}
		p.waitingOrder = next
	}()
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
	selected = make([]detailTask, 0, newCount+refreshCount)
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
	type detailResult struct {
		err      error
		stateErr bool
	}
	results := make(chan detailResult, len(tasks))
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for task := range work {
				detail, err := p.api.AlertDetail(ctx, task.id)
				stateErr := false
				if err == nil {
					var event EncodedEvent
					event, err = BuildEvent(p.sourceInstance, task.id, detail, observedAt)
					if err == nil {
						_, err = p.state.Enqueue(event, task.marker, observedAt)
						stateErr = err != nil
					}
				}
				results <- detailResult{err: err, stateErr: stateErr}
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
	success := len(tasks) > 0
	stateOK := true
	count := 0
	for result := range results {
		count++
		if result.err != nil {
			success = false
			p.logFailure("alert detail processing failed", "alert_detail", result.err)
		}
		stateOK = stateOK && !result.stateErr
	}
	return success && count == len(tasks), stateOK
}

func (p *Pipeline) logFailure(message, component string, err error) {
	if p.logger != nil {
		p.logger.Error(message, "component", component, "failure_class", observability.Classify(err))
	}
}
