# Tasks: Entradas CLI y MCP de Socrática como slice

**Input**: Documentos de diseño en `specs/044-cli-socratica-slice/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `contracts/`, `quickstart.md`

## Phase 1: Línea base y utilidades compartidas

**Purpose**: Capturar comportamiento existente y preparar las dependencias compartidas sin acoplar slices.

- [X] T001 [P] Ejecutar pruebas existentes de CLI/MCP Socrática y guardar resultados de referencia en `specs/044-cli-socratica-slice/evidence/baseline.txt`.
- [X] T002 Añadir pruebas unitarias para selector (orden, exclusión, no disponible, cancelación y catálogo vacío) en `adapters/primary/cli/clikit/socratica_test.go`.
- [X] T003 [P] Añadir pruebas para lectura límite, exceso sin truncamiento y error de lector en `adapters/primary/cli/clikit/read_bounded_test.go`.
- [X] T004 Implementar selector de harness genérico y lectura acotada en `adapters/primary/cli/clikit/socratica.go` y `adapters/primary/cli/clikit/read_bounded.go`; conservar alias/wrappers raíz requeridos por hooks en `adapters/primary/cli/cmd_hook.go`.
- [X] T005 Adaptar `expand evolve` al selector de `clikit` en `adapters/primary/cli/cmd_expand.go`; cubrir opciones/cancelación en `clikit/socratica_test.go` y verificar el flujo desde `cmd_expand_evolve_test.go`.

## Phase 2: User Story 1 - Entrada CLI (Priority: P1)

**Goal**: `mem socratica` mantiene sus salidas, errores e interacciones con implementación dentro del slice.

**Independent Test**: Ejecutar pruebas trasladadas de Socrática en el nuevo paquete y comparar escenarios representativos con la línea base.

- [X] T006 Mover la implementación de CLI a `adapters/primary/cli/socratica/cmd_socratica.go`, usando `clikit.Deps` y utilidades compartidas sin importar el paquete padre.
- [X] T007 Exponer `socratica.Run` y conservar el wrapper `CmdSocratica` con códigos de salida en `adapters/primary/cli/dispatcher.go` y `adapters/primary/cli/cmd_socratica.go`.
- [X] T008 Mover pruebas unitarias que solo ejercen Socrática al paquete `adapters/primary/cli/socratica/`, incluyendo pruebas de stdin, terminal, harness e historias `specify`.
- [X] T009 Conservar pruebas de dispatch/compatibilidad y demostrar que hooks siguen usando lectura acotada en `adapters/primary/cli/`.

## Phase 3: User Story 2 - Entrada MCP (Priority: P2)

**Goal**: el servidor registra `socratica_flow` desde el slice y conserva el contrato integrado.

**Independent Test**: Ejecutar pruebas de integración MCP existentes y verificar herramienta única y escenarios `specify`.

- [X] T010 Mover registro, DTOs, conversiones y helpers de la herramienta a `adapters/primary/cli/socratica/mcp.go`.
- [X] T011 Exponer `socratica.RegisterTools` y cambiar el composition root MCP para delegar el registro una sola vez en `adapters/primary/cli/cmd_mcp.go`.
- [X] T012 Mantener pruebas MCP que ejercen el servidor central y verificar schema/respuestas de `socratica_flow` en `adapters/primary/cli/cmd_mcp_socratica_tool_test.go` y sus pruebas relacionadas.

## Phase 4: Polish y validación transversal

- [X] T013 Ejecutar `go test` de `clikit`, `socratica`, `cli` y `review`; corregir cualquier regresión de paquete o integración.
- [X] T014 Ejecutar `go test ./tests/contract -run TestLayerRules` y demostrar que el slice no importa padre/otros slices ni que `clikit` importa slices.
- [X] T015 Comparar stdout, stderr y códigos de salida en escenarios CLI de referencia; guardar evidencia en `specs/044-cli-socratica-slice/evidence/parity.txt`.
- [X] T016 Ejecutar `go test ./...`, `go test -race ./...`, `go vet ./...` y `go build ./...`; registrar resultados en `specs/044-cli-socratica-slice/evidence/validation.txt`.
- [X] T017 Actualizar `docs/architecture.md` con el paquete `cli/socratica` y su registro MCP, documentando el patrón del siguiente slice.

## Dependencies

- Setup compartido T001–T005 precede a la extracción CLI.
- User Story 1 (T006–T009) precede a User Story 2 (T010–T012), porque la extracción CLI fija la API pública del paquete Socrática.
- Polish (T013–T017) depende de ambas historias completas.

## Parallel opportunities

- T002 y T003 pueden prepararse en paralelo; T001 no modifica código.
- Dentro de validación, T014 y la comparación CLI T015 pueden ejecutarse en paralelo una vez finalizadas las historias.

## Implementation strategy

Completar el slice CLI como primer incremento funcional, después extraer el registro MCP y finalmente cerrar con validación integral. Mantener tests de integración en el paquete padre cuando necesiten el servidor o composition root real.
