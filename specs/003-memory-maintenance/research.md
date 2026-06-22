# Research: Mantenimiento de Memoria (Purga, Compactación y Garbage Collector)

Todas las decisiones de alcance críticas (significado de "uninstall", disparo del GC, alcance por defecto de la purga) ya se resolvieron en `spec.md` vía `/speckit.clarify` durante `/speckit.specify`. Este documento resuelve las decisiones **técnicas** necesarias para pasar de la especificación al diseño.

## 1. Puerto nuevo (`MaintenanceRepository`) en vez de ensanchar `MemoryRepository`

- **Decision**: crear un puerto dedicado `application/ports/maintenance_repository.go` en vez de agregar métodos de borrado/compactación a `ports.MemoryRepository`.
- **Rationale**: `MemoryRepository` (Insert/List/Search) ya es consumido por MCP, TUI y varios comandos CLI de uso cotidiano. Mezclar ahí operaciones destructivas obligaría a todos los mocks y consumidores existentes a lidiar con un puerto más ancho y más peligroso. Un puerto separado respeta segregación de interfaces y deja claro, solo con el import, qué código puede borrar datos.
- **Alternatives considered**:
  - Ensanchar `MemoryRepository` — rechazado por ensanchar innecesariamente un puerto ya estable y muy usado.
  - Capa de casos de uso (`application/usecases/maintenance.go`) — rechazado por sobre-ingeniería: el patrón actual del proyecto (`cmd_save.go`, `cmd_list.go`, etc.) resuelve comandos llamando directo al repositorio desde la CLI/TUI; no hay lógica de orquestación compleja que justifique una capa intermedia.

## 2. Filtro de antigüedad en días enteros, no `time.Time`

- **Decision**: `PurgeFilter.OlderThanDays int`, resuelto en SQL con `datetime('now', '-N days')`.
- **Rationale**: `domain.Memory.CreatedAt` ya es `string` (no `time.Time`), poblado por la constante `Now = "datetime('now', '-5 hours')"` en `db.go`. Mantener esa convención evita conversiones de zona horaria innecesarias — el proyecto fija UTC-5 sin DST a propósito (Principio II) — y reutiliza el mismo patrón SQL ya validado en producción.
- **Alternatives considered**: pasar `time.Time` desde la capa CLI y formatear a string en el adaptador — rechazado por introducir una conversión de zona horaria redundante cuando SQLite ya resuelve `datetime('now', '-N days')` de forma nativa y consistente con `Now`.

## 3. `VACUUM` como mecanismo de compactación

- **Decision**: `Compact()` ejecuta `VACUUM;` sobre la conexión existente.
- **Rationale**: SQLite soporta `VACUUM` nativo para reclamar el espacio de páginas libres dejadas por `DELETE`s; no requiere reescritura manual de filas ni dependencias nuevas.
- **Alternatives considered**: `PRAGMA auto_vacuum=INCREMENTAL` + `PRAGMA incremental_vacuum` — rechazado porque cambiar el modo `auto_vacuum` de una base **ya existente** no es retroactivo en SQLite (requeriría recrear el archivo), mientras que `VACUUM` funciona sobre cualquier base existente sin migración previa.

## 4. Limpieza de relaciones huérfanas explícita (sin `ON DELETE CASCADE`)

- **Decision**: tras cada `DELETE FROM memories`, ejecutar en la misma transacción `DELETE FROM memory_relations WHERE memory_id_a NOT IN (SELECT id FROM memories) OR memory_id_b NOT IN (SELECT id FROM memories)`.
- **Rationale**: el esquema actual (`db.go`) declara las `FOREIGN KEY` en `memory_relations` pero no activa `PRAGMA foreign_keys = ON` ni `ON DELETE CASCADE`, así que SQLite no limpia relaciones automáticamente al borrar una memoria. La limpieza explícita es la única forma de cumplir FR-004 sin modificar el esquema existente (fuera de alcance de este feature).
- **Alternatives considered**: activar `PRAGMA foreign_keys = ON` + migrar el esquema a `ON DELETE CASCADE` — rechazado por ser un cambio de esquema más amplio, no necesario para resolver el requisito, y con riesgo de romper inserciones existentes si hay datos huérfanos previos a esta feature.

