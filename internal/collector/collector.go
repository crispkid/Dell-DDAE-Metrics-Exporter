package collector

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/snapshot"
	"k8s.io/apimachinery/pkg/api/resource"
)

type API interface {
	Ping(context.Context) (ddae.PingResponse, error)
	Clusters(context.Context) ([]ddae.Cluster, error)
	Nodes(context.Context) ([]ddae.InfrastructureNode, error)
	Lock(context.Context) (ddae.LockResponse, error)
	Power(context.Context) (ddae.PowerResponse, error)
}

func normalizeClusters(raw []ddae.Cluster) ([]snapshot.Cluster, bool, error) {
	result := make([]snapshot.Cluster, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	incomplete := false
	for _, item := range raw {
		if strings.TrimSpace(item.ID) == "" {
			return nil, false, validationError("cluster identity")
		}
		if _, exists := seen[item.ID]; exists {
			return nil, false, validationError("duplicate cluster identity")
		}
		seen[item.ID] = struct{}{}
		cluster := snapshot.Cluster{ID: item.ID, State: normalizeClusterState(item.ClusterStatus)}
		var err error
		cluster.CoordinatorCPU, err = cpu(item.Coordinator.CPU)
		incomplete = incomplete || err != nil
		cluster.CoordinatorMemory, err = bytesQuantity(item.Coordinator.Memory)
		incomplete = incomplete || err != nil
		cluster.WorkerCPU, err = cpu(item.Worker.CPU)
		incomplete = incomplete || err != nil
		cluster.WorkerMemory, err = bytesQuantity(item.Worker.Memory)
		incomplete = incomplete || err != nil
		result = append(result, cluster)
	}
	if incomplete {
		return result, true, validationError("cluster optional field")
	}
	return result, true, nil
}

func normalizeNodes(raw []ddae.InfrastructureNode) ([]snapshot.Node, bool, error) {
	result := make([]snapshot.Node, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	incomplete := false
	for _, item := range raw {
		if strings.TrimSpace(item.ID) == "" {
			return nil, false, validationError("node identity")
		}
		if _, exists := seen[item.ID]; exists {
			return nil, false, validationError("duplicate node identity")
		}
		seen[item.ID] = struct{}{}
		node := snapshot.Node{ID: item.ID, State: normalizeNodeState(item.State)}
		var err error
		node.CapacityCPU, err = cpu(item.Capacity.CPU)
		incomplete = incomplete || err != nil
		node.CapacityMemory, err = bytesQuantity(item.Capacity.Memory)
		incomplete = incomplete || err != nil
		node.CapacityEphemeralStorage, err = bytesQuantity(item.Capacity.EphemeralStorage)
		incomplete = incomplete || err != nil
		node.AllocatableCPU, err = cpu(item.Allocatable.CPU)
		incomplete = incomplete || err != nil
		node.AllocatableMemory, err = bytesQuantity(item.Allocatable.Memory)
		incomplete = incomplete || err != nil
		node.AllocatableEphemeralStorage, err = bytesQuantity(item.Allocatable.EphemeralStorage)
		incomplete = incomplete || err != nil
		for _, condition := range item.Conditions {
			value, valid := conditionValue(condition.Status)
			switch normalizeConditionName(condition.Type) {
			case "disk_pressure":
				if !valid {
					incomplete = true
				} else {
					node.DiskPressure = boolPointer(value)
				}
			case "memory_pressure":
				if !valid {
					incomplete = true
				} else {
					node.MemoryPressure = boolPointer(value)
				}
			}
		}
		result = append(result, node)
	}
	if incomplete {
		return result, true, validationError("node optional field")
	}
	return result, true, nil
}

func normalizePower(raw ddae.PowerResponse) (snapshot.Power, bool, error) {
	if raw.ControlPlaneReady == nil || raw.NodesReady == nil || raw.TotalNodes == nil {
		return snapshot.Power{}, false, validationError("power required field")
	}
	if *raw.NodesReady < 0 || *raw.TotalNodes < 0 || *raw.NodesReady > *raw.TotalNodes {
		return snapshot.Power{}, false, validationError("power node count")
	}
	return snapshot.Power{ControlPlaneReady: *raw.ControlPlaneReady, NodesReady: *raw.NodesReady, TotalNodes: *raw.TotalNodes}, true, nil
}

func normalizeClusterState(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "available") {
		return "available"
	}
	return "unknown"
}

var nodeStates = map[string]string{
	"maintenancemode":    "maintenance_mode",
	"schedulingdisabled": "scheduling_disabled",
	"notready":           "not_ready",
	"ready":              "ready",
	"restarting":         "restarting",
	"shuttingdown":       "shutting_down",
	"poweredoff":         "powered_off",
	"poweringon":         "powering_on",
}

func normalizeNodeState(value string) string {
	key := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(value)))
	if normalized, ok := nodeStates[key]; ok {
		return normalized
	}
	return "unknown"
}

func normalizeConditionName(value string) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "")) {
	case "diskpressure":
		return "disk_pressure"
	case "memorypressure":
		return "memory_pressure"
	default:
		return ""
	}
}

func conditionValue(value string) (bool, bool) {
	if strings.EqualFold(value, "true") {
		return true, true
	}
	if strings.EqualFold(value, "false") {
		return false, true
	}
	return false, false
}

func cpu(value *string) (snapshot.OptionalFloat, error) {
	return quantity(value)
}

func bytesQuantity(value *string) (snapshot.OptionalFloat, error) {
	return quantity(value)
}

func quantity(value *string) (snapshot.OptionalFloat, error) {
	if value == nil {
		return snapshot.OptionalFloat{}, nil
	}
	parsed, err := resource.ParseQuantity(*value)
	if err != nil {
		return snapshot.OptionalFloat{}, err
	}
	number := parsed.AsApproximateFloat64()
	if number < 0 || math.IsInf(number, 0) || math.IsNaN(number) {
		return snapshot.OptionalFloat{}, fmt.Errorf("quantity is outside the supported range")
	}
	return snapshot.OptionalFloat{Value: number, Valid: true}, nil
}

func boolPointer(value bool) *bool { return &value }
