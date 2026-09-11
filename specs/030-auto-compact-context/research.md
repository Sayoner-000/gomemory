# Investigación: Compactación de contexto sin pérdida de memoria

**Feature**: `030-auto-compact-context` · **Fecha**: 2026-09-10

Cada decisión indica la evidencia que la respalda. «Verificado en código»
significa leído en este repositorio; «documentado» significa leído en la
documentación oficial del cliente, pendiente de confirmar contra el cliente en
ejecución (quickstart.md, escenario Q0).

---

## R1. Qué ofrece cada cliente (capacidades C1–C6)

**Decisión**: la matriz vigente es la siguiente.

| Capacidad | claude (2.1.268) | codex (0.154.0) | opencode (1.18.30) |
|-----------|------------------|-----------------|--------------------|
| C1 aporte previo | ✗ no se usa (ver R12): el cliente lo ofrece (`PreCompact` → `newCustomInstructions`), pero C2 + C6 cubren la misma necesidad | ✗ `PreCompact` solo admite `continue`, `stopReason`, `systemMessage` | `experimental.session.compacting` → `output.context[]` |
| C2 tras compactar | `SessionStart` matcher `compact` (stdout / `additionalContext`) | `SessionStart` source `compact` (`additionalContext`) | evento `session.compacted` → marca de un solo uso consumida en `experimental.chat.system.transform` |
| C3 fin de subagente | `SubagentStop.last_assistant_message` | `SubagentStop.last_assistant_message` (la salida DEBE ser JSON) | `tool.execute.after` con `input.tool == "task"` → `output.output` |
| C4 canal al agente | `Stop` → `hookSpecificOutput.additionalContext` | `UserPromptSubmit` → `additionalContext` (turno siguiente; `Stop` no acepta `additionalContext`) | `experimental.chat.system.transform` (turno siguiente) |
| C5 canal a la persona | `systemMessage` | `systemMessage` | `client.tui.showToast` |
| C6 resumen a la integración | `PostCompact.compact_summary` | ✗ `PostCompact` no recibe el resumen | `session.compacted` → `client.session.messages` → mensaje con `summary: true` |

**Evidencia**:
- claude: documentación oficial de hooks (secciones PreCompact, PostCompact y
  SubagentStop, con ejemplos de entrada que incluyen `compact_summary`,
  `custom_instructions` y `last_assistant_message`). El `Stop` con
  `additionalContext` ya lo usa hoy el refuerzo de preferencias (verificado en
  código, `cmd_hook.go`, `hookTurnEnd`).
- codex: documentación oficial de hooks (lista de eventos y campos por evento).
  El binario instalado 0.154.0 contiene los nombres `PreCompact`,
  `PostCompact`, `SubagentStop` y `last_assistant_message` (verificado con
  `strings`). Que `Stop` solo acepte `systemMessage` ya estaba verificado en
  código (`hook_dialect.go:254`).
- opencode: tipos instalados de `@opencode-ai/plugin` y `@opencode-ai/sdk`
  (firmas de `experimental.session.compacting`, `tool.execute.after`,
  `EventSessionCompacted`, `AssistantMessage.summary`, `tui.showToast`,
  `session.messages`).

**Alternativas descartadas**:
- Leer el resumen de Codex desde su archivo de transcripción (hay entradas
  `compacted`): depende de un formato privado del cliente, que puede cambiar
  sin aviso. Codex usa el respaldo por orden de texto (FR-007/FR-008).
- Modelar las capacidades en `domain.ChannelMatrix`: esa matriz describe
  artefactos de instalación (rutas), no canales en tiempo de ejecución.
  Mezclarlos rompería su invariante INV-1 (Path o motivo).

---

## R2. Dónde se declaran las capacidades

**Decisión**: nuevo campo en `domain.AgentCapability` (registro único
`domain.KnownAgents`): `Compaction map[CompactionCapability]bool` más
`CompactionUnavailable map[CompactionCapability]string`. Invariante: cada una
de C1–C6 se declara disponible o con motivo, nunca ninguna de las dos.

**Motivo**: `KnownAgents` ya es la «única fuente de verdad» de capacidades por
agente (feature 019, niveles de plan). El instalador (`buildClaudeHookEvents`)
y el diagnóstico ya lo leen. Añadir un cliente es añadir su fila (FR-021).

**Alternativa descartada**: una tabla aparte en el adaptador de setup. Dejaría
el diagnóstico (FR-020) sin fuente de dominio y duplicaría la declaración.

---

