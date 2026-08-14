# Fase 0 — Investigación: Context Optimization & Budgeting

**Feature**: [spec.md](./spec.md) · **Fecha**: 2026-08-14

Todas las incógnitas del Technical Context del plan se resuelven aquí antes de pasar
a diseño (Fase 1). El brief original de la feature venía con decisiones técnicas ya
tomadas (tipos Go, pesos de ranking, layout de paquetes) — donde esas decisiones chocan
con código y convenciones que YA existen en este repo, gana lo que ya existe; se
documenta el porqué en cada caso.

## 1. ¿Dónde vive esto en la arquitectura hexagonal existente?

**Decisión**: Nuevo dominio puro (`domain/context_pack.go`: `ContextItem`, `ContextPack`,
`Priority`, `ContextStats`), dos puertos nuevos (`application/ports/compressor.go`,
`application/ports/token_counter.go`) con adaptadores concretos en
`adapters/secondary/`, y un caso de uso nuevo (`application/usecases/build_context_pack.go`)
que orquesta retrieval + ranking + dedup + compresión + budget, siguiendo el mismo
patrón que `build_context.go` y `build_plan_context.go` ya usan para `get_context()` /
`get_plan_context()`.

**Justificación**: La Constitución (Principio I) exige capas con dependencia estricta y
adaptadores intercambiables; `Compressor` y `TokenCounter` deben ser puertos (no
funciones sueltas) porque el propio spec exige que existan implementaciones futuras
intercambiables (`SemanticCompressor`/`LLMCompressor`, contadores de tokens específicos
de proveedor) sin tocar el caso de uso — exactamente el mismo motivo por el que
`CodeGraphProvider` ya es un puerto con adaptadores swappeables en este proyecto.

**Alternativas consideradas**: Meter la lógica directo en el comando CLI (rechazado:
viola la regla de "sin lógica de negocio en handlers"); un paquete nuevo fuera de
`domain/application/adapters` al estilo del brief original (`gomemory/context/`,
`gomemory/compression/`, `gomemory/dedup/`, `gomemory/tokens/`) (rechazado: este repo
no es un módulo `gomemory/` con sub-paquetes por capa, es un módulo `mem` con capas
hexagonales ya establecidas — replicar el layout del brief fragmentaría la arquitectura
existente sin ganar nada).

## 2. Ranking de relevancia sin campos importance/confidence en el dominio

**Decisión**: La relevancia semántica se deriva del **orden de resultados** que ya
devuelve `MemoryRepository.Search(project, query, limit)` (FTS5/BM25, confirmado en
`adapters/secondary/persistence/memory.go`), normalizado a un score 0–1 por posición.
La importancia se deriva del `MemoryType` (decision/architecture/bugfix > pattern/
discovery/learning > preference > checkpoint, este último siempre excluido — mismo
criterio que ya aplica `DetectDuplicateGroups`). La recencia se deriva de `CreatedAt`.
No se añade ningún campo nuevo a `domain.Memory`.

**Justificación**: `domain/memory.go` no tiene hoy `importance`/`confidence`/
`relevance` — añadirlos exigiría migración de esquema y, peor, pedirle al usuario que
los rellene a mano en cada `save_memory`, lo que rompe FR-018 (la feature no puede
cambiar el comportamiento de guardado existente). Derivar los tres factores de señales
que YA existen (orden de búsqueda, tipo, fecha) cumple FR-003 sin tocar el modelo de
datos ni el flujo de guardado.

**Alternativas consideradas**: Añadir columnas `importance`/`confidence` a la tabla
`memories` (rechazado por lo anterior); embeddings + similitud coseno para
`semantic_relevance` (rechazado para v1: cero dependencias de IA obligatorias por
política de dependencias, y no hay infraestructura de embeddings hoy en gomemory para
memorias — sí existe para el grafo de código externo, pero es un servidor MCP
DISTINTO, fuera de alcance de esta feature).

## 3. Deduplicación: reusar `DetectDuplicateGroups`, no reinventarla

