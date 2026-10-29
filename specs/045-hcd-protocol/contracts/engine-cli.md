# Contrato: motor `mem hcd`

**Consumidores**: las skills `hcd-*` (vía principal), la TUI (mediante los mismos casos de uso) y la persona desde la terminal (vía de respaldo).
**Cubre**: FR-045a, FR-045b, FR-046, FR-047, FR-048, C-11, C-16 a C-19.

## 1. Forma general

```text
mem hcd <comando> [<unit>] [flags] [--json] [--expected-revision N] [--operation-id ID] [--client claude|codex|opencode|terminal|tui]
```

- Las skills siempre pasan `--json` y `--client`.
- Sin `--json`, la salida es texto para la persona. El contenido es el mismo; solo cambia la presentación.
- En ejecución no interactiva y con argumentos insuficientes, el motor muestra la ayuda y sale con código 2, sin elegir ni ejecutar nada.
- En la primera entrega no hay modo guiado interactivo de `mem hcd`: el modo guiado es `/hcd`.

## 2. Comandos

La columna «Fase» de `tasks.md` indica cuándo se implementa cada uno; los de secuencia y consulta forman el MVP, y los de gobierno (`exception`, `playbook`, `migrate`) llegan después.

| Comando | Muta | Flags específicos | Efecto |
|---|---|---|---|
| `start` | sí | `--kind`, `--title` | Crea `HCD-NNN`, la carpeta y `definition.md` desde la plantilla; fija `playbook_rev` y `constitution_rev`, guarda su contenido en `hcd_normative_revisions` si es nuevo, fija la unidad activa y registra la decisión de enfoque. Estado `draft`. |
| `list` | no | `--state` | Lista las unidades HCD y las carpetas Spec Kit detectadas, cada una con su enfoque. |
| `show` | no | — | Definición resumida, revisiones y referencias. |
| `resume` | no | — | Estado, paso, decisiones vinculadas, aclaraciones abiertas y siguiente acción (C-10). |
| `status` | no | — | Vista de progreso de FR-034. |
| `next` | no | — | Solo `next_action` y `allowed_actions`. |
| `define` | sí | `--file` | Registra el contenido; recalcula `definition_rev` y crea los enlaces. |
| `validate` | no | — | Comprobación estructural (FR-015 y FR-016); no muta. |
| `clarify` | sí | `--open --target --question --options --recommendation [--blocking]` · `--answer CL-NN --choice` | Abre o responde una aclaración; al responderla genera una decisión. |
| `approve` | sí | `--revision`, `--quote`, `[--object]`, `[--prompt-id]` · `--request --options` | Aprobación citada (R7). `--request` registra la pregunta con opciones y devuelve `prompt_id`; una respuesta de letra solo se acepta citando ese `prompt_id`. |
| `slice` | sí | `--file` | Registra o actualiza slices en `delivery.md`. |
| `tasks` | sí | `--slice`, `--file` | Registra tareas y valida dependencias sin ciclos. Solo añade o actualiza: una tarea se retira con una decisión aceptada `Tipo: task_cancelled` y `Referencias: T-NNN`, que la excluye de la admisión, la verificación y el cierre. |
| `check` | no | `--task` | Admisión de la tarea: atomicidad, enlaces y autorización. |
| `subject` | no | `--files a,b` | Devuelve en `data.subject_ref` el sujeto `tree:<hash> (archivos: …)` que `verify` reconoce como vigente; el agente lo copia en la línea `Sujeto:` de la evidencia. |
| `run` | sí | `--task`, `[--timeout]` (por defecto `hcd_grant_timeout_hours`, 24 h) | Concede una ejecución y devuelve `execution_id`. Sin `report` antes del plazo, pasa a `uncertain` (R8). |
| `report` | sí | `--run`, `--file` | Registra el resultado y la evidencia de la ejecución. |
| `verify` | sí* | `[--slice]` | Calcula el veredicto y regenera `verification.md` (*solo escribe el informe y transiciones derivadas). |
| `close` | sí | `--revision`, `--quote`, `[--reject --reason]` | Acepta y cierra la revisión exacta; con `--reject`, registra el rechazo de la entrega como decisión y la unidad vuelve a `in_progress`. |
| `cancel` | sí | `--quote`, `[--superseded-by HCD-NNN]` | Cancela una unidad no terminal (`cancelled` o `superseded`), registra la decisión y libera la unidad activa (FR-027a). |
| `decision` | sí | `--file` · `--accept D-NN --quote` · `--supersede D-NN --by D-MM --quote` | Sin cita, solo registra decisiones `proposed`. Aceptar o sustituir una decisión exige `--quote` con las reglas de FR-017a; sin cita válida → `invalid_quote`. Así un agente no puede satisfacer `ATOM-DECISION` por su cuenta (FR-036). |
| `block` | sí | `--file` · `--resolve B-NN` | Abre o resuelve un bloqueo. |
| `memory sync` | sí | — | Reintenta las memorias pendientes de sincronizar (C-13). |
| `classify` | sí | `<carpeta> --approach hcd|speckit`, `--quote` | Clasifica una unidad `unclassified` con la decisión citada del Implementador (FR-004); nunca mueve ni edita sus archivos. |
| `focus` | sí | `[<unit>]` · `--clear` | Fija, muestra o libera la unidad activa del proyecto (FR-041a). `start` y `resume <id>` la fijan automáticamente. |
| `heartbeat` | sí (interno) | `--client`, `--session`, `--files` | Lo invoca el hook `turn-end` en proceso, no las skills. Registra actividad y desvíos de la unidad activa; sin unidad activa no hace nada y sale con 0 (FR-041b, FR-041c). |

