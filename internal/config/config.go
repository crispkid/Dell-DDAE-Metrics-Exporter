package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	secretFileMaxBytes = 64 * 1024
	responseMaxBytes   = 64 * 1024 * 1024
)

// Secret deliberately has no String or marshaling methods. Callers must opt in
// to reading the value at the narrow trust boundary that consumes it.
type Secret struct {
	value string
}

func (s Secret) Value() string { return s.value }
func (s Secret) Empty() bool   { return s.value == "" }

type Config struct {
	ResourceMonitoringEnabled           bool
	AlertMonitoringEnabled              bool
	ServiceabilityLogMonitoringEnabled  bool
	ResourceCollectionInterval          time.Duration
	AlertCollectionInterval             time.Duration
	ServiceabilityLogCollectionInterval time.Duration
	DDAEBaseURL                         *url.URL
	SourceInstance                      string
	DDAEUsername                        Secret
	DDAEPassword                        Secret
	DDAEClientSecret                    Secret
	DDAECAFile                          string
	DDAETLSInsecureSkipVerify           bool
	AllowInsecureTLS                    bool
	ListenAddress                       string
	CollectionInterval                  time.Duration // Legacy alias for the resource interval.
	RequestTimeout                      time.Duration
	CycleTimeout                        time.Duration
	ResponseMaxBytes                    int64
	RetryMax                            int
	StaleAfter                          time.Duration

	AlertListResponseMaxBytes               int64
	AlertDetailResponseMaxBytes             int64
	AlertDetailRefreshInterval              time.Duration
	AlertDetailMaxPerCycle                  int
	AlertDetailConcurrency                  int
	ServiceabilityLogListResponseMaxBytes   int64
	ServiceabilityLogDetailResponseMaxBytes int64
	ServiceabilityLogDetailRefreshInterval  time.Duration
	ServiceabilityLogDetailMaxPerCycle      int
	ServiceabilityLogDetailConcurrency      int

	KafkaBrokers                []string
	KafkaTopic                  string
	KafkaServiceabilityLogTopic string
	KafkaClientID               string
	KafkaCAFile                 string
	KafkaClientCertFile         string
	KafkaClientKeyFile          string
	KafkaTLSInsecureSkipVerify  bool
	KafkaSASLMechanism          string
	KafkaSASLUsername           Secret
	KafkaSASLPassword           Secret
	KafkaPublishTimeout         time.Duration

	StateDir                              string
	KafkaOutboxMaxBytes                   int64
	KafkaOutboxMaxEvents                  int
	CheckpointRetention                   time.Duration
	CheckpointMaxAlerts                   int
	ServiceabilityLogOutboxMaxBytes       int64
	ServiceabilityLogOutboxMaxEvents      int
	ServiceabilityLogCheckpointRetention  time.Duration
	ServiceabilityLogCheckpointMaxRecords int
	ShutdownGracePeriod                   time.Duration
	LogLevel                              string
	LogFormat                             string
}

type lookupFunc func(string) (string, bool)

func (c Config) InsecureTLSTargets() []string {
	targets := make([]string, 0, 2)
	if !c.AllowInsecureTLS {
		return targets
	}
	if c.DDAETLSInsecureSkipVerify {
		targets = append(targets, "ddae")
	}
	if (c.AlertMonitoringEnabled || c.ServiceabilityLogMonitoringEnabled) && c.KafkaTLSInsecureSkipVerify {
		targets = append(targets, "kafka")
	}
	return targets
}

func Load() (Config, error) {
	return LoadFile("")
}

