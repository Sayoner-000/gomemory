# Quickstart: validar HCD de extremo a extremo

**Feature**: `045-hcd-protocol` · Contratos: [engine-cli](./contracts/engine-cli.md), [skills](./contracts/skills.md), [artifact-format](./contracts/artifact-format.md)

Guía de validación, no de implementación. Cada escenario indica los criterios que demuestra.

## Prerrequisitos

- El binario `mem` compilado desde la rama, con el que realmente se va a probar: `go build -o ./mem ./infrastructure`.
- Un repositorio de prueba **aislado** (`HOME` y proyecto en un directorio temporal), para no tocar la memoria real: `export HOME=$(mktemp -d)` y `mkdir proj && cd proj && git init`.
- Las skills instaladas en el agente que se va a validar (`./mem install`). Comprobar que existen `~/.claude/skills/hcd*/SKILL.md` y, en Codex o OpenCode, sus rutas equivalentes.
- La §18 corregida y aprobada (D1) antes de crear una unidad HCD real.

## Escenario 1 — Recorrido mínimo con `/hcd` (SC-001; C-03, C-04, C-05, C-06, C-08, C-23)

En Claude Code, dentro del proyecto de prueba:

1. `/hcd inicia una corrección: el resume de sesión falla con id inexistente` → el agente propone `fix` y un título y, al confirmarlos, `HCD-001` existe en estado `draft`.
2. `/hcd-define` con una definición a la que le falta «Excluido» → `validate` lista `DEF-PENDING` con el campo afectado en `ref` y el estado sigue `draft` (C-03).
3. Completar la definición → `validate` PASS. `/hcd-approve` muestra la revisión; responder «aprobar» deja la aprobación registrada con origen `conversation` y la cita (C-04).
4. `/hcd-slice` y luego `/hcd-tasks` con T-001 válida y T-002 que referencia `C-99` → `check T-002` responde `atomicity` con `ATOM-LINK` (C-05).
5. `/hcd-implement T-001` → `run`, cambio, prueba y `report` con evidencia `pass`.
6. `/hcd-verify` → PASS. `/hcd-close` y responder «aprobar cierre» → `completed`.
7. Comprobar que no se escribió ningún comando de consola a mano y que no existe `specs/` nuevo.

## Escenario 2 — Invariantes del motor (C-11, C-17, C-18, C-19)

Desde la terminal, como respaldo, con `--json`:

- Comparar el `revision.state` de `resume`, `status`, `next`, `show` y `check` antes y después: es idéntico (C-11).
- Repetir `define --operation-id op1` con el mismo archivo devuelve el mismo resultado sin incrementar `revision.state`. Con otro archivo, `operation_conflict`, exit 4 (C-18).
- `define --expected-revision 1` cuando la revisión actual es 3 → `revision_conflict`, exit 4, y el archivo registrado no cambia (C-17).
- `run --task T-001` sin `report` y con la concesión vencida (`--timeout` recibe horas enteras, p. ej. `--timeout 1`; para probarlo sin esperar, baja `hcd_grant_timeout_hours` o adelanta el reloj en la prueba) → `status` muestra la ejecución `uncertain`, y un nuevo `run` de la misma tarea responde `uncertain`, exit 7 (C-19).

## Escenario 3 — Evidencia que caduca (C-09, C-22)

Tras el escenario 1, antes de cerrar: editar `definition.md` a mano. `status` es de solo lectura (FR-046) y no la registra; la primera mutación detecta la edición, responde `revision_conflict`, crea la nueva revisión, deja la evidencia en `stale` y la unidad en `draft` sin aprobación; `verify` responde BLOCKED y `close` responde `not_allowed` (C-09). Para desbloquear hay que aprobar la revisión vigente con `/hcd-approve` y volver a verificar; restaurar el contenido anterior no reemplaza esa aprobación.

## Escenario 4 — Aclaración en lugar de suposición (FR-016a)

