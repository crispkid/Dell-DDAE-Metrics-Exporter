package config

import (
	"strings"
	"testing"
)

func TestDDAEPathPrefixYAMLAndEnvironmentPrecedence(t *testing.T) {
	document := strings.Replace(resourceOnlyYAML(), "  base_url: https://ddae.example.test\n", `  base_url: https://ddae.example.test
  paths:
    ping_prefix: /yaml-ping
    api_prefix: ""
`, 1)
	cfg, err := loadYAMLMap(document, map[string]string{
		"DDAE_PING_PATH_PREFIX": "",
		"DDAE_API_PATH_PREFIX":  "/environment-api",
	}, testSecrets())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DDAEPingPathPrefix != "" || cfg.DDAEAPIPathPrefix != "/environment-api" {
		t.Fatalf("effective prefixes ping=%q api=%q", cfg.DDAEPingPathPrefix, cfg.DDAEAPIPathPrefix)
	}

	cfg, err = loadYAMLMap(document, nil, testSecrets())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DDAEPingPathPrefix != "/yaml-ping" || cfg.DDAEAPIPathPrefix != "" {
		t.Fatalf("YAML prefixes ping=%q api=%q", cfg.DDAEPingPathPrefix, cfg.DDAEAPIPathPrefix)
	}
}

func TestValidateDDAEPathPrefixBoundaries(t *testing.T) {
	for _, value := range []string{
		"",
		"/v1",
		"/rest/v1",
		"/a.b_c-d~e",
		"/" + strings.Repeat("a", 127),
	} {
		if err := ValidateDDAEPathPrefix(value); err != nil {
			t.Fatalf("valid prefix %q rejected: %v", value, err)
		}
	}

	for name, value := range map[string]string{
		"root":            "/",
		"relative":        "v1",
		"trailing-slash":  "/v1/",
		"repeated-slash":  "/rest//v1",
		"dot-segment":     "/./v1",
		"dot-dot-segment": "/rest/../v1",
		"percent":         "/rest/%2Fv1",
		"backslash":       `/rest\v1`,
		"space":           "/rest v1",
		"tab":             "/rest\tv1",
		"query":           "/rest?version=1",
		"fragment":        "/rest#v1",
		"scheme":          "https://example.test/rest",
		"unicode":         "/版本",
		"over-byte-limit": "/" + strings.Repeat("a", 128),
		"control":         "/rest\nv1",
		"embedded-nul":    "/rest\x00v1",
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateDDAEPathPrefix(value); err == nil {
				t.Fatalf("invalid prefix %q accepted", value)
			}
		})
	}
}

func TestDDAEPathPrefixErrorsDoNotEchoValues(t *testing.T) {
	environment := validEnvironment()
	environment["DDAE_PING_PATH_PREFIX"] = "/secret?token=prefix-canary"
	_, err := loadMap(environment, nil)
	if err == nil {
		t.Fatal("invalid prefix was accepted")
	}
	if strings.Contains(err.Error(), "prefix-canary") {
		t.Fatalf("configuration error exposed prefix: %v", err)
	}
}

func TestDDAEPathPrefixYAMLRejectsUnknownAndNonStringFields(t *testing.T) {
	for name, paths := range map[string]string{
		"unknown": "    typo_prefix: /v1\n",
		"integer": "    ping_prefix: 1\n",
		"boolean": "    api_prefix: true\n",
	} {
		t.Run(name, func(t *testing.T) {
			document := strings.Replace(resourceOnlyYAML(), "  base_url: https://ddae.example.test\n", "  base_url: https://ddae.example.test\n  paths:\n"+paths, 1)
			if _, err := loadYAMLMap(document, nil, testSecrets()); err == nil {
				t.Fatal("invalid typed path-prefix YAML was accepted")
			}
		})
	}
}
