# Quickstart: validación de la feature 035 contra el binario real

Regla del proyecto: «verde en tests no es funciona». Cada bloque se valida sobre el binario instalado.

## 0. Preparación

```bash
go test ./... && scripts/coverage.sh          # verde, cobertura ≥ la de v2.27.1
go build -o ~/.local/bin/mem ./infrastructure && mem --version
```

## 1. Hooks duplicados (US1, FR-001/001a/002/003)

```bash
grep -c '"mem hook' ~/.claude/settings.json .claude/settings.json   # antes: ambos > 0
mem doctor | grep -i duplicad                                        # ⚠ con el remedio
mem update                                                           # o mem install --scope project
grep -c '"mem hook' ~/.claude/settings.json                          # → 0 (si agent_scope=project)
grep -c 'cbm-\|herdr' ~/.claude/settings.json                        # hooks ajenos intactos
```

Reentrada (con la duplicación provocada a mano en un proyecto temporal):

```bash
printf '{"session_id":"x"}' | mem hook user-prompt-submit & printf '{"session_id":"x"}' | mem hook user-prompt-submit; wait
# una de las dos salidas es {} ; mem doctor muestra duplicate_dropped ≥ 1
```

## 2. Tope de salida (US1, FR-004…007)

En este repositorio (la memoria abundante es real):

```bash
printf '{"session_id":"t1","source":"startup"}' | mem hook session-start | wc -m          # ≤ 10000 (+ sobre)
printf '{"prompt":"hola","permission_mode":"plan"}' | mem hook user-prompt-submit | \
  python3 -c 'import sys,json;print(len(json.load(sys.stdin)["hookSpecificOutput"]["additionalContext"]))'  # ≤ 10000
```

En vivo: abrir Claude Code en modo plan → ni session-start ni el primer prompt muestran «Output too large / persisted-output»; el agente llama a las tools de memoria en su primer turno.

## 3. Compresión que no destruye (US2)

En Claude Code, con `tool_output_compression=true`:

| Acción | Esperado |
|---|---|
| `sed -n 1,80p adapters/secondary/persistence/compression_stats.go` | idéntico a `cat`, indentación incluida, sin ⟦mem⟧ |
| `search_code` (codebase-memory-mcp) con unos 30 resultados | 30 resultados, sin «omitidos» |
| `mem doctor` | idéntico a la ejecución fuera del agente |
| Bash con 60 líneas de código Go por `awk` (no excluido) | líneas conservadas idénticas |

## 4. Avisos por conversación (US3)

```bash
D=$(mktemp -d); cd "$D" && git init -q && mkdir -p .memory
echo '{"compact_threshold":10,"compact_agent_notice":true}' > .memory/settings.json
printf '{"session_id":"A"}' | mem hook session-start >/dev/null
printf '{}' | mem hook user-prompt-submit --emit=json >/dev/null
echo 999999 > .memory/.footprint
printf '{}' | mem hook turn-end --emit=json >/dev/null          # deja el aviso pendiente de A
printf '{"session_id":"B"}' | mem hook session-start >/dev/null  # conversación nueva
for i in 1 2 3; do printf '{}' | mem hook user-prompt-submit --emit=json | grep -c "AVISO DE MEMORIA"; done   # 0 0 0
```

- **Huella concurrente**: `go test -race -run Footprint ./adapters/primary/cli/` → verde.
- **Rotación**: con una sesión cuya última memoria tenga más de 4 h, `mem hook session-start` con un id nuevo → `mem session list` muestra la sesión vieja cerrada con «cerrada automáticamente por inactividad».
- **En vivo**: en Codex, provocar el aviso, cerrar, abrir otra conversación y enviar 3 prompts → no aparece el aviso.

## 5. Ahorro en listados y métrica (US4)

```bash
grep -rn "func " --include='*.go' . | head -400 > /tmp/l.txt
mem pack compress --level max < /tmp/l.txt | head -30     # 15 líneas + marca + 5 líneas; ref recuperable
mem pack retrieve <ref> | diff - /tmp/l.txt                # sin diferencias
mem pack savings                                           # fila listing con ahorro > 0; no_gain fuera de «degradaciones»
```

## 6. Codex y OpenCode (US5)

```bash
mem doctor | grep "hook de salidas (codex)"     # «solo en origen …»
mem context | grep -i "search_code"             # con el agente codex: la orientación aparece en su protocolo
```

En OpenCode: repetir las pruebas del paso 3 vía `tool.execute.after` → mismo resultado.

## 7. Observabilidad (FR-028/029)

Tras los pasos 1–3: `mem doctor | grep -A4 "protecciones (7 días)"` muestra conteos de `duplicate_dropped`, `budget_trimmed` y `tool_output_excluded` > 0.

## Resultados

### US1 — 2026-10-04 (binario instalado desde el árbol de trabajo, versión 2.27.1 + 035)

| Comprobación | Antes | Después |
|---|---|---|
| `go test ./... -count=1` | — | verde (16 paquetes) |
| Hooks de gomemory en `~/.claude/settings.json` / `.claude/settings.json` | 11 / 12 (11 duplicados) | 11 / 1 (`tool-output`, que el global no cubre) |
| `mem doctor`: duplicados | ✅ en ambos ámbitos (sin aviso) | ⚠ con el remedio antes de `mem update`; sin aviso después |
| `session-start` (memoria real de este repositorio) | 17 777 caracteres → vista previa de 2 KB | 7 349 runas, recortado y declarado con «contexto recortado» |
| Primer prompt en modo plan | 12 944 caracteres → vista previa | 9 862 runas, empieza con «PRIMERA ACCIÓN» |
| Aviso de duplicados en `session-start` | — | `systemMessage` con la lista y «mem update»; configuración intacta |
| `mem update` ya al día | «nada que hacer» | retira del proyecto los 11 duplicados |
| Recortes visibles | — | `mem doctor` → «budget_trimmed 2 (último: plan orig=8567,emit=5147)» |

