# Evidencia T114 — migración representativa (SC-009)

Fecha: 2026-10-28. Binario compilado desde la rama (`go build ./infrastructure`), `HOME` y repositorio temporales, `GOMEMORY_NO_CODE_REFRESH=1`.
Fuente: copia de `specs/044-cli-socratica-slice` (spec, plan, tasks, research, data-model, contracts, checklists, evidence) en `specs/` del repositorio de prueba.

## Secuencia

| Paso | Comando | Resultado |
|---|---|---|
| 0 | `mem hcd list --json` | una sola unidad: `044-cli-socratica-slice` · `speckit` |
| 1 | `mem hcd migrate --preview 044-cli-socratica-slice --json` | `M-01`, revisión `f1ca046bbd6b`, 105 elementos: 17 `migrated`, 38 `consolidated`, 36 `reference`, 14 `discarded` (todos con justificación), 0 `pending`. Sin unidad HCD ni carpeta `hcd/` |
| 2 | `mem hcd migrate --activate M-01 --revision f1ca046bbd6b --quote "vale"` | `invalid_quote` (la cita no nombra la acción ni la revisión) |
| 3 | `mem hcd migrate --activate M-01 --revision f1ca046bbd6b --quote "apruebo f1ca046bbd6b" --implementer dev` | `HCD-001` · `hcd` · `draft` (no verificada ni aprobada) |
| 4 | `mem hcd list --json` | una sola unidad: `HCD-001` · `hcd` (C-21: la carpeta Spec Kit migrada ya no figura como segundo enfoque) |
| 5 | `shasum` de todo `specs/` antes y después | idéntico (`specs/ intacto`): la migración no escribe bajo `specs/` |
| 6 | `mem hcd migrate --activate M-01 …` de nuevo | `not_allowed` (una propuesta activada no se reactiva) |

## Cobertura de criterios

- **C-20** — pasos 0 y 1: tras la propuesta la unidad sigue `speckit` y no existe `hcd/`.
- **C-21** — pasos 3 y 4: queda un único enfoque activo (HCD) y las fuentes se conservan como `legacy-reference` (`hcd_links`).
- **FR-056 / US7 escenario 4** — las tareas `[x]` quedan como `reference` con la justificación «no se promueve a verificada»; la unidad resultante está en `draft` sin evidencia.
- **SC-009** — 105 de 105 elementos clasificados; 0 tareas promovidas a verificadas.
- **Fuentes cambiadas / fallo a mitad** — cubiertos por pruebas en `application/usecases/hcd_migrate_test.go` (`…RequiresNewProposalWhenSourcesChange`, `…FailureLeavesSingleRecoverableApproach`).

## Observación

La definición resultante es la plantilla (`Problema: …`) y `hcd validate` no la marca como pendiente: el motor no trata el marcador `…` como campo vacío. Es el mismo comportamiento de `hcd start`, anterior a la fase 11; queda fuera de esta evidencia.
