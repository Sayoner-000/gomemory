# Fase 0 — Investigación: Fortalecimiento de la revisión ACR

Todo lo que sigue se verificó leyendo el código vigente en la rama `main`
(commit `cb75ac3`), no se dedujo de la documentación. Cada decisión cita el
archivo y la línea donde está hoy el comportamiento que se cambia.

## Estado de partida: no había ninguna NEEDS CLARIFICATION

La especificación pasó su checklist de calidad sin marcadores pendientes. Las
incógnitas de esta fase no eran del *qué* sino del *dónde*: en qué punto exacto
del código vive cada defecto y qué forma mínima tiene su corrección. Eso es lo
que se investigó.

---

## D-001 — La cobertura del consenso se valida en el dominio, no en el caso de uso

**Decisión**: crear `domain/consensus_coverage.go` con una función pura que reciba
los hallazgos fuente de la ronda y la clasificación propuesta, y devuelva error si
algún hallazgo queda sin clasificar, se referencia dos veces, no pertenece a la
ronda o aparece a la vez como emparejado y no emparejado.

**Justificación**: hoy `application/usecases/build_consensus.go` valida hallazgo por
hallazgo mientras recorre la entrada —comprueba que cada ID exista y que los
revisores sean distintos— pero **nunca comprueba el conjunto**. Un hallazgo HIGH que
el orquestador simplemente omita de `matches` y `unmatched` no produce error alguno.
Después `domain.DeriveVerdict` (`domain/verdict.go`) solo falla cerrado cuando
`len(consensus) == 0`; basta una fila inocua para que el HIGH omitido sea invisible
y el veredicto salga `APPROVED`. Poner la comprobación en el dominio la hace
verificable con tests puros y la deja fuera del alcance del prompt, que es
justamente la propiedad que la feature 027 declaró querer.

**Alternativas consideradas**:
- *Validar dentro de `BuildConsensus`*: funcionaría, pero repite el error de 027 —
  la invariante queda en una capa que puede saltarse otro llamador (la CLI, un test,
  una tool futura).
- *Validar en la base de datos con un trigger*: SQLite lo permite, pero la
  constitución prohíbe lógica de negocio fuera del dominio y un trigger no es
  testeable con `go test` puro.

## D-002 — La severidad de una clasificación se deriva, no se acepta

**Decisión**: `ConsensusFinding.Severity` deja de venir de la entrada. Se calcula
como el **máximo** de las severidades de sus hallazgos fuente, con el orden
`CRITICAL > HIGH > MEDIUM > LOW > INFO`. El campo `severity` de `review_consensus`
pasa a ser informativo: si se envía y no coincide con la derivada, la operación se
rechaza en vez de degradar silenciosamente.

**Justificación**: en `build_consensus.go` la severidad de un `match` se copia tal cual
desde `ConsensusMatch.Severity`, aportada por el orquestador. Un HIGH reportado por
ambos revisores puede persistirse como `LOW` sin que nada lo impida, y `DeriveVerdict`
solo mira `finding.Severity.Severe()` — con lo que el defecto desaparece del veredicto.
La rama de `unmatched` ya deriva la severidad de la fuente
(`source.finding.Severity`), así que esta decisión **unifica** el comportamiento en
lugar de inventar uno nuevo.

**Alternativas consideradas**:
- *Permitir bajar la severidad con autorización explícita*: añade una vía de escape a
  la única propiedad que hace útil al protocolo. Rechazada.
- *Usar la severidad más baja de las dos fuentes*: sería conservador al revés —
  premia al revisor que subestima. Rechazada.

## D-003 — Los re-juicios se guardan por revisor en una tabla propia

**Decisión**: nueva tabla `rejudgments (review_id, round, consensus_finding_id,
reviewer, state, evidence, created_at)` con `UNIQUE(review_id, round,
consensus_finding_id, reviewer)`. La columna `consensus_findings.rejudgment_state`
se conserva por compatibilidad, pero pasa a ser **derivada**: se recalcula desde la
tabla nueva y ya no se acepta del llamador.

