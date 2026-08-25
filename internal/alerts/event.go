package alerts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

const (
	SchemaVersion = "1.0"
	EventType     = "ddae.serviceability_alert.upsert"
	SourceSystem  = "dell_ddae"
	MaxEventBytes = 256 * 1024
)

type Event struct {
	SchemaVersion     string `json:"schema_version"`
	EventType         string `json:"event_type"`
	SourceSystem      string `json:"source_system"`
	SourceInstance    string `json:"source_instance"`
	AlertID           string `json:"alert_id"`
	ContentHashSHA256 string `json:"content_hash_sha256"`
	ObservedAt        string `json:"observed_at"`
	Alert             Alert  `json:"alert"`
}

type Alert struct {
	Severity            string         `json:"severity"`
	Acknowledged        *bool          `json:"acknowledged,omitempty"`
	OccurrenceCount     *int64         `json:"occurrence_count,omitempty"`
	CreatedAt           *string        `json:"created_at,omitempty"`
	UpdatedAt           *string        `json:"updated_at,omitempty"`
	ClearType           *string        `json:"clear_type,omitempty"`
	AutoClearTimeoutRaw *int64         `json:"auto_clear_timeout_raw,omitempty"`
	AppName             *string        `json:"app_name,omitempty"`
	Component           *string        `json:"component,omitempty"`
	Namespace           *string        `json:"namespace,omitempty"`
	Message             *string        `json:"message,omitempty"`
	Reason              *string        `json:"reason,omitempty"`
	Remedies            []string       `json:"remedies,omitempty"`
	ResourceID          *string        `json:"resource_id,omitempty"`
	SymptomID           *string        `json:"symptom_id,omitempty"`
	Related             *string        `json:"related,omitempty"`
	RelatedEvents       []RelatedEvent `json:"related_events,omitempty"`
}

type RelatedEvent Alert

type EncodedEvent struct {
	Event       Event
	Payload     []byte
	RecordKey   []byte
	ContentHash string
}

type ValidationError struct{ field string }

func (e ValidationError) Error() string                     { return e.field + " validation failed" }
func (e ValidationError) FailureClass() observability.Class { return observability.ClassValidation }

func BuildEvent(sourceInstance, requestedID string, detail ddae.AlertDetail, observedAt time.Time) (EncodedEvent, error) {
	if err := validateSourceInstance(sourceInstance); err != nil {
		return EncodedEvent{}, err
	}
	if err := ddae.ValidateAlertID(requestedID); err != nil {
		return EncodedEvent{}, err
	}
	if detail.ID != requestedID {
		return EncodedEvent{}, ValidationError{field: "alert_id"}
	}
	alert, err := normalizeAlert(detail)
	if err != nil {
		return EncodedEvent{}, err
	}
	canonical, err := json.Marshal(alert)
	if err != nil {
		return EncodedEvent{}, ValidationError{field: "alert"}
	}
	hash := sha256.Sum256(canonical)
	hashText := hex.EncodeToString(hash[:])
	event := Event{
		SchemaVersion:     SchemaVersion,
		EventType:         EventType,
		SourceSystem:      SourceSystem,
		SourceInstance:    sourceInstance,
		AlertID:           requestedID,
		ContentHashSHA256: hashText,
		ObservedAt:        observedAt.UTC().Format(time.RFC3339Nano),
		Alert:             alert,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return EncodedEvent{}, ValidationError{field: "event"}
	}
	if len(payload) > MaxEventBytes {
		return EncodedEvent{}, ValidationError{field: "event_size"}
	}
	keyHash := sha256.New()
	_, _ = keyHash.Write([]byte(sourceInstance))
	_, _ = keyHash.Write([]byte{0})
	_, _ = keyHash.Write([]byte(requestedID))
	return EncodedEvent{
		Event:       event,
		Payload:     payload,
		RecordKey:   []byte(hex.EncodeToString(keyHash.Sum(nil))),
		ContentHash: hashText,
	}, nil
}