**Decisión**: `application/usecases/build_context_pack.go` llama directamente a la
función pura ya existente `DetectDuplicateGroups(memories, threshold)`
(`application/usecases/detect_duplicates.go`) sobre el subconjunto de candidatos de la
solicitud (no sobre todo el proyecto), y usa `SuggestedKeepID` como el representante
que se conserva; el resto del grupo se cuenta como duplicado descartado en
`ContextStats`.

**Justificación**: Es exactamente el mismo problema (similitud léxica Jaccard sobre
Título+Contenido, ya calibrada contra un caso real y ya excluye `Checkpoint`) aplicado
a un conjunto más chico. Reescribirlo violaría el principio de simplicidad y
duplicaría una implementación ya probada.

**Alternativas consideradas**: Detección solo por texto exacto/normalizado (rechazado:
más débil que lo que ya existe, sin ganar nada a cambio); similitud semántica por
embeddings (mismo rechazo que en la sección 2 — sin infraestructura hoy, no
obligatoria).

## 4. Conteo de tokens

**Decisión**: `TokenCounter` es un puerto con una única implementación v1,
`adapters/secondary/tokens/approximate.go`, que aproxima tokens por caracteres
(heurística simple, sin dependencias). Contadores específicos de proveedor
(tiktoken-style) quedan como adaptadores futuros intercambiables detrás del mismo
puerto, no como parte de esta feature.

**Justificación**: Política de dependencias (spec, sección "Dependency Policy" del
brief original y Assumptions del spec): cero dependencias de IA obligatorias. Una
aproximación por caracteres es determinista, testeable y suficiente para que
`ContextStats` sea internamente consistente (SC-006).

**Alternativas consideradas**: Vendorizar un tokenizer real tipo `tiktoken-go`
(rechazado para v1: dependencia nueva no obligatoria, va contra la Política de
Dependencias del proyecto; queda como adaptador futuro opcional detrás del puerto).

## 5. Compresión estructural determinista

