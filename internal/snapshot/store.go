package snapshot

import (
	"sync"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

var CollectorNames = []string{"ping", "clusters", "nodes", "lock", "power", "alert_list", "alert_detail", "serviceability_log_list", "serviceability_log_detail"}

type OptionalFloat struct {
	Value float64
	Valid bool
}

type Cluster struct {
	ID                string
	State             string
	CoordinatorCPU    OptionalFloat
	CoordinatorMemory OptionalFloat
	WorkerCPU         OptionalFloat
	WorkerMemory      OptionalFloat
}

type Node struct {
	ID                          string
	State                       string
	CapacityCPU                 OptionalFloat
	CapacityMemory              OptionalFloat
	CapacityEphemeralStorage    OptionalFloat
	AllocatableCPU              OptionalFloat
	AllocatableMemory           OptionalFloat
	AllocatableEphemeralStorage OptionalFloat
	DiskPressure                *bool
	MemoryPressure              *bool
}

type Power struct {
	ControlPlaneReady bool
	NodesReady        int64
	TotalNodes        int64
}

type Family[T any] struct {
	Data        T
	CollectedAt time.Time
	Present     bool
}

type CollectorStatus struct {
	Success  bool
	Duration time.Duration
}

type View struct {
	Ping                              Family[bool]
	Clusters                          Family[[]Cluster]
	Nodes                             Family[[]Node]
	Lock                              Family[bool]
	Power                             Family[Power]
	LastCompleteAt                    time.Time
	Collectors                        map[string]CollectorStatus
	AlertListComplete                 bool
	AlertPipelineReady                bool
	AlertCollectionReady              bool
	AlertPipelineStateOK              bool
	AlertPublisherStateOK             bool
	AlertStateFull                    bool
	AlertDeferred                     int
	KafkaPublishSuccess               bool
	KafkaPublishDuration              time.Duration
	KafkaPublishedTotal               uint64
	KafkaFailedTotal                  map[observability.Class]uint64
	KafkaBuffered                     int
	ServiceabilityLogListComplete     bool
	ServiceabilityLogPipelineReady    bool
	ServiceabilityLogCollectionReady  bool
	ServiceabilityLogPipelineStateOK  bool
	ServiceabilityLogPublisherStateOK bool
	ServiceabilityLogStateFull        bool
	ServiceabilityLogDeferred         int
	ServiceabilityLogPublishSuccess   bool
	ServiceabilityLogPublishDuration  time.Duration
	ServiceabilityLogPublishedTotal   uint64
	ServiceabilityLogFailedTotal      map[observability.Class]uint64
	ServiceabilityLogBuffered         int
}

type Store struct {
	mu   sync.RWMutex
	view View
}

func NewStore() *Store {
	statuses := make(map[string]CollectorStatus, len(CollectorNames))
	for _, name := range CollectorNames {
		statuses[name] = CollectorStatus{}
	}
	return &Store{view: View{
		Collectors:                   statuses,
		KafkaFailedTotal:             make(map[observability.Class]uint64),
		ServiceabilityLogFailedTotal: make(map[observability.Class]uint64),
	}}
}

func (s *Store) RecordPing(value bool, usable, success bool, collectedAt time.Time, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if usable {
		s.view.Ping = Family[bool]{Data: value, CollectedAt: collectedAt, Present: true}
	}
	s.view.Collectors["ping"] = CollectorStatus{Success: success, Duration: duration}
}

func (s *Store) RecordClusters(value []Cluster, usable, success bool, collectedAt time.Time, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if usable {
		s.view.Clusters = Family[[]Cluster]{Data: append([]Cluster(nil), value...), CollectedAt: collectedAt, Present: true}
	}
	s.view.Collectors["clusters"] = CollectorStatus{Success: success, Duration: duration}
}

func (s *Store) RecordNodes(value []Node, usable, success bool, collectedAt time.Time, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if usable {
		s.view.Nodes = Family[[]Node]{Data: append([]Node(nil), value...), CollectedAt: collectedAt, Present: true}
	}
	s.view.Collectors["nodes"] = CollectorStatus{Success: success, Duration: duration}
}

func (s *Store) RecordLock(value bool, usable, success bool, collectedAt time.Time, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if usable {
		s.view.Lock = Family[bool]{Data: value, CollectedAt: collectedAt, Present: true}
	}
	s.view.Collectors["lock"] = CollectorStatus{Success: success, Duration: duration}
}

func (s *Store) RecordPower(value Power, usable, success bool, collectedAt time.Time, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if usable {
		s.view.Power = Family[Power]{Data: value, CollectedAt: collectedAt, Present: true}
	}
	s.view.Collectors["power"] = CollectorStatus{Success: success, Duration: duration}
}

func (s *Store) CompleteRequiredCycle(at time.Time, success bool) {
	if !success {
		return
	}
	s.mu.Lock()
	s.view.LastCompleteAt = at
	s.mu.Unlock()
}

func (s *Store) RecordAlertList(success, complete bool, duration time.Duration) {
	s.mu.Lock()
	s.view.Collectors["alert_list"] = CollectorStatus{Success: success, Duration: duration}
	s.view.AlertListComplete = success && complete
	s.mu.Unlock()
}

func (s *Store) RecordAlertDetail(success bool, duration time.Duration, deferred int) {
	s.mu.Lock()
	s.view.Collectors["alert_detail"] = CollectorStatus{Success: success, Duration: duration}
	s.view.AlertDeferred = deferred
	s.mu.Unlock()
}

func (s *Store) SetAlertPipelineReady(ready bool) {
	s.mu.Lock()
	// This compatibility helper represents a fully evaluated alert pipeline.
	// Runtime owners use the component-specific setters below.
	s.view.AlertCollectionReady = ready
	s.view.AlertPipelineStateOK = ready
	s.view.AlertPublisherStateOK = ready
	s.view.AlertStateFull = false
	s.recomputeAlertReadyLocked()
	s.mu.Unlock()
}

func (s *Store) SetAlertCollectionReady(ready bool) {
	s.mu.Lock()
	s.view.AlertCollectionReady = ready
	s.recomputeAlertReadyLocked()
	s.mu.Unlock()
}

func (s *Store) SetAlertPipelineStateHealthy(healthy bool) {
	s.mu.Lock()
	s.view.AlertPipelineStateOK = healthy
	s.recomputeAlertReadyLocked()
	s.mu.Unlock()
}

func (s *Store) SetAlertPublisherStateHealthy(healthy bool) {
	s.mu.Lock()
	s.view.AlertPublisherStateOK = healthy
	s.recomputeAlertReadyLocked()
	s.mu.Unlock()
}

func (s *Store) SetAlertStateFull(full bool) {
	s.mu.Lock()
	s.view.AlertStateFull = full
	s.recomputeAlertReadyLocked()
	s.mu.Unlock()
}

func (s *Store) recomputeAlertReadyLocked() {
	s.view.AlertPipelineReady = s.view.AlertCollectionReady && s.view.AlertPipelineStateOK &&
		s.view.AlertPublisherStateOK && !s.view.AlertStateFull
}

func (s *Store) RecordKafkaPublish(success bool, duration time.Duration, acknowledged int, failure observability.Class, buffered int) {
	s.mu.Lock()
	s.view.KafkaPublishSuccess = success
	s.view.KafkaPublishDuration = duration
	if acknowledged > 0 {
		s.view.KafkaPublishedTotal += uint64(acknowledged)
	}
	if failure != "" {
		s.view.KafkaFailedTotal[failure]++
	}
	s.view.KafkaBuffered = buffered
	s.mu.Unlock()
}

func (s *Store) SetKafkaBuffered(buffered int) {
	s.mu.Lock()
	s.view.KafkaBuffered = buffered
	s.mu.Unlock()
}

func (s *Store) RecordServiceabilityLogList(success, complete bool, duration time.Duration) {
	s.mu.Lock()
	s.view.Collectors["serviceability_log_list"] = CollectorStatus{Success: success, Duration: duration}
	s.view.ServiceabilityLogListComplete = success && complete
	s.mu.Unlock()
}

func (s *Store) RecordServiceabilityLogDetail(success bool, duration time.Duration, deferred int) {
	s.mu.Lock()
	s.view.Collectors["serviceability_log_detail"] = CollectorStatus{Success: success, Duration: duration}
	s.view.ServiceabilityLogDeferred = deferred
	s.mu.Unlock()
}

func (s *Store) SetServiceabilityLogCollectionReady(ready bool) {
	s.mu.Lock()
	s.view.ServiceabilityLogCollectionReady = ready
	s.recomputeServiceabilityLogReadyLocked()
	s.mu.Unlock()
}

func (s *Store) SetServiceabilityLogPipelineStateHealthy(healthy bool) {
	s.mu.Lock()
	s.view.ServiceabilityLogPipelineStateOK = healthy
	s.recomputeServiceabilityLogReadyLocked()
	s.mu.Unlock()
}

func (s *Store) SetServiceabilityLogPublisherStateHealthy(healthy bool) {
	s.mu.Lock()
	s.view.ServiceabilityLogPublisherStateOK = healthy
	s.recomputeServiceabilityLogReadyLocked()
	s.mu.Unlock()
}

func (s *Store) SetServiceabilityLogStateFull(full bool) {
	s.mu.Lock()
	s.view.ServiceabilityLogStateFull = full
	s.recomputeServiceabilityLogReadyLocked()
	s.mu.Unlock()
}

func (s *Store) recomputeServiceabilityLogReadyLocked() {
	s.view.ServiceabilityLogPipelineReady = s.view.ServiceabilityLogCollectionReady &&
		s.view.ServiceabilityLogPipelineStateOK && s.view.ServiceabilityLogPublisherStateOK &&
		!s.view.ServiceabilityLogStateFull
}

func (s *Store) RecordServiceabilityLogPublish(success bool, duration time.Duration, acknowledged int, failure observability.Class, buffered int) {
	s.mu.Lock()
	s.view.ServiceabilityLogPublishSuccess = success
	s.view.ServiceabilityLogPublishDuration = duration
	if acknowledged > 0 {
		s.view.ServiceabilityLogPublishedTotal += uint64(acknowledged)
	}
	if failure != "" {
		s.view.ServiceabilityLogFailedTotal[failure]++
	}
	s.view.ServiceabilityLogBuffered = buffered
	s.mu.Unlock()
}

func (s *Store) SetServiceabilityLogBuffered(buffered int) {
	s.mu.Lock()
	s.view.ServiceabilityLogBuffered = buffered
	s.mu.Unlock()
}

func (s *Store) Load() View {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := s.view
	result.Clusters.Data = append([]Cluster(nil), s.view.Clusters.Data...)
	result.Nodes.Data = append([]Node(nil), s.view.Nodes.Data...)
	result.Collectors = make(map[string]CollectorStatus, len(s.view.Collectors))
	for key, value := range s.view.Collectors {
		result.Collectors[key] = value
	}
	result.KafkaFailedTotal = make(map[observability.Class]uint64, len(s.view.KafkaFailedTotal))
	for key, value := range s.view.KafkaFailedTotal {
		result.KafkaFailedTotal[key] = value
	}
	result.ServiceabilityLogFailedTotal = make(map[observability.Class]uint64, len(s.view.ServiceabilityLogFailedTotal))
	for key, value := range s.view.ServiceabilityLogFailedTotal {
		result.ServiceabilityLogFailedTotal[key] = value
	}
	return result
}

func (s *Store) Ready(now time.Time, staleAfter time.Duration) bool {
	return s.ReadyFor(now, staleAfter, true, true)
}

func (s *Store) ReadyFor(now time.Time, staleAfter time.Duration, resourcesEnabled, alertsEnabled bool, serviceabilityLogsEnabled ...bool) bool {
	view := s.Load()
	logsEnabled := len(serviceabilityLogsEnabled) > 0 && serviceabilityLogsEnabled[0]
	if !resourcesEnabled && !alertsEnabled && !logsEnabled {
		return false
	}
	if resourcesEnabled && !requiredCurrent(view, now, staleAfter) {
		return false
	}
	if alertsEnabled && !view.AlertPipelineReady {
		return false
	}
	if logsEnabled && !view.ServiceabilityLogPipelineReady {
		return false
	}
	return true
}

func RequiredCurrent(view View, now time.Time, staleAfter time.Duration) bool {
	return requiredCurrent(view, now, staleAfter)
}

func requiredCurrent(view View, now time.Time, staleAfter time.Duration) bool {
	if view.LastCompleteAt.IsZero() || now.Sub(view.LastCompleteAt) > staleAfter {
		return false
	}
	for _, collector := range []string{"ping", "clusters", "nodes", "lock", "power"} {
		if !view.Collectors[collector].Success {
			return false
		}
	}
	return true
}