Pendiente de verificar en vivo por la persona: abrir una sesión **nueva** de Claude Code en modo plan y confirmar que no aparece «Output too large / persisted-output» y que el recordatorio sale una sola vez (SC-001/SC-003).

### US2 — 2026-10-04

| Comprobación | Antes | Después |
|---|---|---|
| `sed -n 30,60p compression_stats.go` en una sesión real de Claude Code | indentación borrada, `return fmt.Errorf(...)` → «⟦mem⟧ 1 frases omitidas» | idéntico al archivo; `mem doctor` registra «tool_output_excluded … Bash: sed» |
| `search_code` con 30 resultados (fixture real) | «omitidos 23 de 30» (quedaban 2) | 30 resultados completos |
| `mem doctor` como salida de Bash | una línea de diagnóstico omitida | excluido, sale idéntico |
| Contrato T057 (fixtures existentes) | — | sigue reduciendo: Bash 31 874→3 001 B, Grep 31 990→313 B, MCP 95 742→36 288 B |
| `go test ./... -count=1` | — | verde |

### US3 — 2026-10-04

| Comprobación | Antes | Después |
|---|---|---|
| Aviso de compactación dejado por la conversación A (Codex, `--emit=json`) | el 2.º prompt de la conversación B lo recibía | `.pending-agent-notice` = `{"conv_id":"A",…}`; los 3 prompts de B: 0 avisos |
| Reanudación (mismo `session_id`) | — | no reinicia la huella; el aviso de su conversación llega en el primer prompt, una vez |
| Sesión con más de 4 h sin actividad + conversación nueva | se reutilizaba (el recordatorio de guardado saltaba al instante) | se cierra con «…por inactividad», con snapshot, y se abre otra; con actividad reciente no se toca |
| `.footprint` con 20 escritores concurrentes | llegó a valer `5078217904191461789312457` | siempre un entero; un valor corrupto se reinicia |
| `go test ./... -race -count=1` | `TestBuild_RespetaPresupuesto…` intermitente bajo carga | verde (orden determinista por `id`; 20/20 con `-race`) |

Pendiente de verificar en vivo por la persona: en Codex, provocar el aviso, cerrar, abrir otra conversación y enviar 3 prompts.

### US4 — 2026-10-04

| Comprobación | Antes | Después |
|---|---|---|
| `grep -rn "func " … \| head -400` con `mem pack compress --level max` | prosa → `no_gain` (0,3 %) | `listing`: 10 866 → 790 tokens (−92,7 %); 15 primeras + 5 últimas literales, marca con conteo por archivo |
| `mem pack retrieve <ref>` | — | devuelve el original íntegro (diff vacío) |
| `mem pack savings` | `no_gain` sumado en «degradaciones» | columnas «sin ganancia» y «degradaciones» separadas. Las 304 de la fila `structural` son históricas: se registraron antes de existir la columna y no se pueden reclasificar |

### US5 — 2026-10-04

| Comprobación | Resultado |
|---|---|
| `mem doctor` → hook de salidas (codex) | «solo en origen (Codex no permite reescribir salidas)» |
| Protocolo (instrucciones MCP y archivos de usuario) | orienta a `search_code`/`get_symbol` de gomemory donde el host no permite comprimir salidas externas (p. ej. Codex) |
| `mem install --yes .` en este repositorio | resumen: «Hooks de Claude Code: globales activos: el proyecto solo añade los que el global no cubre»; el proyecto conserva solo `tool-output` |
| Plugin de OpenCode instalado (`~/.config/opencode/plugins/gomemory.ts`) | pasa `--conversation=<id>` en `session.created` y `command` a `tool-output`; umbral de 8 000 caracteres (≈ 2 000 tokens) |

Pendiente de verificar en vivo por la persona: en OpenCode, un `cat` largo llega sin comprimir y un `grep` largo llega resumido como listado. Que `tool.execute.after` exponga `args.command` depende de la versión de OpenCode; si no lo expone, la salida se trata como antes.

### Cierre — 2026-10-04

| Comprobación | Resultado |
|---|---|
| `go test ./... -race -count=1` | verde |
| `scripts/coverage.sh` | cobertura global combinada **81,1 %** (≥ 80 % de la constitución). Un primer intento falló en `TestSessionStart_ConsultaEnSegundoPlano` por su umbral de reloj (1,2 s) con el binario instrumentado bajo carga; aislado, `session-start` tarda 30–40 ms y el test pasa 3/3 |
| `go vet ./...` / `gofmt -l` | limpio (`golangci-lint` no está instalado en el host) |
| §7 Observabilidad (`mem doctor`) | «protecciones (7 días)»: `budget_trimmed` y `tool_output_excluded` con su último detalle |

Verificaciones en vivo que quedan para la persona: sesión nueva de Claude Code en modo plan (sin «persisted-output», recordatorio una sola vez), ciclo de conversaciones en Codex y salidas largas en OpenCode.
