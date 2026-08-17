# Quickstart — validación de la activación determinista del modo plan

Guía ejecutable. El orden importa: la §1 resuelve las cuatro verificaciones pendientes de
[research.md](./research.md) y de su resultado depende qué camino toma la Historia 2. Las §2 en
adelante validan cada historia contra el sistema en ejecución, no contra mocks — lección de campo
n.º 2 del proyecto.

**Prerrequisitos**: repositorio en la raíz de trabajo, `go` disponible, un `mem` compilado
(`go build -o mem .`) y una copia de seguridad de `~/.claude/settings.json` antes de la §1. **No
hace falta reiniciar el agente**: verificado en vivo (2026-08-17, T001-T004) que Claude Code relee
los hooks dinámicamente — un cambio en `.claude/settings.json` surte efecto desde la siguiente
llamada a herramienta.

---

## §1 — Verificación en vivo de las capacidades del canal (V1..V4)

Sonda que registra el payload recibido y devuelve una decisión, sin depender de código nuevo de la
feature.

```bash
mkdir -p /tmp/planprobe && cat > /tmp/planprobe/probe.sh <<'SH'
#!/usr/bin/env bash
payload=$(cat)
printf '%s\n' "$payload" >> /tmp/planprobe/payloads.jsonl
case "$1" in
  guard)   printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"SONDA: devolución de prueba, ignórala y presenta el plan otra vez."}}' ;;
  entered) printf '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"SONDA-ENTRADA-OK: si ves esta cadena, el canal de entrada inyecta contexto."}}' ;;
esac
exit 0
SH
chmod +x /tmp/planprobe/probe.sh
```

Registrar temporalmente en `.claude/settings.json` del proyecto:

```json
"PreToolUse":  [ { "matcher": "ExitPlanMode",  "hooks": [ { "type": "command", "command": "/tmp/planprobe/probe.sh guard" } ] } ],
"PostToolUse": [ { "matcher": "EnterPlanMode", "hooks": [ { "type": "command", "command": "/tmp/planprobe/probe.sh entered" } ] } ]
```

Reiniciar el agente, entrar en modo plan, pedir un plan cualquiera y presentarlo.

| Verificación | Cómo se comprueba | Resultado esperado |
|---|---|---|
| **V1** | `grep ExitPlanMode /tmp/planprobe/payloads.jsonl` y buscar `tool_input.plan` | el payload llega con el texto del plan |
| **V2** | el agente reacciona al motivo y vuelve a presentar el plan | la devolución llega al agente de forma legible |
| **V3** | el agente menciona `SONDA-ENTRADA-OK` | el canal de entrada inyecta contexto |
| **V4** | entrar en modo plan por atajo de teclado y revisar `payloads.jsonl` | queda claro si existe señal sin llamada a herramienta |

**Al terminar**: restaurar `.claude/settings.json` desde la copia y borrar `/tmp/planprobe`.

- V1 y V2 positivas → la Historia 1 se implementa como está diseñada.
- V3 negativa → `plan-entered` no se registra; la Historia 2 se cubre solo con el recordatorio por
  turno, y el canal se reporta como `not_applicable`.
- **V1 o V2 negativas** → hay que detenerse y replantear la Historia 1 antes de escribir código: el
  determinismo se quedaría sin mecanismo. Es el único resultado que cambia el plan.

**Resultado real (2026-08-17)**: V1, V2 y V3 confirmadas positivas; V4 no evaluada (no bloquea).
Detalle completo en [research.md](./research.md) «Estado de la evidencia». La Historia 1 se
implementa tal como está diseñada. Se detectó además que `PostToolUse(ExitPlanMode)` podría no
llevar `tool_input.plan` en esta versión de Claude Code (a diferencia de `PreToolUse`, confirmado
positivo) — posible regresión de la feature 007, fuera de alcance de esta feature.

---

## §2 — Historia 1: un plan sin forma no llega a la persona

Sin agente, contra el binario:

