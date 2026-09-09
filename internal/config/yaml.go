package config

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const yamlFileMaxBytes = 1024 * 1024

type yamlConfig struct {
	Version    int            `yaml:"version"`
	Monitoring yamlMonitoring `yaml:"monitoring"`
	Server     yamlServer     `yaml:"server"`
	Security   yamlSecurity   `yaml:"security"`
	DDAE       yamlDDAE       `yaml:"ddae"`
	Kafka      yamlKafka      `yaml:"kafka"`
	State      yamlState      `yaml:"state"`
	Logging    yamlLogging    `yaml:"logging"`
}

type yamlMonitoring struct {
	Queries            yamlQueries            `yaml:"queries"`
	Resources          yamlResources          `yaml:"resources"`
	Alerts             yamlAlerts             `yaml:"alerts"`
	ServiceabilityLogs yamlServiceabilityLogs `yaml:"serviceability_logs"`
}

type yamlResources struct {
	Enabled    *bool   `yaml:"enabled"`
	Interval   *string `yaml:"interval"`
	StaleAfter *string `yaml:"stale_after"`
}

type yamlAlerts struct {
	Enabled              *bool           `yaml:"enabled"`
	Interval             *string         `yaml:"interval"`
	ListResponseMaxBytes *int64          `yaml:"list_response_max_bytes"`
	Detail               yamlAlertDetail `yaml:"detail"`
}

type yamlAlertDetail struct {
	ResponseMaxBytes *int64  `yaml:"response_max_bytes"`
	RefreshInterval  *string `yaml:"refresh_interval"`
	MaxPerCycle      *int    `yaml:"max_per_cycle"`
	Concurrency      *int    `yaml:"concurrency"`
}

type yamlServiceabilityLogs struct {
	Backfill             yamlBackfill                `yaml:"backfill"`
	Enabled              *bool                       `yaml:"enabled"`
	Interval             *string                     `yaml:"interval"`
	ListResponseMaxBytes *int64                      `yaml:"list_response_max_bytes"`
	Detail               yamlServiceabilityLogDetail `yaml:"detail"`
}

type yamlServiceabilityLogDetail struct {
	ResponseMaxBytes *int64  `yaml:"response_max_bytes"`
	RefreshInterval  *string `yaml:"refresh_interval"`
	MaxPerCycle      *int    `yaml:"max_per_cycle"`
	Concurrency      *int    `yaml:"concurrency"`
}

type yamlServer struct {
	ListenAddress       *string `yaml:"listen_address"`
	ShutdownGracePeriod *string `yaml:"shutdown_grace_period"`
}

type yamlSecurity struct {
	AllowInsecureTLS *bool `yaml:"allow_insecure_tls"`
}

type yamlDDAE struct {
	BaseURL          *string         `yaml:"base_url"`
	Paths            yamlDDAEPaths   `yaml:"paths"`
	SourceInstance   *string         `yaml:"source_instance"`
	Credentials      yamlCredentials `yaml:"credentials"`
	TLS              yamlDDAETLS     `yaml:"tls"`
	RequestTimeout   *string         `yaml:"request_timeout"`
	CycleTimeout     *string         `yaml:"cycle_timeout"`
	ResponseMaxBytes *int64          `yaml:"response_max_bytes"`
	RetryMax         *int            `yaml:"retry_max"`
}

type yamlDDAEPaths struct {
	PingPrefix *yamlPathPrefix `yaml:"ping_prefix"`
	APIPrefix  *yamlPathPrefix `yaml:"api_prefix"`
}

type yamlPathPrefix string

func (value *yamlPathPrefix) UnmarshalYAML(node *yaml.Node) error {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("DDAE path prefix must be a string")
	}
	*value = yamlPathPrefix(node.Value)
	return nil
}

type yamlCredentials struct {
	UsernameFile     *string `yaml:"username_file"`
	PasswordFile     *string `yaml:"password_file"`
	ClientSecretFile *string `yaml:"client_secret_file"`
}

