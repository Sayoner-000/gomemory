# Implementation Plan: Señal de grafo de código en Retrieval de ContextPack

**Branch**: `018-codegraph-pack-retrieval` | **Date**: 2026-08-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/018-codegraph-pack-retrieval/spec.md`

## Resumen

`mem pack build` (el pipeline Retrieval → Dedup → Priority → Compression → Token Budget →
ContextPack de la feature 015) hoy no conoce el grafo de código externo — sus candidatos
vienen solo de `MemoryRepository.Search` y, opcionalmente, Spec Kit. `mem context`
(feature 010/Track A) sí lo usa, vía `ports.CodeGraphProvider`: una sección informativa de
arquitectura y un boost de relevancia por `ImpactFor(filepath)` sobre memorias hotspot.

Este plan lleva ese mismo contrato ya probado (snapshot cacheado, no-bloqueante,
degradación silenciosa) a `BuildContextPack`, sin inventar mecanismo nuevo: reusa
`ports.CodeGraphProvider`, `usecases.FirstAvailable` y el formato que ya produce
`writeCodeProviderSection`. Cero dependencias nuevas, cero cambio de esquema de datos,
cero cambio de comportamiento para quien no tiene un proveedor configurado (FR-009).

## Technical Context

**Language/Version**: Go >=1.22 (stack congelado del proyecto, sin cambios)

**Primary Dependencies**: Ninguna nueva. Reusa `application/ports/code_graph_provider.go`
(`CodeGraphProvider`), `application/usecases/provider_selection.go` (`FirstAvailable`) y
`domain/code_provider.go` (`CodeProviderSnapshot`, `CodeArchitecture`,
`CodeImpactAnnotation`) — todos ya existentes desde la feature 010.

**Storage**: N/A. `ContextPack` sigue sin persistirse (research.md de la feature 015,
§6); el snapshot de grafo de código ya se persiste en `.memory/` por la feature 010, sin
cambios aquí.

**Testing**: `testing` stdlib + `testify`, TDD estricto (Constitución, Principio III):
tests unitarios en `application/usecases/build_context_pack_test.go` (fakes), un caso de
integración en `tests/integration/`, y un caso de contrato CLI en `tests/contract/`.

**Target Platform**: Mismo binario CLI/MCP multiplataforma (Linux/macOS/Windows) ya
existente — sin superficie nueva de despliegue.

**Project Type**: CLI + servidor MCP (subcomando `mem pack build` y tool `pack_build`),
dentro del monolito hexagonal ya existente. No aplica ninguna de las opciones
web/mobile del template.

**Performance Goals**: Cero llamadas nuevas al proceso/proveedor externo durante la
construcción del `ContextPack` (SC-002, <50ms de sobrecarga atribuible) — se cumple por
construcción, ya que `Snapshot()` e `ImpactFor()` son lecturas de memoria/disco local, no
I/O de red ni subprocess.

**Constraints**: Contrato de no-bloqueo ya documentado en `code_graph_provider.go`
(`Snapshot()` nunca invoca al proveedor externo); degradación silenciosa total (FR-002);
la señal del grafo solo puede subir prioridad, nunca bajarla ni descartar nada (FR-004);
compatibilidad hacia atrás total para callers sin proveedor configurado (FR-009).

**Scale/Scope**: 1 archivo de dominio de casos de uso modificado
(`build_context_pack.go`) + una extracción de función reusable en `build_context.go` + 2
call sites (`cmd_pack.go`, `cmd_mcp.go`) + tests + una nota breve en 2 archivos de docs.
Sin cambios en `infrastructure/container.go` (`deps.CodeProviders` ya existe y ya se
construye ahí).

## Constitution Check

*GATE: Debe pasar antes de la Fase 0. Re-chequeado tras el diseño de Fase 1.*

| Principio | Evaluación |
|---|---|
| I. Arquitectura Hexagonal | **PASS**. Todo el cambio de lógica vive en `application/usecases/` y usa exclusivamente el puerto ya existente `ports.CodeGraphProvider` (interfaz, no implementación). Ningún import de `adapters/` entra a `application/` ni a `domain/`. |
| II. SQLite con SQL Directo | **N/A**. No toca persistencia — el snapshot ya se persiste desde la feature 010, sin cambios de esquema aquí. |
| III. Testing First (NO NEGOCIABLE) | **PASS obligatorio**. Fase de tasks debe escribir los tests de `build_context_pack_test.go`/integración/contrato ANTES de tocar `build_context_pack.go` — mismo ciclo Red-Green-Refactor que ya siguió la feature 015. Tests existentes de `build_context_pack.go` y `build_context.go` no se modifican (Prohibición explícita) salvo la extracción interna de `formatCodeArchitecture`, que no cambia comportamiento observable de `writeCodeProviderSection` (mismo output, verificable por los tests ya existentes `TestBuild_HotCodeSection_*`). |
| IV. Configuración y Entorno | **N/A**. `--no-code-graph` es un flag CLI por invocación, no una variable de entorno; no hay config nueva que cambie entre entornos. |
| V. Principios Operativos | **PASS**. Simplicidad (reusa `FirstAvailable`/`ImpactFor`/`writeCodeProviderSection`, no reinventa nada — ver research.md §1); Sin parches temporales (cierra la brecha real entre los dos pipelines en vez de un bypass); Documentar decisiones (spec + plan + research + memoria de gomemory ya guardada); Fallar rápido (la validación de `ContextRequest` en el borde no cambia); Fire-and-forget (mismo contrato de `CodeGraphProvider` ya fire-and-forget); Idempotencia (lectura pura de snapshot, sin efectos secundarios); MCP como integración primaria (el wiring en `cmd_mcp.go` es parte obligatoria del alcance, no un añadido posterior). |

**Resultado**: Sin violaciones. Tabla de Complexity Tracking vacía — no aplica.

## Project Structure

### Documentation (this feature)

```text
specs/018-codegraph-pack-retrieval/
├── plan.md              # Este archivo
├── research.md          # Fase 0
├── data-model.md         # Fase 1 (delta sobre data-model.md de la feature 015)
├── quickstart.md         # Fase 1
├── contracts/
│   ├── go-api.md          # Delta sobre contracts/go-api.md de la feature 015
│   ├── cli.md              # Delta sobre contracts/cli.md de la feature 015
│   └── mcp-tools.md        # Delta sobre contracts/mcp-tools.md de la feature 015
└── tasks.md               # Fase 2 (/speckit-tasks, no este comando)
```

### Source Code (repository root)

```text
application/
├── ports/
│   └── code_graph_provider.go        # Sin cambios — puerto ya existente (feature 010)
└── usecases/
    ├── build_context.go               # Extraer formatCodeArchitecture(snap) de writeCodeProviderSection
    ├── build_context_pack.go          # ContextRequest + codeGraphArchitectureCandidate + boostHotspotCandidates
    └── build_context_pack_test.go     # Tests nuevos (TDD, antes de tocar build_context_pack.go)

adapters/primary/cli/
├── cmd_pack.go                        # Flag --no-code-graph + wiring deps.CodeProviders
└── cmd_mcp.go                         # pack_build: param no_code_graph + wiring deps.CodeProviders

infrastructure/
└── container.go                       # Sin cambios — deps.CodeProviders ya se construye aquí

tests/
├── integration/
│   └── build_context_pack_codegraph_test.go   # Caso de punta a punta nuevo
└── contract/
    └── pack_build_cli_test.go                  # Caso nuevo para --no-code-graph

docs/
├── MANUAL.md            # Nota breve en la sección de mem pack
└── architecture.md      # Nota breve sobre el segundo consumidor de CodeGraphProvider
```

**Structure Decision**: Proyecto único (monolito hexagonal ya existente) — no aplica
ninguna de las opciones web/mobile del template. Todos los archivos tocados ya existen
salvo los dos archivos de test nuevos; no se crea ningún paquete ni adaptador nuevo.
