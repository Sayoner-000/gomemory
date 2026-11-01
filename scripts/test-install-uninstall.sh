#!/usr/bin/env bash
# H-02 (HCD-013): `install.sh --uninstall` solo retira binarios de gomemory. Un
# `mem` ajeno, un enlace o un binario sin marcador se conservan, no se ejecutan
# y generan un aviso con su ruta.
set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Se ejecuta una copia con /usr/local/bin redirigido a un directorio temporal:
# el script real recorre esa ruta y borraría el mem instalado de quien prueba.
mkdir -p "$tmp/sistema"
sed "s#/usr/local/bin#$tmp/sistema#g" "$raiz/scripts/install.sh" > "$tmp/install.sh"
if grep -q '/usr/local/bin' "$tmp/install.sh"; then
  echo "FALLO: la copia del instalador sigue apuntando al sistema" >&2
  exit 1
fi

desinstalar() {
  HOME="$tmp/home" GOMEMORY_BIN_DIR="$1" bash "$tmp/install.sh" --uninstall 2>&1
}
fallo() {
  echo "FALLO: $1" >&2
  exit 1
}
mkdir -p "$tmp/home"

# Ajeno: si se ejecutara, dejaría esta señal.
mkdir -p "$tmp/ajeno"
printf '#!/bin/sh\ntouch "%s/ejecutado"\necho gomemory 9.9.9\n' "$tmp" > "$tmp/ajeno/mem"
chmod +x "$tmp/ajeno/mem"
salida="$(desinstalar "$tmp/ajeno")"
[ -f "$tmp/ajeno/mem" ] || fallo "se borró un mem ajeno"
[ ! -e "$tmp/ejecutado" ] || fallo "se ejecutó un mem ajeno"
printf '%s' "$salida" | grep -q "$tmp/ajeno/mem" || fallo "sin aviso con la ruta del mem ajeno"
if printf '%s' "$salida" | grep -q 'No se encontró'; then
  fallo "dice que no encontró el binario tras conservar uno ajeno"
fi

# Enlace a un archivo con marcador: tampoco se sigue.
mkdir -p "$tmp/enlace"
printf 'gomemory-protocol-v12' > "$tmp/destino"
ln -s "$tmp/destino" "$tmp/enlace/mem"
salida="$(desinstalar "$tmp/enlace")"
[ -L "$tmp/enlace/mem" ] || fallo "se borró un enlace simbólico"
printf '%s' "$salida" | grep -q "$tmp/enlace/mem" || fallo "sin aviso con la ruta del enlace"

# Nada instalado: sí lo dice.
mkdir -p "$tmp/vacio"
salida="$(desinstalar "$tmp/vacio")"
printf '%s' "$salida" | grep -q 'No se encontró' || fallo "sin aviso de que no hay nada instalado"

# Propio: se elimina.
mkdir -p "$tmp/propio"
printf '\177ELF gomemory-protocol-v12 resto' > "$tmp/propio/mem"
desinstalar "$tmp/propio" >/dev/null
[ ! -e "$tmp/propio/mem" ] || fallo "no se borró el binario propio"

echo "OK: --uninstall solo retira binarios propios"