type yamlDDAETLS struct {
	CAFile             *string `yaml:"ca_file"`
	InsecureSkipVerify *bool   `yaml:"insecure_skip_verify"`
}

type yamlKafkaTLS struct {
	CAFile             *string `yaml:"ca_file"`
	ClientCertFile     *string `yaml:"client_cert_file"`
	ClientKeyFile      *string `yaml:"client_key_file"`
	InsecureSkipVerify *bool   `yaml:"insecure_skip_verify"`
}

type yamlKafka struct {
	Brokers                 []string     `yaml:"brokers"`
	Topic                   *string      `yaml:"topic"`
	ServiceabilityLogsTopic *string      `yaml:"serviceability_logs_topic"`
	ClientID                *string      `yaml:"client_id"`
	TLS                     yamlKafkaTLS `yaml:"tls"`
	SASL                    yamlSASL     `yaml:"sasl"`
	PublishTimeout          *string      `yaml:"publish_timeout"`
}

type yamlSASL struct {
	Mechanism    *string `yaml:"mechanism"`
	Username     *string `yaml:"username"`
	PasswordFile *string `yaml:"password_file"`
}

type yamlState struct {
	Dir                                    *string `yaml:"dir"`
	OutboxMaxBytes                         *int64  `yaml:"outbox_max_bytes"`
	OutboxMaxEvents                        *int    `yaml:"outbox_max_events"`
	CheckpointRetention                    *string `yaml:"checkpoint_retention"`
	CheckpointMaxAlerts                    *int    `yaml:"checkpoint_max_alerts"`
	ServiceabilityLogsOutboxMaxBytes       *int64  `yaml:"serviceability_logs_outbox_max_bytes"`
	ServiceabilityLogsOutboxMaxEvents      *int    `yaml:"serviceability_logs_outbox_max_events"`
	ServiceabilityLogsCheckpointRetention  *string `yaml:"serviceability_logs_checkpoint_retention"`
	ServiceabilityLogsCheckpointMaxRecords *int    `yaml:"serviceability_logs_checkpoint_max_records"`
}

type yamlLogging struct {
	Level  *string `yaml:"level"`
	Format *string `yaml:"format"`
}

// LoadFile loads an explicitly selected YAML file, or falls back to the
// DDAE_EXPORTER_CONFIG_FILE selector and finally the environment-only v1
// interface. Individual environment settings always override YAML values.
func LoadFile(commandLinePath string) (Config, error) {
	lookup := os.LookupEnv
	selectedPath, selected := lookup("DDAE_EXPORTER_CONFIG_FILE")
	if commandLinePath != "" {
		selectedPath = commandLinePath
		selected = true
	}
	values := map[string]string{}
	if selected {
		if strings.TrimSpace(selectedPath) == "" {
			return Config{}, fmt.Errorf("configuration file path is empty")
		}
		data, err := readYAMLFile(selectedPath)
		if err != nil {
			return Config{}, err
		}
		values, err = decodeYAML(data)
		if err != nil {
			return Config{}, err
		}
	}
	return load(layeredLookup(values, lookup), readSecretFile)
}

func readYAMLFile(path string) ([]byte, error) {
	pathInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read configuration file")
	}
	if !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("configuration file must be a regular file")
	}
	if pathInfo.Size() > yamlFileMaxBytes {
		return nil, fmt.Errorf("configuration file exceeds the size limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read configuration file")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("configuration file must be a regular file")
	}
	if info.Size() > yamlFileMaxBytes {
		return nil, fmt.Errorf("configuration file exceeds the size limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, yamlFileMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read configuration file")
	}
	if len(data) > yamlFileMaxBytes {
		return nil, fmt.Errorf("configuration file exceeds the size limit")
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("configuration file must be valid UTF-8")
	}
	return data, nil
}

func readSecretFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("secret path must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, secretFileMaxBytes+1))
}