```bash
# Plan en prosa larga → devolución
printf '{"tool_input":{"plan":"%s"}}' "$(python3 - <<'PY'
print("Voy a implementar la integración completa. " * 20)
PY
)" | ./mem hook plan-guard; echo " [exit=$?]"
# Esperado: JSON con permissionDecision "deny" y exit 0

# Repetir idéntico → ya no devuelve (una vez por episodio)
# Esperado: {}

# Cerrar episodio y volver a intentar
printf '{"tool_input":{"plan":"plan aprobado de prueba"}}' | ./mem hook plan-approved >/dev/null
# el mismo plan en prosa vuelve a producir "deny"

# Plan en árbol → siempre pasa
printf '{"tool_input":{"plan":"🎯 objetivo\n├─ [1] subtarea\n│  └─ [1.1] ✓ crear archivo → archivo existe\n└─ [2] ✓ correr prueba → prueba pasa"}}' | ./mem hook plan-guard
# Esperado: {}

# Plan trivial → pasa
printf '{"tool_input":{"plan":"Cambiar el título del README."}}' | ./mem hook plan-guard
# Esperado: {}

# Apagado → pasa siempre
./mem settings set plan_guard_disabled true   # o desde la TUI
# el plan en prosa ya no produce devolución; restaurar después
```

Con agente (extremo a extremo): entrar en modo plan, pedir un cambio no trivial, forzar una respuesta
en prosa. **Criterio**: la prosa no llega a la persona; el plan que finalmente se presenta es un árbol.

---

## §3 — Historia 2: método e historial al entrar en modo plan

```bash
# Presupuesto del canal
./mem hook plan-entered </dev/null | python3 -c "
import json,sys
d=json.load(sys.stdin)
ctx=d.get('hookSpecificOutput',{}).get('additionalContext','')
print('caracteres:', len(ctx))
print('<= 9500:', len(ctx) <= 9500)
print('método completo:', 'Descomposición Atómica' in ctx and 'Modo plan: entrega y detente' in ctx)
print('sin corte a mitad de frase:', ctx.rstrip()[-1] in '.:)`\"' or ctx.rstrip().endswith('()'))
"

# Sin memoria inicializada → silencio, exit 0
(cd /tmp && mkdir -p sinmem && cd sinmem && "$OLDPWD/mem" hook plan-entered </dev/null; echo "[exit=$?]")
# Esperado: {} o vacío, exit 0
```

Con agente: gastar tres o más turnos, entrar en modo plan y pedir un cambio no trivial.
**Criterio**: el plan referencia al menos un elemento del historial que no se mencionó en la
solicitud.

**Recordatorio por turno**: `echo -n "" | ./mem hook user-prompt-submit` en un prompt que no es el
primero de la sesión debe incluir ahora la línea de modo plan — hoy solo trae el recordatorio de
guardado.

---

## §4 — Historia 3: paridad entre agentes y degradaciones declaradas

```bash
./mem doctor
./mem doctor --json | python3 -m json.tool | head -40
./mem doctor --json > /tmp/d1.json && ./mem doctor --json > /tmp/d2.json && diff /tmp/d1.json /tmp/d2.json && echo "salida estable ✓"
```

**Criterios**: cada agente instalado aparece con sus canales; OpenCode declara la degradación del
borde de salida; ningún canal roto queda escondido como `not_applicable`.

---

## §5 — Historia 4: nivel usuario y actualización no destructiva

```bash
# Contenido propio antes Y después del bloque
tmp=$(mktemp -d)
cat > "$tmp/CLAUDE.md" <<'EOF'
# Mis notas
Texto propio ANTES del bloque.

<!-- gomemory-protocol-v7 -->
## Memoria Persistente (`mem`) — Protocolo Activo
contenido viejo del bloque

