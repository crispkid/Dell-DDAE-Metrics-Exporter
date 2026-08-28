package serviceability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

func TestCanonicalHashIgnoresSourceOrderAndUnknownNoise(t *testing.T) {
	inputs := []string{
		`{"id":"log-1","message":"same","component":"component","unknown":{"x":1}}`,
		`{"component":"component","labels":{"noise":true},"message":"same","id":"log-1"}`,
	}
	var encoded []EncodedEvent
	for _, input := range inputs {
		var detail ddae.ServiceabilityLogDetail
		if err := json.Unmarshal([]byte(input), &detail); err != nil {
			t.Fatal(err)
		}
		value, err := BuildEvent("site-a", "log-1", detail, time.Unix(1, 0))
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, value)
	}
	if encoded[0].ContentHash != encoded[1].ContentHash {
		t.Fatalf("semantic hash differs: %s %s", encoded[0].ContentHash, encoded[1].ContentHash)
	}
	changed := ddae.ServiceabilityLogDetail{ID: "log-1", Message: stringPointer("changed"), Component: stringPointer("component")}
	changedEvent, err := BuildEvent("site-a", "log-1", changed, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if changedEvent.ContentHash == encoded[0].ContentHash {
		t.Fatal("allowed field change did not change hash")
	}
	key := sha256.Sum256([]byte("site-a\x00serviceability_log\x00log-1"))
	if string(encoded[0].RecordKey) != hex.EncodeToString(key[:]) {
		t.Fatalf("record key = %s", encoded[0].RecordKey)
	}
}
