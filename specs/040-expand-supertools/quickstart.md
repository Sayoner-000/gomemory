# Quickstart: validación de 040 mem expand

Esta guía recoge los escenarios ejecutables que demuestran la feature en real. Los detalles de los contratos están en [contracts/](contracts/) y los del modelo en [data-model.md](data-model.md).

## 0. Requisitos

- Binario recompilado con `go build -o mem . && ./mem version`. Antes de cualquier `install` o `update`, aísla el `PATH` en los bancos de prueba (memoria 438).
- Claude Code, Codex y OpenCode instalados en la máquina de referencia. Los hooks de Codex solo se pueden verificar en una sesión interactiva: `codex exec` no ejecuta hooks.

## 1. Pruebas

```bash
go test -race ./...
go test -race ./domain/ -run 'Route|Rules|Destructive|Shield|Instinct'
go test -race ./tests/contract/ -run 'ExpandSkills|SkillSourcesInSync|HookCommandIsGomemory'
```

Resultado esperado: todo en verde. El workflow `test.yml` (Go, TS y PS) también en verde.

## 2. Instalación limpia sin habilitar nada (SC-001, SC-003)

```bash
tmp=$(mktemp -d) && cd "$tmp" && git init -q
mem install . --yes --agents claude,codex,opencode
jq '.hooks | to_entries[] | select(.value|tostring|test("hook expand")) | .key' .claude/settings.json
grep -n 'hook expand' ~/.codex/config.toml
grep -n 'hook", "expand"' ~/.config/opencode/plugin/gomemory.ts   # ruta según la instalación del plugin
mem expand status
```

Resultado esperado: los eventos `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse` y `Stop` aparecen con `hook expand`, y `status` marca ✓ en los 3 harnesses.

## 3. Bloqueo destructivo en cada harness (US1, SC-001)

En Claude Code, Codex (sesión interactiva) y OpenCode, pide al agente: «ejecuta `rm -rf /tmp/x`».

Resultado esperado: la herramienta no se ejecuta y aparece el motivo `gomemory expand · regla destructive-shell: …`. Repite con `ls /tmp`: se permite sin ningún texto añadido.

Prueba directa, sin harness:

```bash
echo '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/x"}}' | mem hook expand pre-tool
# → {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny",...}}
echo '{"tool":"bash","args":{"command":"rm -rf /tmp/x"}}' | mem hook expand pre-tool --emit=text
# → BLOCK: gomemory expand · regla destructive-shell: ...
```

## 4. Regla propia y sugerencia única (US1, US3, SC-005)

```bash
mkdir -p .gomemory/rules && cat > .gomemory/rules/no-prod.md <<'EOF'
---
event: prompt
pattern: producción
action: warn
---
Recuerda: nunca toques producción desde el agente.
EOF
echo '{"prompt":"despliega a producción"}' | mem hook expand prompt --emit=claude   # contiene additionalContext
p='{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"s1","tool_input":{"command":"git commit -m x"}}'
echo "$p" | mem hook expand pre-tool   # sugiere verify-loop
echo "$p" | mem hook expand pre-tool   # {} — ya emitida en este episodio
```

## 5. Latencia (SC-002)

Con 64 reglas en `.gomemory/rules/`:

```bash
for i in $(seq 100); do /usr/bin/time -p sh -c "echo '$p' | mem hook expand pre-tool >/dev/null" 2>&1 | awk '/real/{print $2}'; done | sort -n | awk '{a[NR]=$1} END{print "p95:", a[int(NR*0.95)]}'
```

Resultado esperado: p95 < 0,150 s.

## 6. Interruptor (US2, SC-004)

```bash
mem expand off && echo '{"tool_name":"Bash","hook_event_name":"PreToolUse","tool_input":{"command":"rm -rf /tmp/x"}}' | mem hook expand pre-tool   # {}
mem expand on  && …   # deny
mem expand off rules && mem expand status   # rules: off; el resto: on
mem expand on rules
```

## 7. Actualización y desinstalación (US2, SC-003)

1. Sobre una instalación de v2.35.0 sin expand, ejecuta `mem update`. Resultado esperado: `hook expand` presente y protocolo `v10` en las instrucciones.
2. Ejecuta `mem install .` dos veces. Resultado esperado: 0 entradas `hook expand` duplicadas.
3. Ejecuta `mem uninstall` y después:

   ```bash
   grep -c 'hook expand' .claude/settings.json ~/.codex/config.toml   # 0
   ```

   Las entradas de terceros se conservan.

## 8. Shield (US4, SC-006, SC-007)

```bash
mem doctor --shield --json | jq '.shield.findings | group_by(.severity) | map({(.[0].severity): length}) | add'
mem doctor --shield; echo "exit=$?"
```

Resultado esperado en la máquina de referencia: 0 hallazgos critical que sean falsos positivos. Con los fixtures de `contracts/shield.md`, aparecen las 4 categorías y ningún valor de secreto.

Para comprobar que solo se escanea una vez por huella, inicia dos sesiones seguidas sin cambiar la configuración: el `mtime` de `.memory/expand-shield-last.json` cambia solo en la primera.

## 9. Skills (US3, SC-008)

```bash
for d in ~/.claude/skills ~/.codex/skills ~/.config/opencode/skills; do ls "$d" | grep -E '^(verify-loop|search-first|iterative-retrieval|strategic-compact|instincts)$' | wc -l; done   # 5 en cada uno
```

## 10. Instincts (US5, SC-009)

En dos proyectos distintos, ejecuta 8 veces `go test -race ./...` a través del agente y cierra los turnos. Después:

```bash
mem expand instincts list   # cmd:go-test aparece con confianza ≥ 0.6 y scope global
```

Inicia una sesión nueva: `## Instincts` aparece en el contexto solo con los de confianza ≥ 0.7. Prueba también:

```bash
mem expand instincts reject cmd:go-test   # deja de inyectarse
```

## 11. Evolve (US6)

En un TTY, ejecuta `mem expand instincts evolve <slug>`: aparece la propuesta y, si cancelas, no se escribe nada. Sin TTY, devuelve el código 2.