func decodeYAML(data []byte) (map[string]string, error) {
	if err := rejectYAMLAliases(data); err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	var document yamlConfig
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("configuration YAML is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("configuration YAML must contain one document")
		}
		return nil, fmt.Errorf("configuration YAML is invalid")
	}
	if document.Version != 1 {
		return nil, fmt.Errorf("configuration version must be 1")
	}
	return document.environmentValues(), nil
}

func rejectYAMLAliases(data []byte) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("configuration YAML is invalid")
	}
	var visit func(*yaml.Node) bool
	visit = func(node *yaml.Node) bool {
		if node == nil {
			return false
		}
		if node.Kind == yaml.AliasNode || node.Anchor != "" || node.Tag == "!!merge" {
			return true
		}
		for _, child := range node.Content {
			if visit(child) {
				return true
			}
		}
		return false
	}
	if visit(&document) {
		return fmt.Errorf("configuration YAML aliases and merge keys are unsupported")
	}
	return nil
}

func (document yamlConfig) environmentValues() map[string]string {
	values := make(map[string]string)
	document.Monitoring.Queries.Backfill.put(values, "QUERY_BACKFILL_")
	document.Monitoring.ServiceabilityLogs.Backfill.put(values, "SERVICEABILITY_LOG_BACKFILL_")
	putBool(values, "DDAE_RESOURCE_MONITORING_ENABLED", document.Monitoring.Resources.Enabled)
	putString(values, "DDAE_RESOURCE_COLLECTION_INTERVAL", document.Monitoring.Resources.Interval)
	putString(values, "DDAE_STALE_AFTER", document.Monitoring.Resources.StaleAfter)
	putBool(values, "DDAE_ALERT_MONITORING_ENABLED", document.Monitoring.Alerts.Enabled)
	putString(values, "DDAE_ALERT_COLLECTION_INTERVAL", document.Monitoring.Alerts.Interval)
	putInt64(values, "ALERT_LIST_RESPONSE_MAX_BYTES", document.Monitoring.Alerts.ListResponseMaxBytes)
	putInt64(values, "ALERT_DETAIL_RESPONSE_MAX_BYTES", document.Monitoring.Alerts.Detail.ResponseMaxBytes)
	putString(values, "ALERT_DETAIL_REFRESH_INTERVAL", document.Monitoring.Alerts.Detail.RefreshInterval)
	putInt(values, "ALERT_DETAIL_MAX_PER_CYCLE", document.Monitoring.Alerts.Detail.MaxPerCycle)
	putInt(values, "ALERT_DETAIL_CONCURRENCY", document.Monitoring.Alerts.Detail.Concurrency)
	putBool(values, "DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED", document.Monitoring.ServiceabilityLogs.Enabled)
	putString(values, "DDAE_SERVICEABILITY_LOG_COLLECTION_INTERVAL", document.Monitoring.ServiceabilityLogs.Interval)
	putInt64(values, "SERVICEABILITY_LOG_LIST_RESPONSE_MAX_BYTES", document.Monitoring.ServiceabilityLogs.ListResponseMaxBytes)
	putInt64(values, "SERVICEABILITY_LOG_DETAIL_RESPONSE_MAX_BYTES", document.Monitoring.ServiceabilityLogs.Detail.ResponseMaxBytes)
	putString(values, "SERVICEABILITY_LOG_DETAIL_REFRESH_INTERVAL", document.Monitoring.ServiceabilityLogs.Detail.RefreshInterval)
	putInt(values, "SERVICEABILITY_LOG_DETAIL_MAX_PER_CYCLE", document.Monitoring.ServiceabilityLogs.Detail.MaxPerCycle)
	putInt(values, "SERVICEABILITY_LOG_DETAIL_CONCURRENCY", document.Monitoring.ServiceabilityLogs.Detail.Concurrency)

	putBool(values, "QUERY_ENABLED", document.Monitoring.Queries.Enabled)
	putBool(values, "QUERY_EVENTS_ENABLED", document.Monitoring.Queries.Events)
	putString(values, "QUERY_BASE_URL", document.Monitoring.Queries.BaseURL)
	putString(values, "QUERY_AUTH_URL", document.Monitoring.Queries.AuthURL)
	putString(values, "QUERY_REALM", document.Monitoring.Queries.Realm)
	putString(values, "QUERY_ROLE", document.Monitoring.Queries.Role)
	putString(values, "QUERY_USERNAME_FILE", document.Monitoring.Queries.UsernameFile)
	putString(values, "QUERY_PASSWORD_FILE", document.Monitoring.Queries.PasswordFile)
	putString(values, "QUERY_CA_FILE", document.Monitoring.Queries.CAFile)
	putString(values, "QUERY_AUTH_CA_FILE", document.Monitoring.Queries.AuthCAFile)
	putBool(values, "QUERY_TLS_INSECURE_SKIP_VERIFY", document.Monitoring.Queries.Insecure)
	putString(values, "QUERY_KAFKA_TOPIC", document.Monitoring.Queries.Topic)
	putString(values, "QUERY_INTERVAL", document.Monitoring.Queries.Interval)
	putString(values, "QUERY_REQUEST_TIMEOUT", document.Monitoring.Queries.RequestTimeout)
	putString(values, "QUERY_CYCLE_TIMEOUT", document.Monitoring.Queries.CycleTimeout)
	putString(values, "QUERY_STALE_AFTER", document.Monitoring.Queries.StaleAfter)
	putString(values, "QUERY_CHECKPOINT_RETENTION", document.Monitoring.Queries.Retention)
	putInt64(values, "QUERY_OUTBOX_MAX_BYTES", document.Monitoring.Queries.MaxBytes)
	putInt64(values, "QUERY_RESPONSE_MAX_BYTES", document.Monitoring.Queries.ResponseMaxBytes)
	putInt(values, "QUERY_OUTBOX_MAX_EVENTS", document.Monitoring.Queries.MaxEvents)
	putInt(values, "QUERY_CHECKPOINT_MAX_RECORDS", document.Monitoring.Queries.MaxCheckpoints)
	putInt(values, "QUERY_MAX_HISTORY_RECORDS", document.Monitoring.Queries.MaxHistory)
	putInt(values, "QUERY_DETAIL_MAX_PER_CYCLE", document.Monitoring.Queries.MaxPerCycle)
	putInt(values, "QUERY_DETAIL_CONCURRENCY", document.Monitoring.Queries.Concurrency)
	putInt(values, "QUERY_RETRY_MAX", document.Monitoring.Queries.RetryMax)
	putString(values, "EXPORTER_LISTEN_ADDRESS", document.Server.ListenAddress)
	putString(values, "SHUTDOWN_GRACE_PERIOD", document.Server.ShutdownGracePeriod)
	putBool(values, "ALLOW_INSECURE_TLS", document.Security.AllowInsecureTLS)

	putString(values, "DDAE_BASE_URL", document.DDAE.BaseURL)
	putPathPrefix(values, "DDAE_PING_PATH_PREFIX", document.DDAE.Paths.PingPrefix)
	putPathPrefix(values, "DDAE_API_PATH_PREFIX", document.DDAE.Paths.APIPrefix)
	putString(values, "DDAE_SOURCE_INSTANCE", document.DDAE.SourceInstance)
	putString(values, "DDAE_USERNAME_FILE", document.DDAE.Credentials.UsernameFile)
	putString(values, "DDAE_PASSWORD_FILE", document.DDAE.Credentials.PasswordFile)
	putString(values, "DDAE_CLIENT_SECRET_FILE", document.DDAE.Credentials.ClientSecretFile)
	putString(values, "DDAE_CA_FILE", document.DDAE.TLS.CAFile)
	putBool(values, "DDAE_TLS_INSECURE_SKIP_VERIFY", document.DDAE.TLS.InsecureSkipVerify)
	putString(values, "DDAE_REQUEST_TIMEOUT", document.DDAE.RequestTimeout)
	putString(values, "DDAE_CYCLE_TIMEOUT", document.DDAE.CycleTimeout)
	putInt64(values, "DDAE_RESPONSE_MAX_BYTES", document.DDAE.ResponseMaxBytes)
	putInt(values, "DDAE_RETRY_MAX", document.DDAE.RetryMax)

	if document.Kafka.Brokers != nil {
		values["KAFKA_BROKERS"] = strings.Join(document.Kafka.Brokers, ",")
	}
	putString(values, "KAFKA_TOPIC", document.Kafka.Topic)
	putString(values, "KAFKA_SERVICEABILITY_LOG_TOPIC", document.Kafka.ServiceabilityLogsTopic)
	putString(values, "KAFKA_CLIENT_ID", document.Kafka.ClientID)
	putString(values, "KAFKA_CA_FILE", document.Kafka.TLS.CAFile)
	putString(values, "KAFKA_CLIENT_CERT_FILE", document.Kafka.TLS.ClientCertFile)
	putString(values, "KAFKA_CLIENT_KEY_FILE", document.Kafka.TLS.ClientKeyFile)
	putBool(values, "KAFKA_TLS_INSECURE_SKIP_VERIFY", document.Kafka.TLS.InsecureSkipVerify)
	putString(values, "KAFKA_SASL_MECHANISM", document.Kafka.SASL.Mechanism)
	putString(values, "KAFKA_SASL_USERNAME", document.Kafka.SASL.Username)
	putString(values, "KAFKA_SASL_PASSWORD_FILE", document.Kafka.SASL.PasswordFile)
	putString(values, "KAFKA_PUBLISH_TIMEOUT", document.Kafka.PublishTimeout)

	putString(values, "STATE_DIR", document.State.Dir)
	putInt64(values, "KAFKA_OUTBOX_MAX_BYTES", document.State.OutboxMaxBytes)
	putInt(values, "KAFKA_OUTBOX_MAX_EVENTS", document.State.OutboxMaxEvents)
	putString(values, "CHECKPOINT_RETENTION", document.State.CheckpointRetention)
	putInt(values, "CHECKPOINT_MAX_ALERTS", document.State.CheckpointMaxAlerts)
	putInt64(values, "SERVICEABILITY_LOG_OUTBOX_MAX_BYTES", document.State.ServiceabilityLogsOutboxMaxBytes)
	putInt(values, "SERVICEABILITY_LOG_OUTBOX_MAX_EVENTS", document.State.ServiceabilityLogsOutboxMaxEvents)
	putString(values, "SERVICEABILITY_LOG_CHECKPOINT_RETENTION", document.State.ServiceabilityLogsCheckpointRetention)
	putInt(values, "SERVICEABILITY_LOG_CHECKPOINT_MAX_RECORDS", document.State.ServiceabilityLogsCheckpointMaxRecords)
	putString(values, "LOG_LEVEL", document.Logging.Level)
	putString(values, "LOG_FORMAT", document.Logging.Format)
	return values
}

