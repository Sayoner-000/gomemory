# Fase 1 — Modelo de Datos (delta): Señal de grafo de código en Retrieval de ContextPack

**Feature**: [spec.md](./spec.md) · **Investigación**: [research.md](./research.md) ·
**Base**: [../015-context-optimization/data-model.md](../015-context-optimization/data-model.md)

Este documento describe solo el delta sobre el modelo de datos ya establecido por la
feature 015. `Priority`, `ContextItem`, `ContextPack`, `ContextStats` y
`SpecKitFeatureContext` no cambian — ver el documento base para su definición completa.

## ContextRequest (campos nuevos)

| Campo | Tipo | Obligatorio | Notas |
|---|---|---|---|
| `IncludeCodeGraph` | `bool` | no | Default `true` en el CLI (flag `--no-code-graph` invierte); ver research.md §4-§5 para la asimetría con el default de la tool MCP `pack_build`. |
| `CodeProviders` | `[]ports.CodeGraphProvider` | no | Lista de proveedores ya construida por el composition root (`infrastructure/container.go`); vacía o `nil` si no hay ninguno configurado — cero impacto en ese caso (FR-002). |

**Validación**: sin cambios sobre la validación ya existente (`Task`/`Project`/`MaxTokens`)
— `CodeProviders` nunca es parte de la validación de borde: una lista vacía es un estado
válido, no un error (mismo criterio que `specKit == nil`).

## contextCandidate (sin cambios de forma, dos nuevas fuentes de valores)

El tipo interno `contextCandidate` (`build_context_pack.go:198`) no gana ni pierde campos.
Lo que cambia es **quién** puede producir uno y **quién** puede ajustar su `priority`
después de creado:

- **Nueva fuente**: `codeGraphArchitectureCandidate` produce, como máximo, UN
  `contextCandidate` con `id: "codegraph:architecture"`, `source: <nombre del proveedor>`,
  `priority: PriorityOptional`, `importance: 0.4`, `relevance: 1`, `confidence: 1`,
  `content: formatCodeArchitecture(snap)`.
- **Nuevo ajustador**: `boostHotspotCandidates` recorre `items` (memorias + Spec Kit ya
  construidos) y, para cada uno con `source` no vacío que coincida con un hotspot vigente
  según algún `CodeGraphProvider.ImpactFor(source)`, sube su `priority` de
  `PriorityOptional` a `PriorityRelevant` — nunca toca `PriorityCritical`, nunca la baja
  (research.md §6).

## Relaciones (delta sobre el diagrama de la feature 015)

```text
ContextRequest ──(1 llamada)──> BuildContextPack (caso de uso)
                                       │
                                       ├─ retrieve  → []domain.Memory   (MemoryRepository.Search)
                                       ├─ dedup     → DetectDuplicateGroups
                                       ├─ classify  → Priority por item (por MemoryType)
                                       ├─ speckit   → specKitCandidates (si IncludeSpecKit)
                                       ├─ codegraph → boostHotspotCandidates (si IncludeCodeGraph, nuevo)
                                       │              + codeGraphArchitectureCandidate (si hay snapshot, nuevo)
                                       ├─ compress  → Compressor (puerto)
                                       ├─ count     → TokenCounter (puerto)
                                       └─ budget    → asigna Critical → Relevant → Optional
                                                          │
                                                          ▼
                                                    ContextPack { Items, Stats }
```

El paso `codegraph` se inserta **después** de tener todos los candidatos de memorias +
Spec Kit ya construidos (necesita sus `source`/`priority` ya resueltos para el boost) y
**antes** del cálculo de `criticalSum` — mismo punto donde hoy termina el bloque de
ensamblado de `items` en `build_context_pack.go`.

## Sin entidades de dominio nuevas

`domain.CodeProviderSnapshot`, `domain.CodeArchitecture`, `domain.CodeImpactAnnotation`
(todas en `domain/code_provider.go`, feature 010) se **consumen**, no se modifican. Esta
feature no agrega ningún tipo a `domain/` — coherente con Principio I (el dominio ya tenía
todo lo necesario; solo faltaba conectarlo al caso de uso de `ContextPack`).
