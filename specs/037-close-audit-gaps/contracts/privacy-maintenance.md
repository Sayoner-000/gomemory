# Contrato de privacidad y mantenimiento

Requisitos FR-001–008. El contrato es diseño propuesto, todavía sin implementación.

## Entradas y salidas existentes

La consola, herramientas MCP de sesiones, eventos compact-summary/channel-error, creación/edición/importación de relaciones y exportación conservan nombres, parámetros, ids y estructura. Depurar contenido protegido antes de persistir, recortar o emitir, incluyendo respuesta inmediata y registros históricos. No agregar banners a JSON/NDJSON.

No persistir regiones privadas incompletas en originales de compresión. `Put` rechaza contenido protegido; `Get` de un original histórico protegido devuelve error depurado y nunca bytes privados. Originales públicos conservan recuperación exacta y referencias vigentes.

## Nueva interfaz CLI

```text
mem sanitize [--json] [--timeout <duración>]
mem sanitize --apply [--json] [--timeout <duración>]
```

- Alcance: proyecto actual resuelto por ProjectRepo; no parámetro de ruta arbitraria a DB ni modo all-projects.
- Default: preview, sin escritura de filas ni informes. Enumerar categorías, ids y cantidades sin texto original o depurado.
- `--apply`: autorización explícita para sanear ese proyecto y sus artefactos controlados; sin pregunta interactiva adicional ni respaldo crudo. Recalcular candidatos dentro de la transacción; la vista previa anterior es informativa.
- `--timeout`: duración positiva, default 60s; controla consultas y mantenimiento, distinto del cierre plugin. Si vence después del commit, conservar estado pending_cleanup y permitir reintento con duración mayor.
- JSON: un objeto en stdout, diagnóstico depurado en stderr; texto humano usa componentes de consola existentes. No incluir secretos, fragmentos, excepciones crudas o hashes recién calculados de secretos.

Salida de ejemplo válida:

```json
{
  "schema_version": 1,
  "mode": "preview",
  "project": "project-key",
  "status": "preview",
  "categories": [
    {"category": "session", "affected_count": 1, "record_ids": ["session-id"]}
  ],
  "rows_changed": 0,
  "artifacts": [],
  "pending": [],
  "external_copies": "Fuera de control de gomemory; revisar copias externas."
}
```

Apply añade run_id, fechas y resultados de artefactos. Identidades de artefactos se expresan respecto al root administrado; rutas públicas que incluyan patrones protegidos se depuran. Sin cambios: apply puede terminar completed con rows_changed=0, pero debe verificar residuos pendientes de ejecuciones anteriores.

| Código | Significado |
| --- | --- |
| 0 | Preview satisfactorio o aplicación y limpieza verificadas |
| 1 | Fallo de operación, timeout o pending_cleanup; reintentar según diagnóstico |
| 2 | Argumento o duración inválida; no iniciar saneamiento |

Códigos de diagnóstico: SANITIZE_FAILED, SANITIZE_BUSY, SANITIZE_PENDING_CLEANUP y SANITIZE_TIMEOUT. No devolver un mensaje crudo del driver o del contenido del archivo. No agregar herramienta MCP de apply en esta feature: evitar nueva superficie destructiva automática.

## Puertos y transacción

Interfaces previstas en application/ports, todos los métodos de IO reciben context.Context:

- `PrivacyMaintenanceRepository.Preview(ctx, project)` devuelve candidatos internos tipados.
- `PrivacyMaintenanceUnitOfWork.WithTransaction(ctx, project, callback)` inicia transacción y entrega `PrivacyTransaction` sin SQL/driver; callback lista candidatos actuales y aplica reemplazos de texto permitidos. Commit solo si callback finaliza sin error; rollback ante error/cancelación.
- `PrivacyArtifactsPort` inventaría y sanea artefactos permitidos, verifica residuos y publica informes depurados. Nunca sigue enlaces fuera de raíces administradas.

El caso de uso controla orden y transformación; el adaptador controla SQLite, archivos, permisos y publicación. No usar SELECT * ni sentencias construidas con categorías externas. No actualizar created_at, ended_at, confidence ni ids durante saneamiento.

## Limpieza y recuperación

1. Persistir manifest conservador pending_cleanup antes de escritura DB; si no puede persistirse, abortar sin cambios.
2. Transacción inmediata con secure_delete en su conexión: releer candidatos y actualizar campos; rollback completo antes de commit si falla.
3. Reescribir JSON de backups propios con temporal depurado, regenerar `.memory/context.md` y eliminar originales reconocidos como protegidos sin modificar los públicos. JSON desconocido, archivo inaccesible o enlace inseguro mantiene pendiente y se informa; no se elimina a ciegas.
4. Con clientes antiguos detenidos, cerrar statements; compactar con VACUUM y ejecutar checkpoint TRUNCATE fuera de transacción. Comprobar valores de busy/log/checkpointed; nunca borrar WAL/SHM manualmente.
5. Verificar lecturas, copias y residuos sintéticos de prueba antes de completed. Fallos de compactación, espacio o bloqueo dejan pendientes y salida 1. No crear copias crudas para rollback; recuperación repite la transformación idempotente.

No se recorren exportaciones arbitrarias, snapshots de sistema o medios externos. Esos límites aparecen en cada informe, sin declarar borrado físico total. Escrituras de versiones nuevas están protegidas; clientes antiguos deben reiniciarse antes del cierre histórico.