func layeredLookup(values map[string]string, environment lookupFunc) lookupFunc {
	fileToDirect := map[string]string{
		"QUERY_USERNAME_FILE":      "QUERY_USERNAME",
		"QUERY_PASSWORD_FILE":      "QUERY_PASSWORD",
		"DDAE_USERNAME_FILE":       "DDAE_USERNAME",
		"DDAE_PASSWORD_FILE":       "DDAE_PASSWORD",
		"DDAE_CLIENT_SECRET_FILE":  "DDAE_CLIENT_SECRET",
		"KAFKA_SASL_PASSWORD_FILE": "KAFKA_SASL_PASSWORD",
	}
	return func(name string) (string, bool) {
		if value, ok := environment(name); ok {
			return value, true
		}
		if name == "DDAE_RESOURCE_COLLECTION_INTERVAL" || name == "DDAE_ALERT_COLLECTION_INTERVAL" {
			if value, ok := environment("DDAE_COLLECTION_INTERVAL"); ok {
				return value, true
			}
		}
		if direct := fileToDirect[name]; direct != "" {
			if _, ok := environment(direct); ok {
				return "", false
			}
		}
		value, ok := values[name]
		return value, ok
	}
}

func putString(values map[string]string, name string, value *string) {
	if value != nil {
		values[name] = *value
	}
}

