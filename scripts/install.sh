#!/usr/bin/env bash
# Instalador universal de gomemory para Linux y macOS.
#
# Uso:
#   curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.sh | bash
#
# Opciones (variables de entorno):
#   GOMEMORY_VERSION=v1.5.0   Instala una versión específica (por defecto: latest)
#   GOMEMORY_BIN_DIR=/ruta    Directorio de instalación (por defecto: ~/.local/bin)
#
# Desinstalar:
#   curl -fsSL .../install.sh | bash -s -- --uninstall
set -euo pipefail

REPO="Sayoner-000/gomemory"
BIN_NAME="mem"
VERSION="${GOMEMORY_VERSION:-latest}"

info()  { printf '\033[1;34m›\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m✓\033[0m %s\n' "$*"; }
err()   { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; }
die()   { err "$*"; exit 1; }

detect_os() {
  case "$(uname -s)" in
    Linux)  echo "linux" ;;
    Darwin) echo "darwin" ;;
    *) die "SO no soportado: $(uname -s). Usa install.ps1 en Windows." ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) die "Arquitectura no soportada: $(uname -m)" ;;
  esac
}

pick_bin_dir() {
  if [ -n "${GOMEMORY_BIN_DIR:-}" ]; then
    echo "$GOMEMORY_BIN_DIR"; return
  fi
  # Preferir /usr/local/bin si es escribible; si no, ~/.local/bin (sin sudo).
  if [ -w /usr/local/bin ] 2>/dev/null; then
    echo "/usr/local/bin"
  else
    echo "$HOME/.local/bin"
  fi
}

on_path() {
  case ":$PATH:" in
    *":$1:"*) return 0 ;;
    *) return 1 ;;
  esac
}

uninstall() {
  local removed=0
  for dir in "${GOMEMORY_BIN_DIR:-}" "/usr/local/bin" "$HOME/.local/bin"; do
    [ -z "$dir" ] && continue
    if [ -f "$dir/$BIN_NAME" ]; then
      rm -f "$dir/$BIN_NAME" && ok "Eliminado $dir/$BIN_NAME" && removed=1
    fi
  done
  [ "$removed" -eq 0 ] && info "No se encontró el binario $BIN_NAME instalado."
  info "Nota: la config y la memoria por-proyecto se quitan con 'mem uninstall <proyecto>'."
  exit 0
}

main() {
  if [ "${1:-}" = "--uninstall" ]; then uninstall; fi

  local os arch bin_dir asset url tmp
  os="$(detect_os)"
  arch="$(detect_arch)"
  bin_dir="$(pick_bin_dir)"
  asset="mem_${os}_${arch}.tar.gz"

  if [ "$VERSION" = "latest" ]; then
    url="https://github.com/${REPO}/releases/latest/download/${asset}"
  else
    url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
  fi

  info "Instalando gomemory (${os}/${arch}, ${VERSION})"
  info "Descargando ${url}"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  if ! curl -fsSL "$url" -o "$tmp/$asset"; then
    die "No se pudo descargar el release. ¿Existe el asset ${asset}? Revisa https://github.com/${REPO}/releases"
  fi

  tar -xzf "$tmp/$asset" -C "$tmp"
  [ -f "$tmp/$BIN_NAME" ] || die "El archivo no contiene el binario $BIN_NAME"

  mkdir -p "$bin_dir"
  install -m 0755 "$tmp/$BIN_NAME" "$bin_dir/$BIN_NAME" 2>/dev/null \
    || { cp "$tmp/$BIN_NAME" "$bin_dir/$BIN_NAME" && chmod 0755 "$bin_dir/$BIN_NAME"; }

  ok "gomemory instalado en $bin_dir/$BIN_NAME"

  if ! on_path "$bin_dir"; then
    info "Agrega $bin_dir al PATH. Por ejemplo:"
    printf '  echo '\''export PATH="%s:$PATH"'\'' >> ~/.bashrc && source ~/.bashrc\n' "$bin_dir"
  fi

  printf '\n'
  ok "Listo. Próximos pasos:"
  printf '  mem --help                 # Ver comandos\n'
  printf '  cd tu-proyecto && mem install .   # Cablear memoria + agentes (Claude, OpenCode, etc.)\n'
}

main "$@"