**Justificación**: FR-012 exige registrar el re-juicio "por revisor, ronda y hallazgo,
incluyendo estado y evidencia verificable", y FR-013 exige unanimidad de dos revisores
independientes. Hoy `application/usecases/rejudge_review.go` recibe un
`map[string]ReJudgmentState` sin ninguna noción de quién lo emite y escribe
directamente sobre `consensus_findings.rejudgment_state`
(`adapters/secondary/persistence/db.go`, tabla `consensus_findings`). Con un solo
campo agregado es literalmente imposible expresar "un revisor dice resuelto y el otro
no", que es el caso que FR-014 obliga a distinguir. La regla de agregación
(`REGRESSED` de cualquiera manda; sin unanimidad de `RESOLVED` queda `UNRESOLVED`)
vive en `domain/rejudgment.go` como función pura.

**Alternativas consideradas**:
- *Añadir dos columnas `rejudgment_state_a` / `rejudgment_state_b`*: más barato, pero
  no guarda evidencia por revisor ni admite más de una ronda de re-juicio, y FR-012
  pide ambas cosas.
- *Reutilizar `reviewer_results` con un tipo de resultado nuevo*: mezcla el hallazgo
  original con su revalidación en la misma tabla y rompe la lectura de
  `ListReviewerResults`, del que ya depende `DeriveVerdict`.

## D-004 — La corrección se registra en una transacción `BEGIN IMMEDIATE`

**Decisión**: `RecordFix` envuelve en una sola transacción SQLite la lectura del
número de rondas registradas, la inserción del `fix_delta` y la actualización de
`reviews.round` / `reviews.status`. Se abre con `BEGIN IMMEDIATE` y se apoya en el
`UNIQUE(review_id, round)` que la tabla `fix_rounds` ya tiene.

**Justificación**: hoy `application/usecases/record_fix.go` hace
`ListFixDeltas` → `NextFixRound(len(existentes))` → `UpsertFixDelta` →
`UpdateReview` en cuatro operaciones sueltas. Dos procesos concurrentes leen el mismo
`len(existentes)`, derivan el mismo número de ronda y el segundo **sobrescribe** la
corrección del primero por el `UPSERT`. SC-003 exige que en 100 ejecuciones
concurrentes se conserve una única transición válida. `BEGIN IMMEDIATE` toma el
bloqueo de escritura al abrir, no al primer `INSERT`, que es lo que cierra la ventana
entre el `SELECT` y el `INSERT`. El proyecto ya usa WAL con busy timeout de 5 s, así
que el segundo proceso espera y luego falla contra el índice único en vez de pisar.

**Alternativas consideradas**:
- *`INSERT ... ON CONFLICT DO NOTHING` y comprobar filas afectadas*: evita la
  transacción pero deja `reviews.round` actualizable por el perdedor de la carrera.
- *Bloqueo a nivel de aplicación con un mutex*: no protege entre procesos, y `mem`
  corre como CLI y como servidor MCP a la vez.

## D-005 — El `UPSERT` de consenso se sustituye por "idempotente o rechazado"

**Decisión**: `BuildConsensus` calcula una huella determinista de la clasificación
completa de la ronda. Si no hay consenso previo para esa ronda, lo inserta. Si lo hay
y la huella coincide, devuelve lo persistido sin escribir. Si lo hay y la huella
difiere, rechaza con un error explícito.

**Justificación**: FR-005. `ledger.UpsertConsensusFinding` sobrescribe hoy sin
preguntar, y los `ConsensusLocalID` se generan por posición
(`fmt.Sprintf("C-%03d", i+1)`), de modo que reenviar la misma clasificación en otro
orden reasigna identificadores y rompe las referencias que `RecordFix` y
`RejudgeReview` ya guardaron. La huella se calcula sobre las fuentes ordenadas, no
sobre el orden de llegada, y los `ConsensusLocalID` pasan a asignarse tras ordenar
por el menor ID de hallazgo fuente — así el mismo conjunto produce siempre los mismos
identificadores.