func putPathPrefix(values map[string]string, name string, value *yamlPathPrefix) {
	if value != nil {
		values[name] = string(*value)
	}
}

func putBool(values map[string]string, name string, value *bool) {
	if value != nil {
		values[name] = strconv.FormatBool(*value)
	}
}

func putInt(values map[string]string, name string, value *int) {
	if value != nil {
		values[name] = strconv.Itoa(*value)
	}
}

func putInt64(values map[string]string, name string, value *int64) {
	if value != nil {
		values[name] = strconv.FormatInt(*value, 10)
	}
}

type yamlQueries struct {
	Backfill         yamlBackfill `yaml:"backfill"`
	Enabled          *bool        `yaml:"enabled"`
	Events           *bool        `yaml:"events_enabled"`
	BaseURL          *string      `yaml:"base_url"`
	AuthURL          *string      `yaml:"auth_url"`
	Realm            *string      `yaml:"realm"`
	Role             *string      `yaml:"role"`
	UsernameFile     *string      `yaml:"username_file"`
	PasswordFile     *string      `yaml:"password_file"`
	CAFile           *string      `yaml:"ca_file"`
	AuthCAFile       *string      `yaml:"auth_ca_file"`
	Insecure         *bool        `yaml:"insecure_skip_verify"`
	Topic            *string      `yaml:"kafka_topic"`
	Interval         *string      `yaml:"interval"`
	RequestTimeout   *string      `yaml:"request_timeout"`
	CycleTimeout     *string      `yaml:"cycle_timeout"`
	StaleAfter       *string      `yaml:"stale_after"`
	Retention        *string      `yaml:"checkpoint_retention"`
	MaxBytes         *int64       `yaml:"outbox_max_bytes"`
	ResponseMaxBytes *int64       `yaml:"response_max_bytes"`
	MaxEvents        *int         `yaml:"outbox_max_events"`
	MaxCheckpoints   *int         `yaml:"checkpoint_max_records"`
	MaxHistory       *int         `yaml:"max_history_records"`
	MaxPerCycle      *int         `yaml:"detail_max_per_cycle"`
	Concurrency      *int         `yaml:"detail_concurrency"`
	RetryMax         *int         `yaml:"retry_max"`
}

