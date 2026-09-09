package config

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type QueryConfig struct {
	Enabled, Events, Insecure                                                 bool
	BaseURL, AuthURL                                                          *url.URL
	Realm, Role, CAFile, AuthCAFile, Topic                                    string
	Username, Password                                                        Secret
	Interval, RequestTimeout, CycleTimeout, StaleAfter, Retention             time.Duration
	ResponseMaxBytes, MaxBytes                                                int64
	MaxHistory, MaxPerCycle, Concurrency, RetryMax, MaxEvents, MaxCheckpoints int
}

func loadQuery(lookup lookupFunc, readFile func(string) ([]byte, error), allow bool) (QueryConfig, error) {
	var q QueryConfig
	var err error
	if q.Enabled, err = boolean(lookup, "QUERY_ENABLED", false); err != nil || !q.Enabled {
		return q, err
	}
	for _, v := range []struct {
		name string
		dst  **url.URL
	}{{"QUERY_BASE_URL", &q.BaseURL}, {"QUERY_AUTH_URL", &q.AuthURL}} {
		*v.dst, err = validateOrigin(optionalText(lookup, v.name, ""))
		if err != nil {
			return q, errors.New(v.name + " must be an HTTPS origin")
		}
	}
	q.Realm = optionalText(lookup, "QUERY_REALM", "")
	q.Role = optionalText(lookup, "QUERY_ROLE", "")
	valid := regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	if !valid.MatchString(q.Realm) || !valid.MatchString(q.Role) {
		return q, errors.New("QUERY_REALM and QUERY_ROLE must be explicit safe names")
	}
	if q.Username, err = loadSecret(lookup, readFile, "QUERY_USERNAME", "QUERY_USERNAME_FILE", true); err != nil {
		return q, err
	}
	if q.Password, err = loadSecret(lookup, readFile, "QUERY_PASSWORD", "QUERY_PASSWORD_FILE", true); err != nil {
		return q, err
	}
	q.CAFile = optionalText(lookup, "QUERY_CA_FILE", "")
	q.AuthCAFile = optionalText(lookup, "QUERY_AUTH_CA_FILE", "")
	if q.Insecure, err = boolean(lookup, "QUERY_TLS_INSECURE_SKIP_VERIFY", false); err != nil {
		return q, err
	}
	if q.Insecure && (!allow || q.CAFile != "" || q.AuthCAFile != "") {
		return q, errors.New("query insecure TLS requires global opt-in and no CA files")
	}
	for _, v := range []struct {
		name string
		dst  *time.Duration
		def  time.Duration
	}{
		{"QUERY_INTERVAL", &q.Interval, 30 * time.Second}, {"QUERY_REQUEST_TIMEOUT", &q.RequestTimeout, 5 * time.Second}, {"QUERY_CYCLE_TIMEOUT", &q.CycleTimeout, 20 * time.Second}, {"QUERY_STALE_AFTER", &q.StaleAfter, 90 * time.Second}, {"QUERY_CHECKPOINT_RETENTION", &q.Retention, 720 * time.Hour},
	} {
		*v.dst, err = duration(lookup, v.name, v.def)
		if err != nil {
			return q, err
		}
	}
	if q.RequestTimeout >= q.CycleTimeout || q.CycleTimeout >= q.Interval || q.StaleAfter <= q.Interval {
		return q, errors.New("query timing must satisfy request < cycle < interval < stale_after")
	}
	for _, v := range []struct {
		name          string
		dst           *int
		def, min, max int
	}{
		{"QUERY_MAX_HISTORY_RECORDS", &q.MaxHistory, 1000, 1, 10000}, {"QUERY_DETAIL_MAX_PER_CYCLE", &q.MaxPerCycle, 100, 1, 10000}, {"QUERY_DETAIL_CONCURRENCY", &q.Concurrency, 4, 1, 32}, {"QUERY_RETRY_MAX", &q.RetryMax, 2, 0, 10}, {"QUERY_OUTBOX_MAX_EVENTS", &q.MaxEvents, 10000, 1, 1000000}, {"QUERY_CHECKPOINT_MAX_RECORDS", &q.MaxCheckpoints, 100000, 1, 1000000},
	} {
		*v.dst, err = boundedInt(lookup, v.name, v.def, v.min, v.max)
		if err != nil {
			return q, err
		}
	}
	if q.Concurrency > q.MaxPerCycle {
		return q, errors.New("query concurrency exceeds per-cycle detail budget")
	}
	if q.ResponseMaxBytes, err = boundedPositiveInt64(lookup, "QUERY_RESPONSE_MAX_BYTES", 16<<20, 64<<20); err != nil {
		return q, err
	}
	if q.MaxBytes, err = boundedPositiveInt64(lookup, "QUERY_OUTBOX_MAX_BYTES", 256<<20, 1<<30); err != nil {
		return q, err
	}
	if q.Events, err = boolean(lookup, "QUERY_EVENTS_ENABLED", false); err != nil {
		return q, err
	}
	if q.Events {
		q.Topic = optionalText(lookup, "QUERY_KAFKA_TOPIC", "")
		if len(q.Topic) == 0 || len(q.Topic) > 249 || strings.ContainsAny(q.Topic, "\x00\r\n\t ") {
			return q, errors.New("QUERY_KAFKA_TOPIC is required and must be valid")
		}
	}
	return q, nil
}