**Alternativas consideradas**:
- *Permitir el reemplazo mientras la revisión no esté finalizada*: contradice FR-005
  y abre la puerta a reclasificar un hallazgo confirmado como informativo justo antes
  de finalizar.

## D-006 — `review-only` es política persistida, no ausencia de configuración

**Decisión**: `Review` gana `FixAuthorized bool`. `StartReview` lo toma de la entrada
o, si no viene, de `Settings.ReviewFixAuthorized` (campo nuevo, por defecto `true` para
no cambiar el comportamiento de las revisiones existentes). `DeriveVerdict` devuelve
`ESCALATED` cuando hay un confirmado severo sin resolver y `FixAuthorized` es falso,
en vez de devolver la cadena vacía.

**Justificación**: es el defecto reproducido en la revisión original —memoria 55—:
con dos resultados `success`, consenso persistido y un `CONFIRMED HIGH` abierto,
`review_finalize` devolvió `review is not ready to finalize` y dejó la revisión en
`consensus_ready` para siempre, porque `NextFixRound` aún permitía correcciones que
el alcance de solo lectura prohibía hacer. Hoy `Review` no tiene forma de expresar
"esta revisión no va a corregir nada", así que el estado es irrecuperable sin editar
la base de datos a mano.

**Alternativas consideradas**:
- *Una tool `review_escalate` manual*: resuelve el bloqueo pero deja el veredicto a
  criterio del agente, que es exactamente lo que INV-010 prohíbe.
- *Deducir `review-only` de `MaxFixRounds == 0`*: sobrecarga un campo numérico con un
  significado booleano y colisiona con el default de la política.

## D-007 — La política del proyecto se lee en `StartReview`

**Decisión**: `StartReview` recibe la política ya resuelta desde el llamador
(CLI y tool MCP), que la lee de `Settings`. Los defaults hardcodeados de
`start_review.go` desaparecen y se sustituyen por `domain.DefaultMaxFixRounds` y la
severidad por defecto del dominio, que ya existen en `domain/fix.go`.

**Justificación**: FR-017. `Settings` ya tiene `ReviewMaxFixRounds` y
`ReviewAutoFixSeverities` con su `applyReviewDefaults`
(`adapters/secondary/persistence/settings.go`), pero **nadie los lee**:
`start_review.go` reimplanta `maxRounds = 2` y `{CRITICAL, HIGH}` a mano, y
`cmd_review.go` construye el `StartReviewInput` sin tocar la política. El resultado es
que configurar el proyecto no tiene ningún efecto. La corrección no añade
configuración nueva: conecta la que ya está.

## D-008 — El congelado de cambios pendientes reutiliza el patrón de `resolveFileTarget`

**Decisión**: nuevo modo `mem review --pending`. Obtiene la lista de rutas con
`git status --porcelain=v1 -z --untracked-files=all` (respeta `.gitignore`), la
ordena, y hashea `"pending\x00" + ruta + "\x00" + contenido + "\x00"` por archivo,
como ya hace `resolveFileTarget` en `adapters/primary/cli/cmd_review.go`. Si la lista
queda vacía, se rechaza con diagnóstico.

**Justificación**: FR-025 y FR-026. `--diff` usa `git diff --binary`, que **no ve los
archivos nuevos sin seguimiento**: una revisión de trabajo en curso con archivos
recién creados congela un target que no los contiene, y los revisores inspeccionan
menos de lo que se cree que inspeccionan. El `-z` es necesario porque los nombres con
espacios rompen el parseo por líneas, uno de los casos límite que la spec enumera.
Los archivos borrados se hashean como ruta con contenido vacío y un marcador, para
que borrar un archivo cambie la identidad del target.

**Alternativas consideradas**:
- *`git stash create` y usar su árbol*: produce un identificador limpio, pero crea un
  objeto en el repositorio del usuario, y la spec excluye cualquier mutación del
  control de versiones.
- *`git add -A` seguido de `git write-tree`*: modifica el índice del usuario. Rechazada
  por el mismo motivo.

## D-009 — Las métricas se serializan desde un DTO del adaptador

