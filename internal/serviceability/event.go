package serviceability

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

const (
	SchemaVersion = "1.0"
	EventType     = "ddae.serviceability_log.upsert"
	SourceSystem  = "dell_ddae"
	RecordKind    = "serviceability_log"
	MaxEventBytes = 256 * 1024
)

type Event struct {
	SchemaVersion     string `json:"schema_version"`
	EventType         string `json:"event_type"`
	SourceSystem      string `json:"source_system"`
	SourceInstance    string `json:"source_instance"`
	LogID             string `json:"log_id"`
	ContentHashSHA256 string `json:"content_hash_sha256"`
	ObservedAt        string `json:"observed_at"`
	Log               Log    `json:"log"`
}

type Log struct {
	Severity        string   `json:"severity"`
	Acknowledged    *bool    `json:"acknowledged,omitempty"`
	OccurrenceCount *int64   `json:"occurrence_count,omitempty"`
	CreatedAt       *string  `json:"created_at,omitempty"`
	UpdatedAt       *string  `json:"updated_at,omitempty"`
	AppName         *string  `json:"app_name,omitempty"`
	Component       *string  `json:"component,omitempty"`
	Namespace       *string  `json:"namespace,omitempty"`
	Message         *string  `json:"message,omitempty"`
	Reason          *string  `json:"reason,omitempty"`
	Remedies        []string `json:"remedies,omitempty"`
	ResourceID      *string  `json:"resource_id,omitempty"`
	SymptomID       *string  `json:"symptom_id,omitempty"`
	Related         *string  `json:"related,omitempty"`
}

type EncodedEvent struct {
	Event       Event
	Payload     []byte
	RecordKey   []byte
	ContentHash string
}

type ValidationError struct{ field string }

func (e ValidationError) Error() string                     { return e.field + " validation failed" }
func (e ValidationError) FailureClass() observability.Class { return observability.ClassValidation }

func BuildEvent(sourceInstance, requestedID string, detail ddae.ServiceabilityLogDetail, observedAt time.Time) (EncodedEvent, error) {
	if err := validateSourceInstance(sourceInstance); err != nil {
		return EncodedEvent{}, err
	}
	if err := ddae.ValidateServiceabilityLogID(requestedID); err != nil {
		return EncodedEvent{}, err
	}
	if detail.ID != requestedID {
		return EncodedEvent{}, ValidationError{field: "log_id"}
	}
	logValue, err := normalizeLog(detail)
	if err != nil {
		return EncodedEvent{}, err
	}
	canonical, err := json.Marshal(logValue)
	if err != nil {
		return EncodedEvent{}, ValidationError{field: "log"}
	}
	hash := sha256.Sum256(canonical)
	contentHash := hex.EncodeToString(hash[:])
	event := Event{
		SchemaVersion: SchemaVersion, EventType: EventType, SourceSystem: SourceSystem,
		SourceInstance: sourceInstance, LogID: requestedID, ContentHashSHA256: contentHash,
		ObservedAt: observedAt.UTC().Format(time.RFC3339Nano), Log: logValue,
	}
	payload, err := json.Marshal(event)
	if err != nil || len(payload) > MaxEventBytes {
		return EncodedEvent{}, ValidationError{field: "event_size"}
	}
	keyHash := sha256.New()
	_, _ = keyHash.Write([]byte(sourceInstance))
	_, _ = keyHash.Write([]byte{0})
	_, _ = keyHash.Write([]byte(RecordKind))
	_, _ = keyHash.Write([]byte{0})
	_, _ = keyHash.Write([]byte(requestedID))
	return EncodedEvent{
		Event: event, Payload: payload, RecordKey: []byte(hex.EncodeToString(keyHash.Sum(nil))),
		ContentHash: contentHash,
	}, nil
}

