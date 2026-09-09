package portable

import "testing"

func TestPortableProductionParserParity(t *testing.T) {
	for _, tc := range []struct {
		op, body         string
		decode, contract bool
	}{
		{"nodes", `{"results":[]}`, true, true},
		{"nodes", `{"results":null}`, false, false},
		{"clusters", `{"results":[]}`, false, false},
		{"clusters", `[]`, true, true},
		{"alert_list", `{"totalRecords":0}`, true, false},
		{"alert_list", `{"results":[],"totalRecords":0}`, true, true},
		{"alert_list", `{"results":null,"totalRecords":0}`, true, false},
		{"alert_list", `{"results":[{"id":"a"},{"id":"a"}],"totalRecords":2}`, true, false},
		{"alert_list", `{"results":[{"id":"a"}],"totalRecords":1167}`, true, false},
		{"serviceability_log_list", `{"results":[],"totalRecords":1}`, true, false},
		{"nodes", `{"results":[]} trailing`, false, false},
		{"ping", `not-json`, false, false},
	} {
		r, _ := Parse(tc.op, "", []byte(tc.body))
		if r.DecodeOK != tc.decode || r.ContractOK != tc.contract {
			t.Errorf("%s: decode=%v contract=%v", tc.op, r.DecodeOK, r.ContractOK)
		}
	}
	if p, _ := Parse("alert_detail", "expected", []byte(`{"id":"other"}`)); !p.DecodeOK || p.ValidationOK {
		t.Fatal("detail ID mismatch must decode but fail production event validation")
	}
}
