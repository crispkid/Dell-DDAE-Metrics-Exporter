package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TEST-DDAE-10-009. Keep deployable references complete when the supported configuration grows.
func TestCompleteConfigurationExamples(t *testing.T) {
	for _, path := range []string{"deploy/systemd/config.example.yaml", "deploy/kubernetes/configmap.yaml", "deploy/query-monitoring.example.yaml"} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../..", path))
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(path, "configmap.yaml") {
				var cm struct {
					Data map[string]string `yaml:"data"`
				}
				if err := yaml.Unmarshal(data, &cm); err != nil {
					t.Fatal(err)
				}
				data = []byte(cm.Data["config.yaml"])
			}
			var document map[string]any
			if err := yaml.Unmarshal(data, &document); err != nil {
				t.Fatal(err)
			}
			queryOnly := strings.Contains(path, "query-monitoring")
			if queryOnly {
				monitoring, ok := document["monitoring"].(map[string]any)
				if !ok {
					t.Fatal("missing monitoring")
				}
				queries, ok := monitoring["queries"].(map[string]any)
				if !ok {
					t.Fatal("missing queries")
				}
				assertExampleFields(t, reflect.TypeOf(yamlQueries{}), queries, "monitoring.queries.")
			} else {
				assertExampleFields(t, reflect.TypeOf(yamlConfig{}), document, "")
			}
			values, err := decodeYAML(data)
			if err != nil {
				t.Fatal(err)
			}
			// Synthetic secrets and a host-native directory isolate this from the
			// machine environment. No external login or certificate validation occurs.
			environment := map[string]string{"STATE_DIR": t.TempDir()}
			lookup := func(key string) (string, bool) { value, ok := environment[key]; return value, ok }
			readSecret := func(string) ([]byte, error) { return []byte("example-test-secret"), nil }
			cfg, err := load(layeredLookup(values, lookup), readSecret)
			if err != nil {
				t.Fatalf("default example: %v", err)
			}
			if cfg.Query.Enabled != queryOnly || cfg.Query.Events || cfg.ServiceabilityLogMonitoringEnabled {
				t.Fatal("example unexpectedly enables optional pipelines")
			}
			if !queryOnly {
				environment["QUERY_BACKFILL_ENABLED"] = "true"
				environment["SERVICEABILITY_LOG_BACKFILL_ENABLED"] = "true"
				environment["QUERY_ENABLED"] = "true"
				environment["QUERY_EVENTS_ENABLED"] = "true"
				environment["DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED"] = "true"
				cfg, err = load(layeredLookup(values, lookup), readSecret)
				if err != nil {
					t.Fatalf("all features enabled: %v", err)
				}
				if !cfg.LogBackfill.Enabled || !cfg.QueryBackfill.Enabled || !cfg.ResourceMonitoringEnabled || !cfg.AlertMonitoringEnabled || !cfg.ServiceabilityLogMonitoringEnabled || !cfg.Query.Enabled || !cfg.Query.Events {
					t.Fatal("not all pipelines enabled")
				}
			}
		})
	}
}

func assertExampleFields(t *testing.T, schema reflect.Type, values map[string]any, prefix string) {
	t.Helper()
	for i := 0; i < schema.NumField(); i++ {
		field := schema.Field(i)
		key := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if key == "" || key == "-" {
			continue
		}
		value, ok := values[key]
		if !ok || value == nil {
			t.Errorf("missing setting %s%s", prefix, key)
			continue
		}
		if field.Type.Kind() == reflect.Struct {
			mapping, ok := value.(map[string]any)
			if !ok {
				t.Errorf("setting %s%s must be a mapping", prefix, key)
				continue
			}
			assertExampleFields(t, field.Type, mapping, prefix+key+".")
		}
	}
}

func TestQueryExampleKubernetesMount(t *testing.T) {
	data, err := os.ReadFile("../../deploy/kubernetes/deployment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Decode all YAML documents, then verify the optional query secret's mount.
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	found := false
	for i := 0; i < 4; i++ {
		var doc struct {
			Kind string `yaml:"kind"`
			Spec struct {
				Template struct {
					Spec struct {
						Containers []struct {
							VolumeMounts []struct {
								Name      string
								MountPath string `yaml:"mountPath"`
								ReadOnly  bool   `yaml:"readOnly"`
							} `yaml:"volumeMounts"`
						} `yaml:"containers"`
						Volumes []struct {
							Name   string
							Secret struct {
								SecretName string `yaml:"secretName"`
								Optional   bool
							}
						} `yaml:"volumes"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		}
		if err := decoder.Decode(&doc); err != nil {
			t.Fatal(err)
		}
		if doc.Kind != "Deployment" {
			continue
		}
		found = true
		pod := doc.Spec.Template.Spec
		mounted, provided := false, false
		for _, c := range pod.Containers {
			for _, m := range c.VolumeMounts {
				if m.Name == "query-credentials" && m.MountPath == "/run/secrets/query" && m.ReadOnly {
					mounted = true
				}
			}
		}
		for _, v := range pod.Volumes {
			if v.Name == "query-credentials" && v.Secret.SecretName == "ddae-exporter-query" && v.Secret.Optional {
				provided = true
			}
		}
		if !mounted || !provided {
			t.Fatal("query credential mount/optional Secret missing")
		}
	}
	if !found {
		t.Fatal("Deployment missing")
	}
}