func load(lookup lookupFunc, readFile func(string) ([]byte, error)) (Config, error) {
	var cfg Config
	var err error

	if cfg.ResourceMonitoringEnabled, err = boolean(lookup, "DDAE_RESOURCE_MONITORING_ENABLED", true); err != nil {
		return cfg, err
	}
	if cfg.AlertMonitoringEnabled, err = boolean(lookup, "DDAE_ALERT_MONITORING_ENABLED", true); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogMonitoringEnabled, err = boolean(lookup, "DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED", false); err != nil {
		return cfg, err
	}
	if !cfg.ResourceMonitoringEnabled && !cfg.AlertMonitoringEnabled && !cfg.ServiceabilityLogMonitoringEnabled {
		return cfg, errors.New("at least one monitoring pipeline must be enabled")
	}
	if cfg.AllowInsecureTLS, err = boolean(lookup, "ALLOW_INSECURE_TLS", false); err != nil {
		return cfg, err
	}
	if cfg.DDAETLSInsecureSkipVerify, err = boolean(lookup, "DDAE_TLS_INSECURE_SKIP_VERIFY", false); err != nil {
		return cfg, err
	}
	if cfg.KafkaTLSInsecureSkipVerify, err = boolean(lookup, "KAFKA_TLS_INSECURE_SKIP_VERIFY", false); err != nil {
		return cfg, err
	}
	if (cfg.DDAETLSInsecureSkipVerify || cfg.KafkaTLSInsecureSkipVerify) && !cfg.AllowInsecureTLS {
		return cfg, errors.New("target insecure_skip_verify requires ALLOW_INSECURE_TLS=true")
	}

	baseRaw, ok := lookup("DDAE_BASE_URL")
	if !ok || strings.TrimSpace(baseRaw) == "" {
		return cfg, errors.New("DDAE_BASE_URL is required")
	}
	cfg.DDAEBaseURL, err = validateOrigin(baseRaw)
	if err != nil {
		return cfg, fmt.Errorf("DDAE_BASE_URL is invalid: %w", err)
	}

	if source, ok := lookup("DDAE_SOURCE_INSTANCE"); ok {
		cfg.SourceInstance = source
		if err := validateSourceInstance(cfg.SourceInstance); err != nil {
			return cfg, fmt.Errorf("DDAE_SOURCE_INSTANCE is invalid: %w", err)
		}
	} else if cfg.AlertMonitoringEnabled || cfg.ServiceabilityLogMonitoringEnabled {
		return cfg, errors.New("DDAE_SOURCE_INSTANCE is required when alert or serviceability log monitoring is enabled")
	}

	if cfg.DDAEUsername, err = loadSecret(lookup, readFile, "DDAE_USERNAME", "DDAE_USERNAME_FILE", true); err != nil {
		return cfg, err
	}
	if cfg.DDAEPassword, err = loadSecret(lookup, readFile, "DDAE_PASSWORD", "DDAE_PASSWORD_FILE", true); err != nil {
		return cfg, err
	}
	if cfg.DDAEClientSecret, err = loadSecret(lookup, readFile, "DDAE_CLIENT_SECRET", "DDAE_CLIENT_SECRET_FILE", true); err != nil {
		return cfg, err
	}

	cfg.DDAECAFile = optionalText(lookup, "DDAE_CA_FILE", "")
	if cfg.DDAETLSInsecureSkipVerify && cfg.DDAECAFile != "" {
		return cfg, errors.New("DDAE_CA_FILE conflicts with DDAE_TLS_INSECURE_SKIP_VERIFY")
	}
	cfg.ListenAddress = optionalText(lookup, "EXPORTER_LISTEN_ADDRESS", "127.0.0.1:9469")
	if err := validateListenAddress(cfg.ListenAddress); err != nil {
		return cfg, fmt.Errorf("EXPORTER_LISTEN_ADDRESS is invalid: %w", err)
	}

	if cfg.ResourceCollectionInterval, err = durationAlias(lookup, "DDAE_RESOURCE_COLLECTION_INTERVAL", "DDAE_COLLECTION_INTERVAL", 30*time.Second); err != nil {
		return cfg, err
	}
	if cfg.AlertCollectionInterval, err = durationAlias(lookup, "DDAE_ALERT_COLLECTION_INTERVAL", "DDAE_COLLECTION_INTERVAL", 30*time.Second); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogCollectionInterval, err = duration(lookup, "DDAE_SERVICEABILITY_LOG_COLLECTION_INTERVAL", 30*time.Second); err != nil {
		return cfg, err
	}
	cfg.CollectionInterval = cfg.ResourceCollectionInterval
	if cfg.RequestTimeout, err = duration(lookup, "DDAE_REQUEST_TIMEOUT", 5*time.Second); err != nil {
		return cfg, err
	}
	if cfg.CycleTimeout, err = duration(lookup, "DDAE_CYCLE_TIMEOUT", 20*time.Second); err != nil {
		return cfg, err
	}
	if cfg.ResponseMaxBytes, err = boundedPositiveInt64(lookup, "DDAE_RESPONSE_MAX_BYTES", 4*1024*1024, responseMaxBytes); err != nil {
		return cfg, err
	}
	if cfg.RetryMax, err = boundedInt(lookup, "DDAE_RETRY_MAX", 2, 0, 10); err != nil {
		return cfg, err
	}
	if cfg.StaleAfter, err = duration(lookup, "DDAE_STALE_AFTER", 120*time.Second); err != nil {
		return cfg, err
	}
	if cfg.RequestTimeout >= cfg.CycleTimeout {
		return cfg, errors.New("DDAE_REQUEST_TIMEOUT must be less than DDAE_CYCLE_TIMEOUT")
	}
	if cfg.ResourceMonitoringEnabled && cfg.CycleTimeout >= cfg.ResourceCollectionInterval {
		return cfg, errors.New("DDAE_CYCLE_TIMEOUT must be less than the resource collection interval")
	}
	if cfg.AlertMonitoringEnabled && cfg.CycleTimeout >= cfg.AlertCollectionInterval {
		return cfg, errors.New("DDAE_CYCLE_TIMEOUT must be less than the alert collection interval")
	}
	if cfg.ServiceabilityLogMonitoringEnabled && cfg.CycleTimeout >= cfg.ServiceabilityLogCollectionInterval {
		return cfg, errors.New("DDAE_CYCLE_TIMEOUT must be less than the serviceability log collection interval")
	}
	if cfg.ResourceMonitoringEnabled && cfg.StaleAfter <= cfg.ResourceCollectionInterval {
		return cfg, errors.New("DDAE_STALE_AFTER must be greater than the resource collection interval")
	}

	if cfg.AlertListResponseMaxBytes, err = boundedPositiveInt64(lookup, "ALERT_LIST_RESPONSE_MAX_BYTES", 8*1024*1024, responseMaxBytes); err != nil {
		return cfg, err
	}
	if cfg.AlertDetailResponseMaxBytes, err = boundedPositiveInt64(lookup, "ALERT_DETAIL_RESPONSE_MAX_BYTES", 1024*1024, responseMaxBytes); err != nil {
		return cfg, err
	}
	if cfg.AlertDetailRefreshInterval, err = duration(lookup, "ALERT_DETAIL_REFRESH_INTERVAL", 10*time.Minute); err != nil {
		return cfg, err
	}
	if cfg.AlertDetailMaxPerCycle, err = boundedInt(lookup, "ALERT_DETAIL_MAX_PER_CYCLE", 200, 1, 10000); err != nil {
		return cfg, err
	}
	if cfg.AlertDetailConcurrency, err = boundedInt(lookup, "ALERT_DETAIL_CONCURRENCY", 4, 1, 128); err != nil {
		return cfg, err
	}
	if cfg.AlertDetailConcurrency > cfg.AlertDetailMaxPerCycle {
		return cfg, errors.New("ALERT_DETAIL_CONCURRENCY must not exceed ALERT_DETAIL_MAX_PER_CYCLE")
	}
	if cfg.AlertMonitoringEnabled && cfg.AlertDetailRefreshInterval < cfg.AlertCollectionInterval {
		return cfg, errors.New("ALERT_DETAIL_REFRESH_INTERVAL must not be less than the alert collection interval")
	}
	if cfg.ServiceabilityLogListResponseMaxBytes, err = boundedPositiveInt64(lookup, "SERVICEABILITY_LOG_LIST_RESPONSE_MAX_BYTES", 8*1024*1024, responseMaxBytes); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogDetailResponseMaxBytes, err = boundedPositiveInt64(lookup, "SERVICEABILITY_LOG_DETAIL_RESPONSE_MAX_BYTES", 1024*1024, responseMaxBytes); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogDetailRefreshInterval, err = duration(lookup, "SERVICEABILITY_LOG_DETAIL_REFRESH_INTERVAL", 10*time.Minute); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogDetailMaxPerCycle, err = boundedInt(lookup, "SERVICEABILITY_LOG_DETAIL_MAX_PER_CYCLE", 200, 1, 10000); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogDetailConcurrency, err = boundedInt(lookup, "SERVICEABILITY_LOG_DETAIL_CONCURRENCY", 4, 1, 128); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogDetailConcurrency > cfg.ServiceabilityLogDetailMaxPerCycle {
		return cfg, errors.New("SERVICEABILITY_LOG_DETAIL_CONCURRENCY must not exceed SERVICEABILITY_LOG_DETAIL_MAX_PER_CYCLE")
	}
	if cfg.ServiceabilityLogMonitoringEnabled && cfg.ServiceabilityLogDetailRefreshInterval < cfg.ServiceabilityLogCollectionInterval {
		return cfg, errors.New("SERVICEABILITY_LOG_DETAIL_REFRESH_INTERVAL must not be less than the serviceability log collection interval")
	}

	if brokers, ok := lookup("KAFKA_BROKERS"); ok {
		if strings.TrimSpace(brokers) == "" {
			return cfg, errors.New("KAFKA_BROKERS is invalid")
		}
		cfg.KafkaBrokers, err = parseBrokers(brokers)
		if err != nil {
			return cfg, fmt.Errorf("KAFKA_BROKERS is invalid: %w", err)
		}
	} else if cfg.AlertMonitoringEnabled || cfg.ServiceabilityLogMonitoringEnabled {
		return cfg, errors.New("KAFKA_BROKERS is required when alert or serviceability log monitoring is enabled")
	}
	if topic, ok := lookup("KAFKA_TOPIC"); ok {
		cfg.KafkaTopic = topic
		if strings.TrimSpace(topic) == "" || strings.ContainsAny(topic, "\x00\r\n\t ") || len(topic) > 249 {
			return cfg, errors.New("KAFKA_TOPIC is invalid")
		}
	} else if cfg.AlertMonitoringEnabled {
		return cfg, errors.New("KAFKA_TOPIC is required when alert monitoring is enabled")
	}
	cfg.KafkaServiceabilityLogTopic = optionalText(lookup, "KAFKA_SERVICEABILITY_LOG_TOPIC", "ddae-serviceability-logs")
	if strings.TrimSpace(cfg.KafkaServiceabilityLogTopic) == "" || strings.ContainsAny(cfg.KafkaServiceabilityLogTopic, "\x00\r\n\t ") || len(cfg.KafkaServiceabilityLogTopic) > 249 {
		return cfg, errors.New("KAFKA_SERVICEABILITY_LOG_TOPIC is invalid")
	}
	if cfg.AlertMonitoringEnabled && cfg.ServiceabilityLogMonitoringEnabled && cfg.KafkaTopic == cfg.KafkaServiceabilityLogTopic {
		return cfg, errors.New("KAFKA_SERVICEABILITY_LOG_TOPIC must differ from KAFKA_TOPIC")
	}
	cfg.KafkaClientID = optionalText(lookup, "KAFKA_CLIENT_ID", "ddae-exporter")
	if len(cfg.KafkaClientID) == 0 || len(cfg.KafkaClientID) > 128 || strings.ContainsAny(cfg.KafkaClientID, "\x00\r\n") {
		return cfg, errors.New("KAFKA_CLIENT_ID is invalid")
	}
	cfg.KafkaCAFile = optionalText(lookup, "KAFKA_CA_FILE", "")
	if cfg.KafkaTLSInsecureSkipVerify && cfg.KafkaCAFile != "" {
		return cfg, errors.New("KAFKA_CA_FILE conflicts with KAFKA_TLS_INSECURE_SKIP_VERIFY")
	}
	cfg.KafkaClientCertFile = optionalText(lookup, "KAFKA_CLIENT_CERT_FILE", "")
	cfg.KafkaClientKeyFile = optionalText(lookup, "KAFKA_CLIENT_KEY_FILE", "")
	if (cfg.KafkaClientCertFile == "") != (cfg.KafkaClientKeyFile == "") {
		return cfg, errors.New("KAFKA_CLIENT_CERT_FILE and KAFKA_CLIENT_KEY_FILE must be configured together")
	}
	if secretInputsConflict(lookup, "KAFKA_SASL_PASSWORD", "KAFKA_SASL_PASSWORD_FILE") {
		return cfg, errors.New("KAFKA_SASL_PASSWORD and KAFKA_SASL_PASSWORD_FILE conflict")
	}
	cfg.KafkaSASLMechanism = strings.ToUpper(optionalText(lookup, "KAFKA_SASL_MECHANISM", ""))
	switch cfg.KafkaSASLMechanism {
	case "", "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512":
	default:
		return cfg, errors.New("KAFKA_SASL_MECHANISM is unsupported")
	}
	if cfg.KafkaSASLMechanism != "" && (cfg.AlertMonitoringEnabled || cfg.ServiceabilityLogMonitoringEnabled) {
		var username string
		username, err = requiredText(lookup, "KAFKA_SASL_USERNAME")
		if err != nil {
			return cfg, err
		}
		cfg.KafkaSASLUsername = Secret{value: username}
		cfg.KafkaSASLPassword, err = loadSecret(lookup, readFile, "KAFKA_SASL_PASSWORD", "KAFKA_SASL_PASSWORD_FILE", true)
		if err != nil {
			return cfg, err
		}
	}
	if cfg.KafkaPublishTimeout, err = duration(lookup, "KAFKA_PUBLISH_TIMEOUT", 10*time.Second); err != nil {
		return cfg, err
	}
	if cfg.KafkaPublishTimeout < time.Second {
		return cfg, errors.New("KAFKA_PUBLISH_TIMEOUT must be at least 1s")
	}

	cfg.StateDir = optionalText(lookup, "STATE_DIR", "/var/lib/ddae-exporter")
	if !filepath.IsAbs(cfg.StateDir) {
		return cfg, errors.New("STATE_DIR must be an absolute path")
	}
	if cfg.KafkaOutboxMaxBytes, err = positiveInt64(lookup, "KAFKA_OUTBOX_MAX_BYTES", 1024*1024*1024); err != nil {
		return cfg, err
	}
	if cfg.KafkaOutboxMaxEvents, err = boundedInt(lookup, "KAFKA_OUTBOX_MAX_EVENTS", 100000, 1, 10_000_000); err != nil {
		return cfg, err
	}
	if cfg.CheckpointRetention, err = duration(lookup, "CHECKPOINT_RETENTION", 720*time.Hour); err != nil {
		return cfg, err
	}
	if cfg.CheckpointMaxAlerts, err = boundedInt(lookup, "CHECKPOINT_MAX_ALERTS", 100000, 1, 10_000_000); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogOutboxMaxBytes, err = positiveInt64(lookup, "SERVICEABILITY_LOG_OUTBOX_MAX_BYTES", 1024*1024*1024); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogOutboxMaxEvents, err = boundedInt(lookup, "SERVICEABILITY_LOG_OUTBOX_MAX_EVENTS", 100000, 1, 10_000_000); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogCheckpointRetention, err = duration(lookup, "SERVICEABILITY_LOG_CHECKPOINT_RETENTION", 720*time.Hour); err != nil {
		return cfg, err
	}
	if cfg.ServiceabilityLogCheckpointMaxRecords, err = boundedInt(lookup, "SERVICEABILITY_LOG_CHECKPOINT_MAX_RECORDS", 100000, 1, 10_000_000); err != nil {
		return cfg, err
	}
	if cfg.ShutdownGracePeriod, err = duration(lookup, "SHUTDOWN_GRACE_PERIOD", 15*time.Second); err != nil {
		return cfg, err
	}
	cfg.LogLevel = strings.ToLower(optionalText(lookup, "LOG_LEVEL", "info"))
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return cfg, errors.New("LOG_LEVEL must be debug, info, warn or error")
	}
	cfg.LogFormat = strings.ToLower(optionalText(lookup, "LOG_FORMAT", "json"))
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		return cfg, errors.New("LOG_FORMAT must be json or text")
	}

	return cfg, nil
}

func validateOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("not a valid URL")
	}
	if u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("must be an HTTPS origin")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("must not contain user info, path, query or fragment")
	}
	u.Path = ""
	u.RawPath = ""
	return u, nil
}

func validateSourceInstance(value string) error {
	if !utf8.ValidString(value) || len(value) < 1 || len(value) > 128 {
		return errors.New("must be 1-128 valid UTF-8 bytes")
	}
	if strings.ContainsRune(value, '\x00') || strings.Contains(value, "://") || strings.ContainsAny(value, "\r\n") {
		return errors.New("must not be a URL or contain control delimiters")
	}
	return nil
}

func validateListenAddress(value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return errors.New("must be host:port")
	}
	if host == "" {
		return errors.New("host must be explicit")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func requiredText(lookup lookupFunc, name string) (string, error) {
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("%s is invalid", name)
	}
	return value, nil
}

func optionalText(lookup lookupFunc, name, fallback string) string {
	if value, ok := lookup(name); ok {
		return value
	}
	return fallback
}

func loadSecret(lookup lookupFunc, readFile func(string) ([]byte, error), directName, fileName string, required bool) (Secret, error) {
	direct, hasDirect := lookup(directName)
	path, hasFile := lookup(fileName)
	if hasDirect && hasFile {
		return Secret{}, fmt.Errorf("%s and %s conflict", directName, fileName)
	}
	if !hasDirect && !hasFile {
		if required {
			return Secret{}, fmt.Errorf("%s or %s is required", directName, fileName)
		}
		return Secret{}, nil
	}
	var value string
	if hasFile {
		if path == "" {
			return Secret{}, fmt.Errorf("%s is invalid", fileName)
		}
		data, err := readFile(path)
		if err != nil {
			return Secret{}, fmt.Errorf("cannot read %s", fileName)
		}
		if len(data) > secretFileMaxBytes {
			return Secret{}, fmt.Errorf("%s exceeds the size limit", fileName)
		}
		value = string(data)
		value = strings.TrimSuffix(value, "\r\n")
		if !strings.HasSuffix(string(data), "\r\n") {
			value = strings.TrimSuffix(value, "\n")
			value = strings.TrimSuffix(value, "\r")
		}
	} else {
		value = direct
	}
	if value == "" || strings.ContainsRune(value, '\x00') || !utf8.ValidString(value) {
		return Secret{}, fmt.Errorf("%s is invalid", directName)
	}
	return Secret{value: value}, nil
}

