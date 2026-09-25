# Implementation Plan: Motor nativo de compresión de contexto (prácticas de Headroom)

**Branch**: `main` (no hay hook de rama; la spec vive en `specs/033-native-context-compression`) | **Date**: 2026-09-25 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/033-native-context-compression/spec.md`

## Summary

gomemory incorpora un motor de compresión propio, determinista y agnóstico que
aplica dentro del binario las prácticas de Headroom:

- **ContentRouter**: detecta el tipo de cada bloque.
- **Compresores por tipo**: JSON por estadística de campos, código con firmas y
  sin cuerpos, logs y trazas colapsados, diff, tablas y prosa extractiva.
- **CCR**: cada omisión deja un marcador con referencia, y el original se
  recupera byte a byte desde la propia base de gomemory.
- **CacheAligner y live-zone**: el prefijo del contexto es estable y en la
  misma sesión solo viaja lo nuevo.
- **learn**: la agresividad se ajusta según la tasa de recuperación.

Se engancha en el puerto `ports.Compressor` que ya existe, con un nivel nuevo
`CompressionMax`, así que llega sin cambios de firma a todo lo que gomemory
entrega. Opcionalmente, un hook comprime la salida de las herramientas del
agente en los runtimes que lo permiten; está verificado en Claude Code
2.1.282 (todas las herramientas) y en Codex 0.157 (solo MCP).

Línea base medida: la compresión actual ahorra el 3,9 % en JSON, el 0,2 % en
el contexto y el 15,8 % en logs ([research R0](research.md)).

## Technical Context

**Language/Version**: Go 1.27 (`go.mod`: `go 1.27.0`, toolchain 1.27.1); TypeScript solo en el plugin de OpenCode, que ya existe.

**Primary Dependencies**: únicamente la biblioteca estándar (`encoding/json`, `go/parser`, `go/ast`, `compress/gzip`, `crypto/sha256` y `regexp`). **Ninguna dependencia nueva** (constitución §1.2 y §20).

**Storage**: SQLite existente (store global), con 4 tablas nuevas mediante migración aditiva en `adapters/secondary/persistence/db.go`, el patrón vigente ([data-model](data-model.md)).

**Testing**: `go test`. Unitarias junto al código; integración y contrato en `tests/integration` y `tests/contract`; benchmarks del motor; y un corpus de referencia en `tests/testdata/compression_corpus`.

**Target Platform**: el binario único `mem` (Linux, macOS y Windows) y los runtimes Claude Code, Codex y OpenCode 1.x y 2.x.

**Project Type**: CLI, servidor MCP por stdio y hooks de agentes.

**Performance Goals**: menos de 50 ms por cada 10 k tokens en el p95 (SC-007); el hook de salidas de herramientas, menos de 150 ms o se abstiene.

**Constraints**: determinismo byte a byte (FR-003); salida en `structural` idéntica a la v2.25.0 (SC-008); sin red (FR-025); sin cgo.

**Scale/Scope**: originales hasta 256 MB por store; entradas de hasta 2 MB por bloque, y por encima solo estructural.

## Constitution Check

*GATE: se evalúa antes de la fase 0 y se revisa después de la fase 1.*

| Regla | Estado | Nota |
|---|---|---|
| §3 Hexagonal: dominio sin infraestructura | ✅ | Política y tipos en `domain/`; puertos en `application/ports/`; compresores en `adapters/secondary/compression/native/`; SQL en `persistence` |
| §3 Toda dependencia externa por puerto; nombres `{Name}Port` y `{Name}Repository` | ✅ | 4 repositorios nuevos + `ClockPort` ([data-model](data-model.md)); los puertos existentes quedan bajo la excepción E-002 |
| §4 Composition root único | ✅ | Solo `infrastructure/container.go` elige `NativeCompressor` o `StructuralCompressor` según el nivel |
| §1.2 y §20: biblioteca estándar antes que dependencias | ✅ | Cero dependencias nuevas; tree-sitter y zstd descartados (R4, R9) |
| §5 SQL parametrizado, sin `SELECT *` | ✅ | Revisión en tareas |
| §6 Migraciones | ✅ excepción E-001 | Registrada según §23 en la constitución del proyecto (2026-09-25, revisión 2026-12-25): migración en el arranque, solo aditiva, con prueba desde base vacía y desde la v2.25.0 (T013) |
| §8 Degradación y timeouts | ✅ | FR-006, presupuesto del hook de 150 ms, fallback a estructural |
| §9 Configuración tipada y valores por defecto documentados | ✅ | `domain/compression_policy.go` es la única fuente de las cifras |
| §10 Pruebas deterministas, reloj inyectado | ✅ | `ClockPort` para TTL y ajuste adaptativo |
| §12 No registrar datos sensibles | ✅ | Las estadísticas no guardan contenido; los privados no generan originales (FR-024) |
| §13 Caché: nunca secretos | ✅ | Originales con `Private` excluidos (R9) |
| §18 Documentación en español | ✅ | |
| §1.2 Todo I/O con `context.Context` | ✅ | Los **puertos nuevos** llevan `ctx`. Los existentes, como `Compressor`, quedan bajo la excepción E-002; el `Engine` hace de puente con `context.WithTimeout` acotado (`domain.StoreTimeout`), nunca con `Background()` sin plazo |

**Resultado**: PASA. Las dos desviaciones que ya existían en el proyecto quedan registradas como excepciones E-001 y E-002 (§23) en la constitución del proyecto (`mem docs show constitution`). El código nuevo cumple las reglas sin excepción.

**Revisión tras la fase 1**: PASA sin cambios. El diseño no añadió capas, dependencias ni accesos cruzados.

## Desviaciones de la spec aclaradas en el plan

1. **FR-031 frente a SC-008**: el reorden estable → volátil y los deltas
   cambian la salida de `get_context`, así que activarlos en `structural`
   rompería SC-008. Se aplican **solo en `max`**, que es el nivel por defecto
   de las instalaciones nuevas. La spec ya está actualizada (R13).
2. **FR-022 por defecto**: el hook de salidas de herramientas es **opt-in**,
   porque reescribe lo que el modelo ve de herramientas ajenas. Es el mismo
   criterio que Octopus (R11).
3. **Exclusión de Read, Edit y Write**: una restricción de seguridad
   descubierta al investigar; comprimir Read rompe Edit (R11).

## Project Structure

### Documentation (this feature)

```text
specs/033-native-context-compression/
├── spec.md
├── plan.md              # este archivo
├── research.md          # fase 0: R0–R15
├── data-model.md        # fase 1
├── quickstart.md        # fase 1: Q0–Q10 contra el binario real
├── contracts/
│   ├── cli-and-mcp.md
│   └── hook-tool-output.md
├── checklists/requirements.md
└── tasks.md             # fase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
domain/
├── compression.go              # ContentType, Omission, render del marcador, invariantes puros
├── compression_policy.go       # cifras de fábrica, escalones de agresividad
└── mcp_tools.go                # + pack_retrieve, pack_savings (auto-aprobable: pack_retrieve)

