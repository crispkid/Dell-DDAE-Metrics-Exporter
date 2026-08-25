#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

unformatted="$(find cmd internal integration -type f -name '*.go' -print 2>/dev/null | sort | xargs gofmt -l 2>/dev/null || true)"
if [[ -n "$unformatted" ]]; then
  printf 'Go files require gofmt:\n%s\n' "$unformatted" >&2
  exit 1
fi

go vet ./...
