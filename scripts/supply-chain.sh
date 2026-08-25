#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

go mod verify
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  printf 'supply-chain blocked: release inputs must come from a clean committed revision\n' >&2
  exit 3
fi
go list -m all
./scripts/reproducible-build.sh

if [[ ! -x bin/ddae-exporter ]]; then
  ./scripts/build.sh
fi
if ! command -v cyclonedx-gomod >/dev/null 2>&1; then
  printf 'supply-chain blocked: install cyclonedx-gomod v1.10.0\n' >&2
  exit 3
fi

mkdir -p dist
CGO_ENABLED=0 GOOS="$(go env GOOS)" GOARCH="$(go env GOARCH)" \
  cyclonedx-gomod app -json -licenses -output dist/ddae-exporter.cdx.json \
  -main cmd/ddae-exporter .

checksum_command=(shasum -a 256)
if command -v sha256sum >/dev/null 2>&1; then
  checksum_command=(sha256sum)
fi
"${checksum_command[@]}" bin/ddae-exporter dist/ddae-exporter.cdx.json > dist/SHA256SUMS
{
  git rev-parse HEAD 2>/dev/null || printf 'uncommitted-worktree\n'
  go version
  "${checksum_command[@]}" go.mod go.sum Dockerfile
  go version -m bin/ddae-exporter
} > dist/provenance-inputs.txt
