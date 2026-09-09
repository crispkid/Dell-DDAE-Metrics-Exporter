package portable

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"gopkg.in/yaml.v3"
)

var ErrInput = errors.New("invalid configuration, platform, key or input file")
var ErrLimit = errors.New("diagnostic resource limit reached")
var ErrStorage = errors.New("protected diagnostic storage failed")
var ErrIntegrity = errors.New("capture integrity or recipient key validation failed")

type Config struct {
	Version int `yaml:"version"`
	DDAE    struct {
		BaseURL string `yaml:"base_url"`
		Paths   struct {
			Ping string `yaml:"ping_prefix"`
			API  string `yaml:"api_prefix"`
		} `yaml:"paths"`
		Credentials struct {
			Username string `yaml:"username_file"`
			Password string `yaml:"password_file"`
			Secret   string `yaml:"client_secret_file"`
		} `yaml:"credentials"`
		TLS struct {
			CA       string `yaml:"ca_file"`
			Insecure bool   `yaml:"insecure_skip_verify"`
		} `yaml:"tls"`
		Timeout string `yaml:"request_timeout"`
		Retry   int    `yaml:"retry_max"`
	} `yaml:"ddae"`
	Security struct {
		Insecure bool `yaml:"allow_insecure_tls"`
	} `yaml:"security"`
	Checks struct {
		Ping      bool `yaml:"ping"`
		Resources bool `yaml:"resources"`
		Alerts    bool `yaml:"alerts"`
		Logs      bool `yaml:"serviceability_logs"`
	} `yaml:"checks"`
	Run struct {
		Duration    string `yaml:"duration"`
		Interval    string `yaml:"interval"`
		MaxRequests int    `yaml:"max_requests"`
		MaxDetails  int    `yaml:"max_details_per_family_per_cycle"`
		Grace       string `yaml:"shutdown_grace_period"`
	} `yaml:"run"`
	Capture struct {
		Enabled    bool   `yaml:"enabled"`
		PublicKey  string `yaml:"recipient_public_key_file"`
		BodyBytes  int64  `yaml:"max_body_bytes"`
		TotalBytes int64  `yaml:"max_total_bytes"`
	} `yaml:"capture"`
	Output struct {
		Directory string `yaml:"directory"`
		MaxBytes  int64  `yaml:"max_report_bytes"`
	} `yaml:"output"`
	Client                    config.Config `yaml:"-"`
	Duration, Interval, Grace time.Duration `yaml:"-"`
}

func defaultConfig() Config {
	var c Config
	c.Version = 1
	c.DDAE.Paths.API = "/v1"
	c.DDAE.Timeout = "5s"
	c.DDAE.Retry = 2
	c.Checks.Ping = true
	c.Checks.Resources = true
	c.Checks.Alerts = true
	c.Checks.Logs = true
	c.Run.Duration = "5m"
	c.Run.Interval = "30s"
	c.Run.Grace = "15s"
	c.Run.MaxRequests = 1000
	c.Run.MaxDetails = 10
	c.Capture.BodyBytes = 8 << 20
	c.Capture.TotalBytes = 512 << 20
	c.Output.Directory = "results"
	c.Output.MaxBytes = 16 << 20
	return c
}

func parseConfig(data []byte) (Config, error) {
	c := defaultConfig()
	if len(data) > 1<<20 {
		return c, ErrInput
	}
	var tree yaml.Node
	if yaml.Unmarshal(data, &tree) != nil || !validYAML(&tree, 0) {
		return c, ErrInput
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if dec.Decode(&c) != nil {
		return c, ErrInput
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return c, ErrInput
	}
	if c.Version != 1 || !hasVersion(&tree) {
		return c, ErrInput
	}
	var err error
	c.Duration, err = time.ParseDuration(c.Run.Duration)
	if err != nil || c.Duration < time.Second || c.Duration > time.Hour {
		return c, ErrInput
	}
	c.Interval, err = time.ParseDuration(c.Run.Interval)
	if err != nil || c.Interval < time.Second || c.Interval > 300*time.Second {
		return c, ErrInput
	}
	c.Grace, err = time.ParseDuration(c.Run.Grace)
	if err != nil || c.Grace < time.Second || c.Grace > 30*time.Second {
		return c, ErrInput
	}
	if c.Run.MaxRequests < 1 || c.Run.MaxRequests > 10000 || c.Run.MaxDetails < 0 || c.Run.MaxDetails > 100 ||
		c.Capture.BodyBytes < 1 || c.Capture.BodyBytes > 64<<20 || c.Capture.TotalBytes < 1<<20 || c.Capture.TotalBytes > 2<<30 ||
		c.Output.MaxBytes < 1<<20 || c.Output.MaxBytes > 64<<20 || c.Output.Directory == "" ||
		!(c.Checks.Ping || c.Checks.Resources || c.Checks.Alerts || c.Checks.Logs) {
		return c, ErrInput
	}
	return c, nil
}

func hasVersion(n *yaml.Node) bool {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == "version" && n.Content[i+1].Tag == "!!int" {
			return true
		}
	}
	return false
}
func validYAML(n *yaml.Node, depth int) bool {
	if depth > 32 || n.Kind == yaml.AliasNode || n.Anchor != "" || n.Tag == "!!null" || n.Tag == "!!merge" {
		return false
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Tag != "!!str" || seen[k.Value] {
				return false
			}
			seen[k.Value] = true
			want := ""
			switch k.Value {
			case "base_url", "ping_prefix", "api_prefix", "username_file", "password_file", "client_secret_file", "ca_file", "request_timeout", "duration", "interval", "shutdown_grace_period", "recipient_public_key_file", "directory":
				want = "!!str"
			case "insecure_skip_verify", "allow_insecure_tls", "ping", "resources", "alerts", "serviceability_logs", "enabled":
				want = "!!bool"
			case "version", "retry_max", "max_requests", "max_details_per_family_per_cycle", "max_body_bytes", "max_total_bytes", "max_report_bytes":
				want = "!!int"
			}
			if want != "" && n.Content[i+1].Tag != want {
				return false
			}
		}
	}
	for _, child := range n.Content {
		if !validYAML(child, depth+1) {
			return false
		}
	}
	return true
}

