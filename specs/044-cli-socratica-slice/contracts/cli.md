# Contrato CLI de Socrática

**Feature**: 044-cli-socratica-slice | **Requisito**: FR-001, FR-003, FR-004

La entrada `mem socratica <subcomando> [opciones]` conserva los subcomandos, textos, códigos de salida, interacción TTY/no-TTY, elección/cancelación del harness, protección de control terminal y lectura acotada que presenta antes de la extracción. El paquete raíz sigue siendo el punto de despacho y traduce el código devuelto al comportamiento de proceso vigente.

`expand evolve` conserva su selector, incluido catálogo, opciones no disponibles, exclusiones y cancelación.

# Contrato de herramienta MCP

**Feature**: 044-cli-socratica-slice | **Requisito**: FR-002, FR-006

La herramienta MCP conserva el nombre `socratica_flow`, la descripción, schema, campos JSON, fases, acciones permitidas, errores y resultados actuales. El servidor registra la herramienta una vez desde su composition root. Preparar, aprobar, ack y report no cambian semántica ni autoridad.

# Contrato de dependencias

**Feature**: 044-cli-socratica-slice | **Requisito**: FR-005, FR-007

- `cli/socratica` puede importar `cli/clikit`, `application`, `domain`, `console` y paquetes secundarios requeridos por la entrada.
- `cli/socratica` no importa `cli` padre, `cli/review` ni otros slices.
- `cli/clikit` no importa `cli/socratica`, `cli/review` ni ningún slice.
- `cli` raíz mantiene el despacho/wiring y puede importar cada slice.
