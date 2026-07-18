# Contratos internos — Mitigación de riesgos operativos de gomemory

gomemory no expone una API HTTP pública; sus contratos externos son las tools MCP (`save_memory`, `search_memories`, etc.) y los comandos CLI (`mem export`, `mem import`, ...). **Ninguno de esos contratos externos cambia** con esta feature: mismos parámetros de entrada, mismo tipo de retorno, mismo comportamiento observable desde el caller (salvo mejor orden de resultados en `search_memories`, que ya era no determinista frente a relevancia real).

Se documentan aquí los contratos **internos** nuevos que otras partes del código (y features futuras) pueden empezar a depender:

## `domain.RedactSecrets`

```go
// RedactSecrets reemplaza patrones de secretos conocidos por un placeholder.
// No es configurable; complementa a RedactPrivate, no la reemplaza.
func RedactSecrets(s string) string
```

- **Precondición**: ninguna (acepta cualquier string, incluyendo vacío).
- **Postcondición**: el string devuelto no contiene ninguna subcadena que matchee los patrones fijos (AWS, GitHub, proveedores de IA, Slack, JWT, PEM). Contenido que no matchea ningún patrón se devuelve sin cambios.
- **Invariante de composición**: debe poder encadenarse con `RedactPrivate` en cualquier orden sin que el resultado dependa del orden (ambas son reemplazos de subcadenas independientes entre sí).

## `usecases.CreateSnapshot`

```go
// CreateSnapshot exporta project a un archivo JSON con timestamp bajo backupDir,
// y poda los snapshots más antiguos si se supera keep. Nunca retorna un error que
// deba abortar el flujo del caller — el caller decide si lo registra o lo ignora.
func CreateSnapshot(memRepo ports.MemoryRepository, relRepo ports.RelationRepository, project, backupDir string, keep int) (path string, err error)
```

- **Precondición**: `backupDir` no necesita existir de antemano (la función lo crea si falta).
- **Postcondición**: si no hay error, existe un archivo en `path` con un `domain.ExportBundle` válido (mismo formato que `mem export` genera hoy), y el número de archivos de snapshot para `project` en `backupDir` es ≤ `keep`.
- **Contrato de uso desde `hookSessionEnd`**: el caller SIEMPRE debe tratar un error de esta función como no fatal (log opcional, nunca abortar el cierre de sesión).

## `persistence.SearchMemories` (dispatcher — firma sin cambios)

```go
func (r *MemoryRepo) SearchMemories(project, query string, limit int) ([]domain.Memory, error)
```

- **Sin cambios de firma ni de tipo de retorno.** El contrato observable para cualquier caller (CLI, MCP tool `search_memories`) es idéntico al actual.
- **Cambio de comportamiento interno permitido por este contrato**: el orden de los elementos devueltos puede diferir del orden actual (bucket title/content/none + recencia) porque ahora refleja relevancia BM25 cuando FTS5 está disponible. Ningún caller existente depende de un orden específico más allá de "más relevante primero" (ya documentado como best-effort hoy).

## `memory_search` (tabla FTS5 — contrato de esquema, no de API)

- Tabla interna, nunca expuesta directamente a callers de MCP o CLI.
- Cualquier código que escriba en `memories` (`InsertMemory`, futuras `UpdateMemory`/`DeleteMemory`) DEBE mantener el dual-write hacia `memory_search` como parte del mismo commit de transacción — este es el contrato de consistencia que reemplaza a "no hay contrato" hoy.