type yamlBackfill struct {
	Enabled        *bool   `yaml:"enabled"`
	Lookback       *string `yaml:"lookback"`
	Overlap        *string `yaml:"overlap"`
	Interval       *string `yaml:"interval"`
	CycleTimeout   *string `yaml:"cycle_timeout"`
	RescanInterval *string `yaml:"rescan_interval"`
	MaxPages       *int    `yaml:"max_pages_per_cycle"`
	DetailMax      *int    `yaml:"detail_max_per_cycle"`
	Concurrency    *int    `yaml:"detail_concurrency"`
	MaxPending     *int    `yaml:"max_pending_records"`
}

func (b yamlBackfill) put(values map[string]string, prefix string) {
	putBool(values, prefix+"ENABLED", b.Enabled)
	putString(values, prefix+"LOOKBACK", b.Lookback)
	putString(values, prefix+"OVERLAP", b.Overlap)
	putString(values, prefix+"INTERVAL", b.Interval)
	putString(values, prefix+"CYCLE_TIMEOUT", b.CycleTimeout)
	putString(values, prefix+"RESCAN_INTERVAL", b.RescanInterval)
	putInt(values, prefix+"MAX_PAGES_PER_CYCLE", b.MaxPages)
	putInt(values, prefix+"DETAIL_MAX_PER_CYCLE", b.DetailMax)
	putInt(values, prefix+"DETAIL_CONCURRENCY", b.Concurrency)
	putInt(values, prefix+"MAX_PENDING_RECORDS", b.MaxPending)
}
