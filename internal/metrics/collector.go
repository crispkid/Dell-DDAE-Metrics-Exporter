package metrics

import (
	"math"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"github.com/prometheus/client_golang/prometheus"
)

type BuildInfo struct {
	Version   string
	GoVersion string
}

type Collector struct {
	store            *snapshot.Store
	staleAfter       time.Duration
	build            BuildInfo
	resourcesEnabled bool
	alertsEnabled    bool

	monitoringEnabled        *prometheus.Desc
	up                       *prometheus.Desc
	collectorSuccess         *prometheus.Desc
	collectorDuration        *prometheus.Desc
	snapshotAge              *prometheus.Desc
	buildInfo                *prometheus.Desc
	managementAPIUp          *prometheus.Desc
	clusterState             *prometheus.Desc
	clusterCoordinatorCPU    *prometheus.Desc
	clusterCoordinatorMemory *prometheus.Desc
	clusterWorkerCPU         *prometheus.Desc
	clusterWorkerMemory      *prometheus.Desc
	nodeState                *prometheus.Desc
	nodeReady                *prometheus.Desc
	nodeCapacityCPU          *prometheus.Desc
	nodeCapacityMemory       *prometheus.Desc
	nodeCapacityStorage      *prometheus.Desc
	nodeAllocatableCPU       *prometheus.Desc
	nodeAllocatableMemory    *prometheus.Desc
	nodeAllocatableStorage   *prometheus.Desc
	nodeCondition            *prometheus.Desc
	systemLocked             *prometheus.Desc
	controlPlaneReady        *prometheus.Desc
	nodesReady               *prometheus.Desc
	nodesTotal               *prometheus.Desc
	alertListComplete        *prometheus.Desc
	alertDetailDeferred      *prometheus.Desc
	alertPipelineReady       *prometheus.Desc
	kafkaPublishSuccess      *prometheus.Desc
	kafkaPublishDuration     *prometheus.Desc
	kafkaEventsPublished     *prometheus.Desc
	kafkaEventsFailed        *prometheus.Desc
	kafkaBufferedEvents      *prometheus.Desc
}

type PipelineMode struct {
	ResourcesEnabled bool
	AlertsEnabled    bool
}

func NewCollector(store *snapshot.Store, staleAfter time.Duration, build BuildInfo, modes ...PipelineMode) *Collector {
	mode := PipelineMode{ResourcesEnabled: true, AlertsEnabled: true}
	if len(modes) > 0 {
		mode = modes[0]
	}
	return &Collector{
		store: store, staleAfter: staleAfter, build: build,
		resourcesEnabled: mode.ResourcesEnabled, alertsEnabled: mode.AlertsEnabled,
		monitoringEnabled:        descLabels("ddae_monitoring_enabled", "Configured monitoring pipeline is enabled (1) or disabled (0).", "pipeline"),
		up:                       desc("ddae_up", "1 only when the approved target-success policy is satisfied by a current snapshot; otherwise 0."),
		collectorSuccess:         descLabels("ddae_collector_success", "Last attempted collector cycle succeeded (1) or failed (0).", "collector"),
		collectorDuration:        descLabels("ddae_collector_duration_seconds", "Duration of the last collector attempt in seconds.", "collector"),
		snapshotAge:              desc("ddae_snapshot_age_seconds", "Seconds since the newest successfully published required snapshot."),
		buildInfo:                descLabels("ddae_build_info", "Constant 1; changes only when the exporter build changes.", "version", "go_version"),
		managementAPIUp:          desc("ddae_management_api_up", "Documented reachability status and successful authenticated request."),
		clusterState:             descLabels("ddae_cluster_state_info", "One-hot state using available or unknown; cluster is the Dell cluster id.", "cluster", "state"),
		clusterCoordinatorCPU:    descLabels("ddae_cluster_coordinator_configured_cpu_cores", "Configured DDAE coordinator CPU cores, not usage.", "cluster"),
		clusterCoordinatorMemory: descLabels("ddae_cluster_coordinator_configured_memory_bytes", "Configured coordinator memory converted from a validated quantity to bytes.", "cluster"),
		clusterWorkerCPU:         descLabels("ddae_cluster_worker_configured_cpu_cores", "CPU quantity returned by the cluster worker configuration object; no aggregate or utilization meaning is implied.", "cluster"),
		clusterWorkerMemory:      descLabels("ddae_cluster_worker_configured_memory_bytes", "Memory quantity returned by the cluster worker configuration object, converted to bytes; no aggregate or utilization meaning is implied.", "cluster"),
		nodeState:                descLabels("ddae_node_state_info", "One-hot state from the documented bounded node-state enum; node is the Dell node id.", "node", "state"),
		nodeReady:                descLabels("ddae_node_ready", "1 only for documented Ready; 0 for other known states.", "node"),
		nodeCapacityCPU:          descLabels("ddae_node_capacity_cpu_cores", "Total capacity in CPU cores, not utilization.", "node"),
		nodeCapacityMemory:       descLabels("ddae_node_capacity_memory_bytes", "Total memory capacity converted to bytes.", "node"),
		nodeCapacityStorage:      descLabels("ddae_node_capacity_ephemeral_storage_bytes", "Total ephemeral storage capacity converted to bytes.", "node"),
		nodeAllocatableCPU:       descLabels("ddae_node_allocatable_cpu_cores", "Allocatable CPU cores, not current unused CPU.", "node"),
		nodeAllocatableMemory:    descLabels("ddae_node_allocatable_memory_bytes", "Allocatable memory bytes, not current unused memory.", "node"),
		nodeAllocatableStorage:   descLabels("ddae_node_allocatable_ephemeral_storage_bytes", "Allocatable ephemeral storage bytes.", "node"),
		nodeCondition:            descLabels("ddae_node_condition", "Boolean condition for the fixed set disk_pressure, memory_pressure.", "node", "condition"),
		systemLocked:             desc("ddae_system_locked", "1 when the appliance is locked by another job."),
		controlPlaneReady:        desc("ddae_control_plane_ready", "Boolean readiness of control-plane nodes."),
		nodesReady:               desc("ddae_nodes_ready", "Current ready-node count."),
		nodesTotal:               desc("ddae_nodes_total", "Current total-node count."),
		alertListComplete:        desc("ddae_alert_list_complete", "1 only when the returned list is structurally valid and totalRecords is not greater than the usable returned result count."),
		alertDetailDeferred:      desc("ddae_alert_detail_deferred", "Number of still-listed alert IDs deferred after the last cycle because of the configured request cap."),
		alertPipelineReady:       desc("ddae_alert_pipeline_ready", "1 only when list/detail collection, persistent state and outbox-capacity requirements are satisfied."),
		kafkaPublishSuccess:      desc("ddae_kafka_publish_success", "Last required Kafka publish batch was acknowledged (1) or failed (0)."),
		kafkaPublishDuration:     desc("ddae_kafka_publish_duration_seconds", "Duration of the last required Kafka publish batch in seconds."),
		kafkaEventsPublished:     desc("ddae_kafka_events_published_total", "Total events acknowledged by Kafka; producer retry attempts do not increment it."),
		kafkaEventsFailed:        descLabels("ddae_kafka_events_failed_total", "Total events whose approved delivery policy ended in failure, using a fixed reason set.", "reason"),
		kafkaBufferedEvents:      desc("ddae_kafka_buffered_events", "Current number of events retained for retry under the approved bounded buffering policy."),
	}
}

