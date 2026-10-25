# Contrato: criterios de evaluación (declarados antes de aplicarse)

**Feature**: 043-vertical-slices-assessment | **Requisitos**: FR-006, FR-012 | **Criterio preparado**: C4

Estos criterios se fijan **antes** de mirar los resultados y se aplican a los tres slices de [research.md](../research.md) R2. Cambiar un criterio durante la evaluación exige anotarlo en el informe junto con su motivo.

## Capas (según la constitución, §3)

| ID | Pregunta | Cómo se comprueba | Cumple si |
|---|---|---|---|
| **L1** | ¿`domain` depende solo de la librería estándar y de sí mismo? | `go list` de `mem/domain`: importaciones de producción dentro de `mem/` | No importa ningún otro paquete de `mem/` ni drivers como `database/sql` |
| **L2** | ¿`application` depende solo de `domain` y de sus ports? | `go list` de `application/...` | No importa `adapters/…` ni `infrastructure` |
| **L3** | ¿Los adapters dependen hacia dentro (de `application`/`domain`) y no entre sí sin un motivo? | `go list` de `adapters/...` | No hay ciclos; un adapter primario no importa uno secundario salvo que haya un motivo documentado |
| **L4** | ¿Los ports ocultan los tipos de infraestructura y la ausencia esperada? | Firmas en `application/ports/*.go` y errores devueltos por las implementaciones de los slices | Ninguna firma expone `*sql.DB`, `*sql.Tx`, `sql.Row` ni tipos de driver; la ausencia se expresa con un bool, `nil` o un error de `domain` (calibración: research R4) |

## Módulos y slices

| ID | Pregunta | Cómo se comprueba | Cumple si |
|---|---|---|---|
| **M1** | ¿La lógica de negocio del slice vive en `domain`/`usecases` y no en los adaptadores primarios? | Recorrido del flujo: decisiones de negocio (validaciones, transiciones, reglas) encontradas en `adapters/primary` | Los adaptadores primarios solo traducen la entrada y la salida; no hay reglas duplicadas entre CLI, MCP y TUI |
| **M2** | ¿Un cambio dentro de un slice queda contenido en él? | Para el slice, contar los paquetes y archivos ajenos que habría que tocar en un cambio típico (añadir un campo o una acción) | El cambio no obliga a tocar archivos de otro slice, salvo el registro del punto de entrada y el composition root |
| **M3** | ¿El wiring está centralizado? | `infrastructure/container.go` frente a construcciones de adaptadores fuera de él | Los adaptadores del slice se construyen en el composition root; no hay service locator ni selección dispersa |
| **M4** | ¿Los puntos de entrada tienen un tamaño y una responsabilidad acotados? | Tamaño y número de responsabilidades por archivo de entrada del slice | Un archivo de entrada no mezcla varios slices ni contiene flujos completos de aplicación |

## Pruebas

| ID | Pregunta | Cómo se comprueba | Cumple si |
|---|---|---|---|
| **T1** | ¿Los casos de uso del slice se prueban sin infraestructura real? | Pruebas de `application/usecases` del slice: dobles de los ports frente a SQLite o procesos reales | Hay pruebas con dobles de los ports |
| **T2** | ¿Las pruebas del slice se pueden ejecutar de forma aislada? | `go test ./application/usecases -run '<patrón del slice>'` y equivalentes en los adapters | Pasan aisladas, sin depender de otras pruebas ni del entorno del usuario |
| **T3** | ¿Hay comprobación automática de las reglas de capas? | CI (`.github/workflows/test.yml`) y pruebas que inspeccionen importaciones (research R5) | Existe y falla ante una importación prohibida |

## Uso en la conclusión

- Una celda "no cumple" solo genera un hallazgo si además cumple la condición 3 de research R3 (consecuencia para los slices).
- La conclusión pondera el número y la gravedad de los hallazgos **verificados** frente a las prácticas a conservar:
  - **mantener**: no hay hallazgos de riesgo alto ni medio, o los que hay no tienen una consecuencia práctica actual;
  - **cambios-puntuales**: los hallazgos se concentran en pocos puntos, pueden cerrarse sin mover límites de paquetes y cada uno tiene un cierre local;
  - **refactorizar**: los hallazgos se repiten en varios slices y su cierre exige cambiar límites de paquetes o la forma de organizar el código.