Definición con «Resultado esperado: mejorar el rendimiento». `/hcd-define` debe abrir `hcd-clarify` con una pregunta y opciones medibles. `approve` responde `blocked` hasta registrar la respuesta, que aparece como decisión `D-01` y como memoria `type=decision` con `topic_key hcd:HCD-001:D-01`.

## Escenario 5 — Memoria y retomar (C-10, C-12, C-13)

- Cerrar la sesión del agente, abrir otra y ejecutar `/hcd HCD-001` → muestra el paso, las decisiones y la siguiente acción en una sola respuesta (SC-007), sin un nuevo `run`.
- Simular un fallo de memoria (base de solo lectura durante el registro de la decisión) → `memory_sync_status: pending`. Restaurar y ejecutar `memory sync` → `synced`, con una sola memoria `hcd:HCD-001:D-0N` (C-13).

## Escenario 6 — Coexistencia con Spec Kit (C-02, C-24)

- `git status -- specs/ adapters/secondary/speckit .claude/skills/speckit-*` antes y después de todo lo anterior: sin cambios.
- `mem hcd list --json` muestra `specs/044-cli-socratica-slice` con `approach: speckit`. `mem hcd define` sobre esa unidad responde `approach_mismatch`.

## Escenario 7 — Entornos reales (SC-008)

Repetir los pasos 1 y 6 del escenario 1 con `$hcd` en Codex y con `/hcd` en OpenCode. Guardar la transcripción como evidencia en `specs/045-hcd-protocol/evidence/`. Un entorno sin esa evidencia no se documenta como compatible.

## Escenario 8 — Unidad activa, heartbeat y recuperación entre agentes (FR-041a–d, SC-011)

1. En Claude Code, `/hcd HCD-001` → `mem hcd focus --json` devuelve `HCD-001`.
2. Pedir al agente un cambio de archivo **sin** pasar por `/hcd-implement`. Al terminar el turno, `mem hcd status HCD-001 --json` muestra un desvío `files_without_execution`, sin decisiones nuevas, y `mem search checkpoint` sigue mostrando el checkpoint normal, independiente del heartbeat.
3. Abrir Codex (o OpenCode) en el mismo proyecto: el contexto de arranque contiene `## HCD activo` con HCD-001, su paso, las decisiones recientes y el desvío. `$hcd` sin argumentos retoma HCD-001.
4. `git status` no muestra por causa de HCD ningún cambio fuera de `hcd/` (ni archivos de contexto del agente ni `.specify/feature.json`).
5. `/hcd-close` → `focus` queda vacío y la sección «HCD activo» desaparece en la siguiente sesión.

## Escenario 9 — Los cuatro tipos de unidad (SC-002; FR-014)

El escenario 1 ya cubre `fix`. Repetir el recorrido mínimo con los otros tres tipos:

| Tipo | Solicitud con `/hcd` | Campo particular que `validate` debe exigir |
|---|---|---|
| `feature` | «añadir el filtro por estado a `mem hcd list`» | nuevo comportamiento y actor beneficiado |
| `evolution` | «que `status` muestre también las verificaciones desactualizadas» | comportamiento actual, cambio esperado y compatibilidad |
| `technical` | «reducir el tiempo de `resume` con 50 tareas» | problema técnico observable, beneficiario y condición de mejora medible |

Para cada tipo, primero con el campo particular ausente (`validate` → `DEF-PENDING`) y después completo hasta `close`. Con el escenario 1 (`fix`), el 5 (retomada) y la migración (T114), quedan los seis escenarios de SC-002.

## Pruebas automatizadas

```bash
go test ./domain/... ./application/usecases/... -run HCD
go test ./adapters/secondary/persistence/... -run HCD
go test ./adapters/primary/cli/hcd/...
go test ./tests/contract/... ./tests/integration/... -run HCD
go test ./...          # incluye layer_rules_test y el golden de schemaVersion
```
