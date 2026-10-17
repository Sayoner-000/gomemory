#!/usr/bin/env bash
# Instalador universal de gomemory para Linux y macOS.
#
# Uso:
#   curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.sh | bash
#
# Opciones (variables de entorno):
#   GOMEMORY_VERSION=v1.6.0   Instala una versión estable específica (por defecto: latest)
#   GOMEMORY_BIN_DIR=/ruta    Directorio de instalación (por defecto: ~/.local/bin)
#
# Integridad (spec 037, H-08): la versión se resuelve una sola vez a un tag
# estable; el archivo y checksums.txt salen de ese mismo release y el SHA-256
# se verifica ANTES de extraer. Solo se extrae el miembro `mem`, que debe ser
# un ejecutable regular de la plataforma. Cualquier fallo deja intacto el
# binario instalado. checksums.txt viene del mismo origen: verifica la
# integridad publicada, no es una firma independiente.
#
# Desinstalar:
#   curl -fsSL .../install.sh | bash -s -- --uninstall
set -euo pipefail

REPO="Sayoner-000/gomemory"
BIN_NAME="mem"
# Topes de descarga (un release ronda los 8 MB): curl corta antes de que un
# archivo sobredimensionado llegue a disco y al cálculo del SHA-256.
MAX_ARCHIVE_BYTES=134217728
MAX_METADATA_BYTES=1048576
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

# resolve_tag convierte la selección en UN tag estable concreto. `latest` se
# consulta una sola vez: el archivo y checksums.txt se piden después a ese tag
# exacto, así un release publicado entre ambas descargas no los mezcla.
resolve_tag() {
  local tag="$VERSION"
  if [ "$tag" = "latest" ]; then
    local api
    api="$(curl -fsSL --max-filesize "$MAX_METADATA_BYTES" "https://api.github.com/repos/${REPO}/releases/latest")" \
      || die "No se pudo consultar la última versión estable de ${REPO}."
    tag="$(printf '%s' "$api" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  fi
  case "$tag" in
    v*) ;;
    *) tag="v$tag" ;;
  esac
  printf '%s' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' \
    || die "Versión no soportada: '${tag}'. Usa un release estable vX.Y.Z."
  printf '%s' "$tag"
}

# sha256_of imprime el SHA-256 de un archivo. Sin herramienta no se instala.
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    die "No hay sha256sum ni shasum: no se puede verificar la descarga."
  fi
}

# expected_sha256 extrae el digest del asset desde checksums.txt: exige una
# única línea cuyo nombre (con asterisco binario opcional) coincida EXACTO
# con el asset y un digest de 64 hex. Una entrada duplicada se rechaza aunque
# ambos digests coincidan.
expected_sha256() {
  local sums="$1" name="$2" matches count
  matches="$(awk -v want="$name" '{ f = $2; sub(/^\*/, "", f); if (NF == 2 && f == want) print $1 }' "$sums")"
  count="$(printf '%s' "$matches" | grep -c . || true)"
  [ "$count" -eq 1 ] || die "checksums.txt no tiene exactamente una entrada para ${name} (${count})."
  printf '%s' "$matches" | grep -Eqi '^[0-9a-f]{64}$' || die "checksums.txt tiene un digest inválido para ${name}."
  printf '%s' "$matches" | tr 'A-F' 'a-f'
}

# assert_platform_binary valida sin ejecutarlo que el archivo es un
# ejecutable regular, no vacío, no enlace, con la cabecera de la plataforma.
assert_platform_binary() {
  local f="$1" os="$2" magic
  [ ! -L "$f" ] && [ -f "$f" ] && [ -s "$f" ] || die "El archivo no contiene un binario ${BIN_NAME} regular y no vacío."
  magic="$(head -c 4 "$f" | od -An -tx1 | tr -d ' \n')"
  case "$os:$magic" in
    linux:7f454c46) ;;
    darwin:cffaedfe|darwin:cefaedfe|darwin:feedfacf|darwin:feedface|darwin:cafebabe) ;;
    *) die "El binario descargado no es un ejecutable para ${os}." ;;
  esac
}

main() {
  if [ "${1:-}" = "--uninstall" ]; then
    print_brand "Desinstalar"
    uninstall
  fi
  print_brand "Instalar"

  local os arch bin_dir asset tag base want got
  os="$(detect_os)"
  arch="$(detect_arch)"
  bin_dir="$(pick_bin_dir)"
  asset="mem_${os}_${arch}.tar.gz"
  tag="$(resolve_tag)"
  base="https://github.com/${REPO}/releases/download/${tag}"

  info "Instalando gomemory (${os}/${arch}, ${tag})"
  info "Descargando ${base}/${asset}"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  curl -fsSL --max-filesize "$MAX_ARCHIVE_BYTES" "${base}/${asset}" -o "$tmp/$asset" \
    || die "No se pudo descargar ${asset} de ${tag}. Revisa https://github.com/${REPO}/releases"
  curl -fsSL --max-filesize "$MAX_METADATA_BYTES" "${base}/checksums.txt" -o "$tmp/checksums.txt" \
    || die "No se pudo descargar checksums.txt de ${tag}: no se instala sin verificar."

  want="$(expected_sha256 "$tmp/checksums.txt" "$asset")"
  got="$(sha256_of "$tmp/$asset")"
  [ "$got" = "$want" ] || die "El SHA-256 de ${asset} no coincide con checksums.txt. No se instaló nada."
  ok "Integridad verificada (SHA-256)"

  # Solo el miembro esperado: un nombre con ruta o traversal no coincide.
  mkdir "$tmp/extract"
  tar -xzf "$tmp/$asset" -C "$tmp/extract" "$BIN_NAME" 2>/dev/null \
    || die "El archivo no contiene el binario ${BIN_NAME}."
  assert_platform_binary "$tmp/extract/$BIN_NAME" "$os"

  mkdir -p "$bin_dir" || die "No se pudo crear ${bin_dir}."
  # Se prepara al lado del destino y se publica con mv: el binario anterior
  # sigue completo hasta el rename y el resultado es un inodo nuevo (en macOS
  # la firma se cachea por inodo; reescribir encima deja un `mem` que muere
  # con SIGKILL sin explicación).
  local staged
  staged="$(mktemp "$bin_dir/.${BIN_NAME}.XXXXXX" 2>/dev/null)" \
    || die "No se puede escribir en ${bin_dir}. El binario anterior no se modificó."
  if ! { cp "$tmp/extract/$BIN_NAME" "$staged" && chmod 0755 "$staged" && mv -f "$staged" "$bin_dir/$BIN_NAME"; }; then
    rm -f "$staged"
    die "No se pudo instalar en ${bin_dir}. El binario anterior no se modificó."
  fi

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
