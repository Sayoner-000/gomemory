# Data Model: Motor nativo de compresión de contexto

**Feature**: 033-native-context-compression · Fuente de decisiones: [research.md](research.md)

## Dominio (`domain/`, sin dependencias de infraestructura)

### ContentType
Enumeración: `json`, `code`, `log`, `diff`, `table`, `prose`, `mixed`, `unknown`.
`code` lleva el subtipo de lenguaje: `go`, `python`, `java`, `ts`, `js`, `sql` o `generic`.

### CompressionLevel (ampliado en `application/ports`)
`CompressionStructural` (0, el valor cero, sin cambios) · `CompressionNone` (1, sin cambios) · **`CompressionMax` (2, nuevo)**.
Se pasa a ajustes como `"structural" | "none" | "max"`.

### CompressionPolicy (`domain/compression_policy.go`)
Valores de fábrica y reglas puras. Es la única fuente de las cifras:

| Campo | Fábrica | Uso |
|---|---|---|
| MinTokens | 64 (≈ 50 palabras) | FR-004: por debajo, el bloque sale intacto |
| OriginalsTTLDays | 7 | FR-014 |
| OriginalsMaxBytes | 256 MB | FR-014 |
| AdaptiveThreshold | 0,20 | FR-027 |
| AdaptiveMinOmissions | 20 | FR-027: muestra mínima |
| ToolOutputMinTokens | 400 | R11 |
| MaxInputBytes | 2 MB | R14: por encima, solo estructural |
| HookBudget | 150 ms | R11 |
| StoreTimeout | 500 ms | plazo del puente sin `ctx` a los puertos nuevos (E-002) |

`Aggressiveness(level 0..3)` → umbrales por tipo (R12):

| Escalón | JSON N_min | Código min_body_lines | Log: serie mínima | Prosa max_para_sentences |
|---|---|---|---|---|
| 3 (inicial) | 8 | 6 | 3 | 8 |
| 2 | 16 | 12 | 5 | 12 |
| 1 | 32 | 25 | 10 | 20 |
| 0 | — (solo estructural) | — | — | — |

### Block
| Campo | Tipo | Regla |
|---|---|---|
| Content | string | original, inmutable |
| Type | ContentType | resultado del router |
| Origin | enum `context \| pack \| search \| tool_output \| adhoc \| delegated` | para estadísticas |
| Hash | string | sha256 hex de Content |
| Private | bool | true si `RedactPrivate` o `RedactSecrets` alteran el contenido → sin pérdida (R9) |

### Omission (marcador)
| Campo | Tipo | Regla |
|---|---|---|
| Ref | string (12 a 16 hex) | prefijo del sha256 del original; se amplía ante colisión |
| Summary | string | "omitidos 187 de 200 elementos", "cuerpo omitido: 42 líneas"… |
| Render | string | `⟦mem⟧ <Summary> · ref=<Ref>` (texto) o `{"⟦mem⟧": Summary, "ref": Ref, …}` (JSON) |

### CompressionOutcome (amplía `ports.CompressionResult`)
Los campos existentes no cambian (`Content`, `RawTokens`, `Tokens`, `Compressed`). Se añaden:

| Campo | Tipo | Nota |
|---|---|---|
| Compressor | string | `none \| structural \| json \| code \| log \| diff \| table \| prose \| mixed` |
| ContentType | ContentType | |
| StructuralTokens | int | tokens de la etapa estructural, para el informe por etapa |
| Refs | []string | referencias emitidas |
| FallbackReason | string | `"" \| error \| literal_guard \| no_gain \| too_large \| store_unavailable \| private` |
| LatencyMicros | int64 | |

**Invariantes**
- INV-C1: con `Level=None`, `Content == input`.
- INV-C2: con `Level=Structural`, la salida es igual byte a byte a la de `StructuralCompressor` de la v2.25.0.
- INV-C3: con `Level=Max`, `Tokens <= StructuralTokens` (FR-005).
- INV-C4: toda `Ref` emitida es recuperable mientras no caduque (FR-013). Un bloque `Private` nunca emite `Ref`.
- INV-C5: guarda de literalidad superada, o `FallbackReason=literal_guard` (FR-011).
- INV-C6: la misma entrada y la misma configuración (nivel + agresividad) dan la misma salida (FR-003).

## Persistencia (SQLite, migración aditiva en `adapters/secondary/persistence/db.go`)

### compression_originals
| Columna | Tipo | Restricción |
|---|---|---|
| ref | TEXT | PK |
| hash | TEXT | NOT NULL, UNIQUE (sha256 completo) |
| project | TEXT | NOT NULL |
| content_gz | BLOB | NOT NULL |
| raw_bytes | INTEGER | NOT NULL |
| content_type | TEXT | NOT NULL |
| compressor | TEXT | NOT NULL |
| created_at | TEXT | NOT NULL |
| last_access_at | TEXT | NOT NULL |
| expires_at | TEXT | NOT NULL, índice |

