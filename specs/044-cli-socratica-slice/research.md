# Research: Entradas CLI y MCP de Socrática como slice

**Feature**: 044-cli-socratica-slice | **Date**: 2026-10-26

## R1. Paquete compartido

- **Decision**: Reutilizar `adapters/primary/cli/clikit` creado por el piloto Review.
- **Rationale**: El slice nuevo no puede importar `cli` padre ni `cli/review`; `clikit` contiene las dependencias y utilidades compartidas sin importar slices.
- **Alternatives considered**: Hacer que Socrática importe `cli` para reutilizar funciones; rechazado porque forma ciclos/acoplamiento y contradice `layer_rules_test.go`.

## R2. Selector de harness

- **Decision**: Mover el selector a `clikit` como función que recibe opciones/catálogo, UI y escritores, sin depender de `SocraticaFlow`.
- **Rationale**: `cmd_expand.go:329` construye actualmente `socraticaCmd` solo para llamar `chooseHarness`; al aceptar entradas en `clikit`, el uso de `expand evolve` queda desacoplado del slice.
- **Compatibility**: Mantener orden y etiquetas del catálogo, motivos de unavailable, exclusiones, texto del selector y errores `ErrNoGenerator`/`ErrCanceled`.

## R3. Lectura acotada

- **Decision**: Mover implementación y error centinela de `readBounded` a `clikit`; retener alias/envoltorio en el paquete CLI para los usos existentes de hooks y sus pruebas.
- **Rationale**: `cmd_hook.go` es propietario accidental de una utilidad usada por Socrática; moverla evita importar la CLI desde el slice.
- **Compatibility**: Conservar límite inclusivo, detección de `limit+1`, texto de error y rechazo sin truncamiento.

## R4. Integración CLI/MCP

- **Decision**: Exportar `socratica.Run(*clikit.Deps, []string) int` y `socratica.RegisterTools(*mcp.Server, *clikit.Deps, string)`. Mantener wrappers en el paquete raíz para despacho local y códigos de salida; conservar las pruebas de servidor MCP en `cli` como integración.
- **Rationale**: Sigue el patrón `review.Cmd`/`review.RegisterTools`, conserva el composition root y no expone tipos internos.

## R5. Pruebas

- **Decision**: Mover las pruebas internas que solo ejercen Socrática; mantener tests de registro y flujo MCP integrado donde crean `Deps`/servidor central; añadir pruebas unitarias del selector y compatibilidad de wrappers cuando no estén cubiertos.
- **Rationale**: Separa propiedad del slice y mantiene comprobación real de la integración.
