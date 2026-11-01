#!/usr/bin/env bash
# Reproducción de H-01 (HCD-013) con el binario real y datos temporales: tras
# `mem purge`, sin ningún comando más, el texto purgado no puede seguir en
# mem.db ni en su WAL, y la memoria superviviente debe seguir encontrándose.
set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

(cd "$raiz" && go build -o "$tmp/mem" ./infrastructure)
mkdir -p "$tmp/data" "$tmp/proj"
cd "$tmp/proj"
git init -q .
export GOMEMORY_DATA_HOME="$tmp/data"

"$tmp/mem" init >/dev/null
"$tmp/mem" save -t uno -y learning "CANARIOVIVO conservar" >/dev/null
"$tmp/mem" save -t dos -y decision "CANARIOPURGA borrar" >/dev/null
# Historial real: las fusiones del índice dejan texto en páginas libres, que una
# base recién creada no tiene.
for i in $(seq 1 70); do
  "$tmp/mem" save -t "relleno $i" -y learning "RELLENO$i palabra$i texto distinto $i" >/dev/null 2>&1
done
"$tmp/mem" purge --type decision --yes >/dev/null

base="$(find "$tmp/data" -name mem.db | head -1)"
for archivo in "$base" "$base-wal"; do
  [ -f "$archivo" ] || continue
  if grep -aq CANARIOPURGA "$archivo"; then
    echo "FALLO: el texto purgado sigue en $(basename "$archivo")" >&2
    exit 1
  fi
done

"$tmp/mem" search CANARIOVIVO | grep -q uno || {
  echo "FALLO: la memoria superviviente ya no se encuentra" >&2
  exit 1
}
echo "OK: el texto purgado no está en disco y la superviviente se encuentra"
