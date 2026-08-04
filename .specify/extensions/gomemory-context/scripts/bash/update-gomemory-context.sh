#!/usr/bin/env bash
# update-gomemory-context.sh
#
# Incorpora el resumen de historial del proyecto que gomemory ya construye
# (`mem context`) al contexto de la especificación. Nunca falla el flujo de
# spec-kit: siempre termina con código de salida 0, con o sin salida.
#
# Ver contracts/update-gomemory-context-script.md
# (specs/011-gomemory-spec-context) para el contrato completo.
#
# Usage: update-gomemory-context.sh (sin argumentos)

set -uo pipefail  # sin -e: cada paso maneja su propio fallo sin abortar el hook

PROJECT_ROOT="$(pwd)"
SETTINGS_FILE="$PROJECT_ROOT/.memory/settings.json"

# 1. Localizar el binario mem: ./mem (raíz del proyecto, lo deja `mem install`)
#    primero, luego `mem` en PATH. Sin ninguno de los dos, no hay nada que
#    hacer (proyecto sin gomemory disponible localmente).
MEM_BIN=""
if [[ -x "$PROJECT_ROOT/mem" ]]; then
  MEM_BIN="$PROJECT_ROOT/mem"
elif command -v mem >/dev/null 2>&1; then
  MEM_BIN="mem"
fi

if [[ -z "$MEM_BIN" ]]; then
  exit 0
fi

# 2. Interruptor (feature 011, historia 4): si speckit_context_disabled=true
#    en settings.json, la integración está apagada. Lectura directa del JSON
#    con grep — deliberadamente NO se invoca `mem settings`, para que este
#    script no dependa de que la CLI/TUI ya expongan el toggle.
if [[ -f "$SETTINGS_FILE" ]] && grep -Eq '"speckit_context_disabled"[[:space:]]*:[[:space:]]*true' "$SETTINGS_FILE"; then
  exit 0
fi

# 3. Obtener el resumen. Si falla (proyecto sin memoria inicializada, error
#    interno), salir en silencio — degradación transparente (FR-004).
if ! output="$("$MEM_BIN" context 2>/dev/null)"; then
  exit 0
fi

# 4. Emitir el resumen tal cual: ya viene acotado por el presupuesto de
#    caracteres configurado en gomemory (Budget), sin recorte adicional aquí.
printf '%s\n' "$output"
exit 0