**Decisión**: `ReviewMetrics` gana `Duration`, `MemoryPromoted` y `MemoryDeduplicated`,
y la tool MCP los expone a través de un DTO con etiquetas JSON en `snake_case`
declarado en `cmd_mcp_review_tools.go`. `duration` se calcula como
`review.UpdatedAt - review.CreatedAt` en el momento de finalizar y se emite en
segundos.

**Justificación**: el contrato publicado en
`specs/027-adversarial-consensus-review/contracts/mcp-tools.md` promete
`metrics{duration, findings_total, findings_confirmed, findings_suspect,
contradictions, fix_rounds, memory_promoted, memory_deduplicated}`. El struct real en
`application/usecases/finalize_review.go` omite tres de esos ocho campos y **no tiene
ninguna etiqueta JSON**, así que `json.Marshal` publica `FindingsTotal`,
`FixRounds`… en PascalCase. La respuesta incumple el contrato en nombres y en
contenido. El DTO va en el adaptador y no en el caso de uso porque la serialización
es un detalle del canal, no del dominio — la constitución sitúa ahí esa
responsabilidad.

**Alternativas consideradas**:
- *Poner las etiquetas JSON en `ReviewMetrics`*: más corto, pero acopla un tipo de la
  capa de aplicación al formato de un adaptador concreto.

## D-010 — La promoción exige veredicto `APPROVED`

**Decisión**: `PromoteReviewMemory` rechaza si `review.Verdict != APPROVED`.

**Justificación**: FR-021 y el escenario 5 de US3. El caso de uso ya comprueba que el
hallazgo sea `CONFIRMED` y esté `RESOLVED`
(`application/usecases/promote_review_memory.go`), pero **no mira el veredicto de la
revisión**: se puede promover conocimiento de una revisión que todavía va a escalar.
Es una línea, y es la que faltaba.

## D-011 — Los estados terminales se protegen en el único sitio que los escribe

**Decisión**: toda escritura de estado pasa por un helper de dominio
`Review.TransitionTo(next)` que aplica el `CanTransitionTo` ya existente y devuelve
error si el estado actual es terminal. Los casos de uso dejan de asignar
`review.Status = ...` directamente.

**Justificación**: FR-015 y FR-016. `domain/review.go` ya tiene `Terminal()` y
`CanTransitionTo()` completos y correctos, pero **ningún caso de uso los llama**:
`submit_reviewer_result.go`, `record_fix.go` y `finalize_review.go` asignan el estado
a pelo. Una revisión `APPROVED` acepta hoy resultados nuevos, consenso nuevo y
correcciones nuevas sin protestar. La máquina de estados existe; lo que falta es que
alguien la use.

---

## Resumen de puntos de cambio verificados

| Archivo | Qué hace hoy | Qué debe hacer |
|---|---|---|
| `application/usecases/build_consensus.go` | Valida hallazgo a hallazgo; copia la severidad de la entrada; `UPSERT` ciego | Valida el conjunto; deriva la severidad; idempotente o rechaza |
| `domain/verdict.go` | Falla cerrado solo con cero filas de consenso | Falla cerrado ante cualquier fuente sin clasificar; escala si no hay corrección autorizada |
| `application/usecases/rejudge_review.go` | Estado agregado, sin revisor | Estado por revisor con evidencia, agregación en el dominio |
| `application/usecases/record_fix.go` | Cuatro operaciones sueltas | Una transacción `BEGIN IMMEDIATE` |
| `application/usecases/finalize_review.go` | Métricas incompletas y en PascalCase | Ocho campos del contrato en `snake_case` |
| `application/usecases/promote_review_memory.go` | No mira el veredicto | Exige `APPROVED` |
| `application/usecases/start_review.go` | Defaults hardcodeados | Lee la política del proyecto |
| `adapters/primary/cli/cmd_review.go` | `--diff` ignora archivos nuevos | `--pending` congela todo lo pendiente |
| `adapters/primary/cli/cmd_mcp_review_tools.go` | `review_status` devuelve 4 campos | Resumen por clasificación y estado de re-juicio |