func normalizeAlert(detail ddae.AlertDetail) (Alert, error) {
	alert := Alert{Severity: normalizeSeverity(detail.Type)}
	var err error
	if alert.Acknowledged, err = optionalBool(detail.Acknowledged, "acknowledged"); err != nil {
		return Alert{}, err
	}
	if detail.Count != nil {
		if *detail.Count < 0 {
			return Alert{}, ValidationError{field: "occurrence_count"}
		}
		alert.OccurrenceCount = copyInt(detail.Count)
	}
	if alert.CreatedAt, err = optionalTimestamp(detail.CreatedOn, "created_at"); err != nil {
		return Alert{}, err
	}
	if alert.UpdatedAt, err = optionalTimestamp(detail.UpdatedOn, "updated_at"); err != nil {
		return Alert{}, err
	}
	if alert.ClearType, err = optionalString(detail.ClearType, 64, "clear_type"); err != nil {
		return Alert{}, err
	}
	if detail.AutoClearTimeout != nil {
		if *detail.AutoClearTimeout < 0 {
			return Alert{}, ValidationError{field: "auto_clear_timeout_raw"}
		}
		alert.AutoClearTimeoutRaw = copyInt(detail.AutoClearTimeout)
	}
	fields := []struct {
		source *string
		limit  int
		name   string
		target **string
	}{
		{detail.AppName, 256, "app_name", &alert.AppName},
		{detail.Component, 256, "component", &alert.Component},
		{detail.Namespace, 256, "namespace", &alert.Namespace},
		{detail.Message, 8192, "message", &alert.Message},
		{detail.Reason, 4096, "reason", &alert.Reason},
		{detail.ResourceID, 512, "resource_id", &alert.ResourceID},
		{detail.SymptomID, 256, "symptom_id", &alert.SymptomID},
		{detail.Related, 512, "related", &alert.Related},
	}
	for _, field := range fields {
		*field.target, err = optionalString(field.source, field.limit, field.name)
		if err != nil {
			return Alert{}, err
		}
	}
	if len(detail.Remedies) > 32 {
		return Alert{}, ValidationError{field: "remedies"}
	}
	for _, remedy := range detail.Remedies {
		if err := validString(remedy, 2048); err != nil {
			return Alert{}, ValidationError{field: "remedies"}
		}
		alert.Remedies = append(alert.Remedies, remedy)
	}
	if len(detail.Events) > 100 {
		return Alert{}, ValidationError{field: "related_events"}
	}
	for _, raw := range detail.Events {
		related, err := normalizeRelated(raw)
		if err != nil {
			return Alert{}, err
		}
		alert.RelatedEvents = append(alert.RelatedEvents, related)
	}
	return alert, nil
}

func normalizeRelated(raw ddae.RelatedAlertRaw) (RelatedEvent, error) {
	detail := ddae.AlertDetail{
		Type: raw.Type, Acknowledged: raw.Acknowledged, Count: raw.Count,
		CreatedOn: raw.CreatedOn, UpdatedOn: raw.UpdatedOn, ClearType: raw.ClearType,
		AutoClearTimeout: raw.AutoClearTimeout, AppName: raw.AppName,
		Component: raw.Component, Namespace: raw.Namespace, Message: raw.Message,
		Reason: raw.Reason, Remedies: raw.Remedies, ResourceID: raw.ResourceID,
		SymptomID: raw.SymptomID, Related: raw.Related,
	}
	alert, err := normalizeAlert(detail)
	return RelatedEvent(alert), err
}

func normalizeSeverity(value *string) string {
	if value == nil {
		return "unknown"
	}
	switch strings.ToLower(strings.TrimSpace(*value)) {
	case "critical", "error", "warning", "info", "normal":
		return strings.ToLower(strings.TrimSpace(*value))
	default:
		return "unknown"
	}
}

func optionalBool(value *string, field string) (*bool, error) {
	if value == nil {
		return nil, nil
	}
	var result bool
	switch strings.ToLower(strings.TrimSpace(*value)) {
	case "true":
		result = true
	case "false":
		result = false
	default:
		return nil, ValidationError{field: field}
	}
	return &result, nil
}

func optionalTimestamp(value *string, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil, ValidationError{field: field}
	}
	result := parsed.UTC().Format(time.RFC3339Nano)
	return &result, nil
}

func optionalString(value *string, limit int, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	if err := validString(*value, limit); err != nil {
		return nil, ValidationError{field: field}
	}
	result := *value
	return &result, nil
}

func validString(value string, limit int) error {
	if !utf8.ValidString(value) || len(value) > limit || strings.ContainsRune(value, '\x00') {
		return errors.New("invalid string")
	}
	return nil
}

func validateSourceInstance(value string) error {
	if err := validString(value, 128); err != nil || value == "" || strings.Contains(value, "://") || strings.ContainsAny(value, "\r\n") {
		return ValidationError{field: "source_instance"}
	}
	return nil
}

func copyInt(value *int64) *int64 {
	copy := *value
	return &copy
}

var _ = fmt.Sprintf
