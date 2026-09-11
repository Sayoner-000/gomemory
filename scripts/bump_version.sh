#!/usr/bin/env bash
# bump_version.sh <nueva_version>
# Fija la versión en el único literal vivo: version/version.go. Es la que usa
# un `go build` local; los binarios de release la reciben del tag vía
# goreleaser (-X mem/version.Version=...). El badge de release del README es
# dinámico (shields.io lee GitHub) y las menciones "vX.Y.Z" en comentarios y
# CHANGELOG son históricas: no se tocan.
# Uso: ./scripts/bump_version.sh 2.22.0
set -euo pipefail

if [[ $# -ne 1 ]]; then
	echo "Uso: $0 <nueva_version>  (ej: 2.22.0)" >&2
	exit 1
fi

NEW="${1#v}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FILE="$ROOT/version/version.go"

if ! [[ "$NEW" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
	echo "bump_version: versión inválida '$NEW' (se espera X.Y.Z)" >&2
	exit 2
fi

CURRENT="$(sed -n 's/^var Version = "\(.*\)"$/\1/p' "$FILE")"
if [[ -z "$CURRENT" ]]; then
	echo "bump_version: no se encontró 'var Version = \"...\"' en $FILE" >&2
	exit 2
fi

if [[ "$CURRENT" == "$NEW" ]]; then
	echo "version.go ya está en $NEW"
	exit 0
fi

# perl -pi en vez de sed -i: mismo comando en macOS (BSD) y Linux (GNU).
perl -pi -e "s/^var Version = \".*\"\$/var Version = \"$NEW\"/" "$FILE"
echo "Bumping $CURRENT → $NEW"
