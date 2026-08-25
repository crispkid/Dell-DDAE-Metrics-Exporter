#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

mkdir -p coverage
profile='coverage/coverage.out'
go test -covermode=atomic -coverprofile="$profile" ./...

total="$(go tool cover -func="$profile" | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')"
threshold="${HARNESS_COVERAGE_THRESHOLD:-80}"
awk -v total="$total" -v threshold="$threshold" 'BEGIN { if (total + 0 < threshold + 0) exit 1 }' || {
  printf 'coverage %s%% is below required %s%%\n' "$total" "$threshold" >&2
  exit 1
}
printf 'coverage: %s%%\n' "$total"
