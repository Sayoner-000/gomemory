# Contrato CLI (delta): `mem pack build --no-code-graph`

**Feature**: [../spec.md](../spec.md) ·
**Base**: [../../015-context-optimization/contracts/cli.md](../../015-context-optimization/contracts/cli.md)

## `mem pack build` (flag nuevo)

```text
mem pack build --task "<descripción de la tarea>" --max-tokens <N> [flags]

Flags nuevos de esta feature:
  --no-code-graph          Opcional. Desactiva IncludeCodeGraph para esta llamada.
                            Default: activado (FR-007) — mismo criterio que --no-speckit.
```

**Sin proveedor de grafo de código configurado**: `--no-code-graph` es un no-op explícito
(mismo resultado con o sin el flag) — no es un error pasarlo aunque no haya nada que
desactivar.

**Salida**: sin cambios de formato. Un ítem de arquitectura de código, cuando aparece, se
renderiza igual que cualquier otro `ContextItem` (`formatContextPack`,
`cmd_pack.go:201`): encabezado `## <Source>` seguido del contenido — en este caso
`Source` es el nombre del proveedor (p. ej. `## codebase-memory-mcp`).

**Wiring** (`cmdPackBuild`, `cmd_pack.go:85`): agrega `req.CodeProviders =
deps.CodeProviders` antes de invocar `usecases.BuildContextPack` — `deps.CodeProviders`
ya existe en `Deps` (`adapters/primary/cli/deps.go:25`) y ya se construye en
`infrastructure/container.go` (`buildCodeProviders`); cero cambios en el composition root.