**Unidad implícita**: los comandos que reciben `<unit>` usan la unidad activa si se omite. Sin unidad activa, responden `usage` con la lista de unidades.

**Incrementos posteriores**: `exception`, `playbook show|validate|adopt` (la propuesta es exportar, editar e importar el documento fijado, R18) y `migrate --preview|--activate`, con la misma forma de respuesta.

## 3. Respuesta JSON (todas)

```json
{
  "protocol_version": 1,
  "ok": true,
  "unit_id": "HCD-001",
  "approach": "hcd",
  "state": "draft",
  "stage": "definition",
  "revision": {"state": 4, "definition": "a1b2c3d4e5f6", "delivery": "", "approved": ""},
  "playbook_revision": "9f8e7d6c5b4a",
  "allowed_actions": ["define", "validate", "clarify"],
  "pending_checks": [{"code": "DEF-MISSING", "ref": "definition", "detail": "falta: Alcance excluido"}],
  "blockers": [],
  "next_action": {"skill": "hcd-define", "command": "mem hcd define HCD-001 --file …", "why": "la definición está incompleta"},
  "execution_id": "",
  "memory_sync_status": "synced",
  "operation_id": "op-…",
  "data": {}
}
```

- `stage` es uno de: `context`, `intent`, `definition`, `clarification`, `approval`, `slicing`, `decomposition`, `execution`, `verification`, `continuity`, `closure`.
- `next_action.skill` dice a la skill qué paso de la secuencia sugerir. Las skills no calculan acciones por su cuenta.
- `data` lleva la carga útil específica del comando (lista, informe, etc.).

## 4. Errores

```json
{"protocol_version": 1, "ok": false, "error": {"code": "revision_conflict", "message": "…", "expected": 3, "actual": 4}, "unit_id": "HCD-001", "allowed_actions": ["show", "resume"]}
```

