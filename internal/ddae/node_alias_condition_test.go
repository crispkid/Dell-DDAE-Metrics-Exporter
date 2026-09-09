package ddae

import (
	"encoding/json"
	"testing"
)

func TestNodeResourceStorageAliases(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "documented", input: `{"ephemeralStorage":"100Gi"}`, want: "100Gi"},
		{name: "legacy", input: `{"ephemeral-storage":"100Gi"}`, want: "100Gi"},
		{name: "equal-dual", input: `{"ephemeralStorage":"100Gi","ephemeral-storage":"100Gi"}`, want: "100Gi"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var quantities ResourceQuantities
			if err := json.Unmarshal([]byte(test.input), &quantities); err != nil {
				t.Fatal(err)
			}
			if quantities.EphemeralStorage == nil || *quantities.EphemeralStorage != test.want {
				t.Fatalf("storage = %#v", quantities.EphemeralStorage)
			}
		})
	}

	var quantities ResourceQuantities
	if err := json.Unmarshal([]byte(`{"ephemeralStorage":"100Gi","ephemeral-storage":"99Gi"}`), &quantities); err == nil {
		t.Fatal("conflicting storage aliases were accepted")
	}
}

func TestNodeConditionsAcceptDocumentedObjectAndLegacyArray(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
	}{
		{name: "documented", input: `{"conditions":{"diskPressure":"False","memoryPressure":"True","future":"ignored"}}`},
		{name: "legacy", input: `{"conditions":[{"type":"DiskPressure","status":"False"},{"type":"MemoryPressure","status":"True"}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var node InfrastructureNode
			if err := json.Unmarshal([]byte(test.input), &node); err != nil {
				t.Fatal(err)
			}
			if len(node.Conditions) != 2 || node.Conditions[0].Type != "DiskPressure" || node.Conditions[0].Status != "False" ||
				node.Conditions[1].Type != "MemoryPressure" || node.Conditions[1].Status != "True" {
				t.Fatalf("conditions = %#v", node.Conditions)
			}
		})
	}

	for _, input := range []string{
		`{"conditions":{"diskPressure":true}}`,
		`{"conditions":"False"}`,
		`{"conditions":[{"type":"DiskPressure","status":false}]}`,
	} {
		var node InfrastructureNode
		if err := json.Unmarshal([]byte(input), &node); err == nil {
			t.Fatalf("accepted invalid conditions %s", input)
		}
	}
}
