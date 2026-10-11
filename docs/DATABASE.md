# Persistencia de gomemory

GoMemory usa SQLite mediante `modernc.org/sqlite`, sin CGO. Cada proyecto tiene una base `mem.db` aislada en el almacén global del usuario, bajo `projects/<clave>/`. La clave deriva de la ruta canónica del proyecto; el archivo `projects/<clave>/root` registra la ruta para que la desinstalación pueda encontrar las integraciones del proyecto.

`GOMEMORY_DATA_HOME` permite elegir la raíz del almacén. La caché de versiones (`update-check.json`) vive en esa raíz y no forma parte de la base SQLite. Los archivos privados de datos y exportación se crean con permisos de propietario (0600 para archivos, 0700 para directorios).

El inventario de componentes y sus dependencias está en [architecture.md](architecture.md). La evolución del esquema y las consultas viven en `adapters/secondary/persistence/`; las operaciones de aplicación acceden a ellas por los puertos de `application/ports/`.

## Texto protegido y saneamiento

Los campos de texto libre que pueden recibir contenido del usuario o del agente —memorias, `sessions.summary`, `memory_relations.reasoning` y `channel_activity.last_error`— pasan por `domain.ProtectText` al escribirse y al leerse: se eliminan las regiones `<private>` (anidadas o sin cierre) y se ocultan las credenciales reconocidas. `compression_originals` rechaza guardar o devolver un original con contenido protegido. La lectura nunca reescribe filas.

Las filas guardadas antes de esa protección se sanean con `mem sanitize --apply` (ver el manual, sección 8). No hay tabla ni migración nueva: el saneamiento actualiza solo el campo de texto dentro de una transacción inmediata con `secure_delete`, elimina los originales protegidos y después compacta (`VACUUM` + `wal_checkpoint(TRUNCATE)`) comprobando que el checkpoint no quedó bloqueado. Su informe depurado vive fuera de la base, en `<raíz>/maintenance/privacy/<run_id>.json`; los snapshots que reescribe están en `<raíz>/backups/<clave>/`.