func secretInputsConflict(lookup lookupFunc, directName, fileName string) bool {
	_, hasDirect := lookup(directName)
	_, hasFile := lookup(fileName)
	return hasDirect && hasFile
}

func duration(lookup lookupFunc, name string, fallback time.Duration) (time.Duration, error) {
	value, ok := lookup(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return parsed, nil
}

func durationAlias(lookup lookupFunc, name, legacyName string, fallback time.Duration) (time.Duration, error) {
	if _, ok := lookup(name); ok {
		return duration(lookup, name, fallback)
	}
	return duration(lookup, legacyName, fallback)
}

func boolean(lookup lookupFunc, name string, fallback bool) (bool, error) {
	value, ok := lookup(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.ToLower(value))
	if err != nil || (strings.ToLower(value) != "true" && strings.ToLower(value) != "false") {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}

func positiveInt64(lookup lookupFunc, name string, fallback int64) (int64, error) {
	value, ok := lookup(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func boundedPositiveInt64(lookup lookupFunc, name string, fallback, maximum int64) (int64, error) {
	value, err := positiveInt64(lookup, name, fallback)
	if err != nil {
		return 0, err
	}
	if value > maximum {
		return 0, fmt.Errorf("%s must be between 1 and %d", name, maximum)
	}
	return value, nil
}

func boundedInt(lookup lookupFunc, name string, fallback, minimum, maximum int) (int, error) {
	value, ok := lookup(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minimum, maximum)
	}
	return parsed, nil
}

func parseBrokers(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	if len(parts) == 0 || len(parts) > 64 {
		return nil, errors.New("must contain 1-64 brokers")
	}
	brokers := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		broker := strings.TrimSpace(part)
		if broker == "" || strings.ContainsAny(broker, "\x00\r\n") {
			return nil, errors.New("contains an empty or invalid broker")
		}
		if _, ok := seen[broker]; ok {
			continue
		}
		seen[broker] = struct{}{}
		brokers = append(brokers, broker)
	}
	return brokers, nil
}

func TLSMinVersion() uint16 { return tls.VersionTLS12 }
