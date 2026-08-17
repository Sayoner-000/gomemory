#!/usr/bin/env bash
# test-codebase-memory-activation.sh — Regresión de los DOS brazos de gomemory:
#   - Brazo propio (modo plan atómico, feature 019): canales de mem doctor
#     --json, no-duplicación tras doble instalación, contrato neutral con un
#     agente desconocido.
#   - Brazo extensor (codebase-memory-mcp): activación en los 4 canales
#     (Claude Code hook, OpenCode plugin, integración AGENTS.md, subagentes) Y
#     no-regresión: su activación debe seguir intacta tras cualquier cambio de
#     esta feature, y su ausencia no debe generar avisos.
#
# Uso: ./scripts/test-codebase-memory-activation.sh
set -euo pipefail

BIN=""
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0
FAIL=0
TOOLS=(
  "search_graph"
  "trace_path"
  "get_code_snippet"
  "query_graph"
  "get_architecture"
  "search_code"
)
ADMIN_TOOLS=(
  "index_repository"
  "delete_project"
  "manage_adr"
  "ingest_traces"
)

# ── Helpers ──────────────────────────────────────────────────────────────────

red()   { printf "\033[31m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
bold()  { printf "\033[1m%s\033[0m\n" "$*"; }

pass() { PASS=$((PASS + 1)); green "  ✓ $*"; }
fail() { FAIL=$((FAIL + 1)); red "  ✗ $*"; }

assert_contains() {
  local haystack="$1" needle="$2" label="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    pass "$label"
  else
    fail "$label — no encontré: $needle"
  fi
}

assert_not_contains() {
  local haystack="$1" needle="$2" label="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    fail "$label — encontró (no debía): $needle"
  else
    pass "$label"
  fi
}

build_binary() {
  bold "1. Compilando binario..."
  local bindir
  bindir=$(mktemp -d)
  BIN="$bindir/mem-test"
  go build -o "$BIN" ./infrastructure
  pass "Binario compilado en $BIN"
}

# ── Canal 1: Claude Code — hook user-prompt-submit ──────────────────────────

test_claude_code_hook() {
  bold "2. Claude Code — hook user-prompt-submit (bootstrap)"

  local tmpdir
  tmpdir=$(mktemp -d)
  mkdir -p "$tmpdir/.memory"

  # Crear DB vacía para que el hook funcione
  "$BIN" hook session-start "$tmpdir" >/dev/null 2>&1 || true

  local out
  out=$("$BIN" hook user-prompt-submit "$tmpdir" 2>/dev/null || true)

  # Debe ser JSON válido
  if ! echo "$out" | python3 -m json.tool >/dev/null 2>&1; then
    fail "Salida no es JSON válido"
    return
  fi

  # NO debe tener systemMessage (el modelo nunca lo ve)
  assert_not_contains "$out" '"systemMessage"' "systemMessage ausente (correcto)"

  # Debe tener hookSpecificOutput.additionalContext
  assert_contains "$out" '"additionalContext"' "additionalContext presente"

  # additionalContext debe contener select: con las 6 tools
  for tool in "${TOOLS[@]}"; do
    assert_contains "$out" "mcp__codebase-memory-mcp__${tool}" "Claude Code: ${tool} en select:"
  done

  # NO debe incluir operaciones admin
  for tool in "${ADMIN_TOOLS[@]}"; do
    assert_not_contains "$out" "mcp__codebase-memory-mcp__${tool}" "Claude Code: admin ${tool} ausente"
  done

  # Debe contener la instrucción de modo plan
  assert_contains "$out" "get_plan_context" "Claude Code: instrucción get_plan_context presente"

  rm -rf "$tmpdir"
}

# ── Canal 1b: Claude Code — hook subagent-start ─────────────────────────────

test_claude_code_subagent() {
  bold "3. Claude Code — hook subagent-start (subagentes)"

  local tmpdir
  tmpdir=$(mktemp -d)
  mkdir -p "$tmpdir/.memory"

  local out
  out=$("$BIN" hook subagent-start "$tmpdir" 2>/dev/null || true)

  if ! echo "$out" | python3 -m json.tool >/dev/null 2>&1; then
    fail "Salida subagent-start no es JSON válido"
    return
  fi

  assert_not_contains "$out" '"systemMessage"' "subagent: systemMessage ausente"
  assert_contains "$out" '"additionalContext"' "subagent: additionalContext presente"

  for tool in "${TOOLS[@]}"; do
    assert_contains "$out" "mcp__codebase-memory-mcp__${tool}" "subagent: ${tool} en select:"
  done
}

# ── Canal 2: OpenCode — plugin gomemory.ts ─────────────────────────────────

test_opencode_plugin() {
  bold "4. OpenCode — plugin gomemory.ts (EXTERNAL CODE GRAPH)"

  local plugin="$ROOT/infrastructure/plugin/opencode/gomemory.ts"
  if [[ ! -f "$plugin" ]]; then
    fail "Plugin gomemory.ts no encontrado"
    return
  fi

  local content
  content=$(cat "$plugin")

  assert_contains "$content" "EXTERNAL CODE GRAPH" "OpenCode: sección EXTERNAL CODE GRAPH"

  for tool in "${TOOLS[@]}"; do
    assert_contains "$content" "codebase-memory-mcp_${tool}" "OpenCode: ${tool} nombrado"
  done

  # Protocolo completo: debe mencionar get_plan_context para modo plan
  assert_contains "$content" "PLAN MODE" "OpenCode: sección PLAN MODE"
  assert_contains "$content" "get_plan_context" "OpenCode: get_plan_context en protocolo"
}

# ── Canal 3: Integración — buildIntegrationBlock (AGENTS.md / .cursorrules) ─

test_integration_block() {
  bold "5. Integración — buildIntegrationBlock (AGENTS.md/.cursorrules)"

  local plugin="$ROOT/infrastructure/plugin/opencode/gomemory.ts"
  local install="$ROOT/adapters/primary/cli/cmd_install.go"

  # Verificar que cmd_install.go contiene la referencia a codebase-memory-mcp
  local install_content
  install_content=$(cat "$install")

  assert_contains "$install_content" "codebase-memory-mcp" "install: referencia a codebase-memory-mcp"
  assert_contains "$install_content" "exploración de código" "install: instrucción de grafo externo"

  # Las tools se referencian vía la variable Go, no como literales
  assert_contains "$install_content" "CodebaseMemoryMCPDiscoveryTools" "install: variable Go de discovery tools"
}

# ── Canal 4: MCP instructions — cmd_mcp.go ─────────────────────────────────

test_mcp_instructions() {
  bold "6. MCP instructions — cmd_mcp.go"

  local mcp_file="$ROOT/adapters/primary/cli/cmd_mcp.go"
  if [[ ! -f "$mcp_file" ]]; then
    fail "cmd_mcp.go no encontrado"
    return
  fi

  local content
  content=$(cat "$mcp_file")

  assert_contains "$content" "buildIntegrationBlock" "MCP: instructions usa buildIntegrationBlock"
}

# ── Test de contrato: todas las tools de gomemory en el bootstrap ───────────

test_bootstrap_completeness() {
  bold "7. Contrato — todas las tools de gomemory en bootstrap"

  local tmpdir
  tmpdir=$(mktemp -d)
  mkdir -p "$tmpdir/.memory"
  "$BIN" hook session-start "$tmpdir" >/dev/null 2>&1 || true

  local out
  out=$("$BIN" hook user-prompt-submit "$tmpdir" 2>/dev/null || true)

  # Tools de gomemory que deben aparecer
  local gomemory_tools=(
    "save_memory"
    "search_memories"
    "list_memories"
    "get_memory"
    "forget_memory"
    "judge_memories"
    "start_session"
    "end_session"
    "get_context"
    "get_plan_context"
    "search_code"
    "get_symbol"
    "list_dependencies"
    "graph_status"
    "index_project"
  )

  for tool in "${gomemory_tools[@]}"; do
    assert_contains "$out" "mcp__gomemory__${tool}" "contrato: gomemory ${tool}"
  done

  rm -rf "$tmpdir"
}

# ── Canal 8: mem doctor — canales del modo plan atómico (feature 019) ──────

test_plan_mode_channels() {
  bold "8. mem doctor — canales del modo plan atómico"

  local tmpdir out
  tmpdir=$(mktemp -d)
  (cd "$tmpdir" && git init -q)

  # Sin listas de agentes propias en este script: mem doctor --json ES la
  # fuente única (FR-A4/SC-A2). Si el registro de capacidades gana un agente
  # nuevo, este chequeo lo ve sin que nadie tenga que tocar este script.
  out=$(cd "$tmpdir" && HOME="$(mktemp -d)" "$BIN" doctor --json 2>/dev/null || true)

  if ! echo "$out" | python3 -m json.tool >/dev/null 2>&1; then
    fail "mem doctor --json no produjo JSON válido"
  else
    pass "mem doctor --json produce JSON válido"
  fi

  assert_contains "$out" '"kind": "plan_guard"' "doctor: canal plan_guard presente"
  assert_contains "$out" '"kind": "plan_entry"' "doctor: canal plan_entry presente"
  assert_contains "$out" '"kind": "turn_reminder"' "doctor: canal turn_reminder presente"
  assert_contains "$out" '"kind": "instructions"' "doctor: canal instructions presente"

  # Sin nada instalado en este HOME/proyecto de prueba, debe haber problemas
  # reales (missing) — mem doctor no debe fingir cobertura completa.
  assert_contains "$out" '"problems"' "doctor: campo problems presente"

  rm -rf "$tmpdir"
}

# ── Canal 9: no-regresión del brazo extensor ────────────────────────────────

test_codegraph_no_regression() {
  bold "9. No-regresión del brazo extensor (codegraph)"

  local fakehome tmpdir out
  tmpdir=$(mktemp -d)
  (cd "$tmpdir" && git init -q)

  # Ausente: 0 canales codegraph, 0 avisos por su ausencia (INV-4).
  fakehome=$(mktemp -d)
  out=$(cd "$tmpdir" && HOME="$fakehome" "$BIN" doctor --json 2>/dev/null || true)
  assert_not_contains "$out" '"arm": "codegraph"' "codegraph ausente: sin canales codegraph"

  # Presente (hooks cbm-* simulados en el HOME de prueba): debe reportarse ok,
  # de solo lectura — nunca se escribe ni se corrige (INV-1).
  fakehome=$(mktemp -d)
  mkdir -p "$fakehome/.claude"
  cat >"$fakehome/.claude/settings.json" <<'JSON'
{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"~/.claude/hooks/cbm-session-reminder"}]}]}}
JSON
  local before after
  before=$(cat "$fakehome/.claude/settings.json")
  out=$(cd "$tmpdir" && HOME="$fakehome" "$BIN" doctor --json 2>/dev/null || true)
  after=$(cat "$fakehome/.claude/settings.json")

  assert_contains "$out" '"arm": "codegraph"' "codegraph presente: canal reportado"
  if [[ "$before" == "$after" ]]; then
    pass "codegraph: mem doctor no escribió nada (solo lectura)"
  else
    fail "codegraph: mem doctor modificó ~/.claude/settings.json — viola INV-1"
  fi

  rm -rf "$tmpdir"
}

