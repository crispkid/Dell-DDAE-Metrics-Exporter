package alerts

import (
	"strings"
	"time"
)

type detailTask struct {
	id          string
	marker      string
	priority    int
	lastFetched time.Time
}

// usableMarker accepts only an unambiguous RFC 3339 timestamp. Empty or
// malformed list markers never suppress the bounded periodic detail refresh.
func usableMarker(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}
