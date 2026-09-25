# Research: Motor nativo de compresión de contexto

**Feature**: 033-native-context-compression · **Fecha**: 2026-09-25

Cada decisión se verificó contra el código actual (`mem` 2.25.0) o contra los
binarios instalados de los runtimes. No se han leído solo documentos.

---

## R0. Línea base medida (regla de trabajo 1: primero reproduce la realidad)

Medido con `mem pack compress <archivo>` (compresión estructural actual):

| Muestra | Tamaño | Tokens antes → después | Ahorro |
|---|---:|---:|---:|
| `go list -json ./...` (JSON real) | 75 KB | 18 788 → 18 062 | 3,9 % |
| `mem context` (contexto del proyecto) | 17 KB | 4 225 → 4 218 | 0,2 % |
| `git log -200 --stat` (listado/log) | 268 KB | 66 785 → 56 249 | 15,8 % |

**Conclusión**: la compresión estructural casi no ahorra en JSON ni en el
contexto propio. Hay margen real, y las metas de SC-001 y SC-002 son
alcanzables en estos tipos de contenido.

**Hallazgo colateral**: `mem pack compress` imprime `tokens: X → Y` pegado a
la última línea del contenido, sin salto de línea. Se corrige en esta feature
(tarea de cierre, regla de trabajo 5).

---

## R1. Punto de integración del motor

- **Decision**: el motor nativo se integra como una implementación nueva del
  puerto existente `ports.Compressor` (`application/ports/compressor.go`), con
  un nivel nuevo `CompressionMax` añadido **al final** del `iota`.
  `CompressionStructural` sigue siendo el valor cero y `CompressionNone` conserva su
  valor.
- **Rationale**: el puerto ya reserva niveles superiores "para adaptadores
  futuros sin cambiar esta interfaz". Todos los consumidores actuales
  (`BuildContextPack`, `CompressText`, `OptimizeToolDescription`,
  `NewPackContractUseCase`) reciben el compresor por inyección desde
  `infrastructure/container.go`. Cambiar el adaptador en el composition root
  lleva el motor a todos a la vez (FR-020).
- **Contrato ampliado**: `CompressionResult` gana campos **opcionales**
  (`Compressor`, `ContentType`, `Refs`, `FallbackReason`) sin romper a los
  llamadores actuales, que solo leen `Content/RawTokens/Tokens/Compressed`.
- **Alternatives considered**:
  - Un puerto nuevo `ContextEngine` paralelo: duplica el flujo y deja a
    `BuildContextPack` con dos caminos. Descartado.
  - Integrar Headroom como proceso: descartado por la persona ("no actor").

## R2. Detección del tipo de contenido (ContentRouter)