# ── Canal 10: doble instalación sin duplicados ──────────────────────────────

test_double_install_no_duplicates() {
  bold "10. Doble instalación — sin duplicados, sin pérdida de entradas ajenas"

  local tmpdir out
  tmpdir=$(mktemp -d)
  mkdir -p "$tmpdir/.claude"
  cat >"$tmpdir/.claude/settings.json" <<'JSON'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"otra-tool hook antes-de-bash"}]}]}}
JSON

  "$BIN" install "$tmpdir" >/dev/null 2>&1 || true
  "$BIN" install "$tmpdir" >/dev/null 2>&1 || true

  out=$(cd "$tmpdir" && HOME="$(mktemp -d)" "$BIN" doctor --json 2>/dev/null || true)
  assert_not_contains "$out" '"state": "duplicated"' "doble instalación: sin canales duplicated"

  local settings
  settings=$(cat "$tmpdir/.claude/settings.json")
  assert_contains "$settings" "otra-tool hook antes-de-bash" "doble instalación: entrada ajena preservada"

  rm -rf "$tmpdir"
}

# ── Canal 11: contrato neutral con un agente desconocido ───────────────────

test_foreign_agent_neutral_contract() {
  bold "11. Contrato neutral — agente desconocido (docs/AGENT-INTEGRATION.md)"

  local tmpdir plan_file
  tmpdir=$(mktemp -d)

  # Plan trivial en prosa larga, sin ninguna estructura de árbol: dialecto
  # neutral (ninguna bandera --emit, ningún payload de Claude Code).
  plan_file="$tmpdir/plan.txt"
  python3 -c "print('Voy a implementar la integración completa. ' * 20)" >"$plan_file"

  # cd al tmpdir: plan-guard resuelve la raíz del proyecto desde el directorio
  # de trabajo (FindRoot camina hacia arriba buscando .git). Sin aislar esto,
  # el chequeo heredaría el episodio de plan de ESTE repositorio — que ya
  # puede estar "gastado" por otras pruebas — y daría un falso negativo.
  if (cd "$tmpdir" && "$BIN" hook plan-guard <"$plan_file" >/dev/null 2>/dev/null); then
    fail "agente desconocido: un plan en prosa debía devolverse (exit != 0)"
  else
    pass "agente desconocido: plan en prosa devuelto por el dialecto neutral"
  fi

  local arbol_file="$tmpdir/arbol.txt"
  printf '🎯 objetivo\n├─ [1] subtarea\n│  └─ [1.1] ✓ verbo + objeto → resultado' >"$arbol_file"
  if (cd "$tmpdir" && "$BIN" hook plan-guard <"$arbol_file" >/dev/null 2>/dev/null); then
    pass "agente desconocido: plan en árbol permitido (exit 0)"
  else
    fail "agente desconocido: un plan en árbol no debía devolverse"
  fi

  rm -rf "$tmpdir"
}

# ── Runner ──────────────────────────────────────────────────────────────────

main() {
  bold "═══ Test: activación de codebase-memory-mcp en todos los canales ═══"
  echo

  build_binary
  echo

  test_claude_code_hook
  echo

  test_claude_code_subagent
  echo

  test_opencode_plugin
  echo

  test_integration_block
  echo

  test_mcp_instructions
  echo

  test_bootstrap_completeness
  echo

  test_plan_mode_channels
  echo

  test_codegraph_no_regression
  echo

  test_double_install_no_duplicates
  echo

  test_foreign_agent_neutral_contract
  echo

  bold "═══ Resultado ═══"
  green "  Pasaron: $PASS"
  if [[ $FAIL -gt 0 ]]; then
    red "  Fallaron: $FAIL"
    echo
    exit 1
  else
    echo "  Fallaron: 0"
  fi
  echo

  # Limpiar
  rm -f "$BIN"
  rmdir "$(dirname "$BIN")" 2>/dev/null || true
}

main
