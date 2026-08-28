#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

if ! grep -Fqx 'toolchain go1.26.6' go.mod; then
  printf 'security failed: go.mod must pin toolchain go1.26.6\n' >&2
  exit 1
fi
if ! grep -Fqx 'FROM golang:1.26.6-alpine3.23@sha256:e57c41c1d5864341031181b0db34b9a537bb5773eb6428e4e5bdaea0f9135406 AS build' Dockerfile; then
  printf 'security failed: Dockerfile must pin the approved Go 1.26.6 builder digest\n' >&2
  exit 1
fi
if [[ "$(go env GOVERSION)" != 'go1.26.6' ]]; then
  printf 'security failed: effective Go toolchain is not go1.26.6\n' >&2
  exit 1
fi
if ! rg -q 'kgo\.AllowIdempotentProduceCancellation\(\)' internal/kafka/producer.go; then
  printf 'security failed: Kafka producer must permit cancellation of uncertain in-flight idempotent produce requests\n' >&2
  exit 1
fi
if ! rg -q 'maxResponseBodyBytes[[:space:]]*=[[:space:]]*64 \* 1024 \* 1024' internal/ddae/client.go ||
  ! rg -q 'maxResponseHeaderBytes[[:space:]]*=[[:space:]]*1024 \* 1024' internal/ddae/client.go ||
  ! rg -q 'MaxResponseHeaderBytes:[[:space:]]*maxResponseHeaderBytes' internal/ddae/client.go; then
  printf 'security failed: DDAE body/header response ceilings differ from the approved bounds\n' >&2
  exit 1
fi
if ! rg -q 'MaxRequestsInFlight:[[:space:]]*5' internal/server/server.go ||
  ! rg -q 'Timeout:[[:space:]]*9 \* time\.Second' internal/server/server.go; then
  printf 'security failed: Prometheus handler concurrency/timeout differs from the approved bounds\n' >&2
  exit 1
fi
if ! rg -q 'serviceabilityLogListPath[[:space:]]*=[[:space:]]*"/rest/v1/serviceability-events"' internal/ddae/allowlist.go ||
  ! rg -q 'serviceabilityLogDetailPath[[:space:]]*=[[:space:]]*"/rest/v1/serviceability-events/"' internal/ddae/allowlist.go ||
  ! rg -q 'filepath\.Join\(options\.StateDir, "serviceability-logs\.db"\)' internal/logstate/store.go ||
  ! rg -q 'PublishServiceabilityLog' internal/kafka/producer.go ||
  ! rg -q '"serviceability_log"' internal/kafka/producer.go ||
  ! rg -q 'Key: "ddae-record-kind"' internal/kafka/producer.go; then
  printf 'security failed: Serviceability Logs route/state/record-kind isolation differs from the approved contract\n' >&2
  exit 1
fi
if rg -n 'descLabels\("ddae_serviceability_log_[^"]+"[^\n]*(log_id|message|resource|topic|endpoint|timestamp)' internal/metrics --glob '*.go'; then
  printf 'security failed: Serviceability Log content entered a Prometheus label contract\n' >&2
  exit 1
fi
serviceability_reason_labels="$(rg -n 'descLabels\("ddae_serviceability_log_[^"]+"[^\n]*"reason"' internal/metrics --glob '*.go' || true)"
if [[ "$(printf '%s\n' "$serviceability_reason_labels" | sed '/^$/d' | wc -l | tr -d ' ')" != "1" ]] ||
  ! printf '%s\n' "$serviceability_reason_labels" | rg -q 'ddae_serviceability_log_records_failed_total'; then
  printf 'security failed: Serviceability Log reason label differs from the single fixed failure-class contract\n' >&2
  exit 1
fi

go vet ./...
go test \
  ./cmd/ddae-exporter \
  ./internal/config \
  ./internal/contract \
  ./internal/ddae \
  ./internal/kafka \
  ./internal/observability \
  ./internal/alerts \
  ./internal/serviceability \
  ./internal/logstate \
  ./internal/logpublisher \
  -run 'YAML|Secret|TLS|Insecure|Warn|Allowlist|Mutation|Redact|DetailPath|Sensitive'

if rg -n 'ProxyFromEnvironment|http\.Method(Patch|Put|Delete)' internal/ddae --glob '*.go' --glob '!*_test.go'; then
	printf 'security failed: unsafe proxy setting or DDAE mutation method found\n' >&2
	exit 1
fi
if rg -n 'http\.MethodPost' internal/ddae --glob '*.go' --glob '!*_test.go' --glob '!auth.go'; then
  printf 'security failed: DDAE POST exists outside the fixed token flow\n' >&2
	exit 1
fi

if rg -n 'InsecureSkipVerify:[[:space:]]*true' internal --glob '*.go' --glob '!*_test.go'; then
  printf 'security failed: unconditional TLS verification bypass found\n' >&2
  exit 1
fi
if rg -n '\.InsecureSkipVerify[[:space:]]*=' internal --glob '*.go' --glob '!*_test.go'; then
  printf 'security failed: post-construction TLS verification mutation found\n' >&2
  exit 1
fi

tls_assignments="$(rg -n 'InsecureSkipVerify:' internal --glob '*.go' --glob '!*_test.go' || true)"
if [[ "$(printf '%s\n' "$tls_assignments" | sed '/^$/d' | wc -l | tr -d ' ')" != "2" ]] ||
  ! printf '%s\n' "$tls_assignments" | rg -q '^internal/ddae/client\.go:.*InsecureSkipVerify: skipVerification.*#nosec G402' ||
  ! printf '%s\n' "$tls_assignments" | rg -q '^internal/kafka/producer\.go:.*InsecureSkipVerify: cfg\.KafkaTLSInsecureSkipVerify.*#nosec G402'; then
  printf 'security failed: TLS verification bypass differs from the two approved guarded targets\n' >&2
  exit 1
fi

if rg -n '^[[:space:]]+(password|client_secret|private_key):' \
  deploy --glob '*.yaml' --glob '*.yml'; then
  printf 'security failed: plaintext-secret YAML field found in a deployment profile\n' >&2
  exit 1
fi

if ! rg -q '^\s*gopkg\.in/yaml\.v3 v3\.0\.1$' go.mod; then
  printf 'security failed: approved strict YAML parser is not a direct pinned dependency\n' >&2
  exit 1
fi

if ! command -v govulncheck >/dev/null 2>&1; then
  printf 'security blocked: install the approved govulncheck tool\n' >&2
  exit 3
fi
govulncheck ./...
