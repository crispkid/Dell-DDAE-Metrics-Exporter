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

// UnmarshalJSON maps wire variants into the stable DTO used by normalizers.
func (c *Cluster) UnmarshalJSON(data []byte) error {
	body := bytes.TrimSpace(data)
	if len(body) == 0 || body[0] != '{' {
		return errors.New("cluster must be an object")
	}
	var wire struct {
		ID          string          `json:"id"`
		Status      json.RawMessage `json:"clusterStatus"`
		Coordinator ResourceConfig  `json:"coordinator"`
		Worker      ResourceConfig  `json:"worker"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return err
	}
	var status string
	raw := bytes.TrimSpace(wire.Status)
	if !isMissingOrNull(raw) {
		if raw[0] == '{' {
			var object struct {
				Status *string `json:"status"`
			}
			if err := json.Unmarshal(raw, &object); err != nil {
				return err
			}
			if object.Status == nil {
				return errors.New("cluster status object requires status")
			}
			status = *object.Status
		} else if err := json.Unmarshal(raw, &status); err != nil {
			return err
		}
	}
	*c = Cluster{ID: wire.ID, ClusterStatus: status, Coordinator: wire.Coordinator, Worker: wire.Worker}
	return nil
}

// clusterList accepts both API envelope and legacy array responses. Keep this
// decoder shared by live collection and recorded-body replay.
type clusterList []Cluster

func (l *clusterList) UnmarshalJSON(data []byte) error {
	body := bytes.TrimSpace(data)
	if len(body) == 0 {
		return errors.New("cluster response is empty")
	}
	if body[0] == '{' {
		var envelope struct {
			Results json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return err
		}
		body = bytes.TrimSpace(envelope.Results)
	}
	if len(body) == 0 || body[0] != '[' {
		return errors.New("cluster response requires an array")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return err
	}
	clusters := make([]Cluster, 0, len(items))
	for _, item := range items {
		item = bytes.TrimSpace(item)
		if len(item) == 0 || item[0] != '{' {
			return errors.New("cluster must be an object")
		}
		var cluster Cluster
		if err := json.Unmarshal(item, &cluster); err != nil {
			return err
		}
		clusters = append(clusters, cluster)
	}
	*l = clusters
	return nil
}

type ResourceConfig struct {
	CPU    *string `json:"cpu"`
	Memory *string `json:"memory"`
}

// UnmarshalJSON keeps resource layout selection explicit so mixed forms cannot
// silently override each other. Optional legacy null values remain absent.
func (r *ResourceConfig) UnmarshalJSON(data []byte) error {
	body := bytes.TrimSpace(data)
	if bytes.Equal(body, []byte("null")) {
		*r = ResourceConfig{}
		return nil
	}
	if len(body) == 0 || body[0] != '{' {
		return errors.New("cluster resources must be an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return err
	}
	if nested, ok := fields["resources"]; ok {
		_, directCPU := fields["cpu"]
		_, directMemory := fields["memory"]
		if directCPU || directMemory {
			return errors.New("mixed cluster resource layouts")
		}
		nested = bytes.TrimSpace(nested)
		if len(nested) == 0 || nested[0] != '{' {
			return errors.New("nested cluster resources must be an object")
		}
		var selected map[string]json.RawMessage
		if err := json.Unmarshal(nested, &selected); err != nil {
			return err
		}
		fields = selected
	}
	var result ResourceConfig
	if cpu, ok := fields["cpu"]; ok && !isMissingOrNull(cpu) {
		value, err := decodeNodeCPU(cpu)
		if err != nil {
			return errors.New("cluster CPU must be an integer or quantity string")
		}
		result.CPU = value
	}
	if memory, ok := fields["memory"]; ok {
		value, err := decodeOptionalJSONString(memory)
		if err != nil {
			return errors.New("cluster memory must be a quantity string")
		}
		result.Memory = value
	}
	*r = result
	return nil
}

type InfrastructureNode struct {
	ID          string             `json:"id"`
	State       string             `json:"state"`
	Capacity    ResourceQuantities `json:"capacity"`
	Allocatable ResourceQuantities `json:"allocatable"`
	Conditions  []NodeCondition    `json:"conditions"`
}

func (n *InfrastructureNode) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("node must be an object")
	}
	var wire struct {
		ID          string             `json:"id"`
		State       string             `json:"state"`
		Capacity    ResourceQuantities `json:"capacity"`
		Allocatable ResourceQuantities `json:"allocatable"`
		Conditions  json.RawMessage    `json:"conditions"`
	}
	if err := json.Unmarshal(trimmed, &wire); err != nil {
		return err
	}
	conditions, err := decodeNodeConditions(wire.Conditions)
	if err != nil {
		return err
	}
	*n = InfrastructureNode{
		ID: wire.ID, State: wire.State, Capacity: wire.Capacity,
		Allocatable: wire.Allocatable, Conditions: conditions,
	}
	return nil
}

type infrastructureNodeList []InfrastructureNode

func (l *infrastructureNodeList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return errors.New("node response is empty")
	}
	var nodes []InfrastructureNode
	switch trimmed[0] {
	case '[':
		if err := json.Unmarshal(trimmed, &nodes); err != nil {
			return err
		}
	case '{':
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &envelope); err != nil {
			return err
		}
		results, present := envelope["results"]
		if !present || isMissingOrNull(results) {
			return errors.New("node response results are missing")
		}
		if err := json.Unmarshal(results, &nodes); err != nil {
			return err
		}
	default:
		return errors.New("unsupported node response shape")
	}
	*l = nodes
	return nil
}

type ResourceQuantities struct {
	CPU              *string `json:"cpu"`
	Memory           *string `json:"memory"`
	EphemeralStorage *string `json:"ephemeral-storage"`
}

func (q *ResourceQuantities) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("node resources must be an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return err
	}
	var decoded ResourceQuantities
	if raw, present := fields["cpu"]; present {
		cpu, err := decodeNodeCPU(raw)
		if err != nil {
			return err
		}
		decoded.CPU = cpu
	}
	if raw, present := fields["memory"]; present {
		memory, err := decodeOptionalJSONString(raw)
		if err != nil {
			return err
		}
		decoded.Memory = memory
	}
	documented, documentedPresent := fields["ephemeralStorage"]
	legacy, legacyPresent := fields["ephemeral-storage"]
	var documentedValue, legacyValue *string
	var err error
	if documentedPresent {
		documentedValue, err = decodeOptionalJSONString(documented)
		if err != nil {
			return err
		}
	}
	if legacyPresent {
		legacyValue, err = decodeOptionalJSONString(legacy)
		if err != nil {
			return err
		}
	}
	if documentedPresent && legacyPresent && !optionalStringsEqual(documentedValue, legacyValue) {
		return errors.New("conflicting ephemeral storage aliases")
	}
	if documentedPresent {
		decoded.EphemeralStorage = documentedValue
	} else if legacyPresent {
		decoded.EphemeralStorage = legacyValue
	}
	*q = decoded
	return nil
}

func decodeNodeCPU(data json.RawMessage) (*string, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, errors.New("node CPU is missing")
	}
	if trimmed[0] == '"' {
		return decodeOptionalJSONString(trimmed)
	}
	for _, character := range trimmed {
		if character < '0' || character > '9' {
			return nil, errors.New("node CPU must be an integer or quantity string")
		}
	}
	value := string(trimmed)
	return &value, nil
}

func decodeOptionalJSONString(data json.RawMessage) (*string, error) {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, errors.New("node quantity or condition must be a string")
	}
	return &value, nil
}

func optionalStringsEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func decodeNodeConditions(data json.RawMessage) ([]NodeCondition, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var conditions []NodeCondition
		if err := json.Unmarshal(trimmed, &conditions); err != nil {
			return nil, err
		}
		return conditions, nil
	}
	if trimmed[0] != '{' {
		return nil, errors.New("node conditions must be an object or array")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return nil, err
	}
	conditions := make([]NodeCondition, 0, 2)
	for _, field := range []struct {
		name     string
		typeName string
	}{
		{name: "diskPressure", typeName: "DiskPressure"},
		{name: "memoryPressure", typeName: "MemoryPressure"},
	} {
		raw, present := fields[field.name]
		if !present {
			continue
		}
		status, err := decodeOptionalJSONString(raw)
		if err != nil {
			return nil, err
		}
		if status != nil {
			conditions = append(conditions, NodeCondition{Type: field.typeName, Status: *status})
		}
	}
	return conditions, nil
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
