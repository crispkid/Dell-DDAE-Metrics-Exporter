#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

expected_go_version='go1.26.6'
actual_go_version="$(go env GOVERSION)"
if [[ "$actual_go_version" != "$expected_go_version" ]]; then
  printf 'build requires %s, got %s\n' "$expected_go_version" "$actual_go_version" >&2
  exit 1
fi

version="${VERSION:-dev}"
revision="${REVISION:-$(git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)}"
build_date="${BUILD_DATE:-1970-01-01T00:00:00Z}"
mkdir -p bin
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=$version -X main.revision=$revision -X main.buildDate=$build_date" \
  -o bin/ddae-exporter ./cmd/ddae-exporter