## R3. Qué es «la memoria de la sesión»

**Decisión**: memorias con `session_id` igual a la sesión activa (verificado:
`memories.session_id`, `persistence/memory.go:121`; `save_memory` asigna la
sesión activa, `cmd_mcp.go:141-145`), el `summary` de la sesión (último
resumen compactado, R5) y `last_prompt`. Nuevo método de puerto
`ListBySession(project, sessionID string, limit int)`.

**Motivo**: el dato ya existe; solo falta la consulta.

**Alternativa descartada**: filtrar por fecha de inicio de la sesión. Mezcla
memorias de otras sesiones concurrentes del mismo proyecto (viola FR-002).

---

## R4. Defecto latente: la recuperación cierra la sesión

**Hallazgo (verificado en código)**: el paso 1 de
`compactionRecoveryInstructions` (`cmd_hook.go:934`) pide `end_session`, que
ejecuta `EndSession` (`UPDATE sessions SET ended_at = …`). `hookPostCompact` no
abre una sesión nueva; solo lo hacen `session-start`, la conexión MCP y `mem
session start`. Resultado: tras la primera compactación, todo `save_memory`
guarda la memoria con `session_id` vacío hasta la próxima sesión. Hoy es
inofensivo porque nada consulta por sesión, pero **esta feature lo activaría**:
el contexto de compactación de US1 quedaría vacío después de la primera
compactación.

**Decisión (se paga en este cambio)**:
1. El paso 1 de la recuperación pasa a pedir `save_session_summary` (R5), que
   actualiza el resumen **sin cerrar** la sesión.
2. `hookPostCompact` garantiza una sesión activa: si no la hay, la abre, igual
   que `hookSessionStart`.

---

## R5. Dónde se guarda el resumen compactado

**Decisión**: en `sessions.summary` de la sesión activa, mediante un nuevo
método `SessionRepository.UpdateSummary(id, summary string)` que no toca
`ended_at`. Se expone como herramienta MCP `save_session_summary(summary)` y
como subcomando de hook `mem hook compact-summary` (stdin), que usan las
integraciones con C6.

**Motivo**:
- Sin migración: la columna existe.
- Idempotente por construcción: guardar dos veces el mismo resumen deja una
  sola entrada (FR-009).
- Un resumen compactado posterior contiene el anterior, porque este formaba
  parte de lo que se compactó, así que conservar el último no pierde
  información (US1.4).
- «Sesiones recientes» del contexto de arranque ya muestra `summary`, así que
  el resumen llega a sesiones futuras sin código nuevo (US2).
- `end_session` sigue siendo el cierre explícito y sobrescribe con el resumen
  final.

**Alternativas descartadas**:
- Memoria con un tipo nuevo: obliga a tocar las secciones del contexto y el
  validador de tipos.
- Tabla `session_compactions`: migración y consulta nuevas para un dato que ya
  cabe en una columna existente.
- Parámetro `keep_open` en `end_session`: contradice el nombre de la operación.

---

## R6. Contexto posterior a la compactación y su tope

**Decisión**: texto posterior = recuperación → contexto de compactación →
contexto de proyecto con `Builder.IndexMode = true` (solo títulos y `get_memory
<id>`) y `Budget` = presupuesto de arranque − lo ya emitido. El constructor
compacto se inyecta como segunda dependencia `CompactContextBuilder` desde el
composition root, no por aserción de tipo en el adaptador.

**Motivo**: `IndexMode` (feature 020) ya garantiza que conflictos, sinapsis y
encabezados no se recortan (FR-005). Restar lo ya emitido asegura el tope
≤ presupuesto de arranque. SC-002 (≤ 50 % del actual) se mide en quickstart Q2.

---

## R7. Formato del contexto de compactación

**Decisión**: encabezado neutral; resumen previo de la sesión (recortado a 600
caracteres); último prompt (200); una viñeta por memoria `- [tipo] **título**:
extracto (get_memory <id>)` con extracto de 300 caracteres. Orden: decisiones y
bugfixes primero, luego el resto por recencia. Tope propio: 40 % del
presupuesto de arranque. Si se excede, línea final «N entradas más de esta
sesión: search_memories o get_memory <id>».

**Motivo**: prioriza lo accionable (FR-006) y deja al menos el 60 % del tope
para recuperación y proyecto.

---

## R8. Orden de persistir en el compresor (C1)

