# Modelo de datos — feature 037

La feature conserva las tablas existentes y agrega modelos de dominio/transporte y un informe de mantenimiento en archivo. No modifica ids, fechas ni esquema de producción. Véase [contrato de privacidad](contracts/privacy-maintenance.md).

## Texto protegido

Transformación pura: eliminar regiones privadas completas o abiertas hasta fin de texto; después ocultar credenciales reconocidas. Etiquetas ASCII `<private>` y `</private>` sin distinguir mayúsculas. Mantener profundidad y texto exterior; cierre aislado se descarta sin ocultar el texto público posterior. La transformación es idempotente.

Los índices se calculan sobre el texto original sin convertir toda la cadena a minúsculas, evitando cambiar offsets Unicode. Detector de privacidad y redactor comparten la misma regla. Originales públicos recuperables conservan exactitud; originales protegidos se rechazan en escritura/lectura y se incluyen en saneamiento histórico.

## Registros existentes

| Entidad | Campo protegido | Identidad/público preservado | Operaciones |
| --- | --- | --- | --- |
| Memoria | Campos textuales ya cubiertos por política actual | id, tipo, proyecto y metadatos | Mantener protección de guardado/importación/consulta |
| Sesión | summary | id, proyecto, conversación, created_at, ended_at | Guardar, actualizar, terminar, consultar y emitir contexto |
| Relación | reasoning | id, extremos A/B, proyecto, tipo, confidence, created_at | Crear, actualizar, importar, consultar y exportar |
| Actividad de canal | last_error | proyecto, agente, ámbito, tipo y marcas temporales | Registrar error y consultar diagnóstico |
| Original comprimido | Contenido recuperable | referencia y contenido público exacto | Rechazar protegido, mantener público intacto |

La consulta normal devuelve valores depurados sin persistir cambios de lectura. El mantenimiento usa candidatos internos no expuestos al usuario para inspeccionar texto original. No se deben reutilizar lectores depurados para descubrir los candidatos, pues ocultarían qué filas requieren saneamiento.

## Candidato interno de saneamiento

Campos: `category`, `record_id` (string para identidad simple o compuesta), `project`, `field`, `text` solo en memoria del proceso. `text` nunca sale en preview, informe, log o error. Categorías de filas: session, relation y channel_error. Las copias y derivados se representan separadamente como artefactos.

Validación: categoría/campo en lista permitida; proyecto coincide con el alcance; identificador proviene de persistencia, no de SQL suministrado por usuario. Actualizar solo texto; conservar metadatos. El adapter usa parámetros para los valores y sentencias fijas por categoría.

## Informe de saneamiento

Archivo `DataHome/maintenance/privacy/<run_id>.json`, directorio privado y archivo 0600 donde sea aplicable; publicar por temporal y reemplazo seguro. Campos:

| Campo | Tipo | Regla |
| --- | --- | --- |
| schema_version | entero | 1 |
| run_id | identificador | Generado al aplicar, no incluye contenido |
| project | identificador | Proyecto resuelto por ProjectRepo |
| mode | enum | preview o apply |
| status | enum | preview, pending_cleanup, completed o failed |
| categories | lista | category, affected_count, record_ids; sin fragmentos ni hashes nuevos de secretos |
| rows_changed | entero | Solo filas realmente modificadas en esta aplicación |
| artifacts | lista | Identidad relativa, categoría, status y error_code depurado |
| pending | lista | Códigos y acciones operativas sin contenido sensible |
| external_copies | aviso | Fuera de control; no se inventa inventario de rutas externas |
| started_at / finished_at | fechas | Reloj inyectable en pruebas |

Preview es transitorio y no crea informe persistente ni muta registros. Apply crea un informe pending_cleanup antes de cambios; ese estado es conservador tanto si falta commit como si falta limpieza. Reintento vuelve a evaluar filas y artefactos, sin restaurar texto privado. No existe una promesa de commit atómico entre DB y manifest.

Estados:

```text
preview --sin apply--> sin cambios
apply --> pending_cleanup --> completed
                  └--fallo/busy/crash--> pending_cleanup --reintento--> completed
fallo antes de poder crear informe --> failed, sin cambios DB
```

La aplicación lógica utiliza una transacción inmediata única. Si se cancela antes del commit, rollback; si se cancela después, mantener filas ya saneadas e informe pendiente. Antes de completed deben verificarse copias controladas, contexto, originales y checkpoint. Ningún código de salida cero acompaña pending_cleanup.

## Resultado de contexto

Configuración estable compartida: proyecto, root, límites, flags y ports. Estado por llamada: contador de descartes, documento construido y métricas de emisión. `Build` no escribe el contador de otra llamada; recorder debe persistir cada observación separadamente sin acumulaciones falsas. Referencias temporales equivalentes son requisito para comparar igualdad de contenido.

## Artefacto instalable

Campos conceptuales: release_tag, platform, asset_name, expected_sha256, actual_sha256, staged_path y state. Estados: resolved -> downloaded -> verified -> prepared -> published. Cualquier fallo previo a published conserva el destino anterior; no se ejecuta contenido en estados previos a verified. Una selección latest se convierte una sola vez en release_tag estable.

## Resultado y lifecycle OpenCode

Resultado soportado: `content: string`, `content: Part[]` o `output: string`. Prioridad de selección coincide con resultText existente. Part textual tiene type=text y text; otras partes son opacas. Al reducir un arreglo, sustituir primera parte textual conservando sus metadatos, eliminar posteriores partes textuales que formaban el mismo texto agregado y conservar no-textuales en su orden relativo. Sin partes textuales no hay transformación.

Estado propio: initializing -> active -> closing -> closed; fallo inicial pasa a closing con rollback. Recursos: registros de hooks, controller, subscription promise, operaciones en curso, hijos y cleanup promise única. Una llamada después de closing no inicia IO. Un cierre fallido conserva error observable y no se declara closed satisfactorio con recursos pendientes.

## Registro de evidencia

Artefacto de validación, no nueva tabla: finding_id, requirement_ids, scenario_ids, revision, environment, command, expected, observed, result y evidence_paths. `result`: confirmed_open, corrected_pending_verification o closed. Host/Windows ausentes mantienen corrected_pending_verification. No adjuntar payloads reales o credenciales a evidencia.