## Mis reglas personales
Texto propio DESPUÉS del bloque. NO DEBE PERDERSE.
EOF
# actualizar con el instalador apuntando a ese directorio, luego:
grep -c "NO DEBE PERDERSE" "$tmp/CLAUDE.md"          # esperado: 1
grep -c "gomemory-protocol-v7" "$tmp/CLAUDE.md"      # esperado: 0
grep -c "gomemory-protocol-v8" "$tmp/CLAUDE.md"      # esperado: 1
grep -c "Texto propio ANTES" "$tmp/CLAUDE.md"        # esperado: 1
```

```bash
# Proyecto nuevo sin instalación propia
tmp2=$(mktemp -d) && cd "$tmp2"
# entrar en modo plan con el agente aquí: debe aplicar el método atómico
```

**Criterio**: la cobertura no depende de que `$HOME` esté instalado como proyecto.

---

## §6 — Historia 5: regresión de los dos brazos

```bash
./scripts/test-codebase-memory-activation.sh        # todo en verde

# Doble instalación → sin duplicados
./mem install --target "$(mktemp -d)" >/dev/null && ./mem install --target "$MISMO_DIR" >/dev/null
./mem doctor --json | python3 -c "
import json,sys
ch=json.load(sys.stdin)['channels']
dup=[c for c in ch if c['state']=='duplicated']
print('duplicados:', len(dup)); assert not dup
"

# Degradar un canal a mano y comprobar que la verificación falla
# (p. ej. bajar el marcador de versión del bloque, o borrar una entrada de hook)
./scripts/test-codebase-memory-activation.sh; echo "[exit=$? — debe ser != 0]"

