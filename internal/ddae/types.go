package ddae

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type PingResponse struct {
	Status string `json:"status"`
}

type Cluster struct {
	ID            string         `json:"id"`
	ClusterStatus string         `json:"clusterStatus"`
	Coordinator   ResourceConfig `json:"coordinator"`
	Worker        ResourceConfig `json:"worker"`
}

type ResourceConfig struct {
	CPU    *string `json:"cpu"`
	Memory *string `json:"memory"`
}

type InfrastructureNode struct {
	ID          string             `json:"id"`
	State       string             `json:"state"`
	Capacity    ResourceQuantities `json:"capacity"`
	Allocatable ResourceQuantities `json:"allocatable"`
	Conditions  []NodeCondition    `json:"conditions"`
}

type ResourceQuantities struct {
	CPU              *string `json:"cpu"`
	Memory           *string `json:"memory"`
	EphemeralStorage *string `json:"ephemeral-storage"`
}

type NodeCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type LockResponse struct {
	Status BoolStatus `json:"status"`
}

type BoolStatus struct {
	value bool
	valid bool
}

func (s BoolStatus) Value() (bool, bool) { return s.value, s.valid }

func (s *BoolStatus) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("true")) {
		s.value, s.valid = true, true
		return nil
	}
	if bytes.Equal(trimmed, []byte("false")) {
		s.value, s.valid = false, true
		return nil
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return errors.New("status must be a boolean or bounded status string")
	}
	switch strings.ToLower(value) {
	case "locked", "true":
		s.value, s.valid = true, true
	case "unlocked", "false":
		s.value, s.valid = false, true
	default:
		return fmt.Errorf("unsupported lock status")
	}
	return nil
}

type PowerResponse struct {
	ControlPlaneReady *bool  `json:"controlPlaneReady"`
	NodesReady        *int64 `json:"nodesReady"`
	TotalNodes        *int64 `json:"totalNodes"`
}

type AlertList struct {
	Results      []AlertListItem `json:"results"`
	Threshold    *int64          `json:"threshold"`
	TotalRecords *int64          `json:"totalRecords"`
}

type AlertListItem struct {
	ID        string  `json:"id"`
	UpdatedOn *string `json:"updatedon"`
}

type AlertDetail struct {
	ID               string            `json:"id"`
	Type             *string           `json:"type"`
	Acknowledged     *string           `json:"acknowledged"`
	Count            *int64            `json:"count"`
	CreatedOn        *string           `json:"createdon"`
	UpdatedOn        *string           `json:"updatedon"`
	ClearType        *string           `json:"clearType"`
	AutoClearTimeout *int64            `json:"autoClearTimeOut"`
	AppName          *string           `json:"appname"`
	Component        *string           `json:"component"`
	Namespace        *string           `json:"namespace"`
	Message          *string           `json:"message"`
	Reason           *string           `json:"reason"`
	Remedies         []string          `json:"remedies"`
	ResourceID       *string           `json:"resourceID"`
	SymptomID        *string           `json:"symptomid"`
	Related          *string           `json:"related"`
	Events           []RelatedAlertRaw `json:"events"`
}

type ServiceabilityLogList struct {
	Results      []ServiceabilityLogListItem `json:"results"`
	Threshold    *int64                      `json:"threshold"`
	TotalRecords *int64                      `json:"totalRecords"`
	// Malformed records whether the weakly typed list omitted or malformed any
	// field needed to prove that the returned ID set is complete. Valid items
	// remain available so the caller can safely refresh those details.
	Malformed bool `json:"-"`
}

func (l *ServiceabilityLogList) UnmarshalJSON(data []byte) error {
	var raw struct {
		Results      json.RawMessage `json:"results"`
		Threshold    json.RawMessage `json:"threshold"`
		TotalRecords json.RawMessage `json:"totalRecords"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*l = ServiceabilityLogList{}

	var results []json.RawMessage
	if isMissingOrNull(raw.Results) || json.Unmarshal(raw.Results, &results) != nil {
		l.Malformed = true
	} else {
		for _, encoded := range results {
			var item ServiceabilityLogListItem
			if err := json.Unmarshal(encoded, &item); err != nil {
				l.Malformed = true
				continue
			}
			l.Results = append(l.Results, item)
		}
	}

	if !isMissingOrNull(raw.Threshold) {
		var threshold int64
		if err := json.Unmarshal(raw.Threshold, &threshold); err != nil {
			l.Malformed = true
		} else {
			l.Threshold = &threshold
		}
	}
	if isMissingOrNull(raw.TotalRecords) {
		l.Malformed = true
	} else {
		var total int64
		if err := json.Unmarshal(raw.TotalRecords, &total); err != nil {
			l.Malformed = true
		} else {
			l.TotalRecords = &total
		}
	}
	return nil
}

func isMissingOrNull(data json.RawMessage) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

type ServiceabilityLogListItem struct {
	ID        string  `json:"id"`
	UpdatedOn *string `json:"updatedon"`
}

// ServiceabilityLogDetail deliberately models only the documented DDAE-4
// allowlist. encoding/json ignores labels, links, and unknown source fields.
type ServiceabilityLogDetail struct {
	ID           string   `json:"id"`
	Type         *string  `json:"type"`
	Acknowledged *string  `json:"acknowledged"`
	Count        *int64   `json:"count"`
	CreatedOn    *string  `json:"createdon"`
	UpdatedOn    *string  `json:"updatedon"`
	AppName      *string  `json:"appname"`
	Component    *string  `json:"component"`
	Namespace    *string  `json:"namespace"`
	Message      *string  `json:"message"`
	Reason       *string  `json:"reason"`
	Remedies     []string `json:"remedies"`
	ResourceID   *string  `json:"resourceID"`
	SymptomID    *string  `json:"symptomid"`
	Related      *string  `json:"related"`
}

type RelatedAlertRaw struct {
	Type             *string  `json:"type"`
	Acknowledged     *string  `json:"acknowledged"`
	Count            *int64   `json:"count"`
	CreatedOn        *string  `json:"createdon"`
	UpdatedOn        *string  `json:"updatedon"`
	ClearType        *string  `json:"clearType"`
	AutoClearTimeout *int64   `json:"autoClearTimeOut"`
	AppName          *string  `json:"appname"`
	Component        *string  `json:"component"`
	Namespace        *string  `json:"namespace"`
	Message          *string  `json:"message"`
	Reason           *string  `json:"reason"`
	Remedies         []string `json:"remedies"`
	ResourceID       *string  `json:"resourceID"`
	SymptomID        *string  `json:"symptomid"`
	Related          *string  `json:"related"`
}