Ciclo de vida: `created` → (cada `retrieve` refresca `last_access_at` y `expires_at`) → `expired` (purga perezosa al insertar o recuperar, y bajo demanda con `mem pack purge`; `mem doctor` no tiene `--fix`, solo `--json` y `--strict`) o `evicted` (LRU al superar el tope).

### delivered_blocks
| Columna | Tipo | Restricción |
|---|---|---|
| session_id | TEXT | NOT NULL |
| block_hash | TEXT | NOT NULL |
| delivered_at | TEXT | NOT NULL |
| | | PK (session_id, block_hash) |

Se reinicia (`DELETE WHERE session_id=?`) en pre-compact, post-compact, `session.compacted` e inicio de sesión (FR-019). La tabla existente `context_deliveries` se vacía en los mismos puntos.

### compression_stats
| Columna | Tipo |
|---|---|
| project, compressor, content_type | TEXT, PK compuesta |
| uses, raw_tokens, structural_tokens, final_tokens | INTEGER |
| omissions, retrievals, fallbacks | INTEGER |
| latency_us_total | INTEGER |
| updated_at | TEXT |

Actualización con `INSERT … ON CONFLICT DO UPDATE SET x = x + excluded.x`. No guarda contenido, así que no puede filtrar nada privado (FR-024).

### compression_tuning
| Columna | Tipo |
|---|---|
| project, content_type | TEXT, PK |
| aggressiveness | INTEGER (0..3) |
| reason | TEXT (p. ej. "recuperación 34 % sobre 41 omisiones") |
| updated_at | TEXT |

La ausencia de fila equivale a agresividad 3.

## Ajustes (`ports.SettingsData` **y** `persistence.Settings`)

| Campo JSON | Tipo | Ausente = |
|---|---|---|
| `context_compression_level` | string | `"structural"`, o `"none"` si `context_compression_disabled=true` |
| `tool_output_compression` | bool | false |
| `concise_output_directive` | bool | false |
| `compression_min_tokens` | int | fábrica |
| `compression_originals_ttl_days` | int | fábrica |
| `compression_originals_max_mb` | int | fábrica |
| `compression_adaptive_threshold_pct` | int | fábrica |

## Puertos nuevos (`application/ports/`)

Todos reciben `ctx context.Context` como primer parámetro en cada operación de
I/O y siguen la convención de nombres de §3 (constitución). La excepción E-002
cubre solo los puertos existentes, como `Compressor`.

- `OriginalStoreRepository`: `Put(ctx, Block, compressor) (ref, error)`, `Get(ctx, ref) (content, meta, found, error)`, `Purge(ctx) (n, error)`, `Usage(ctx) (bytes, count, error)`.
- `DeliveredBlocksRepository`: `Seen(ctx, hash) (bool, error)`, `Mark(ctx, hashes ...string) error`, `Reset(ctx) error` (siempre sobre la sesión activa, igual que `DeliveryLogRepository`).
- `CompressionStatsRepository`: `Record(ctx, project, outcome) error`, `RecordRetrieval(ctx, project, compressor, type) error`, `Summary(ctx, project) ([]CompressorStats, error)`.
- `CompressionTuningRepository`: `Get(ctx, project, type) (int, error)`, `Lower(ctx, project, type, reason) error`, `Reset(ctx, project, type) error`.
- `ClockPort`: `Now() time.Time`, inyectado para TTL y marcas de tiempo (constitución §10). No hace I/O, así que no lleva `ctx`.

**Puente con `ports.Compressor` (sin `ctx`, E-002)**: el `Engine` implementa
`Compress(input, opts)` y, para llamar a los puertos nuevos, crea un
`context.WithTimeout` acotado a `domain.StoreTimeout` (500 ms; 150 ms dentro del
hook). Nunca usa `context.Background()` sin plazo.

**Privacidad y estadísticas dentro del motor** (corrige I2 del análisis): el
`Engine` recibe `OriginalStoreRepository` y `CompressionStatsRepository` (el
segundo puede ser nil hasta US4) y aplica él mismo la comprobación de
privacidad (`domain.RedactPrivate` y `domain.RedactSecrets`) y el registro de
estadísticas. Así, **todo** llamador de `ports.Compressor`
(`BuildContextPack`, el paquete de Octopus, `CompressText` y los hooks) tiene
las mismas garantías, y `CompressContent` queda como una fachada fina para el
origen y el nivel.
