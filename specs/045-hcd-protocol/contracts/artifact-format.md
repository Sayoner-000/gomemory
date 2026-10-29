# Contrato: documento de la unidad HCD (v2)

**Cubre**: FR-005 a FR-012, FR-016. **v1**: `git show a0503b0:specs/045-hcd-protocol/contracts/artifact-format.md`. **Implementación**: `domain/hcd_doc.go` (`ParseUnitDoc`), plantilla `infrastructure/templates/hcd/unit/unit.md`.

## Reglas generales

- Un archivo por unidad: `hcd/HCD-NNN-<slug>.md`, Markdown UTF-8.
- Antes de la primera sección van `# Título` y las líneas `ID:`, `Tipo:`, `Carril:` y `Motivo:`.
- Cada sección empieza con `## Nombre`. Los comentarios HTML (`<!-- … -->`) se ignoran.
- Un valor vacío, `pendiente`, `[pendiente]`, `TODO` o `…` cuenta como faltante (`pending_checks`).
- `[NEEDS CLARIFICATION: …]` en cualquier sección de la revisión de trabajo (definición, Historias, Slices o Checklist) cuenta como aclaración abierta; en una sección obligatoria sustituye al faltante.

## Secciones

| Sección | Carril | Sintaxis | Faltante si… |
|---|---|---|---|
| `## Intención` | ambos | texto | vacía |
| `## Reproducción` | ambos, solo `fix` | comando o pasos que hoy fallan | vacía |
| `## Alcance` | ambos | `Incluido: …` y `Excluido: …` | falta cualquiera de las dos |
| `## ¿Qué pasa si no lo hacemos?` | ambos | texto | vacía |
| `## Alternativa más barata` | ambos | texto | vacía |
| `## Historias` | `full` | `### USn. título`, una línea «Como…, quiero…, para…» y `- C-NN: Dado…, cuando…, entonces…` | sin historias, historia sin texto o sin criterios, criterio vacío |
| `## Aclaraciones` | ambos | `- [ ] Qn. pregunta` (abierta) · `- [x] Qn. pregunta → respuesta` | — (las abiertas llevan al paso `clarify`) |
| `## Slices` | `full` | `### VS-NN. comportamiento`, `Tareas: 1, 2`, `Criterios: C-01` | — (sin slices, paso `slice`) |
| `## Checklist` | ambos | `- [ ] N. tarea → comprobación` (también `->`) | línea con otro formato o sin comprobación |
| `## Evidencia` | ambos | la escribe `mem hcd verify`; lo escrito a mano no cuenta | — |
| `## Siguiente` | ambos | texto libre | — |

Un `fix` nace con la tarea `- [ ] 1. La reproducción ya no falla → re-ejecutar la reproducción`.

## Revisiones

Las dos son `ContentRevision` (SHA-256 abreviado a 12 hexadecimales) de las secciones citadas, sin comentarios ni espacios finales:

- **Definición**: Intención, Reproducción, Alcance, «¿Qué pasa si no lo hacemos?», «Alternativa más barata», Historias y Aclaraciones. Es la que aprueba `approve`.
- **Trabajo**: la definición más Slices y Checklist, con las casillas `[x]` del checklist normalizadas a `[ ]`. Es la que usan la evidencia y el cierre.

Evidencia y Siguiente no forman parte de ninguna revisión, para que el motor pueda reescribirlas sin caducar lo que acaba de registrar.

## Escrituras del motor

- `clarify` añade o resuelve líneas `Qn` en Aclaraciones.
- `verify` marca `[x]` las tareas que pasaron y reescribe Evidencia, con cada salida sangrada para que nunca abra una sección.
- `close` y `cancel` reescriben Siguiente.
- `promote` añade Historias y Slices si faltan.
