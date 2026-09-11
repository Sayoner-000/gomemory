#!/usr/bin/env bash
# sync_version.sh — Lleva version/version.go a la última release publicada.
#
# Si el literal no sigue a las releases, los `go build` locales se anuncian con
# una versión vieja (y `mem update` los cree desactualizados). El job
# sync-version de .github/workflows/release.yml lo ejecuta tras cada release.
#
# Uso:
#   ./scripts/sync_version.sh          # sincroniza con la release latest (vía bump_version.sh)
#   ./scripts/sync_version.sh --check  # exit 1 si version.go no coincide con la release latest
#
# Fuente: `gh release view` (release marcada latest: ignora prereleases) si gh
# está disponible y autenticado; si no, el último tag v* tras git fetch --tags.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FILE="$ROOT/version/version.go"

latest_release() {
	local tag=""
	if command -v gh >/dev/null 2>&1; then
		tag="$(gh release view --json tagName -q .tagName 2>/dev/null || true)"
	fi
	if [[ -z "$tag" ]]; then
		git -C "$ROOT" fetch --tags --quiet 2>/dev/null || true
		tag="$(git -C "$ROOT" tag --list 'v*' --sort=-v:refname | head -1)"
	fi
	if [[ -z "$tag" ]]; then
		echo "sync_version: no se encontró ninguna release ni tag v*" >&2
		exit 2
	fi
	echo "${tag#v}"
}

current_version() {
	sed -n 's/^var Version = "\(.*\)"$/\1/p' "$FILE"
}

LATEST="$(latest_release)"

case "${1:-}" in
	--check)
		if [[ "$(current_version)" != "$LATEST" ]]; then
			echo "version.go está en $(current_version), la última release es $LATEST — ejecuta scripts/sync_version.sh" >&2
			exit 1
		fi
		;;
	"")
		"$ROOT/scripts/bump_version.sh" "$LATEST"
		;;
	*)
		echo "Uso: $0 [--check]   (para fijar una versión concreta: scripts/bump_version.sh X.Y.Z)" >&2
		exit 1
		;;
esac

echo ""
echo "=== Verificación de versión ==="
echo "  release latest: $LATEST"
echo "  version.go:     $(current_version)"