application/
├── ports/
│   ├── compressor.go           # + CompressionMax, campos opcionales del resultado
│   ├── original_store.go       # OriginalStoreRepository
│   ├── delivered_blocks.go     # DeliveredBlocksRepository
│   ├── compression_stats.go    # CompressionStatsRepository, CompressionTuningRepository
│   ├── clock.go                # ClockPort
│   └── settings_repository.go  # + campos de compresión
└── usecases/
    ├── compress_content.go     # fachada fina: origen + nivel (la privacidad y las estadísticas viven en el Engine)
    ├── retrieve_original.go    # + suma de recuperaciones + ajuste adaptativo
    ├── compression_savings.go  # informe
    ├── session_delta.go        # sustitución de bloques ya entregados
    └── build_context.go        # (mod) orden estable → volátil en nivel max, sin horas en el prefijo

adapters/
├── secondary/compression/native/
│   ├── engine.go               # implementa ports.Compressor; umbral → router → compresor → guarda → no_gain → privacidad → store → stats
│   ├── router.go  segment.go
│   ├── json.go  code_go.go  code_heuristic.go  log.go  diff.go  table.go  prose.go
│   ├── guard.go                # guarda de literalidad
│   └── *_test.go, bench_test.go
├── secondary/clock/system.go   # ClockPort real
├── secondary/persistence/
│   ├── db.go                   # (mod) 4 tablas nuevas
│   ├── compression_originals.go  delivered_blocks.go  compression_stats.go
│   └── settings.go             # (mod) Settings ↔ SettingsData, campos nuevos en ambos
└── primary/
    ├── cli/cmd_pack.go         # (mod) --level, --compare, --json, retrieve, savings, tune, purge; salto de línea antes de "tokens:"
    ├── cli/cmd_mcp.go          # (mod) pack_compress{level,compare}, pack_retrieve, pack_savings
    ├── cli/cmd_hook.go         # (mod) tool-output; reinicio de delivered_blocks y context_deliveries en pre/post-compact y session-start
    ├── cli/cmd_settings.go     # (mod) --compression-level, --tool-output-compression, --concise-output
    ├── cli/cmd_doctor.go       # (mod) sección «Compresión»
    ├── setup/claude_code_setup.go  # (mod) PostToolUse tool-output solo con el ajuste activo
    ├── setup/codex_setup.go        # (mod) ídem, para MCP
    └── tui/tui.go              # (mod) fila de nivel + interruptores

