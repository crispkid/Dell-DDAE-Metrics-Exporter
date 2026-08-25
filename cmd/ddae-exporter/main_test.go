package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
)

func TestWarnInsecureTLSIsBoundedAndOncePerEffectiveTarget(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	cfg := config.Config{
		AllowInsecureTLS:           true,
		AlertMonitoringEnabled:     true,
		DDAETLSInsecureSkipVerify:  true,
		KafkaTLSInsecureSkipVerify: true,
	}
	warnInsecureTLS(logger, cfg)

	logOutput := output.String()
	if strings.Count(logOutput, "TLS certificate and hostname verification disabled") != 2 {
		t.Fatalf("warning count/output = %q", logOutput)
	}
	if strings.Count(logOutput, `"target":"ddae"`) != 1 || strings.Count(logOutput, `"target":"kafka"`) != 1 {
		t.Fatalf("target warning count/output = %q", logOutput)
	}
	for _, forbidden := range []string{"https://", "broker", "topic", "password", "secret"} {
		if strings.Contains(strings.ToLower(logOutput), forbidden) {
			t.Fatalf("warning exposed forbidden value %q: %s", forbidden, logOutput)
		}
	}
}

func TestWarnInsecureTLSOmitsDisabledKafkaTarget(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	warnInsecureTLS(logger, config.Config{KafkaTLSInsecureSkipVerify: true})
	if output.Len() != 0 {
		t.Fatalf("disabled Kafka warning = %q", output.String())
	}
}

func TestCommandLineRejectsUnknownPositionalAndEmptyConfig(t *testing.T) {
	for name, arguments := range map[string][]string{
		"unknown":    {"--unknown"},
		"positional": {"config.yaml"},
		"empty":      {"--config="},
	} {
		t.Run(name, func(t *testing.T) {
			if err := runWithArgs(arguments); err == nil {
				t.Fatal("invalid command line was accepted")
			}
		})
	}
}