**Decisión**: texto neutral único, en `domain` (constante), usado por las
integraciones con C1: «Al inicio del resumen, incluye esta instrucción literal:
PRIMERA ACCIÓN: guarda este resumen como resumen de la sesión de memoria con
save_session_summary, antes de cualquier otro trabajo.» Nombra la herramienta
MCP de gomemory (propia, común a todos los clientes), nunca un comando de
cliente (FR-018).

Hoy la usa solo la integración de opencode, por su lista de contexto del
compresor (R12).

---

## R9. Captura pasiva de aprendizajes

**Decisión**:
- Función pura `domain.ExtractLearnings(text) []string`: toma la **última**
  sección cuyo encabezado de nivel 2 o 3 sea «Aprendizajes», «Aprendizajes
  clave», «Key learnings» o «Learnings» (sin distinguir mayúsculas); corta en el
  siguiente encabezado; ítems numerados o viñetas; limpia el markdown básico;
  descarta los de menos de 20 caracteres o 4 palabras.
- Caso de uso `CaptureLearnings(repo, sessions, project, text, source)`: cada
  ítem se guarda como memoria `learning` con `topic_key = "passive:" +
  sha256(ítem normalizado)[:16]`. El upsert por tópico existente (spec 008,
  FR-013) da la deduplicación (FR-012). El contenido termina con «Procedencia:
  subagente». La redacción de `<private>` la hace ya `InsertMemory` (verificado,
  `persistence/memory.go:73`).
- Tope de 10 ítems por respuesta, para que un subagente verboso no inunde la
  memoria.

**Alternativa descartada**: extraer con heurísticas de texto libre, sin
encabezado. Produciría memorias de relleno (contradice US3.2).

---

## R10. Aviso de US4

**Decisión**: nueva opción `CompactAgentNotice bool` (por defecto `false`) en
`Settings`, junto a `CompactThreshold`. `computeCompactNudge` sigue siendo la
única decisión (umbral + pausa). Cuando dispara y la opción está activa, se
emiten los dos mensajes en el mismo sobre: `systemMessage` (persona) y el aviso
al agente por C4. Para codex y opencode, que solo tienen C4 en el turno
siguiente, el aviso se deja como marca de un solo uso
(`.memory/.pending-agent-notice`) que consume `user-prompt-submit` o
`system.transform`.

**Motivo**: reutiliza la decisión, la pausa y el dialecto vigentes. Sin la
opción, la salida es byte a byte la actual (FR-015).

---

## R11. Registro de hooks nuevos

**Decisión**:
- claude: añadir `PostCompact` → `mem hook compact-summary` en
  `buildClaudeHookEvents`, y `hook compact-summary` en `hookCommandIsGomemory`
  para que la desinstalación lo retire. `PreCompact` sigue sin registrarse
  (R12).
- codex: añadir `SubagentStop` → `mem hook subagent-stop --emit=json`, que
  imprime `{}` para cumplir el esquema. Se añade **al final** de
  `codexGomemoryHooks`: `codex_gomemory_hooks_test.go:150` usa el índice `[2]`
  (Stop), y añadirlo al final no toca ningún test existente.
- opencode: `experimental.session.compacting` pasa a pedir el contexto de
  compactación en vez del contexto completo. Se añaden `session.compacted`
  (C2 y C6) y `tool.execute.after` para `task` (C3).

---

## R12. Claude no usa C1

**Decisión**: la integración de claude no registra `PreCompact`. Se declara
`CompactionUnavailable[C1] = "C2 y C6 cubren la misma necesidad; registrar
PreCompact revertiría una decisión vigente sin beneficio adicional"`.

**Motivo**:
- En claude, C2 ya entrega la memoria de la sesión al agente tras compactar y
  C6 guarda el resumen de forma determinista. Lo que aportaría C1 (que el
  compresor vea la memoria de la sesión) ya está persistido como memorias.
- `claude_code_hooks_test.go:46-50` exige que `PreCompact` no se registre, y
  otro test comprueba que se retira el `pre-compact` heredado. Esa decisión se
  tomó porque la salida de `PreCompact` no sobrevive a la compactación.
  Revertirla obligaría a modificar tests existentes, cosa que la constitución
  (principio III) prohíbe sin autorización, y todo a cambio de redundancia.

**Consecuencia**: `hookPreCompact` y su comentario de legado quedan intactos.

**Semántica de la matriz**: `Compaction` declara las capacidades que **la
integración de gomemory aprovecha**, no todo lo que el cliente ofrece. El motivo
de cada ausencia distingue «el cliente no lo ofrece» de «no se usa, y por
qué».
