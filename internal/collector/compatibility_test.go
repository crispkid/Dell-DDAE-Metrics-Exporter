package collector

import (
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func text(value string) *string { return &value }

func TestNormalizeClustersUnknownStateAndMissingOptional(t *testing.T) {
	clusters, usable, err := normalizeClusters([]ddae.Cluster{{ID: "cluster-1", ClusterStatus: "future-state"}})
	if !usable || err != nil {
		t.Fatalf("normalize: usable=%v err=%v", usable, err)
	}
	if clusters[0].State != "unknown" || clusters[0].CoordinatorCPU.Valid {
		t.Fatalf("cluster = %#v", clusters[0])
	}
}

func TestNormalizeClustersRejectsDuplicateIdentity(t *testing.T) {
	_, usable, err := normalizeClusters([]ddae.Cluster{{ID: "same"}, {ID: "same"}})
	if usable || err == nil {
		t.Fatalf("expected unusable duplicate, usable=%v err=%v", usable, err)
	}
}

func TestNormalizeQuantitiesAndNodeConditions(t *testing.T) {
	nodes, usable, err := normalizeNodes([]ddae.InfrastructureNode{{
		ID:    "node-1",
		State: "Ready",
		Capacity: ddae.ResourceQuantities{
			CPU: text("2500m"), Memory: text("2Gi"), EphemeralStorage: text("10Gi"),
		},
		Allocatable: ddae.ResourceQuantities{CPU: text("2"), Memory: text("1Gi")},
		Conditions:  []ddae.NodeCondition{{Type: "DiskPressure", Status: "False"}},
	}})
	if !usable || err != nil {
		t.Fatalf("normalize: usable=%v err=%v", usable, err)
	}
	node := nodes[0]
	if node.CapacityCPU.Value != 2.5 || node.CapacityMemory.Value != 2*1024*1024*1024 {
		t.Fatalf("quantities = %#v", node)
	}
	if node.DiskPressure == nil || *node.DiskPressure {
		t.Fatalf("condition = %#v", node.DiskPressure)
	}
}

func TestValidationErrorIsBoundedlyClassified(t *testing.T) {
	err := validationError("test")
	if err.Error() == "" || err.FailureClass() == "" {
		t.Fatal("validation error lacks classification")
	}
}
