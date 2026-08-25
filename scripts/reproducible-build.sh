#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

if [[ "$(go env GOVERSION)" != 'go1.26.6' ]]; then
  printf 'reproducible build failed: effective Go toolchain is not go1.26.6\n' >&2
  exit 1
fi

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/ddae-build.XXXXXX")"
trap 'rm -rf "$tmp_dir"' EXIT

for output in first second; do
  CGO_ENABLED=0 go build -trimpath \
    -ldflags '-s -w -buildid= -X main.version=test -X main.revision=verified -X main.buildDate=1970-01-01T00:00:00Z' \
    -o "$tmp_dir/$output" ./cmd/ddae-exporter
done
cmp "$tmp_dir/first" "$tmp_dir/second"
sha256sum_command='sha256sum'
command -v sha256sum >/dev/null 2>&1 || sha256sum_command='shasum -a 256'
$sha256sum_command "$tmp_dir/first"