**Decisión**: `Compressor` es un puerto con dos adaptadores v1:
`adapters/secondary/compression/noop.go` (no toca el contenido) y
`adapters/secondary/compression/structural.go` (colapsa whitespace repetido,
encabezados/párrafos duplicados idénticos, sin tocar bloques de código, comandos,
URLs, paths, identificadores, mensajes de error, números o versiones — detectados por
patrones deterministas: fences ```` ``` ````, regex de URL/path/número de versión).

**Justificación**: FR-009 exige que la compresión base sea determinista y no dependa
de una llamada externa; FR-010/5.4 exige que el original quede recuperable —
`Compressor.Compress` nunca escribe sobre `domain.Memory`, solo produce un
`ContextItem.Content` derivado en memoria, efímero por request.

**Alternativas consideradas**: `SemanticCompressor`/`LLMCompressor` (fuera de alcance
v1, ver Assumptions del spec — quedan como adaptadores futuros detrás del mismo
puerto, sin romper el caso de uso).

## 6. Persistencia: ninguna tabla nueva

**Decisión**: `ContextPack` es efímero — se construye y se devuelve en la misma
llamada (CLI/API/MCP), sin persistirse en SQLite. No hay migración nueva.

**Justificación**: Nada en el spec exige recuperar un `ContextPack` pasado; persistir
uno introduciría estado mutable y una tabla nueva sin requisito que lo respalde,
violando el Principio de Simplicidad. Las estadísticas (`ContextStats`) viajan solo en
la respuesta de la llamada que las generó.

**Alternativas consideradas**: Tabla `context_packs` para auditoría histórica
(rechazada: no hay historia de usuario que lo pida; se puede añadir después sin romper
nada si aparece la necesidad real).

## 7. Configuración: extender `SettingsData`, no un YAML nuevo

**Decisión**: Los ajustes configurables de FR-019 (presupuesto por defecto, relevancia
mínima, máximo de candidatos, agresividad de compresión, dedup on/off, Spec Kit
on/off) se agregan como campos nuevos de `application/ports/SettingsData`
(`.memory/settings.json`), siguiendo el mismo patrón opt-out ya usado por
`SpeckitContextDisabled`/`AtomicPlanDisabled` (booleano `...Disabled`, ausente = activo,
default sensato).

**Justificación**: Constitución Principio IV: "Una sola struct de config para todo el
proyecto". El brief original proponía un YAML `context:` separado (sección 30) — eso
duplicaría el mecanismo de configuración existente y violaría ese principio
directamente.

**Alternativas consideradas**: Archivo `context.yaml` independiente tal como sugiere el
brief (rechazado por lo anterior); variables de entorno nuevas por cada ajuste
(rechazado: estos valores son por-proyecto y ya viven en `.memory/settings.json`, no
son secretos ni valores de entorno de despliegue).

## 8. Superficie CLI: comando nuevo `mem pack`, no reusar `mem context`

**Decisión**: Nuevo comando `mem pack {build|show|compress|stats}`
(`adapters/primary/cli/cmd_pack.go`), registrado en `dispatcher.go`, siguiendo el
patrón de subcomando exacto de `cmd_session.go` (`CmdPack` despacha a
`cmdPackBuild`/`cmdPackShow`/`cmdPackCompress`/`cmdPackStats`).

**Justificación**: `mem context [-w]` YA EXISTE y hace algo distinto (el resumen
markdown de `get_context()`, `adapters/primary/cli/cmd_context.go`) — reusar el
nombre `context` para este comando rompería o confundiría ese comando existente,
violando FR-018 (la feature es aditiva, no debe alterar comportamiento existente).
`pack` refleja el término del dominio (`ContextPack`) sin colisionar.

**Alternativas consideradas**: `mem context build/show/compress/stats` como subcomandos
del comando existente (rechazado: exigiría reescribir `CmdContext` para distinguir
"sin subcomando = comportamiento legado" de "con subcomando = feature nueva", una
ambigüedad de UX innecesaria cuando un nombre nuevo la evita del todo).

## 9. Exposición vía MCP

**Decisión**: Además del CLI, se exponen tools MCP nuevas siguiendo el patrón de única
fuente de verdad de `domain/mcp_tools.go` (`ToolPackBuild = "pack_build"`, etc.,
agregadas a un nuevo grupo `MCPContextPackTools`, incluido en `MCPAllTools()` y
`MCPAutoApprovableTools()` salvo que se marque destructivo — ninguna operación de esta
feature borra datos, así que ninguna va en `MCPDestructiveTools`).

**Justificación**: Constitución, Principios Operativos #9: "MCP como integración
primaria: exponer funcionalidad vía MCP sobre stdio". El CLI solo (FR-013) no basta
para cumplir la constitución del proyecto; ambas superficies comparten el mismo caso de
uso (`BuildContextPack`), sin lógica duplicada.

**Alternativas consideradas**: Solo CLI, sin tools MCP (rechazado: contradice
explícitamente el Principio Operativo #9).

## Technical Context — resuelto

| Campo | Valor |
|---|---|
| Language/Version | Go >=1.22 (stack ya congelado, sin cambios) |
| Primary Dependencies | Ninguna dependencia externa nueva obligatoria — stdlib + reuso de `Search` (FTS5), `DetectDuplicateGroups`, `SettingsData` ya existentes |
| Storage | SQLite existente (`modernc.org/sqlite`, `.memory/memory.db`), sin migraciones nuevas — `ContextPack` es efímero, no se persiste |
| Testing | `testing` stdlib + `testify`, TDD Red-Green-Refactor, `tests/unit` + `tests/integration` + `tests/contract` |
| Target Platform | Binario Go multiplataforma existente (Linux/macOS/Windows), CLI + servidor MCP sobre stdio |
| Project Type | Extensión de biblioteca/CLI dentro del módulo hexagonal único ya existente (no un proyecto/módulo nuevo) |
| Performance Goals | SC-007: <100ms de procesamiento añadido sobre las llamadas de retrieval ya existentes, para proyectos de tamaño típico |
| Constraints | Determinismo de la compresión estructural (SC-006); nunca soltar contenido crítico sin error explícito (FR-008); aditivo/opt-in (FR-018/SC-005); cero dependencias de IA obligatorias; documentación en español latino; 120 columnas `gofumpt`; cobertura ≥80% |
| Scale/Scope | Escala de un proyecto gomemory típico (cientos a pocos miles de memorias en SQLite local); 5 historias de usuario P1→P5 |