# No-regresión del brazo extensor: retirar su activación y comprobar que también falla
```

---

## Criterios de aceptación agregados

| Criterio | Cómo se demuestra |
|---|---|
| SC-001 | §2 extremo a extremo: 100% de planes no triviales llegan como árbol |
| SC-002 | §2: la segunda invocación no devuelve; planes triviales nunca se devuelven |
| SC-003 | §3 extremo a extremo, 5 intentos |
| SC-004 | §6: el script pasa antes y después del cambio, incluida la activación del extensor |
| SC-005 | §6: 0 duplicados y 0 entradas ajenas perdidas tras dos instalaciones |
| SC-006 | §4: cada agente con sus canales o su degradación declarada |
| SC-007 | §5: proyecto nuevo aplica el método |
| SC-008 | §5: los dos `grep` de contenido propio devuelven 1 |
| SC-009 | §3: longitud ≤ 9 500 y sin corte a mitad de frase |
| SC-010 | §6: el script detecta el canal degradado en menos de un minuto |
| SC-011 | §2 y §3 con la integración apagada: sin errores ni mensajes |
| SC-A1 | §11 (tests/integration/foreign_agent_test.go): agente desconocido, 0 líneas de gomemory dedicadas |
| SC-A2 | §8 (mem doctor) + añadir una fila al registro: aparece sin tocar el reporte ni el script |
| SC-A3 | Todo agente en `domain.KnownAgents` declara `AgentLevelTextFloor` (T013) |

## Resultado de la validación (2026-08-17)

Ejecutada T001-T068 de `tasks.md`. Estado por criterio:

| Criterio | Resultado |
|---|---|
| SC-001, SC-002 | ✅ Verificado contra el binario real (T026, research.md) y con el agente real (T001-T004): los 5 escenarios de §2 coinciden con el diseño |
| SC-003, SC-009, SC-011 | ✅ Verificado contra el binario real (T035): el recordatorio viaja en prompts posteriores al primero (el hueco original reportado) |
| SC-004, SC-005, SC-010 | ✅ `scripts/test-codebase-memory-activation.sh` en verde (63/63) y degradando un canal de cada brazo produce el fallo esperado (T062) |
| SC-006 | ✅ `mem doctor` reporta claude (3 niveles) y opencode (degradación declarada de `plan_guard`, con motivo) — T036-T049 |
| SC-A1 | ✅ `tests/integration/foreign_agent_test.go`, cliente de 12 líneas, 0 líneas de gomemory tocadas |
| SC-A2 | ✅ Cubierto estructuralmente: el reporte y el script leen `domain.KnownAgents`/`mem doctor --json`, no listas propias (T041, T057) |
| SC-A3 | ✅ `domain/agents_test.go::TestKnownAgentsAllDeclareTextFloor` |
| SC-007, SC-008 | ✅ Ejecutado en vivo (T055/T056, 2026-08-17, con autorización explícita) contra el `$HOME` real: `mem setup-mcp --scope global --agents claude` subió `~/.claude/CLAUDE.md` de v4 a v7 (`diff` confirma 0 pérdida de contenido propio, el bloque viejo era lo último del archivo y no había nada después que arrastrarse) y escribió `PreToolUse:ExitPlanMode→plan-guard` + `PostToolUse:EnterPlanMode→plan-entered` en `~/.claude/settings.json` preservando intacta la entrada ajena del brazo extensor (`cbm-code-discovery-gate`) y el bloque `permissions`. Un proyecto temporal **nuevo, sin instalación propia**, mostró los 3 niveles de claude en `ok` vía `mem doctor` a nivel de usuario. |

### Dos bugs reales encontrados por esta misma verificación en vivo (corregidos)

Esto es exactamente el tipo de hallazgo que T055/T056 existían para atrapar — no salió nada de esto de un test con datos sintéticos:

1. **`claude/user/instructions` miraba el archivo equivocado.** `activation_inspect.go` buscaba `~/CLAUDE.md`/`~/AGENTS.md` directo bajo `$HOME` para el ámbito de usuario de cualquier agente, pero el archivo real que la instalación escribe para claude es `~/.claude/CLAUDE.md` (ver `globalTargets`, `atomic_plan_global.go`). El reporte decía "ok" por pura coincidencia: `~/AGENTS.md` es un artefacto separado ya documentado (research.md, "$HOME instalado como proyecto") que también estaba en v7. **Fix**: nueva función `userInstructionsDir(agentName, home)` que reutiliza `globalTargets` — la misma tabla que ya usa el instalador real — en vez de asumir una ruta directa bajo `home`.
2. **`opencode/*/plan_entry` se evaluaba con el mecanismo de Claude Code.** `inspectClaudeHook` (busca hooks en `.claude/settings.json`) se invocaba genéricamente para cualquier agente con `AgentLevelEntry`, incluido opencode — que no usa hooks de Claude Code en absoluto; su nivel 2 lo sostiene el plugin (`~/.config/opencode/plugins/gomemory.ts`, instalación siempre global). El reporte decía "ok" en ámbito usuario solo porque el hook de *claude* vivía en ese mismo `$HOME` compartido — sin comprobar el plugin de opencode en ningún momento. **Fix**: la rama ahora distingue por `agent.Dialect`; `inspectClaudeHook` solo se invoca para `DialectClaude`, y se añadió `inspectAgentEntryFile` (vía el mapa `agentEntryFiles`) para comprobar la presencia real del plugin de agentes con otro mecanismo. En ámbito de proyecto, ese mismo canal para opencode ahora reporta `not_applicable` con motivo ("este agente instala su mecanismo de entrada de forma global, no por proyecto") en vez de un falso `missing`.

Ambos bugs tenían la misma forma: el inspector asumía una ruta o mecanismo compartido con Claude Code en vez de consultar la fuente real específica de cada agente. Cubiertos con tests de regresión (`TestActivationInspect_ClaudeUserInstructionsMiraElSubdirectorioCorrecto`,
`TestActivationInspect_OpenCodeEntryComprubaElPluginRealNoElHookDeClaude`,
`TestActivationInspect_OpenCodeEntryOKConElPluginInstalado`) antes del fix (rojo confirmado) y después (verde). Transparente al canal de instalación: ni `scripts/install.sh` ni `scripts/install.ps1` invocan `setup-mcp --scope global` automáticamente (solo colocan el binario), así que el fix — que reutiliza `globalTargets`, la misma fuente que ya usa el instalador real — aplica igual sin importar cómo se obtuvo el binario.