infrastructure/
├── container.go                # (mod) wiring del motor y los repositorios
└── plugin/opencode/gomemory.ts # (mod) tool.execute.after → mem hook tool-output (v1; v2 según Q7)

tests/
├── testdata/compression_corpus/   # muestras reales anonimizadas + expectations.json
├── integration/compression_corpus_test.go  session_delta_test.go  compression_privacy_test.go
└── contract/hook_tool_output_test.go  mcp_pack_tools_test.go
```

**Structure Decision**: se respeta la estructura hexagonal del repositorio. El
motor es un adaptador secundario nuevo (`compression/native`), hermano de
`structural.go` y `noop.go`, y lo que decide qué se comprime y cuándo vive en
casos de uso.

## Orden de implementación (para /speckit-tasks)

Aplicando la descomposición atómica. Cada hoja es verificable por sí sola.

```
🎯 El nivel max entrega ≥40 % menos tokens que structural en el corpus, con 100 % de recuperación y structural idéntico a v2.25.0
├─ [1] Fundamentos
│  ├─ [1.1] ✓ Crear corpus + expectations.json → tests/testdata/compression_corpus existe con ≥10 muestras
│  ├─ [1.2] ✓ Añadir CompressionMax y campos opcionales a ports.Compressor → compila; tests actuales en verde        ∥
│  ├─ [1.3] ✓ Definir domain/compression{,_policy}.go → pruebas del render del marcador y de los escalones         ∥
│  └─ [1.4] ✓ Test de regresión SC-008: la salida structural es igual a la de la v2.25.0 en el corpus → en verde antes y después  (dep: 1.1)
├─ [2] Motor (adapters/secondary/compression/native)                                 (dep: 1)
│  ├─ [2.1] ✓ Router + segmentador → clasifica bien todas las muestras del corpus
│  ├─ [2.2] ✓ Guarda de literalidad → rechaza una salida alterada a propósito
│  ├─ [2.3] ✓ Compresor JSON → golist.json ahorra ≥70 %; errores, atípicos y extremos conservados        (dep: 2.2)  ∥
│  ├─ [2.4] ✓ Compresor de código Go (AST) → firmas intactas, cuerpos con marcador   (dep: 2.2)  ∥
│  ├─ [2.5] ✓ Compresor de código heurístico (py/java/ts/js/sql) → degrada a estructural si hay baja confianza  (dep: 2.2)  ∥
│  ├─ [2.6] ✓ Compresor de logs/trazas → ERROR y cabeceras de traza intactos                    (dep: 2.2)  ∥
│  ├─ [2.7] ✓ Compresores de diff y tabla → pruebas del corpus                           (dep: 2.2)  ∥
│  ├─ [2.8] ✓ Compresor de prosa extractivo → sin frases alteradas                     (dep: 2.2)  ∥
│  └─ [2.9] ✓ Engine: umbral, no_gain, too_large y determinismo → la misma entrada da la misma salida 100 veces  (dep: 2.1–2.8)
├─ [3] Persistencia                                                                   (∥ con 2)
│  ├─ [3.1] ✓ Migración de las 4 tablas → prueba desde una base vacía y desde una base v2.25.0
│  ├─ [3.2] ✓ OriginalStore: gzip, dedup, TTL y LRU → pruebas con reloj inyectado
│  ├─ [3.3] ✓ DeliveredBlocks por sesión → Reset limpia solo la sesión activa
│  └─ [3.4] ✓ Stats + Tuning → las sumas y el descenso de escalón funcionan
├─ [4] Casos de uso                                                                   (dep: 2, 3)
│  ├─ [4.1] ✓ CompressContent con privacidad (sin refs para lo privado) → Q6 en verde
│  ├─ [4.2] ✓ RetrieveOriginal + ajuste adaptativo → Q9 en verde
│  ├─ [4.3] ✓ SessionDelta + reinicio en pre/post-compact y session-start → Q5 (t4 = 0)
│  └─ [4.4] ✓ BuildContext: orden estable → volátil solo en max → Q5 «prefijo estable»; structural igual a la v2.25.0
├─ [5] Superficie                                                                      (dep: 4)
│  ├─ [5.1] ✓ Ajustes: campos en SettingsData y en Settings + mem settings + TUI → sobreviven a Write de otro ajuste
│  ├─ [5.2] ✓ Instalación nueva escribe "max"; una existente no toca el nivel → prueba de install
│  ├─ [5.3] ✓ CLI pack compress/retrieve/savings/tune/purge + salto de línea antes de "tokens:" → contrato cli
│  ├─ [5.4] ✓ MCP pack_compress{level,compare}, pack_retrieve (auto-aprobable), pack_savings → prueba de contrato MCP
│  ├─ [5.5] ✓ Línea de protocolo ⟦mem⟧ → pack_retrieve solo en max → mem context la contiene
│  ├─ [5.6] ✓ Directiva de concisión en la zona volátil (opt-in) → US5
│  └─ [5.7] ✓ mem doctor: sección Compresión (+ --json y --strict) → Q8
├─ [6] Hook de salidas de herramientas (opt-in)                                            (dep: 4.1, 5.1)
│  ├─ [6.1] ✓ mem hook tool-output claude|codex|opencode con forma preservada y exclusiones → contrato hook
│  ├─ [6.2] ✓ Registro en Claude (PostToolUse con matcher) y Codex solo con el ajuste activo → setup tests
│  ├─ [6.3] ✓ Plugin OpenCode v1: tool.execute.after asigna output.output → Q7, fila 1.18.32
│  └─ [6.4] ⚠ no atómica → OpenCode v2: depende de verificar en Q7 si mutar ev.result surte efecto; si no, se marca como no soportado
└─ [7] Validación y cierre                                                             (dep: 1–6)
   ├─ [7.1] ✓ Benchmarks SC-007 → p95 < 50 ms/10 k tokens
   ├─ [7.2] ✓ Ejecutar quickstart Q0–Q10 contra el binario compilado → resultados anotados
   ├─ [7.3] ✓ Actualizar docs/MANUAL.md y docs/MEMORY-PROTOCOL.md (marcadores, niveles, pack_retrieve) → docs en español
   └─ [7.4] ✓ Guardar en gomemory el resultado real por runtime (topic_key runtime-tool-output-rewrite)
```

## Riesgos y cierre

| Riesgo | Cierre propuesto | Validación |
|---|---|---|
| El agente ignora los marcadores y trabaja con datos incompletos | Línea de protocolo + resumen útil dentro del marcador (recuentos y distribución de campos) + ajuste adaptativo | Q7 con un agente real; tasa de recuperación en `savings` |
| Un compresor altera un literal | La guarda común rechaza el resultado (INV-C5) | Q4 + fuzz de la guarda |
| Comprimir la salida de una herramienta rompe un flujo del agente | Opt-in, exclusiones fijas, forma preservada y abstención ante cualquier duda | Contrato del hook + Q7 |
| El reorden rompe consumidores que parsean `mem context` | Solo en max; structural igual a la v2.25.0 | 1.4 + Q1 |
| Los originales crecen sin control | TTL + LRU + `mem pack purge` + aviso de doctor al 90 % | 3.2 + Q8 |
| Un secreto persiste como original | Los bloques privados no emiten refs | Q6 |

## Complexity Tracking

Sin violaciones. Las dos desviaciones que ya existían están registradas como E-001 y E-002 según §23.
