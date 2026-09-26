#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

out_dir="${GOMEMORY_COVERAGE_OUTPUT:-$(mktemp -d "${TMPDIR:-/tmp}/gomemory-coverage.XXXXXX")}"
mkdir -p "$out_dir/contract"

go test ./... -count=1 -coverpkg=./... -coverprofile="$out_dir/unit.cover"
GOMEMORY_COVERAGE_DIR="$out_dir/contract" go test ./tests/contract -count=1
go tool covdata textfmt -i="$out_dir/contract" -o="$out_dir/contract.cover"
python3 scripts/merge_coverage.py "$out_dir/combined.cover" "$out_dir/unit.cover" "$out_dir/contract.cover"

total="$(go tool cover -func="$out_dir/combined.cover" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"
printf 'Cobertura global combinada: %s%%\nPerfil: %s\n' "$total" "$out_dir/combined.cover"
awk -v total="$total" 'BEGIN { if (total + 0 < 80) exit 1 }'