## 5. Auto-eliminación segura del binario en `mem uninstall`

- **Decision**: en Linux/macOS, `mem uninstall` borra el binario destino con `os.Remove` aunque sea el mismo ejecutable en uso. En Windows, si `os.Remove` falla por bloqueo de archivo, se completa el resto de la limpieza (hooks, configs, `.memory/`) y se informa al usuario que debe borrar el binario manualmente tras cerrar el proceso.
- **Rationale**: en POSIX, `unlink()` sobre un ejecutable en ejecución es válido — el inodo persiste hasta que el proceso termina, el archivo simplemente desaparece del directorio. En Windows, el sistema de archivos bloquea ejecutables en uso y la eliminación falla. Este comportamiento asimétrico ya se documentó como lección previa (memoria de proyecto: bug de auto-copia en `cmd_install.go`, donde no comparar identidad de archivo con `os.SameFile` antes de operar sobre el binario propio causó "text file busy" / auto-borrado accidental). `mem uninstall` aplica la misma cautela pero en sentido inverso: identificar primero si el destino es el binario en ejecución, y manejar el resultado de la eliminación sin abortar el resto de la limpieza.
- **Alternatives considered**: requerir que el usuario ejecute `mem uninstall` desde un binario **distinto** al que va a eliminar (ej. copia temporal) — rechazado por agregar friction a la UX sin necesidad en Linux/macOS, que son las plataformas de uso real del proyecto (Principio de Stack: "portable entre Linux, macOS y Windows", no exclusivamente Windows-first).

## 6. Exclusión deliberada de MCP para estas 4 capacidades

- **Decision**: `purge`, `gc`, `compact` y `uninstall` se exponen solo vía CLI (`mem purge|gc|compact|uninstall`) y vía TUI (purge/gc/compact; uninstall queda CLI-only, simétrico a `mem install` que tampoco tiene equivalente en TUI). Ninguna se registra como tool MCP en `adapters/primary/mcp/server.go`.
- **Rationale**: el Principio IX de la constitución pide MCP como integración primaria para exponer funcionalidad a agentes AI, pero estas 4 operaciones son destructivas y la especificación exige confirmación humana explícita (FR-002, FR-013, SC-005). Un tool MCP con `autoApprove` habilitado (mecanismo ya existente en `settings.go` para otras tools) permitiría que un agente AI purgara o desinstalara sin supervisión humana real — lo cual viola la intención de la confirmación, no solo la letra. Se documenta como decisión de alcance explícita y acotada a este feature, no como deuda técnica pendiente.
- **Alternatives considered**: exponerlas vía MCP pero excluidas de la lista `AutoApproveTools` por defecto — rechazado porque un usuario podría agregarlas manualmente a `auto_approve_tools` en `settings.json` sin entender el riesgo, y el costo de prevenir ese caso (validación especial en `ApplyAutoApprove`) es mayor que simplemente no exponerlas.

## 7. Formato de tamaños legibles con `dustin/go-humanize`

- **Decision**: usar `humanize.Bytes(uint64)` para mostrar tamaños de archivo antes/después en los reportes de `mem compact` y `mem purge|gc` (FR-007, FR-011).
- **Rationale**: la librería ya está en el árbol de dependencias como indirecta (`go.mod`, probablemente traída por `bubbles`/`bubbletea`); promoverla a dependencia directa no agrega peso nuevo al binario y evita reinventar el formateo de bytes a KB/MB/GB.
- **Alternatives considered**: formatear a mano con `fmt.Sprintf` y división manual por 1024 — rechazado por ser código repetido sin necesidad cuando la librería ya está presente.

**Output**: todas las incógnitas técnicas quedaron resueltas; no quedan `NEEDS CLARIFICATION` pendientes para Fase 1.