func NewRegistry(store *snapshot.Store, staleAfter time.Duration, build BuildInfo, modes ...PipelineMode) (*prometheus.Registry, error) {
	registry := prometheus.NewRegistry()
	if err := registry.Register(NewCollector(store, staleAfter, build, modes...)); err != nil {
		return nil, err
	}
	return registry, nil
}

func desc(name, help string) *prometheus.Desc {
	return prometheus.NewDesc(name, help, nil, nil)
}

func descLabels(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(name, help, labels, nil)
}

func (c *Collector) Describe(output chan<- *prometheus.Desc) {
	for _, descriptor := range []*prometheus.Desc{c.monitoringEnabled, c.buildInfo, c.collectorSuccess, c.collectorDuration} {
		output <- descriptor
	}
	if c.resourcesEnabled {
		for _, descriptor := range []*prometheus.Desc{
			c.up, c.snapshotAge,
			c.managementAPIUp, c.clusterState, c.clusterCoordinatorCPU,
			c.clusterCoordinatorMemory, c.clusterWorkerCPU, c.clusterWorkerMemory,
			c.nodeState, c.nodeReady, c.nodeCapacityCPU, c.nodeCapacityMemory,
			c.nodeCapacityStorage, c.nodeAllocatableCPU, c.nodeAllocatableMemory,
			c.nodeAllocatableStorage, c.nodeCondition, c.systemLocked,
			c.controlPlaneReady, c.nodesReady, c.nodesTotal,
		} {
			output <- descriptor
		}
	}
	if c.alertsEnabled {
		for _, descriptor := range []*prometheus.Desc{
			c.alertListComplete, c.alertDetailDeferred, c.alertPipelineReady, c.kafkaPublishSuccess,
			c.kafkaPublishDuration, c.kafkaEventsPublished, c.kafkaEventsFailed,
			c.kafkaBufferedEvents,
		} {
			output <- descriptor
		}
	}
}