| Código | Exit | Cuándo |
|---|---|---|
| `usage` | 2 | Argumentos inválidos o insuficientes. |
| `not_found` | 3 | Unidad, tarea, slice o ejecución inexistentes. |
| `revision_conflict` | 4 | `expected_revision` desactualizada (C-17); no se escribe nada. |
| `operation_conflict` | 4 | Mismo `operation_id` con otro contenido (C-18). |
| `not_allowed` | 5 | La acción no está en `allowed_actions` del estado actual, o es una acción citada enviada con `--client tui` (FR-017c). |
| `blocked` | 5 | Hay aclaraciones o bloqueos abiertos, o una excepción vencida. |
| `atomicity` | 5 | `check`/`run` de una tarea no atómica; `pending_checks` lista los códigos `ATOM-*`. |
| `no_executor` | 5 | `run` sin un `--client` de agente reconocido (FR-025). |
| `already_approved` | 5 | `approve` o `approve --request` sobre un objeto y una revisión ya aprobados con el mismo alcance (FR-017: no se vuelve a pedir). |
| `invalid_quote` | 5 | Aprobación sin cita válida (FR-017a). |
| `constitution_conflict` | 5 | `start` con una Constitución vigente que exige artefactos Spec Kit por unidad y no reconoce HCD (§18 sin la enmienda); se aplica con `mem constitution`. |
| `approach_mismatch` | 6 | Se intenta usar HCD sobre una unidad Spec Kit o `unclassified`. |
| `uncertain` | 7 | La ejecución está incierta; hace falta reconciliarla. |
| `internal` | 1 | Fallo inesperado, sin detalles del driver. |

## 5. Garantías

1. `list`, `show`, `resume`, `status`, `next`, `validate` y `check` no cambian `state_revision` ni consumen concesiones (C-11). `resume <id>` solo fija la unidad activa, que no es un cambio de estado (FR-046). Hay una prueba de contrato por cada uno.
2. Toda mutación corre en una transacción `UnitOfWork`: o se escriben el estado y la operación, o no se escribe nada.
3. Antes de mutar, el motor compara el hash de los artefactos con las revisiones registradas. Si hubo una edición externa, crea una revisión nueva y aplica la invalidación (FR-040c, FR-053).
4. La TUI y la terminal invocan los mismos casos de uso y obtienen el mismo resultado ante la misma entrada (C-16).
5. El motor nunca escribe bajo `specs/` ni en `adapters/secondary/speckit` (FR-006), ni en archivos de contexto del agente o marcadores `SPECKIT`: fuera de la base, solo escribe bajo `hcd/` (FR-041a).
6. El heartbeat corre dentro del hook `turn-end` con el payload ya leído, con un plazo propio (`hcd_heartbeat_timeout_ms`, 200 ms por defecto). Si falla o vence, no bloquea: el hook escribe un aviso estructurado en stderr con la misma convención que el resto de `cmd_hook.go` (`mem hook: hcd heartbeat: <causa>; se ignora`), sigue y sale con 0 (§8: un fallo no crítico deja rastro).

## 6. Contexto de arranque: sección «HCD activo» (FR-041d)

La genera `ContextBuilder` solo si existe `hcd_focus`. Se entrega por el contexto de arranque (`SessionStart` en Claude y Codex, plugin en OpenCode) y por `get_context`:

```markdown
## HCD activo

- **HCD-001 · fix · «resume falla con id inexistente»** — paso `execution`, estado `in_progress`, rev `a1b2c3d4e5f6`
- Siguiente: `/hcd-implement T-002` — T-001 verificada; T-002 lista para `check`
- Aclaraciones abiertas: CL-02 (bloqueante, sobre T-003)
- Decisiones recientes: D-03 aceptada «el id inexistente responde not_found exit 3» (rev a1b2…); D-02 …
- ⚠ Desvíos: 2 archivos modificados sin ejecución vigente en la sesión 2026-10-27 18:10 (claude)
- Recuperar todo: `/hcd` o `$hcd`
```

La sección entra en el presupuesto de caracteres y en `deltaSections` del delta de sesión: una vez entregada, no se repite si no cambió.
