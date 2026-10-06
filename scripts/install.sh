#!/usr/bin/env bash
# Instalador universal de gomemory para Linux y macOS.
#
# Uso:
#   curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.sh | bash
#
# Opciones (variables de entorno):
#   GOMEMORY_VERSION=v1.6.0   Instala una versión específica (por defecto: latest)
#   GOMEMORY_BIN_DIR=/ruta    Directorio de instalación (por defecto: ~/.local/bin)
#
# Desinstalar:
#   curl -fsSL .../install.sh | bash -s -- --uninstall
set -euo pipefail

REPO="Sayoner-000/gomemory"
BIN_NAME="mem"
VERSION="${GOMEMORY_VERSION:-latest}"
# Global para que la trap de limpieza en EXIT lo vea (no debe ser local de main,
# o bajo `set -u` daría "tmp: unbound variable" al salir).
tmp=""

# Logo derivado de assets/gomemory-terminal.txt; funciona sin el binario.
print_brand() {
  [ -t 1 ] && [ -z "${CI:-}" ] || return 0
  local width
  width="$(tput cols 2>/dev/null || printf '80')"
  case "$width" in *[!0-9]*|'') width=80 ;; esac
  if [ -n "${NO_COLOR:-}" ] || [ "${TERM:-}" = dumb ] || [ "$width" -lt 78 ]; then
    printf '\ngoMemory · %s · %s\n\n' "$VERSION" "$1"
    return 0
  fi
  local theme="${GOMEMORY_THEME:-auto}"
  if [ "$theme" != dark ] && [ "$theme" != light ]; then
    local colorfgbg="${COLORFGBG:-}"
    case "${colorfgbg##*;}" in 7|15) theme=light ;; *) theme=dark ;; esac
  fi
  if [ "$theme" = light ]; then printf '\n\033[38;2;57;38;227m'; else printf '\n\033[38;2;6;193;238m'; fi
  cat <<'GOMEMORY_LOGO'
             ⣴⣶  ⢀⣀
        ⢰⣷   ⣿⣿  ⣼⣿⠇
     ⣀   ⠿⠇  ⣉⡉  ⠿⠟ ⢀⣼⣷⡄
    ⠘⢿⣦    ⣴⣿⣿⣿⣆    ⠻⡿⠋
      ⠁  ⢀⣾⣿⣿⣿⣿⣿⣾⣆    ⢠⣴⣿⣧
  ⠈⣐⡦  ⢀⢢⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⣄  ⠈⠟⠋
    ⠁ ⢠⣯⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣆   ⣤⣴⣶
      ⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣆  ⠿⠟⠛
     ⢸⣿⣿⣿⣿⠻⠙⣿⣿⣿⠱⡻⣿⣿⣿⣿⣿⡄ ⣠⣀⣀
     ⢸⣿⣿⣿⣿⣦⣼⠿⣿⣿⣦⣴⣿⣿⣿⣿⣿⡧⠐⠿⣿⣿
   ⠈⠆⢸⣿⣿⣿⣿⣿⣿⣿⣯⣿⣿⣿⣿⣿⣿⣿⣿⡇⣀⡀
     ⠈⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠸⣿⣿⡦
     ⠈⠛⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣥  ⠙⠁
        ⠙⠿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠁⠙⠃
GOMEMORY_LOGO
  printf '\033[0m\n  goMemory · %s  ›  %s\n\n' "$VERSION" "$1"
}

info()  { if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-}" != dumb ]; then printf '\033[1;34m›\033[0m %s\n' "$*"; else printf '› %s\n' "$*"; fi; }
ok()    { if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-}" != dumb ]; then printf '\033[1;32m✓\033[0m %s\n' "$*"; else printf '✓ %s\n' "$*"; fi; }
err()   { if [ -t 2 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-}" != dumb ]; then printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; else printf '✗ %s\n' "$*" >&2; fi; }
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
  if [ "${1:-}" = "--uninstall" ]; then
    print_brand "Desinstalar"
    uninstall
  fi
  print_brand "Instalar"

  local os arch bin_dir asset url
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
  # El `rm -f` previo NO es limpieza cosmética: en macOS la firma de código se
  # cachea por inodo, y `cp` sobre un ejecutable existente reutiliza el inodo.
  # El binario resultante muere con SIGKILL en cada invocación, sin mensaje ni
  # error de firma —`codesign -v` lo da por válido—, así que el síntoma es un
  # `mem` que no responde y nada que explique por qué. `install` sí crea un
  # inodo nuevo (comprobado), pero el fallback tenía que hacerlo también.
  install -m 0755 "$tmp/$BIN_NAME" "$bin_dir/$BIN_NAME" 2>/dev/null \
    || { rm -f "$bin_dir/$BIN_NAME" \
      && cp "$tmp/$BIN_NAME" "$bin_dir/$BIN_NAME" \
      && chmod 0755 "$bin_dir/$BIN_NAME"; }

  ok "gomemory instalado en $bin_dir/$BIN_NAME"

  if ! on_path "$bin_dir"; then
    info "Agrega $bin_dir al PATH. Por ejemplo:"
    printf '  echo '\''export PATH="%s:$PATH"'\'' >> ~/.bashrc && source ~/.bashrc\n' "$bin_dir"
  fi

  printf '\n'
  ok "Listo. Próximos pasos:"
  printf '  mem --help                 # Ver comandos\n'
  printf '  cd tu-proyecto && mem install .   # Cablear memoria + agentes (Claude, OpenCode, etc.)\n'

  offer_guided_install "$bin_dir/$BIN_NAME"
}

# offer_guided_install ofrece, al terminar, la instalación guiada en el
# directorio actual (feature 034, FR-026). Solo con terminal interactiva y si
# el directorio es un repositorio git. La respuesta se lee de /dev/tty: con
# `curl | sh` la entrada estándar es la propia tubería del script.
offer_guided_install() {
  bin="$1"
  [ -t 1 ] || return 0
  { : </dev/tty; } 2>/dev/null || return 0
  command -v git >/dev/null 2>&1 || return 0
  git rev-parse --show-toplevel >/dev/null 2>&1 || return 0
  printf '\n¿Configurar gomemory en %s ahora? [S/n] ' "$(pwd)"
  read -r answer </dev/tty || return 0
  case "$answer" in
    n|N|no|No) info "Puedes hacerlo luego con: mem install ." ;;
    *) "$bin" install . </dev/tty ;;
  esac
}

main "$@"
