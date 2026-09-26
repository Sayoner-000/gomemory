# Persistencia de gomemory

GoMemory usa SQLite mediante `modernc.org/sqlite`, sin CGO. Cada proyecto tiene una base `mem.db` aislada en el almacén global del usuario, bajo `projects/<clave>/`. La clave deriva de la ruta canónica del proyecto; el archivo `projects/<clave>/root` registra la ruta para que la desinstalación pueda encontrar las integraciones del proyecto.

`GOMEMORY_DATA_HOME` permite elegir la raíz del almacén. La caché de versiones (`update-check.json`) vive en esa raíz y no forma parte de la base SQLite. Los archivos privados de datos y exportación se crean con permisos de propietario (0600 para archivos, 0700 para directorios).

El inventario de componentes y sus dependencias está en [architecture.md](architecture.md). La evolución del esquema y las consultas viven en `adapters/secondary/persistence/`; las operaciones de aplicación acceden a ellas por los puertos de `application/ports/`.
