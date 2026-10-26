# Implementation Plan: Entradas CLI y MCP de Socrática como slice

**Branch**: `044-cli-socratica-slice` | **Date**: 2026-10-26 | **Spec**: [spec.md](spec.md)

## Summary

Completar H-04 extrayendo las entradas CLI y MCP de Socrática a `adapters/primary/cli/socratica`, siguiendo el piloto `cli/review`. Reutilizar `cli/clikit` para dependencias, registro de herramientas y utilidades compartidas; conservar contratos de CLI/MCP, lógica de aplicación y dominio. Extraer también a `clikit` el selector de harness requerido por `expand evolve` y la lectura acotada actualmente compartida con hooks.

## Technical Context

**Language/Version**: Go 1.24 (versión declarada en `go.mod`)  
**Primary Dependencies**: biblioteca estándar; `go-sdk/mcp`; `application/usecases`, `application/ports`, `domain`; `cli/clikit`  
**Storage**: Sin cambios; repositorios y adaptadores actuales  
**Testing**: `go test`, `go test -race`, `go vet`, `go build`; pruebas de contrato en `tests/contract`  
**Target Platform**: Plataformas soportadas por el repositorio  
**Project Type**: CLI y servidor MCP en arquitectura hexagonal  
**Performance Goals**: Sin cambios de rendimiento observable  
**Constraints**: Sin cambios funcionales, de protocolo o persistencia; slices no importan el paquete padre ni entre sí; `clikit` no importa slices  
**Scale/Scope**: Una entrada primaria con adaptadores CLI y MCP, más dos utilidades compartidas y un consumidor existente (`expand evolve`)

## Constitution Check

- Arquitectura hexagonal: PASS; dominio, casos de uso, ports y persistencia no se mueven.
- Composition root: PASS; `dispatcher.go` y el registro MCP central siguen cableando las entradas.
- Regla de dependencias entre slices: PASS; se mantiene `clikit` como único punto compartido y se valida con `tests/contract/layer_rules_test.go`.
- Compatibilidad: PASS; los wrappers de CLI preservan el despacho y los códigos de salida; el registro MCP conserva nombre y esquema.

## Project Structure

```text
adapters/primary/cli/
├── clikit/                 # Deps, salida segura, MCP, lectura acotada, selector de harness
├── socratica/              # entrada CLI, registro/herramienta MCP y pruebas internas
├── review/                 # slice piloto existente
├── cmd_socratica.go        # wrapper estable del dispatcher
├── cmd_mcp.go              # composition/registro MCP central
├── cmd_expand.go           # consumidor del selector compartido
└── cmd_hook.go             # consumidor de lectura acotada compartida
tests/contract/layer_rules_test.go
```

**Structure Decision**: El paquete `cli/socratica` es dueño del comportamiento de sus adaptadores de entrada. Pruebas de integración que necesitan el servidor completo permanecen en el paquete `cli`; pruebas de comportamiento privado que solo ejecutan Socrática se mueven junto al slice. APIs exportadas: `socratica.Run(*clikit.Deps, []string) int` y `socratica.RegisterTools(*mcp.Server, *clikit.Deps, string)`. El paquete raíz conserva `CmdSocratica` como wrapper y el dispatcher MCP invoca `RegisterTools`.

## Research

Decisiones verificadas en el código actual y el piloto `701e45e`:

1. El paquete padre importa `cli/socratica` para despachar; el nuevo paquete no puede importar el padre por la guarda de capas.
2. `expand evolve` construye `socraticaCmd` para seleccionar un harness. Mover selección a `clikit` evita que expand dependa de Socrática.
3. `readBounded` está definido dentro de `cmd_hook.go` y lo usa Socrática y hooks. Mover implementación/error a `clikit`, conservando alias o wrapper en `cli` para callers actuales.
4. La entrada MCP se registra en `cmd_mcp.go`; las pruebas MCP existentes crean el servidor desde `Deps`, por lo que se mantienen como pruebas de integración y cubren el dispatcher real.
5. Las reglas de capa ya verifican que un slice no importe `cli`, otro slice, ni que `clikit` importe un slice.

No se cambian contratos ni se añaden dependencias externas.

## Validation Strategy

Probar primero el paquete compartido, el slice y los consumidores (`expand`, hooks, Review y MCP); ejecutar la guarda de capas. Comparar binarios pre/post con escenarios estables de CLI, normalizando IDs/tiempos aleatorios. Después ejecutar `go test ./...`, `go test -race ./...`, `go vet ./...` y `go build ./...`. No se requieren cambios de despliegue ni migración.
