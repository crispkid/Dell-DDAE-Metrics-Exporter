package portable

import (
	"bytes"
	"encoding/json"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/alerts"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/collector"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/serviceability"
	"sort"
	"time"
)

type ParseResult struct {
	DecodeOK     bool     `json:"decode_ok"`
	ValidationOK bool     `json:"validation_ok"`
	ContractOK   bool     `json:"contract_ok"`
	Shape        string   `json:"top_level_type"`
	Reason       string   `json:"reason,omitempty"`
	Available    int      `json:"available_ids"`
	Total        *int64   `json:"total_records,omitempty"`
	IDs          []string `json:"-"`
}

func Parse(op, id string, body []byte) (ParseResult, any) {
	r := ParseResult{Shape: shape(body)}
	value, err := ddae.DecodeRecorded(op, id, body)
	if err != nil {
		r.Reason = "production_decode_or_validation"
		return r, nil
	}
	r.DecodeOK = true
	r.ValidationOK = true
	r.ContractOK = true
	if collector.ValidateDiagnostic(value) != nil {
		r.ValidationOK = false
		r.Reason = "resource_validation"
	}
	switch v := value.(type) {
	case ddae.AlertDetail:
		if _, err := alerts.BuildEvent("portable-diagnostics", id, v, time.Unix(0, 0).UTC()); err != nil {
			r.ValidationOK = false
			r.Reason = "detail_validation"
		}
	case ddae.ServiceabilityLogDetail:
		if _, err := serviceability.BuildEvent("portable-diagnostics", id, v, time.Unix(0, 0).UTC()); err != nil {
			r.ValidationOK = false
			r.Reason = "detail_validation"
		}
	case ddae.AlertList:
		for _, item := range v.Results {
			r.IDs = append(r.IDs, item.ID)
		}
		r.Total = v.TotalRecords
		checkList(&r, body, false)
	case ddae.ServiceabilityLogList:
		for _, item := range v.Results {
			r.IDs = append(r.IDs, item.ID)
		}
		r.Total = v.TotalRecords
		checkList(&r, body, v.Malformed)
	case []ddae.Cluster:
		if r.Shape != "array" {
			r.ContractOK = false
			r.Reason = "required_array"
		}
	}
	return r, value
}
func shape(body []byte) string {
	b := bytes.TrimSpace(body)
	if !json.Valid(b) {
		return "invalid"
	}
	if len(b) == 0 {
		return "invalid"
	}
	switch b[0] {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 'n':
		return "null"
	case 't', 'f':
		return "boolean"
	default:
		return "number"
	}
}
func checkList(r *ParseResult, body []byte, malformed bool) {
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || shape(object["results"]) != "array" || r.Total == nil || *r.Total < 0 || malformed {
		r.ContractOK = false
	}
	seen := map[string]bool{}
	unique := []string{}
	for _, id := range r.IDs {
		if ddae.ValidateAlertID(id) != nil || seen[id] {
			r.ContractOK = false
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	sort.Strings(unique)
	r.IDs = unique
	r.Available = len(unique)
	if r.Total != nil && *r.Total != int64(len(unique)) {
		r.ContractOK = false
	}
	if !r.ContractOK {
		r.Reason = "list_incomplete_or_malformed"
	}
}
