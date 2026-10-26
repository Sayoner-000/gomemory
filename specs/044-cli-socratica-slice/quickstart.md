# Quickstart: Socrática como slice

**Feature**: 044-cli-socratica-slice

## Pruebas focalizadas

```sh
go test ./adapters/primary/cli/clikit ./adapters/primary/cli/socratica ./adapters/primary/cli ./adapters/primary/cli/review
go test ./tests/contract -run TestLayerRules
```

Verificar en particular selección/cancelación de harness (incluido `expand evolve`), stdin por encima y en el límite, salidas/códigos de Socrática, registro MCP único, y flujos `specify` desde preparación hasta report.

## Validación del repositorio

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

Para paridad CLI, compilar la revisión anterior y la actual en rutas/binarios separados, ejecutar entradas representativas con almacenamiento temporal independiente, y comparar stdout, stderr y exit code después de normalizar IDs y timestamps no deterministas.
