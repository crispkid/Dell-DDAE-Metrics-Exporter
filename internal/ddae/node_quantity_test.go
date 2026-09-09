package ddae

import (
	"encoding/json"
	"testing"
)

func TestNodeCPUAcceptsDocumentedIntegersAndLegacyStrings(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: `0`, want: "0"},
		{input: `12`, want: "12"},
		{input: `"2500m"`, want: "2500m"},
		{input: `"2"`, want: "2"},
	} {
		var quantities ResourceQuantities
		if err := json.Unmarshal([]byte(`{"cpu":`+test.input+`}`), &quantities); err != nil {
			t.Fatalf("cpu %s: %v", test.input, err)
		}
		if quantities.CPU == nil || *quantities.CPU != test.want {
			t.Fatalf("cpu %s decoded as %#v", test.input, quantities.CPU)
		}
	}
}

func TestNodeCPURejectsInvalidRepresentations(t *testing.T) {
	for _, input := range []string{`-1`, `1.5`, `1e3`, `true`, `null`, `[]`, `{}`} {
		var quantities ResourceQuantities
		if err := json.Unmarshal([]byte(`{"cpu":`+input+`}`), &quantities); err == nil {
			t.Fatalf("accepted invalid CPU %s", input)
		}
	}
}