func (c *Collector) Collect(output chan<- prometheus.Metric) {
	now := time.Now()
	view := c.store.Load()
	output <- gauge(c.monitoringEnabled, boolean(c.resourcesEnabled), "resources")
	output <- gauge(c.monitoringEnabled, boolean(c.alertsEnabled), "alerts")
	output <- gauge(c.buildInfo, 1, c.build.Version, c.build.GoVersion)

	if c.resourcesEnabled {
		up := 0.0
		if snapshot.RequiredCurrent(view, now, c.staleAfter) {
			up = 1
		}
		output <- gauge(c.up, up)
		for _, name := range []string{"ping", "clusters", "nodes", "lock", "power"} {
			status := view.Collectors[name]
			output <- gauge(c.collectorSuccess, boolean(status.Success), name)
			output <- gauge(c.collectorDuration, status.Duration.Seconds(), name)
		}
		age := 0.0
		if !view.LastCompleteAt.IsZero() {
			age = max(0, now.Sub(view.LastCompleteAt).Seconds())
		}
		output <- gauge(c.snapshotAge, age)
		apiUp := view.Collectors["ping"].Success && current(view.Ping.CollectedAt, view.Ping.Present, now, c.staleAfter) && view.Ping.Data
		output <- gauge(c.managementAPIUp, boolean(apiUp))

		if current(view.Clusters.CollectedAt, view.Clusters.Present, now, c.staleAfter) {
			for _, cluster := range view.Clusters.Data {
				for _, state := range []string{"available", "unknown"} {
					output <- gauge(c.clusterState, boolean(cluster.State == state), cluster.ID, state)
				}
				optional(output, c.clusterCoordinatorCPU, cluster.CoordinatorCPU, cluster.ID)
				optional(output, c.clusterCoordinatorMemory, cluster.CoordinatorMemory, cluster.ID)
				optional(output, c.clusterWorkerCPU, cluster.WorkerCPU, cluster.ID)
				optional(output, c.clusterWorkerMemory, cluster.WorkerMemory, cluster.ID)
			}
		}
		if current(view.Nodes.CollectedAt, view.Nodes.Present, now, c.staleAfter) {
			for _, node := range view.Nodes.Data {
				for _, state := range nodeStateValues {
					output <- gauge(c.nodeState, boolean(node.State == state), node.ID, state)
				}
				output <- gauge(c.nodeReady, boolean(node.State == "ready"), node.ID)
				optional(output, c.nodeCapacityCPU, node.CapacityCPU, node.ID)
				optional(output, c.nodeCapacityMemory, node.CapacityMemory, node.ID)
				optional(output, c.nodeCapacityStorage, node.CapacityEphemeralStorage, node.ID)
				optional(output, c.nodeAllocatableCPU, node.AllocatableCPU, node.ID)
				optional(output, c.nodeAllocatableMemory, node.AllocatableMemory, node.ID)
				optional(output, c.nodeAllocatableStorage, node.AllocatableEphemeralStorage, node.ID)
				if node.DiskPressure != nil {
					output <- gauge(c.nodeCondition, boolean(*node.DiskPressure), node.ID, "disk_pressure")
				}
				if node.MemoryPressure != nil {
					output <- gauge(c.nodeCondition, boolean(*node.MemoryPressure), node.ID, "memory_pressure")
				}
			}
		}
		if current(view.Lock.CollectedAt, view.Lock.Present, now, c.staleAfter) {
			output <- gauge(c.systemLocked, boolean(view.Lock.Data))
		}
		if current(view.Power.CollectedAt, view.Power.Present, now, c.staleAfter) {
			output <- gauge(c.controlPlaneReady, boolean(view.Power.Data.ControlPlaneReady))
			output <- gauge(c.nodesReady, float64(view.Power.Data.NodesReady))
			output <- gauge(c.nodesTotal, float64(view.Power.Data.TotalNodes))
		}
	}

	if c.alertsEnabled {
		for _, name := range []string{"alert_list", "alert_detail"} {
			status := view.Collectors[name]
			output <- gauge(c.collectorSuccess, boolean(status.Success), name)
			output <- gauge(c.collectorDuration, status.Duration.Seconds(), name)
		}
		output <- gauge(c.alertListComplete, boolean(view.AlertListComplete))
		output <- gauge(c.alertDetailDeferred, float64(view.AlertDeferred))
		output <- gauge(c.alertPipelineReady, boolean(view.AlertPipelineReady))
		output <- gauge(c.kafkaPublishSuccess, boolean(view.KafkaPublishSuccess))
		output <- gauge(c.kafkaPublishDuration, view.KafkaPublishDuration.Seconds())
		output <- prometheus.MustNewConstMetric(c.kafkaEventsPublished, prometheus.CounterValue, float64(view.KafkaPublishedTotal))
		for _, class := range failureClasses {
			output <- prometheus.MustNewConstMetric(c.kafkaEventsFailed, prometheus.CounterValue, float64(view.KafkaFailedTotal[class]), string(class))
		}
		output <- gauge(c.kafkaBufferedEvents, float64(view.KafkaBuffered))
	}
}

func gauge(descriptor *prometheus.Desc, value float64, labels ...string) prometheus.Metric {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}
	return prometheus.MustNewConstMetric(descriptor, prometheus.GaugeValue, value, labels...)
}

func optional(output chan<- prometheus.Metric, descriptor *prometheus.Desc, value snapshot.OptionalFloat, labels ...string) {
	if value.Valid {
		output <- gauge(descriptor, value.Value, labels...)
	}
}

func current(at time.Time, present bool, now time.Time, staleAfter time.Duration) bool {
	return present && !at.IsZero() && now.Sub(at) <= staleAfter
}

func boolean(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
