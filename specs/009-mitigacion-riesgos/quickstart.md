# Quickstart de validación — Mitigación de riesgos operativos de gomemory

Guía para comprobar end-to-end que cada mejora funciona, una vez implementada. No repite el diseño (ver `plan.md`, `data-model.md`, `contracts/`).

## Prerrequisitos

- Repo compilado (`go build ./...`) con la feature implementada.
- Un proyecto de prueba con al menos unas pocas memorias guardadas (puede ser el propio `go_memory`).

## 1. Backup automático (User Story 1, P1)

```sh
# Cerrar una sesión activa dispara el snapshot best-effort
mem session end -s "prueba de snapshot"

# Verificar que apareció un snapshot nuevo
ls -la "$(mem config datahome 2>/dev/null || echo ~/.local/share/gomemory)/backups/<project-key>/"

# Simular pérdida de datos: mover el mem.db activo
mv .memory/mem.db /tmp/mem.db.bak 2>/dev/null || true  # o la ruta global resuelta

# Restaurar desde el snapshot más reciente
mem import <ruta-del-snapshot-mas-reciente>.json

# Confirmar que las memorias vuelven a estar disponibles
mem search "" --limit 50
```

**Resultado esperado**: el snapshot existe sin haber corrido `mem export` manualmente; tras `mem import`, las memorias reaparecen; repetir el `import` no duplica (dedup por hash de contenido, ya existente en `ImportBundle`).

## 2. Redacción de secretos + permisos (User Story 2, P2)

```sh
# Guardar una memoria con un secreto de PRUEBA (no una clave real) fuera de <private>
mem save -t "prueba redaccion" -y decision "clave de ejemplo: AKIAIOSFODNN7EXAMPLE"

# Inspeccionar el contenido persistido directamente en la BD
sqlite3 "$(mem config dbpath 2>/dev/null)" "SELECT content FROM memories ORDER BY id DESC LIMIT 1;"
# Esperado: contiene "[REDACTED:aws-key]", no la clave literal

# Verificar permisos del archivo de BD
ls -l "$(mem config dbpath 2>/dev/null)"
# Esperado: -rw------- (0600), solo el propietario puede leer/escribir
```

**Resultado esperado**: el patrón de secreto queda redactado sin que el usuario lo envolviera en `<private>`; contenido sin patrones conocidos se guarda intacto (probar con una memoria normal para confirmar que no hay falsos positivos); el archivo de BD no es legible por otros usuarios del sistema.

## 3. Búsqueda por relevancia (User Story 3, P3)

```sh
# Guardar dos memorias donde el término aparece con distinta relevancia
mem save -t "gomemory arquitectura" -y decision "nota corta sobre otra cosa, menciona gomemory una vez"
mem save -t "nota random" -y decision "gomemory gomemory gomemory: todo el contenido es sobre gomemory"

# Buscar el término
mem search "gomemory"
```

**Resultado esperado**: la memoria con mayor densidad/relevancia del término aparece primero, no necesariamente la más reciente. Repetir la búsqueda tras forzar (si es posible en el entorno de prueba) un build sin FTS5, y confirmar que sigue devolviendo resultados correctos vía el fallback LIKE.

```sh
# Verificar sincronización del índice tras editar/borrar
mem forget <id-de-una-de-las-memorias-anteriores>
sqlite3 "$(mem config dbpath 2>/dev/null)" "SELECT count(*) FROM memory_search WHERE memory_id = <id-borrado>;"
# Esperado: 0
```

## 4. Convención de compatibilidad documentada (User Story 4, P4)

```sh
grep -n "aditiv" adapters/secondary/persistence/db.go
grep -n "ExportVersion" -A3 domain/portability.go
```

**Resultado esperado**: ambos comentarios existen exactamente en los puntos donde se aplican los cambios (no en un documento separado que se pueda perder de contexto).

## Notas

- Los comandos `mem config datahome`/`mem config dbpath` son ilustrativos — usar el mecanismo real de inspección que exista en el CLI (o `echo ~/.local/share/gomemory/projects/<key>/mem.db` si no hay subcomando dedicado).
- Ninguna prueba de este quickstart requiere una clave o token real — todos los ejemplos de secretos son literales de prueba sin valor.