- **Decision**: segmentador y clasificador deterministas por reglas, en este
  orden de precedencia:
  1. Bloques con cerca Markdown (```` ```lang ````): tipo por etiqueta de lenguaje.
  2. JSON: el bloque entero es un valor JSON válido (`encoding/json`, `Valid`),
     o una línea por objeto (JSON Lines).
  3. Diff: cabeceras `diff --git`, `@@ -a,b +c,d @@` o `--- a/` y `+++ b/`.
  4. Log o traza: al menos el 30 % de las líneas empiezan con marca de tiempo o
     nivel (`ERROR|WARN|INFO|DEBUG|TRACE`), o hay patrones de traza de pila
     (`goroutine N [`, `Traceback (most recent call last)`, `\tat `, `  File "`).
  5. Código: parseo exitoso con `go/parser` para Go; para el resto, una
     puntuación heurística de palabras clave y llaves o sangría por lenguaje
     (Python, Java, TS/JS y SQL) con un umbral de confianza.
  6. Tabla o listado: al menos el 60 % de las líneas comparten el mismo
     separador (`|`, tabulador o columnas alineadas) o prefijo de viñeta.
  7. Prosa: el resto.
- **Mixto**: un bloque con cercas se segmenta. Cada cerca se trata con su tipo
  y el texto entre cercas se trata como prosa.
- **Rationale**: todo con biblioteca estándar, determinista y sin modelo
  (FR-001, FR-003). La precedencia evita clasificar mal un JSON que "parece"
  log.
- **Alternatives considered**: tree-sitter para todos los lenguajes (necesita
  cgo o WASM, rompe el binario único estático y añade una dependencia);
  clasificador ML (prohibido por FR-001).

## R3. Compresor de JSON (práctica SmartCrusher)

- **Decision**: para cada array de objetos con más de `N_min` elementos (por
  defecto 8):
  1. Calcula por campo la varianza: cardinalidad de valores distintos y, en
     campos numéricos, la media y la desviación típica.
  2. **Conserva siempre**: el primer y el último elemento; los elementos con un
     campo cuyo *nombre de clave* o *valor* indica error (`error`, `err`,
     `status>=400`, `level=error`, `ok=false`, `failed`, `exception`); y los
     elementos con algún valor numérico a más de 3σ o con un valor categórico
     de frecuencia menor o igual al 1 %.
  3. Sustituye el resto por **un único objeto marcador** en la posición donde
     estaban:
     `{"⟦mem⟧": "omitidos 187 de 200 elementos", "ref": "c7a1f9e2b3d4", "campos": {"status": "200×185, 404×2"}}`.
  4. Arrays de escalares: conserva los extremos más un resumen (recuento y
     mín./máx.).
  5. Objetos profundamente anidados: recursión con el mismo criterio.
- La salida es JSON **válido** y con el orden original (el marcador es un
  elemento más). Los elementos conservados son iguales, en profundidad, a los
  originales: guarda de FR-011.
- **Rationale**: es la regla publicada de SmartCrusher ("keeps error items,
  values outside the normal statistical range, and first/last boundaries…
  from field-variance statistics rather than a keyword list"). Las palabras
  de error son un **suelo de seguridad**; la selección principal es
  estadística.
- **Alternatives considered**: muestreo aleatorio (no determinista); truncado
  a los N primeros (pierde los errores del final).

## R4. Compresor de código (práctica CodeCompressor)

- **Decision**:
  - **Go**: con `go/parser` y `go/ast` conserva `package`, `import` (solo los
    identificadores usados en lo que queda), declaraciones de tipos,
    constantes, variables de paquete, firmas de funciones y métodos, y
    comentarios de documentación. El cuerpo de cada función se sustituye por
    `{ /* ⟦mem⟧ cuerpo omitido: 42 líneas · ref=… */ }`. Las funciones de
    menos de `min_body_lines` (por defecto 6) se conservan enteras.
  - **Python, Java, TS/JS**: segmentador por sangría (Python) o por balance de
    llaves (resto) que reconoce cabeceras de función, clase y método mediante
    expresiones regulares por lenguaje; conserva cabeceras, decoradores y
    anotaciones, docstrings o JSDoc, e importaciones, y omite los cuerpos
    largos con un marcador. Si el balance de llaves no cuadra, o la confianza
    es baja, ese bloque pasa a la compresión estructural.
  - **SQL**: conserva `CREATE` con columnas y omite `INSERT` repetidos
    (colapsados con su recuento).
  - **Lenguaje no reconocido**: compresión genérica por bloques de sangría,
    que solo omite bloques internos largos.
- **Rationale**: Go se trata con AST de la biblioteca estándar, sin
  dependencias. Para el resto, la heurística cubre los stacks de la
  constitución sin cgo. Siempre degrada a estructural (FR-006).
- **Alternatives considered**: tree-sitter vía WASM (wazero): aumenta mucho
  el binario y añade dependencia. Se reevalúa si la medición muestra que la
  heurística se queda corta.

## R5. Compresor de logs y trazas

- **Decision**:
  1. Normaliza cada línea sustituyendo marcas de tiempo, UUID, hex largos,
     números y rutas temporales por comodines. **Esto solo sirve para agrupar**:
     la línea emitida es siempre una original literal.
  2. Colapsa las series consecutivas de líneas con la misma forma normalizada:
     conserva la primera y la última literales más
     `⟦mem⟧ ×312 líneas similares · ref=…`.
  3. **Nunca colapsa** líneas de nivel `ERROR`, `FATAL` o `PANIC`, ni la
     cabecera y los 5 primeros marcos de cada traza de pila; los marcos
     restantes de la traza se omiten con un marcador.
  4. En `WARN` colapsa solo las repeticiones idénticas, con recuento.
- **Diff**: conserva cabeceras, hunks con cambios y 1 línea de contexto (en
  vez de 3), y omite los ficheros renombrados o binarios con un marcador.
- **Tabla o listado**: conserva la cabecera, las N primeras y las N últimas
  filas, y las filas atípicas por columna numérica (mismo criterio que R3).
- **Rationale**: los logs son el caso de más ahorro (57 % en el caso SRE de
  Headroom) y el patrón "forma normalizada + literal representativo" no
  inventa texto (FR-011).

## R6. Compresor de prosa (sustituto determinista de Kompress)

- **Decision**: extractivo y sin reescritura:
  1. Divide en frases.
  2. Elimina frases duplicadas: igualdad exacta, o igualdad tras normalizar
     espacios y mayúsculas.
  3. Elimina frases cuya huella ya se entregó en la sesión (se apoya en R8).
  4. En párrafos largos (más de `max_para_sentences`, por defecto 8) conserva
     las primeras 3 y las últimas 2 frases más las que contienen elementos
     protegidos (código en línea, rutas, URLs, números o nombres propios en
     `CamelCase`), y omite el resto con un marcador.
- **Rationale**: Kompress es un modelo, y aquí no hay modelos (FR-001). La
  prosa de las memorias ya es densa; la meta de ahorro se concentra en JSON,
  logs y código (SC-001 mide el corpus mixto).
- **Alternatives considered**: resumen abstractivo con LLM: no determinista,
  rompe el binario único y parafrasea (FR-010).

## R7. Guarda de literalidad (FR-011) y regla "solo si mejora" (FR-005)

- **Decision**: después de cada compresor se ejecuta un verificador común:
  - **Texto** (código, log, diff, prosa, tabla): toda línea de la salida que
    no sea marcador tiene que existir **literal** en la entrada, y en el mismo
    orden relativo (subsecuencia de líneas). La única excepción es la línea
    Go reconstruida `func …{ /* ⟦mem⟧ … */ }`, cuyo prefijo hasta `{` tiene
    que ser literal.
  - **JSON**: todo valor conservado es igual, en profundidad, a un valor del
    original en la misma ruta.
  - Si falla, el bloque se entrega con la compresión estructural y se cuenta
    como `fallback: literal_guard`.
- Después: si `tokens(motor) >= tokens(estructural)`, se usa el estructural
  (`fallback: no_gain`).
- **Rationale**: convierte FR-011 en un invariante comprobable por máquina,
  no en una promesa del compresor.

## R8. Deltas de sesión (live-zone) y estabilidad de caché (CacheAligner)

- **Existe hoy**: `context_deliveries` guarda **un hash por canal** (`context`
  o `plan_context`) por sesión, y `PlanContext` suprime el documento **entero**
  si el hash coincide. Es todo o nada.
- **Decision**:
  1. **Registro por bloque**: una tabla nueva, `delivered_blocks(session_id,
     block_hash)`. Un "bloque" es cada memoria renderizada, cada sección fija
     del contexto y cada resultado de búsqueda.
  2. **Re-entrega**: al construir contexto, paquetes o búsquedas en la misma
     sesión, un bloque ya entregado e inalterado se sustituye por una línea
     compacta: `- [id] título ⟦ya entregado⟧`. Si se omiten varios seguidos,
     se agrupan: `⟦mem⟧ 14 memorias ya entregadas en esta sesión, sin
     cambios · get_memory(id) para reconsultar`.
  3. **Reinicio**: `pre-compact`, `post-compact`, el evento
     `session.compacted` de OpenCode y el inicio de sesión (`SessionStart`
     con `source=clear|startup`) vacían el registro de la sesión (FR-019).
     Hoy `hookPostCompact` no toca `context_deliveries`; se añade el
     reinicio de las dos tablas.
  4. **Orden estable → volátil** en `Builder.Build`
     (`application/usecases/build_context.go`): el protocolo, las reglas
     fijadas, las preferencias, las decisiones, los patrones, los bugfixes,
     los aprendizajes y las sinapsis van al principio, **sin marcas de
     tiempo** ni contadores cambiantes. Al final, bajo un separador
     `<!-- mem:volatile -->`, van la actividad reciente, «Código indexado»
     (cambia con cada indexado), la memoria conectada a código activo, la
     sesión activa, las sesiones recientes, las anclas sin evidencia y la
     directiva de concisión.
  5. **Orden determinista** dentro de cada sección: los empates se resuelven
     por `id`, nunca por el orden del mapa ni por la hora de lectura.
- **Rationale**: la caché de prefijo del proveedor solo funciona si los
  primeros bytes no cambian. Hoy «Actividad Reciente (auto)» y «Código
  indexado» van en medio y la invalidan en cada turno.
- **Alternatives considered**: un hash por documento (el actual) no permite
  deltas; un diff textual entre entregas es frágil ante reordenaciones.

## R9. Almacén de originales (CCR)

- **Decision**: una tabla `compression_originals` en la misma base SQLite:
  `ref` (los 12 primeros hex del sha256, con colisión resuelta ampliando a 16),
  `project`, `content` comprimido con `compress/gzip`, `raw_bytes`,
  `content_type`, `compressor`, `created_at`, `last_access_at`, `expires_at`.
  - Deduplicación: la clave es la huella, así que dos contenidos iguales
    comparten fila (`INSERT … ON CONFLICT DO UPDATE SET expires_at = max(...)`).
  - Caducidad: 7 días desde el último acceso (`last_access_at`), por defecto.
  - Tope: 256 MB por store global. La expulsión LRU corre al insertar cuando
    se supera el tope. Si falla o el disco está lleno, el motor desactiva la
    compresión con pérdida para esa petición (edge case «almacenamiento
    lleno»).
  - Los originales **no se guardan** si contienen marcas `<private>` o
    secretos detectados (`domain.RedactPrivate` y `domain.RedactSecrets`):
    ese bloque se comprime **sin pérdida** (solo estructural). Así nunca
    existe una marca que apunte a un original no recuperable (FR-024).
- **Rationale**: todo en la base existente (la migración aditiva ya es el
  patrón de `db.go`), sin ficheros sueltos y con el mismo aislamiento por
  `GOMEMORY_DATA_HOME` en pruebas (memoria 14).
- **Alternatives considered**: ficheros en disco (difícil de limpiar y de
  aislar); zstd (añade dependencia; gzip de la biblioteca estándar basta).

## R10. Formato del marcador

- **Decision**: `⟦mem⟧ <descripción breve> · ref=<12hex>`. En JSON es un
  objeto con la clave `"⟦mem⟧"`. Los delimitadores `⟦ ⟧` (U+27E6 y U+27E7) no
  aparecen en código real, son baratos en tokens y un agente los reconoce sin
  instrucciones.
- El protocolo inyectado lleva **una línea** que explica cómo recuperar:
  `⟦mem⟧ … ref=X → pack_retrieve(ref=X)`.

## R11. Reescritura de salidas de herramientas por runtime (FR-022)

Verificado con `strings` sobre los binarios instalados:

| Runtime (versión instalada) | Mecanismo | Alcance | Estado |
|---|---|---|---|
| Claude Code 2.1.282 | PostToolUse → `hookSpecificOutput.updatedToolOutput` | Todas las herramientas | **Soportado.** Si la forma no coincide con la de `tool_response`, se descarta ("does not match … output shape; using original output") |
| Codex 0.157.0 | PostToolUse → `hookSpecificOutput.updatedMCPToolOutput` | Solo herramientas MCP | **Parcial**: las salidas de shell no se pueden reescribir |
| OpenCode 1.18.x (plugin 1.18.20) | `tool.execute.after(input, output)` con `output.output` mutable | Todas | **Probable**: se valida en quickstart Q7 contra el binario 1.18.32 |
| OpenCode 2.x (rama v2 del plugin) | `ctx.tool.hook("execute.after", ev)` | ? | **Sin verificar**: se valida en Q7. Si `ev.result` no es mutable, se marca como no soportado |

- **Decision**:
  - Subcomando nuevo `mem hook tool-output <runtime>`. Recibe el JSON del
    evento, comprime **solo los campos de texto** de `tool_response`
    (`stdout`, `stderr`, `content[].text`, `output`) y devuelve **el mismo
    objeto con la misma forma**, en el campo que corresponda al runtime.
  - **Excluidas siempre**: `Read`, `Edit`, `Write`, `MultiEdit`,
    `NotebookEdit` y las herramientas de gomemory (`mcp__gomemory__*`).
    Motivo: Edit necesita el texto exacto que devolvió Read para
    `old_string`, así que comprimir Read rompe la edición, y las salidas de
    gomemory ya salen comprimidas.
  - Filtro de Claude Code en la instalación: `Bash|Grep|Glob|WebFetch|WebSearch|mcp__.*`,
    con `mcp__gomemory__` excluido dentro del subcomando.
  - Umbral de activación: salida de al menos 400 tokens (por debajo, no
    compensa el coste del hook).
  - **Opt-in** (`ToolOutputCompression`, apagado por defecto), porque
    reescribe lo que ve el modelo en herramientas ajenas a gomemory. Es el
    mismo criterio que Octopus: un flujo nuevo e invasivo nace apagado
    (memoria 5).
  - Presupuesto de tiempo: el subcomando corta a los 150 ms. Si se pasa,
    devuelve salida vacía (el runtime usa la original).
- **Alternatives considered**: comprimir `Read` "solo en archivos grandes":
  sigue rompiendo Edit en esos archivos. Descartado.

## R12. Ajuste adaptativo (práctica `headroom learn`)

- **Decision**: la tabla `compression_stats` acumula por
  `(project, compressor, content_type)`: `uses`, `raw_tokens`, `final_tokens`,
  `omissions` (marcadores emitidos), `retrievals`, `fallbacks` y
  `latency_us_total`. Cada `pack_retrieve` suma una recuperación al par
  `(compressor, content_type)` guardado en el original.
- **Agresividad** por `(project, content_type)`, del 0 al 3 (3 = máxima, valor
  inicial). Si `retrievals/omissions > 20 %` con al menos 20 omisiones en la
  ventana de 7 días, baja un escalón y se registra el motivo. Nunca sube
  sola: la persona la restablece con `mem pack tune --reset`. Así la salida
  es predecible (FR-003: la agresividad forma parte de la configuración).
- **Qué cambia cada escalón**: umbrales de `N_min` (JSON), `min_body_lines`
  (código), longitud mínima de serie a colapsar (logs) y `max_para_sentences`
  (prosa). El escalón 0 equivale a solo estructural para ese tipo.

## R13. Niveles, valores por defecto y migración de ajustes (FR-030 a FR-032)

- **Decision**: en `SettingsData` y en el `Settings` de persistencia (los
  **dos**: `Write` reconstruye `Settings` desde `SettingsData`, y un campo que
  falte se borra en silencio, como ya pasó con la política de revisión):
  - `ContextCompressionLevel string` (`"none" | "structural" | "max"`).
    Ausente o vacío → `"structural"` (**instalación existente, comportamiento
    actual**). El heredado `ContextCompressionDisabled=true` se lee como
    `"none"`.
  - `mem install` en un proyecto **sin fichero de ajustes previo** escribe
    `"max"` (instalación nueva).
  - `ToolOutputCompression bool` (opt-in, R11).
  - `ConciseOutputDirective bool` (opt-in, FR-032).
  - `CompressionMinTokens`, `CompressionOriginalsTTLDays`,
    `CompressionOriginalsMaxMB` y `CompressionAdaptiveThresholdPct` (0 = valor
    de fábrica en `domain/compression_policy.go`: 50 palabras ≈ 64 tokens, 7
    días, 256 MB y 20 %).
  - Estabilidad de caché, deltas de sesión y umbral mínimo: **siempre activos**
    en `structural` y `max`, porque no pierden información (FR-031).
    `none` los desactiva.
- **Salida idéntica en `structural` (SC-008)**: el reorden estable → volátil
  cambia la salida de `get_context`. Para cumplir SC-008 al pie de la letra,
  el reorden y los deltas **solo aplican en `max`**, y en `structural` la
  salida sigue igual byte a byte. Esto refina FR-031: lo sin pérdida está
  activo por defecto en las instalaciones nuevas, que arrancan en `max`.
  → Se deja constancia en la spec como aclaración del plan (ver plan.md,
  «Desviaciones de la spec»).

## R14. Rendimiento (SC-007)

- **Decision**: presupuesto de 50 ms por cada 10 k tokens (unos 40 KB). Todo
  es lineal salvo `go/parser` (lineal en la práctica) y la estadística de
  JSON (O(n·campos)). Los benchmarks `go test -bench` sobre el corpus
  verifican el p95. Por encima de 2 MB de entrada se aplica solo la
  compresión estructural.
- El coste de arranque del hook (proceso `mem`) ya existe en los demás hooks.

## R15. Corpus de referencia

- **Decision**: `tests/testdata/compression_corpus/` con muestras reales y
  **anonimizadas**: `go list -json`, `git log --stat`, `go test -v` con fallos,
  una traza de pánico de Go, un traceback de Python, resultados de búsqueda de
  gomemory, contexto `mem context`, un diff real del repo, ficheros Go, Python,
  TS y Java del propio repositorio o de licencia compatible, y prosa de
  memorias. Un fichero `expectations.json` fija el ahorro mínimo por
  muestra. La prueba de integración calcula SC-001, SC-002, SC-005 y SC-006.
