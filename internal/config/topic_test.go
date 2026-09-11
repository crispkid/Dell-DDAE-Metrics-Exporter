package config

import (
	"strconv"
	"strings"
	"testing"
)

// TEST-DDAE-12-008: broker grammar, across all three configuration paths.
func TestAuditKafkaTopicGrammar(t *testing.T) {
	for _, key := range []string{"KAFKA_TOPIC", "KAFKA_SERVICEABILITY_LOG_TOPIC", "QUERY_KAFKA_TOPIC"} {
		for _, topic := range []string{"a", "A_Z-09.events", strings.Repeat("x", 249), "", ".", "..", "bad/topic", "中文", "two words", "x\x00", strings.Repeat("x", 250)} {
			t.Run(key+"/"+strconv.Quote(topic), func(t *testing.T) {
				env := queryEventsSASLEnvironment(t)
				env[key] = topic
				_, err := loadMap(env, nil)
				valid := topic == "a" || topic == "A_Z-09.events" || len(topic) == 249
				if (err == nil) != valid {
					t.Fatalf("valid=%v load error=%v", valid, err)
				}
			})
		}
	}
}

func TestAuditKafkaTopicYAMLPrecedenceAndDisabledIsolation(t *testing.T) {
	for key, yamlKey := range map[string]string{"KAFKA_TOPIC": "kafka:\n  topic:", "KAFKA_SERVICEABILITY_LOG_TOPIC": "kafka:\n  serviceability_logs_topic:", "QUERY_KAFKA_TOPIC": "monitoring:\n  queries:\n    kafka_topic:"} {
		env := queryEventsSASLEnvironment(t)
		delete(env, key)
		document := "version: 1\n" + yamlKey + " bad/topic\n"
		if _, err := loadYAMLMap(document, env, nil); err == nil {
			t.Fatalf("invalid YAML %s accepted", key)
		}
		env[key] = "valid-topic"
		if _, err := loadYAMLMap(document, env, nil); err != nil {
			t.Fatalf("env precedence %s: %v", key, err)
		}
	}
	env := queryEventsSASLEnvironment(t)
	env["QUERY_EVENTS_ENABLED"] = "false"
	env["QUERY_KAFKA_TOPIC"] = "bad/topic"
	if _, err := loadMap(env, nil); err != nil {
		t.Fatalf("disabled events topic consulted: %v", err)
	}
}
