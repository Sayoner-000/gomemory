# Contrato API Go (delta): grafo de código en `BuildContextPack`

**Feature**: [../spec.md](../spec.md) · Modelo: [../data-model.md](../data-model.md) ·
**Base**: [../../015-context-optimization/contracts/go-api.md](../../015-context-optimization/contracts/go-api.md)

## `ContextRequest` (campos agregados)

```go
type ContextRequest struct {
    Task           string
    Project        string
    Namespace      string
    MaxTokens      int
    MinRelevance   float32
    MaxItems       int
    IncludeSpecKit bool
    Compression    ports.CompressionLevel
    Root           string

    // Nuevos (esta feature):
    IncludeCodeGraph bool
    CodeProviders    []ports.CodeGraphProvider
}
```

## `BuildContextPack` (firma sin cambios)

```go
func BuildContextPack(
    memRepo ports.MemoryRepository,
    compressor ports.Compressor,
    counter ports.TokenCounter,
    specKit ports.SpecKitReader,
    req ContextRequest, // ahora también trae IncludeCodeGraph + CodeProviders
) (domain.ContextPack, error)
```

Se decidió **no** agregar un parámetro posicional nuevo (p. ej.
`codeGraphProviders []ports.CodeGraphProvider` como quinto argumento): los proveedores ya
viajan dentro de `req`, igual que `Root` ya viaja ahí para Spec Kit en vez de como
parámetro aparte — mismo criterio de la feature 015 (research.md, feature 015, §sobre
`Root` vs `Project`).

## Funciones nuevas, no exportadas (`build_context_pack.go`)

```go
// codeGraphArchitectureCandidate arma, como máximo, un contextCandidate con el resumen
// compacto de arquitectura del primer proveedor con snapshot disponible. Segundo valor
// de retorno false si no hay ningún proveedor disponible (cero impacto, no es error).
func codeGraphArchitectureCandidate(providers []ports.CodeGraphProvider) (contextCandidate, bool)

// boostHotspotCandidates sube la prioridad de items[i] de PriorityOptional a
// PriorityRelevant cuando items[i].source coincide con un hotspot vigente según
// CUALQUIERA de los proveedores dados (ImpactFor). Nunca toca PriorityCritical, nunca
// baja una prioridad. Muta items in-place (mismo patrón que el resto del ensamblado de
// candidatos en BuildContextPack, que no usa un builder inmutable).
func boostHotspotCandidates(items []contextCandidate, providers []ports.CodeGraphProvider)
```

## Función extraída (`build_context.go`), sin cambio de comportamiento

```go
// formatCodeArchitecture es el cuerpo puro que antes vivía inline en
// writeCodeProviderSection — mismo output exacto, ahora reusable desde
// build_context_pack.go sin duplicar el formato.
func formatCodeArchitecture(snap domain.CodeProviderSnapshot) string
```

`writeCodeProviderSection` pasa a ser:

```go
func writeCodeProviderSection(sb *strings.Builder, snap domain.CodeProviderSnapshot) {
    sb.WriteString(formatCodeArchitecture(snap))
}
```

**Contrato de comportamiento agregado** (deriva 1:1 de los FR de este spec):

- `req.IncludeCodeGraph == false` → ni `codeGraphArchitectureCandidate` ni
  `boostHotspotCandidates` se invocan; comportamiento idéntico al de antes de esta feature
  (FR-006, FR-009).
- `req.CodeProviders` vacío/nil, o ningún proveedor con snapshot disponible → ambas
  funciones son no-ops; `BuildContextPack` no falla ni cambia su salida (FR-002).
- El boost de prioridad se aplica ANTES de calcular `criticalSum` y de la fase de
  compresión/presupuesto — un ítem recién promovido a `Relevant` participa en la
  compresión y el reparto de presupuesto exactamente como cualquier otro `Relevant`
  (FR-003).
- El candidato de arquitectura, si existe, se agrega a `items` en el mismo punto, y
  compite por presupuesto con prioridad `Optional` — puede terminar en
  `ContextStats.ItemsDiscarded` como cualquier otro candidato Optional (FR-005).