func DecodeStoredEvent(payload []byte) (Event, error) {
	if len(payload) == 0 || len(payload) > MaxEventBytes || !utf8.Valid(payload) {
		return Event{}, ValidationError{field: "event_size"}
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var event Event
	if err := decoder.Decode(&event); err != nil {
		return Event{}, ValidationError{field: "event"}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Event{}, ValidationError{field: "event"}
	}
	if err := ValidateStoredEvent(event); err != nil {
		return Event{}, err
	}
	canonical, err := json.Marshal(event)
	if err != nil || !bytes.Equal(canonical, payload) {
		return Event{}, ValidationError{field: "event_canonical"}
	}
	return event, nil
}

func ValidateStoredEvent(event Event) error {
	if event.SchemaVersion != SchemaVersion || event.EventType != EventType || event.SourceSystem != SourceSystem {
		return ValidationError{field: "event_identity"}
	}
	if err := validateSourceInstance(event.SourceInstance); err != nil {
		return err
	}
	if err := ddae.ValidateServiceabilityLogID(event.LogID); err != nil {
		return err
	}
	if !validLowerHash(event.ContentHashSHA256) {
		return ValidationError{field: "content_hash_sha256"}
	}
	parsed, err := time.Parse(time.RFC3339Nano, event.ObservedAt)
	if err != nil || parsed.UTC().Format(time.RFC3339Nano) != event.ObservedAt {
		return ValidationError{field: "observed_at"}
	}
	if err := validateLog(event.Log); err != nil {
		return err
	}
	canonical, err := json.Marshal(event.Log)
	if err != nil {
		return ValidationError{field: "log"}
	}
	hash := sha256.Sum256(canonical)
	if hex.EncodeToString(hash[:]) != event.ContentHashSHA256 {
		return ValidationError{field: "content_hash_sha256"}
	}
	return nil
}

func normalizeLog(detail ddae.ServiceabilityLogDetail) (Log, error) {
	result := Log{Severity: normalizeSeverity(detail.Type)}
	var err error
	if result.Acknowledged, err = normalizeBoolean(detail.Acknowledged); err != nil {
		return Log{}, err
	}
	if detail.Count != nil {
		value := *detail.Count
		if value < 0 {
			return Log{}, ValidationError{field: "occurrence_count"}
		}
		result.OccurrenceCount = &value
	}
	if result.CreatedAt, err = normalizeTime(detail.CreatedOn, "created_at"); err != nil {
		return Log{}, err
	}
	if result.UpdatedAt, err = normalizeTime(detail.UpdatedOn, "updated_at"); err != nil {
		return Log{}, err
	}
	for _, field := range []struct {
		source *string
		target **string
		limit  int
		name   string
	}{
		{detail.AppName, &result.AppName, 256, "app_name"},
		{detail.Component, &result.Component, 256, "component"},
		{detail.Namespace, &result.Namespace, 256, "namespace"},
		{detail.Message, &result.Message, 8192, "message"},
		{detail.Reason, &result.Reason, 4096, "reason"},
		{detail.ResourceID, &result.ResourceID, 512, "resource_id"},
		{detail.SymptomID, &result.SymptomID, 256, "symptom_id"},
		{detail.Related, &result.Related, 512, "related"},
	} {
		if field.source == nil {
			continue
		}
		if err := validString(*field.source, field.limit); err != nil {
			return Log{}, ValidationError{field: field.name}
		}
		value := *field.source
		*field.target = &value
	}
	if detail.Remedies != nil {
		if len(detail.Remedies) > 32 {
			return Log{}, ValidationError{field: "remedies"}
		}
		result.Remedies = append([]string(nil), detail.Remedies...)
		for _, remedy := range result.Remedies {
			if err := validString(remedy, 2048); err != nil {
				return Log{}, ValidationError{field: "remedies"}
			}
		}
	}
	return result, nil
}

func validateLog(value Log) error {
	switch value.Severity {
	case "critical", "error", "warning", "info", "normal", "unknown":
	default:
		return ValidationError{field: "severity"}
	}
	if value.OccurrenceCount != nil && *value.OccurrenceCount < 0 {
		return ValidationError{field: "occurrence_count"}
	}
	for _, field := range []struct {
		value *string
		limit int
		name  string
	}{
		{value.AppName, 256, "app_name"}, {value.Component, 256, "component"},
		{value.Namespace, 256, "namespace"}, {value.Message, 8192, "message"},
		{value.Reason, 4096, "reason"}, {value.ResourceID, 512, "resource_id"},
		{value.SymptomID, 256, "symptom_id"}, {value.Related, 512, "related"},
	} {
		if field.value != nil && validString(*field.value, field.limit) != nil {
			return ValidationError{field: field.name}
		}
	}
	for _, field := range []struct {
		value *string
		name  string
	}{{value.CreatedAt, "created_at"}, {value.UpdatedAt, "updated_at"}} {
		if field.value == nil {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, *field.value)
		if err != nil || parsed.UTC().Format(time.RFC3339Nano) != *field.value {
			return ValidationError{field: field.name}
		}
	}
	if len(value.Remedies) > 32 {
		return ValidationError{field: "remedies"}
	}
	for _, remedy := range value.Remedies {
		if validString(remedy, 2048) != nil {
			return ValidationError{field: "remedies"}
		}
	}
	return nil
}

func normalizeSeverity(value *string) string {
	if value == nil {
		return "unknown"
	}
	normalized := strings.ToLower(*value)
	switch normalized {
	case "critical", "error", "warning", "info", "normal", "unknown":
		return normalized
	default:
		return "unknown"
	}
}

func normalizeBoolean(value *string) (*bool, error) {
	if value == nil {
		return nil, nil
	}
	var result bool
	switch strings.ToLower(*value) {
	case "true":
		result = true
	case "false":
		result = false
	default:
		return nil, ValidationError{field: "acknowledged"}
	}
	return &result, nil
}

func normalizeTime(value *string, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil, ValidationError{field: field}
	}
	normalized := parsed.UTC().Format(time.RFC3339Nano)
	return &normalized, nil
}

func validString(value string, limit int) error {
	if !utf8.ValidString(value) || len(value) > limit || strings.ContainsRune(value, '\x00') {
		return errors.New("invalid string")
	}
	return nil
}

func validateSourceInstance(value string) error {
	if len(value) < 1 || len(value) > 128 || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, "\r\n") || strings.Contains(value, "://") {
		return ValidationError{field: "source_instance"}
	}
	return nil
}

func validLowerHash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}
