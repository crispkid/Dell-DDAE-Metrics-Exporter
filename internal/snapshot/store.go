package snapshot

import (
	"sync"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

var CollectorNames = []string{"ping", "clusters", "nodes", "lock", "power", "alert_list", "alert_detail"}

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
	Ping                 Family[bool]
	Clusters             Family[[]Cluster]
	Nodes                Family[[]Node]
	Lock                 Family[bool]
	Power                Family[Power]
	LastCompleteAt       time.Time
	Collectors           map[string]CollectorStatus
	AlertListComplete    bool
	AlertPipelineReady   bool
	AlertDeferred        int
	KafkaPublishSuccess  bool
	KafkaPublishDuration time.Duration
	KafkaPublishedTotal  uint64
	KafkaFailedTotal     map[observability.Class]uint64
	KafkaBuffered        int
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
		Collectors:       statuses,
		KafkaFailedTotal: make(map[observability.Class]uint64),
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
	s.view.AlertPipelineReady = ready
	s.mu.Unlock()
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
	return result
}

func (s *Store) Ready(now time.Time, staleAfter time.Duration) bool {
	return s.ReadyFor(now, staleAfter, true, true)
}

func (s *Store) ReadyFor(now time.Time, staleAfter time.Duration, resourcesEnabled, alertsEnabled bool) bool {
	view := s.Load()
	if !resourcesEnabled && !alertsEnabled {
		return false
	}
	if resourcesEnabled && !requiredCurrent(view, now, staleAfter) {
		return false
	}
	if alertsEnabled && !view.AlertPipelineReady {
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
