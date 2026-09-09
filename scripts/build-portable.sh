#!/usr/bin/env bash
set -euo pipefail
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"
if [[ "$(go env GOVERSION)" != 'go1.26.6' ]]; then
  printf 'Portable build requires Go 1.26.6\n' >&2
  exit 1
fi
stage_base="${TMPDIR:-/tmp}"
stage_base="$(cd "$stage_base" && pwd -P)"
stage_dir="$(mktemp -d "$stage_base/ddae-portable-build.XXXXXX")"
source_id="$(go run ./cmd/portable-package --identity)"
revision="$(git rev-parse --short=12 HEAD)"
for arch in amd64 arm64; do
  mkdir -p "$stage_dir/bin/windows-$arch"
  GOOS=windows GOARCH="$arch" CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=ddae7-local -X main.revision=$revision -X main.source=$source_id" \
    -o "$stage_dir/bin/windows-$arch/ddae-diagnose.exe" ./cmd/ddae-diagnose
  GOOS=windows GOARCH="$arch" CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=ddae7-local -X main.revision=$revision" \
    -o "$stage_dir/bin/windows-$arch/ddae-exporter.exe" ./cmd/ddae-exporter
done
mkdir -p "$stage_dir/tools/darwin-arm64"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=ddae7-local -X main.revision=$revision -X main.source=$source_id" \
  -o "$stage_dir/tools/darwin-arm64/ddae-diagnose" ./cmd/ddae-diagnose
go run ./cmd/portable-package --stage "$stage_dir" --source "$source_id"
printf 'Portable package built; Windows runtime and authenticated DDAE remain unverified.\n'
