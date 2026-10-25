# Research: Evaluación de la base Go para vertical slices

**Feature**: 043-vertical-slices-assessment | **Date**: 2026-10-25

El Technical Context no dejó ningún `NEEDS CLARIFICATION`. Esta fase fija las decisiones de método que la spec dejó abiertas, sobre todo el pendiente P1, y recoge comprobaciones previas que orientan la evaluación sin adelantar su conclusión.

## R1 — Fuente de verdad para las dependencias entre capas

- **Decision**: el grafo de importaciones se obtiene de `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`, que incluye solo producción (para separar pruebas se usan `.TestImports` y `.XTestImports`), filtrado al módulo `mem/`. CodeGraph y codebase-memory solo sirven para localizar símbolos y recorrer llamadas, nunca como prueba de una dependencia.
- **Rationale**: `go list` es lo que compila el toolchain: no tiene desfase de índice y es reproducible por cualquiera (FR-003, FR-011). Las importaciones de prueba se separan porque un paquete `_test` puede cruzar capas legítimamente sin que eso sea acoplamiento de producción.
- **Alternatives considered**: (a) solo CodeGraph: es rápido, pero el índice puede ir por detrás de las escrituras y no es evidencia reproducible fuera de la sesión; (b) `grep` de líneas `import`: falla con bloques de importación, alias y etiquetas de compilación (`_unix.go`, `_windows.go`).

## R2 — Slices de muestra (resuelve el pendiente P1)

- **Decision**: se recorren tres slices, cada uno desde un punto de entrada distinto, más una comprobación transversal:

  | Slice | Punto de entrada | Capas que recorre (rutas verificadas) | Por qué es representativo |
  |---|---|---|---|
  | **Socratica** | CLI `mem socratica` (`adapters/primary/cli/cmd_socratica.go`) y TUI (`adapters/primary/tui/socratica.go`); también MCP (`cmd_mcp_socratica_tool.go`) | `application/usecases/socratica_flow.go`, `socratica_specify.go` → `application/ports/socratica_repository.go`, `socratica_unit_of_work.go` → `adapters/secondary/persistence/socratica*.go` | Es el slice más reciente y grande, con tres entradas: muestra si una funcionalidad con varias entradas mantiene la lógica en un solo sitio |
  | **Review consensuado** | MCP (`adapters/primary/cli/cmd_mcp_review_tools.go`) y CLI (`cmd_review.go`, `cmd_review_query.go`) | `application/usecases/start_review.go`, `submit_reviewer_result.go`, `build_consensus.go`, `rejudge_review.go`, `finalize_review.go`, `promote_review_memory.go` → `ports/review_repository.go`, `consensus_repository.go` → `persistence/review.go` | Tiene varios casos de uso cooperando sobre un agregado: prueba los límites entre casos de uso del mismo slice |
  | **Instincts** | Hook (`adapters/primary/cli/cmd_hook.go`, `hook_expand.go`) | `application/usecases/instincts.go` → `ports/instinct_repository.go` → `persistence/instincts.go` (base de datos global separada) | La entrada por hook tiene restricciones propias (presupuesto, sin TTY); además usa un segundo almacén |
  | *Transversal*: composition root | `infrastructure/main.go`, `infrastructure/container.go` | Construcción del grafo de los tres slices | Criterio M3 (§4 de la constitución) |

- **Rationale**: cubre los cuatro tipos de entrada que exige C1 (CLI, MCP, TUI y hook) con solo tres slices; incluye el más grande y el más reciente; mezcla slices con un caso de uso y con varios. El tamaño de `adapters/primary/cli` (20.154 líneas frente a 7.931 de `usecases`) justifica revisar en particular si hay lógica de aplicación en los adaptadores primarios. Esto queda como **hipótesis a comprobar**, no como hallazgo.
- **Alternatives considered**: (a) contexto/memorias (`save_memory`, `get_context`): es el núcleo más antiguo, pero su flujo pasa por las mismas capas que Review sin aportar un tipo de entrada nuevo; queda como reserva si un slice de la tabla resulta trivial; (b) Octopus: es pequeño y casi solo de cálculo, aporta poco sobre límites de persistencia.

## R3 — Cómo se distingue un hallazgo de una preferencia

- **Decision**: una observación es **hallazgo** solo si cumple las tres condiciones: (1) incumple un criterio declarado en [contracts/evaluation-criteria.md](contracts/evaluation-criteria.md); (2) tiene evidencia reproducible; (3) tiene una consecuencia demostrable para desarrollar por slices (un cambio en un slice obliga a tocar otro, un caso de uso no se puede probar sin un adaptador real, la lógica se duplica entre entradas, etc.). Si falta la condición 3, es **preferencia**.
- **Rationale**: sin la condición 3, cualquier desviación estética de una arquitectura de libro se volvería defecto, que es justo lo que la spec pide evitar (FR-004).
- **Alternatives considered**: clasificar por severidad, sin distinguir preferencias. Se descarta porque mezcla dos preguntas distintas: si es un problema y cuán grave es.

## R4 — Ejemplo ya observado de fuga de infraestructura (calibración)

- **Decision**: usar como ejemplo de calibración el hallazgo C-001, corregido hoy en esta misma sesión (sin commit): `SocraticaStore.GetFlow` devolvía `sql.ErrNoRows` sin traducir, y la CLI lo mostraba al usuario.
- **Rationale**: incumple directamente la regla de §3 ("para ausencia esperada… no se lanza una excepción de infraestructura") y tenía una consecuencia visible, así que sirve de referencia de qué es un hallazgo L-tipo. La evaluación debe buscar **otros** `GetX` del mismo patrón en `persistence` en lugar de dar por resuelta la categoría.
- **Alternatives considered**: no citarlo, por ser ya conocido. Se descarta porque es la mejor muestra de que el patrón existió y puede repetirse.

## R5 — ¿Hay reglas de paquetes en CI? (comprobación previa)

- **Decision**: la evaluación debe verificar si existe una comprobación automática de las reglas de dependencia entre capas, que la constitución exige en §3 ("reglas de paquetes en Go" en CI).
- **Rationale / evidencia preliminar**: `.github/workflows/test.yml` ejecuta `go vet ./...`, `go test ./...` y `go test -race` sobre usecases, persistence y cli (líneas 36, 39 y 49), pero a primera vista no hay un paso específico de reglas de importación ni configuración de linter (no existe `.golangci*`). **No verificado** si alguna prueba de `go test` cumple ese papel: queda como pregunta de la evaluación, no como hallazgo.
- **Alternatives considered**: darlo por hallazgo ya. Se descarta: violaría FR-011.

## R6 — Dónde va la salida para cumplir el modo de solo lectura

- **Decision**: la línea base (`git status --porcelain`, `git stash list` y `git rev-parse HEAD`) y toda salida intermedia (grafo de importaciones, logs de pruebas) se guardan en el scratchpad de la sesión. El informe final se entrega en la conversación o en un archivo del scratchpad.
- **Rationale**: FR-010 y SC-007. El estado de partida **no está limpio**: hay cambios sin commit de C-001 y de `specs/043`. Por eso la comparación debe hacerse contra la línea base registrada, no contra "árbol limpio".
- **Alternatives considered**: escribir el informe en `docs/` o en `specs/043/`. Se descarta porque rompe C6.