func readBounded(path string, limit int64) ([]byte, error) {
	if safePath(path) != nil {
		return nil, ErrInput
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrInput
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > limit {
		return nil, ErrInput
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, ErrInput
	}
	return data, nil
}

func LoadConfig(path string) (Config, error) {
	data, err := readBounded(path, 1<<20)
	if err != nil {
		return Config{}, err
	}
	c, err := parseConfig(data)
	if err != nil {
		return c, err
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return c, ErrInput
	}
	resolve := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}
	c.Output.Directory = resolve(c.Output.Directory)
	c.Capture.PublicKey = resolve(c.Capture.PublicKey)
	c.DDAE.TLS.CA = resolve(c.DDAE.TLS.CA)
	c.DDAE.Credentials.Username = resolve(c.DDAE.Credentials.Username)
	c.DDAE.Credentials.Password = resolve(c.DDAE.Credentials.Password)
	c.DDAE.Credentials.Secret = resolve(c.DDAE.Credentials.Secret)
	for _, p := range []string{c.DDAE.Credentials.Username, c.DDAE.Credentials.Password, c.DDAE.Credentials.Secret} {
		b, e := readBounded(p, 64<<10)
		if e != nil || bytes.HasPrefix(b, []byte{239, 187, 191}) {
			return c, ErrInput
		}
	}
	if c.DDAE.TLS.CA != "" {
		if _, err := readBounded(c.DDAE.TLS.CA, 1<<20); err != nil {
			return c, ErrInput
		}
	}
	timeout, err := time.ParseDuration(c.DDAE.Timeout)
	if err != nil || timeout <= 0 || timeout > time.Hour {
		return c, ErrInput
	}
	// These are validation-only resource scheduler values, not diagnostic timing.
	values := map[string]string{
		"DDAE_BASE_URL": c.DDAE.BaseURL, "DDAE_SOURCE_INSTANCE": "portable-diagnostics",
		"DDAE_PING_PATH_PREFIX": c.DDAE.Paths.Ping, "DDAE_API_PATH_PREFIX": c.DDAE.Paths.API,
		"DDAE_USERNAME_FILE": c.DDAE.Credentials.Username, "DDAE_PASSWORD_FILE": c.DDAE.Credentials.Password,
		"DDAE_CLIENT_SECRET_FILE":          c.DDAE.Credentials.Secret,
		"DDAE_RESOURCE_MONITORING_ENABLED": "true", "DDAE_ALERT_MONITORING_ENABLED": "false",
		"DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED": "false",
		"ALLOW_INSECURE_TLS":                         strconv.FormatBool(c.Security.Insecure),
		"DDAE_TLS_INSECURE_SKIP_VERIFY":              strconv.FormatBool(c.DDAE.TLS.Insecure),
		"DDAE_CA_FILE":                               c.DDAE.TLS.CA, "DDAE_REQUEST_TIMEOUT": c.DDAE.Timeout,
		"DDAE_CYCLE_TIMEOUT":                (timeout + time.Second).String(),
		"DDAE_RESOURCE_COLLECTION_INTERVAL": (timeout + 2*time.Second).String(),
		"DDAE_STALE_AFTER":                  (timeout + 3*time.Second).String(), "DDAE_RETRY_MAX": strconv.Itoa(c.DDAE.Retry),
	}
	c.Client, err = config.LoadIsolated(values)
	if err != nil || strings.HasSuffix(c.Client.DDAEBaseURL.Hostname(), ".invalid") {
		return c, ErrInput
	}
	if c.Capture.Enabled {
		if _, err := ReadPublicKey(c.Capture.PublicKey); err != nil {
			return c, ErrInput
		}
	}
	return c, nil
}
