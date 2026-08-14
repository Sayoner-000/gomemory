# Quickstart — Context Optimization & Budgeting

**Feature**: [spec.md](./spec.md) · Contratos: [contracts/cli.md](./contracts/cli.md),
[contracts/go-api.md](./contracts/go-api.md), [contracts/mcp-tools.md](./contracts/mcp-tools.md)

Guía de validación manual, no una suite de tests. Cada paso demuestra un criterio de
aceptación de una historia de usuario del spec.

## Prerrequisitos

- Binario compilado con esta feature: `go build -o mem ./infrastructure`.
- Un proyecto gomemory ya inicializado con memorias variadas guardadas (varios tipos:
  `decision`, `pattern`, `preference`, algunas repetidas/parecidas entre sí a propósito
  para poder probar dedup).

```bash
cd /ruta/a/tu/proyecto
mem save -t "Se usa Redis para sesiones" -y decision "El servicio de auth usa Redis para guardar sesiones de refresh token."
mem save -t "Redis para sesiones (nota 2)" -y decision "Las sesiones de refresh token se guardan en Redis."
mem save -t "Preferencia de estilo" -y preference "Prefiero respuestas cortas sin resúmenes finales."
```

## Historia 1 — Paquete de contexto acotado a una tarea y a un presupuesto

```bash
mem pack build --task "Implementar rotación refresh tokens" --max-tokens 500
```

**Resultado esperado**: la salida incluye la memoria sobre Redis/sesiones (relevante a
"refresh tokens"), no incluye la de "Preferencia de estilo" (irrelevante a la tarea), y
el bloque de estadísticas al final muestra `Final tokens <= 500`.

```bash
mem pack build --task "Implementar rotación de refresh tokens" --max-tokens 1
```

**Resultado esperado**: sale con código de salida `1` y un mensaje que nombra
explícitamente el desbordamiento de presupuesto crítico (`ErrCriticalContextOverflow`),
no un paquete vacío ni truncado silenciosamente.

## Historia 2 — Deduplicación

```bash
mem pack build --task "sesiones Redis" --max-tokens 2000
```

**Resultado esperado**: de las dos memorias casi idénticas sobre Redis, el paquete
incluye solo una; el bloque de estadísticas reporta `Duplicados removidos: 1`.

## Historia 3 — Estadísticas de reducción

```bash
mem pack build --task "sesiones Redis" --max-tokens 2000 --json > pack.json
mem pack stats pack.json
```

**Resultado esperado**: `Raw tokens`, `Final tokens`, `Ahorrados` y el desglose por
prioridad se imprimen y son internamente consistentes (`Raw - Ahorrados == Final`).

```bash
printf "## Título\n\nTexto repetido.\n\nTexto repetido.\n" | mem pack compress -
```

**Resultado esperado**: stdout con el texto colapsado; stderr con el conteo de tokens
antes/después.

## Historia 4 — Ingesta de Spec Kit acotada a la feature activa

Corrido desde este mismo repo, con `.specify/feature.json` apuntando a
`specs/015-context-optimization`:

```bash
mem pack build --task "budget de tokens crítico" --max-tokens 3000
```

**Resultado esperado**: el paquete incluye contenido derivado de
`specs/015-context-optimization/spec.md` (p. ej. FR-008, sobre el overflow crítico) y
NO incluye contenido de `specs/013-atomic-plan-mode` ni de ninguna otra feature.

```bash
mem pack build --task "budget de tokens crítico" --max-tokens 3000 --no-speckit
```

**Resultado esperado**: ningún contenido de `specs/` aparece en el paquete.

## Historia 5 — Optimización de descripción de tool MCP

Validación vía test de integración (no hay comando CLI dedicado — ver
contracts/go-api.md, "No se expone por MCP"):

```bash
go test ./tests/integration/... -run TestOptimizeToolDescription -v
```

**Resultado esperado**: el test confirma que `Name`/`Schema` quedan idénticos byte a
byte y que `Description` se acorta, para un `ToolDescriptor` de ejemplo con
descripción verbosa.

## No-regresión (FR-018 / SC-005)

```bash
mem context            # comportamiento existente, get_context()
mem search "Redis"      # comportamiento existente, search_memories
```

**Resultado esperado**: ambos se comportan exactamente igual que antes de esta
feature — ninguno pasa por `BuildContextPack`, ninguno cambia de formato ni de
contenido.
