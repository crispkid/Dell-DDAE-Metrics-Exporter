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
if ! grep -Eq 'kgo\.AllowIdempotentProduceCancellation\(\)' internal/kafka/producer.go; then
  printf 'security failed: Kafka producer must permit cancellation of uncertain in-flight idempotent produce requests\n' >&2
  exit 1
fi
if ! grep -Eq 'maxResponseBodyBytes[[:space:]]*=[[:space:]]*64 \* 1024 \* 1024' internal/ddae/client.go ||
  ! grep -Eq 'maxResponseHeaderBytes[[:space:]]*=[[:space:]]*1024 \* 1024' internal/ddae/client.go ||
  ! grep -Eq 'MaxResponseHeaderBytes:[[:space:]]*maxResponseHeaderBytes' internal/ddae/client.go; then
  printf 'security failed: DDAE body/header response ceilings differ from the approved bounds\n' >&2
  exit 1
fi
if ! grep -Eq 'MaxRequestsInFlight:[[:space:]]*5' internal/server/server.go ||
  ! grep -Eq 'Timeout:[[:space:]]*9 \* time\.Second' internal/server/server.go; then
  printf 'security failed: Prometheus handler concurrency/timeout differs from the approved bounds\n' >&2
  exit 1
fi
if ! grep -Eq 'DefaultDDAEPingPathPrefix[[:space:]]*=[[:space:]]*""' internal/config/config.go ||
  ! grep -Eq 'DefaultDDAEAPIPathPrefix[[:space:]]*=[[:space:]]*"/v1"' internal/config/config.go ||
  ! grep -Eq 'pingSuffix[[:space:]]*=[[:space:]]*"/ping"' internal/ddae/allowlist.go ||
  ! grep -Eq 'serviceabilityLogListSuffix[[:space:]]*=[[:space:]]*"/serviceability-events"' internal/ddae/allowlist.go ||
  ! grep -Eq 'serviceabilityLogDetailSuffix[[:space:]]*=[[:space:]]*"/serviceability-events/"' internal/ddae/allowlist.go ||
  ! grep -Eq 'tokenPath[[:space:]]*=[[:space:]]*"/auth/realms/ddae/protocol/openid-connect/token"' internal/ddae/allowlist.go ||
  ! grep -Eq 'ping:[[:space:]]+pingPrefix \+ pingSuffix' internal/ddae/allowlist.go ||
  ! grep -Eq 'serviceabilityLogList:[[:space:]]+apiPrefix \+ serviceabilityLogListSuffix' internal/ddae/allowlist.go ||
  ! grep -Eq 'filepath\.Join\(options\.StateDir, "serviceability-logs\.db"\)' internal/logstate/store.go ||
  ! grep -Eq 'PublishServiceabilityLog' internal/kafka/producer.go ||
  ! grep -Eq '"serviceability_log"' internal/kafka/producer.go ||
  ! grep -Eq 'Key: "ddae-record-kind"' internal/kafka/producer.go; then
  printf 'security failed: DDAE prefix/suffix/token or Serviceability Logs isolation differs from the approved contract\n' >&2
  exit 1
fi
if grep -ERn --include='*.go' --exclude='*_test.go' 'ResolveReference|path\.Join|url\.JoinPath' internal/ddae; then
  printf 'security failed: DDAE routes must use exact validated prefix-plus-suffix composition\n' >&2
  exit 1
fi
if grep -ERn --include='*.go' 'descLabels\("ddae_serviceability_log_[^"]+".*(log_id|message|resource|topic|endpoint|timestamp)' internal/metrics; then
  printf 'security failed: Serviceability Log content entered a Prometheus label contract\n' >&2
  exit 1
fi
serviceability_reason_labels="$(grep -ERn --include='*.go' 'descLabels\("ddae_serviceability_log_[^"]+".*"reason"' internal/metrics || true)"
if [[ "$(printf '%s\n' "$serviceability_reason_labels" | sed '/^$/d' | wc -l | tr -d ' ')" != "1" ]] ||
  ! printf '%s\n' "$serviceability_reason_labels" | grep -Eq 'ddae_serviceability_log_records_failed_total'; then
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
  -run 'YAML|Secret|TLS|Insecure|Warn|Allowlist|Mutation|Redact|DetailPath|Sensitive|PathPrefix|ConfiguredPath'

if grep -ERn --include='*.go' --exclude='*_test.go' 'ProxyFromEnvironment|http\.Method(Patch|Put|Delete)' internal/ddae; then
	printf 'security failed: unsafe proxy setting or DDAE mutation method found\n' >&2
	exit 1
fi
if grep -ERn --include='*.go' --exclude='*_test.go' --exclude='auth.go' 'http\.MethodPost' internal/ddae; then
  printf 'security failed: DDAE POST exists outside the fixed token flow\n' >&2
	exit 1
fi

if grep -ERn --include='*.go' --exclude='*_test.go' 'InsecureSkipVerify:[[:space:]]*true' internal; then
  printf 'security failed: unconditional TLS verification bypass found\n' >&2
  exit 1
fi
if grep -ERn --include='*.go' --exclude='*_test.go' '\.InsecureSkipVerify[[:space:]]*=' internal; then
  printf 'security failed: post-construction TLS verification mutation found\n' >&2
  exit 1
fi

tls_assignments="$(grep -ERn --include='*.go' --exclude='*_test.go' 'InsecureSkipVerify:' internal || true)"
if [[ "$(printf '%s\n' "$tls_assignments" | sed '/^$/d' | wc -l | tr -d ' ')" != "2" ]] ||
  ! printf '%s\n' "$tls_assignments" | grep -Eq '^internal/ddae/client\.go:.*InsecureSkipVerify: skipVerification.*#nosec G402' ||
  ! printf '%s\n' "$tls_assignments" | grep -Eq '^internal/kafka/producer\.go:.*InsecureSkipVerify: cfg\.KafkaTLSInsecureSkipVerify.*#nosec G402'; then
  printf 'security failed: TLS verification bypass differs from the two approved guarded targets\n' >&2
  exit 1
fi

if grep -ERn --include='*.yaml' --include='*.yml' '^[[:space:]]+(password|client_secret|private_key):' \
  deploy; then
  printf 'security failed: plaintext-secret YAML field found in a deployment profile\n' >&2
  exit 1
fi

if ! grep -Eq '^[[:space:]]*gopkg\.in/yaml\.v3 v3\.0\.1$' go.mod; then
  printf 'security failed: approved strict YAML parser is not a direct pinned dependency\n' >&2
  exit 1
fi

if ! command -v govulncheck >/dev/null 2>&1; then
  printf 'security blocked: install the approved govulncheck tool\n' >&2
  exit 3
fi
govulncheck ./...
