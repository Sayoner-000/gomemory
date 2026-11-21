# Informe ACR + ADM — búsqueda dirigida de bugs

Fecha: 2026-11-21. Revisión: `7cd5e35768c65459cc0798274a0d7aed84a91fcf`.
Árbol auditado limpio; tree: `91aa9c2b65c110ae6467036e4cfe7e9e26f8a308`.
Revisión persistida: `acr_6d40dfd8-adce-48d9-bff2-9c6073de210f`.

## Seguimiento — corrección e investigación autorizadas

Estado posterior al informe original, publicado en `v2.55.1`.

| Hallazgo | Resultado actual |
|---|---|
| C-002 | **CORREGIDO**, con recuperación durable y pruebas de reinicio |
| C-001 | **CORREGIDO**, con validación de privacidad en descubrimiento, recuperación y persistencia |
| C-003 | **CONFIRMADO EN EJECUCIÓN y CORREGIDO** por guardado exclusivo de metadatos |
| C-004 | **CORREGIDO**, con renombre atómico no-replace por plataforma y prueba determinista |

### Corrección C-002

Esquema SQLite 14: `hcd_pending_retargets`. `SaveMetadata` guarda metadatos e intención de reapunte en una transacción; `Show` recupera pendientes antes de cargar/reconciliar. El pendiente solo se elimina tras completar el reapunte idempotente. Si falla el historial o el acuse, se informa y se conserva la intención para otro proceso. Se mantiene la separación entre archivos y SQLite; no se promete una transacción distribuida entre ambos.

Prueba rojo observada por el implementador sobre el código original: `retry falsely succeeded while retarget still fails`. Verde: seis combinaciones de renombre explícito/manual y fallos inyectados en memorias, enlaces y eliminación del pendiente, reabriendo SQLite. Archivos: `application/usecases/hcd_rename_recovery_test.go`, `adapters/secondary/persistence/hcd_metadata_test.go`.

Límite: enlaces ya rotos por ejecuciones anteriores que perdieron la ruta original no se reconstruyen automáticamente.

### Corrección C-001

`domain.ValidateMemoryMetadata` ahora rechaza rutas o slugs con texto protegido/credenciales antes de leer, listar ambigüedades, recuperar pendientes o persistir:
- Casos de uso (`application/usecases/hcd.go`): `loadDoc`, `findDoc`, `recoverRetarget` y `HistoryEntries`.
- Fronteras de persistencia: `HCDStore.CreateActive`, `HCDStore.Save` (`adapters/secondary/persistence/hcd.go`), `HCDStore.SaveMetadata` (`adapters/secondary/persistence/hcd_metadata.go`) y `DecisionHistoryRepository.RetargetPath` (`adapters/secondary/persistence/decision_history.go`, incluido el caso `from == to`).
- Los errores devuelven `domain.ErrValidation` sin exponer la ruta sensible.

Pruebas: `TestAuditC001DiscoveredPrivatePathReachesMemory` (convertida en regresión en `application/usecases/audit_suspicions_test.go`) y `TestHCDPrivatePathsRejectedBeforeUse` / `TestHCDPrivacyPersistenceBoundaries` (`application/usecases/hcd_path_privacy_test.go`). Rojo observado antes del parche; verde tras aplicarlo.

Nota: en `HistoryEntries`, un único documento en `hcd/` con nombre sensible detiene la reconstrucción del historial del proyecto (fail-closed). Filas históricas ya corruptas previas a este cambio siguen requiriendo saneamiento aparte.

### Investigación de las sospechas

- **C-003:** se suspendió el guardado de reconciliación, se completó `Cancel`, y luego se liberó el guardado antiguo: el código original restauró activo/active=true manteniendo la aprobación de cancelación. El nuevo `SaveMetadata` no escribe estado, foco, carril ni motivo. La reproducción se convirtió en `TestAuditC003ReconcilePreservesCancelledUnit`, que verifica que la cancelación persiste bajo el mismo intercalado.
- **C-004 (CORREGIDO):** `MoveDoc` (`adapters/secondary/hcdfiles/workspace.go`) reemplaza el patrón `Lstat + os.Rename` por `renameNoReplace` atómico no-replace por plataforma:
  - Darwin (`move_darwin.go`): `unix.RenamexNp(..., unix.RENAME_EXCL)`, con fallback `linkAndRemove` ante `ENOTSUP`/`EINVAL`.
  - Linux (`move_linux.go`): `unix.Renameat2(..., unix.RENAME_NOREPLACE)`, con fallback `linkAndRemove` ante `ENOSYS`/`EINVAL`/`EOPNOTSUPP`.
  - Windows (`move_windows.go`): `windows.MoveFileEx(..., 0)` (sin `MOVEFILE_REPLACE_EXISTING`).
  - Otros Unix (`move_other.go` + `move_fallback.go`): `os.Link` + `os.Remove` (en caso de fallo de `Remove`, solo revierte `dst` si `src` sigue existiendo y `os.SameFile(src, dst)` confirma el mismo inodo).
  - Si `src == dst`, verifica existencia y rechaza con `domain.ErrNotAllowed` (evitando el no-op de APFS).
  - Pruebas: `TestAuditC004DeterministicMoveDoc` (overlay aislado, 20/20 con `-race`: `MoveDoc` rechaza destino interviniente con `domain.ErrNotAllowed` sin alterar destino ni origen), `TestMoveDocConcurrentSingleWinner` (8 goroutines compitiendo), `TestMoveDocEdgeCases` y `TestLinkAndRemoveNoOverwrite`.

### Validación posterior

- Compilación cruzada `adapters/secondary/hcdfiles` para `darwin`, `linux`, `windows` y `freebsd`: **PASA**.
- `go test ./... -count=1 -timeout=180s`: **PASA**, incluidos contratos e integración.
- `go test -race ./adapters/secondary/hcdfiles -count=1 -timeout=120s`: **PASA** (incluyendo 20 repeticiones de la prueba determinista con overlay).
- `go test -race ./application/usecases ./adapters/secondary/persistence ./adapters/secondary/hcdfiles -run 'TestAuditC00[134]|TestHCDPrivate|TestHCDPrivacy|TestHCDRetargetRecovery|TestHCDMetadata' -count=1 -timeout=120s`: **PASA**.
- `git diff --check`: **PASA**.
- Verificación independiente posterior (solo lectura): Claude y Codex **RESOLVED para C-002** y **C-001**; para **C-004**, Claude **RESOLVED sin regresiones**, y Codex validó las llamadas nativas y señaló un riesgo en el rollback de `linkAndRemove` (corregido mediante comprobación `os.SameFile` antes de que Codex alcanzara su límite de cuota).
- El ledger ACR original (`acr_6d40dfd8-adce-48d9-bff2-9c6073de210f`) se conserva como registro histórico de la auditoría inicial; las verificaciones posteriores documentan los cierres publicados en `v2.55.1`.

El resto del documento conserva el informe original y sus estados al momento de la auditoría.

## 1. Resumen de salud del código

Auditoría dirigida, no exhaustiva del repositorio. Alcance efectivo: renombre y reconciliación HCD, persistencia de unidades, enlaces de memorias y protección de metadatos. Se hicieron lecturas parciales adicionales de saneamiento histórico e instincts; no representan una auditoría completa de privacidad.

Tecnologías inspeccionadas: Go en producción y pruebas, SQL SQLite en persistencia. No se auditó frontend, instaladores, CI ni dependencias.

**Resultado: un bug MEDIO confirmado por dos revisores y reproducido; tres sospechas independientes (una ALTA, una MEDIA y una BAJA).** Riesgo demostrado: medio. Riesgo potencial pendiente de confirmar: alto.

El proyecto ya dispone de escritura de documentos por temporal y rename, validación de metadatos de memorias, transacción para reapuntar enlaces y pruebas del error inicial de reapunte. El hueco confirmado está en la recuperación entre operaciones que individualmente pueden ser correctas.

Revisores independientes, con el mismo encargo y sin intercambiar hallazgos:
- A: Codex CLI, modelo observado `gpt-6.1-sol`, sandbox read-only.
- B: Claude CLI, modelo observado `claude-opus-5-5`, herramientas limitadas a Read/Glob/Grep, modo plan.

El registro del motor identifica los ejecutores como `codex CLI` y `claude CLI`; los modelos anteriores proceden de sus salidas reales.

**Veredicto del motor: APPROVED.** Su política no bloqueó por un confirmado MEDIO ni por sospechas. Esto NO significa ausencia de bugs, cierre del defecto ni validación integral. No se aplicaron correcciones ni se autorizó publicación.

### Cobertura y validación

| Comprobación | Resultado |
|---|---|
| HEAD/tree y estado Git | Identidad congelada; árbol limpio antes de escribir este informe |
| CodeGraph | Consultado; extracción 27 frente a motor 29: limitación declarada |
| codebase-memory | Consultado; generación 2026-11-08, sin watcher; varios archivos modificados desde indexado |
| Cobertura de rutas probatorias | Consultada; fuente actual leída para rutas obsoletas; test de renombre no registrado |
| Trazas | RedactSecrets inbound/outbound, paginación parcial; no se afirma exhaustividad |
| Dos revisores | Completados independientemente; 3 y 2 hallazgos respectivamente |
| `go test ./application/usecases ./adapters/secondary/hcdfiles ./adapters/secondary/persistence -run 'Rename|Reconcile' -count=1` | Pasa; los dos paquetes de adaptadores informan **no tests to run** para ese filtro |
| Reproducción por overlay Go | FALLA como se esperaba: el segundo intento no llama a RetargetPath |
| Suite completa, race, análisis de vulnerabilidades | No ejecutados |
| Cambios de producción | Ninguno |

## 2. Hallazgos y fugas detectadas

| ID motor | Severidad | Estado ACR | Problema |
|---|---|---|---|
| C-002 | Media | CONFIRMED, reproducido | Reintento devuelve éxito sin reparar reapunte fallido |
| C-001 | Alta | SUSPECT, solo A | Rutas descubiertas con credenciales pueden persistirse sin validación |
| C-003 | Media | SUSPECT, solo A | Reconciliación concurrente puede sobrescribir estado de cancelación |
| C-004 | Baja | SUSPECT, solo B | Carrera entre comprobar destino y renombrar documento |

### C-002 — Reintento con éxito falso tras fallar el reapunte

- **Categoría:** integridad / recuperación / éxito falso.
- **Ubicación:** `application/usecases/hcd.go:458-486` y `:521-529`; `adapters/secondary/persistence/decision_history.go:56-77`.
- **Alcance:** comando o tool de renombre → HCD.Rename; también Show → loadDoc ante un archivo movido manualmente.
- **Causa:** se guarda la nueva ruta antes de ejecutar retarget. Si este falla, la unidad ya contiene la ruta nueva. Repetir el renombre toma el retorno temprano sin cambios; Show tampoco detecta divergencia. El mapeo pendiente no se conserva.
- **Efecto:** memorias y enlaces conservan indefinidamente una ruta antigua aunque el reintento dé éxito. El error inicial sí se informa; el bug es la recuperación posterior.
- **Evidencia independiente:** A-002 y B-001 describieron el mismo fallo.
- **Reproducción:** prueba temporal con los fixtures existentes. Start Original; primer Rename Nuevo con RetargetPath devolviendo error sintético; retirar el error; repetir Rename Nuevo; exigir dos invocaciones de RetargetPath.

Comando ejecutado:

```sh
go test -overlay /tmp/gomemory-acr-overlay.json ./application/usecases -run '^TestAuditRenameRetryMustRetarget$' -count=1
```

Salida:

```text
--- FAIL: TestAuditRenameRetryMustRetarget
    audit_retry_test.go:18: retry returned success but RetargetPath calls = 1; want 2
FAIL
```

La prueba está en `/tmp/gomemory-acr-retry_test.go`; el overlay añade virtualmente un test sin modificar fuentes del repositorio. Son artefactos temporales, no evidencia portable permanente.

**Propuesta de cierre (no aplicada):** hacer atómica la actualización de unidad y enlaces en persistencia, coordinando la compensación del archivo, o registrar durablemente un reapunte pendiente con rutas anterior/nueva que se reintente antes de devolver éxito. Cambiar solo el orden de dos llamadas no resuelve todos los fallos parciales.

**Validación del cierre:** la reproducción anterior debe pasar; añadir equivalente para loadDoc y comprobar rutas reales de memories/memory_files, no solo cantidad de llamadas. La prueba existente TestHCDRenameReportsRetargetError cubre únicamente el primer error.

### C-001 — Ruta descubierta con credencial persistida sin protección

- **Categoría:** posible fuga de credenciales a almacenamiento secundario.
- **Estado:** SUSPECT en ACR; traza estática de A, revisada en fuente por el orquestador, sin reproducción runtime ni corroboración independiente de B.
- **Ubicación:** `adapters/secondary/hcdfiles/unit_docs.go:22-24`; `application/usecases/hcd.go:509-529`; `adapters/secondary/persistence/hcd.go:109-118`; `adapters/secondary/persistence/decision_history.go:65-69`.
- **Alcance:** mover manualmente el documento a un nombre HCD-ID con un token sintético → Show descubre el archivo y adopta su ruta.
- **Mecanismo:** UnitDocs filtra prefijo/sufijo/tipo, pero no contenido sensible; loadDoc asigna found sin validar; Save guarda doc_path y RetargetPath actualiza filepath/path directamente. `domain/memory.go:60-66` exige rechazar metadatos sensibles en la vía normal de guardado.
- **Impacto potencial:** duplicar una credencial del nombre de archivo en tablas de unidades y memorias. No se afirma exposición remota ni fuga observada de secretos reales.
- **Propuesta:** rechazar rutas descubiertas con texto protegido antes de guardar o reapuntar; no transformar silenciosamente identificadores. Aplicar también la validación en el borde de persistencia que actualiza metadatos.
- **Validación pendiente:** usar nombre sintético con formato de credencial reconocido, invocar Show y consultar las tres tablas; ninguna debe recibir la ruta sensible, y errores/salidas tampoco deben exponerla. Confirmar por segundo revisor antes de tratarlo como consenso.

### C-003 — Reconciliación sobrescribe estado desde snapshot antiguo

- **Categoría:** concurrencia / integridad.
- **Estado:** SUSPECT, solo A; intercalado estático, no reproducido.
- **Ubicación:** `application/usecases/hcd.go:136-140,525,631-634`; `adapters/secondary/persistence/hcd.go:109-118`.
- **Alcance:** dos clientes concurrentes sobre la misma unidad, uno reconciliando título/ruta y otro cancelando.
- **Secuencia propuesta:** A lee unidad activa; B cancela y persiste inactive/cancelled; A guarda su snapshot para reconciliar título/ruta. Save actualiza también state y active sin predicado de versión.
- **Impacto potencial:** restaurar estado anterior de una unidad ya cancelada.
- **Propuesta:** actualización específica de campos de reconciliación o concurrencia optimista con versión y relectura.
- **Validación pendiente:** barreras deterministas entre lectura de A, fin de Cancel de B y Save de A; verificar estado final cancelado/inactivo. `-race` por sí solo no prueba este fallo lógico de base de datos.

### C-004 — Comprobación de destino no atómica

- **Categoría:** concurrencia / posible pérdida de documento.
- **Estado:** SUSPECT, solo B; no reproducido. Se mantiene severidad baja asignada por el revisor, condicionada al intercalado.
- **Ubicación:** `adapters/secondary/hcdfiles/workspace.go:80-85`.
- **Alcance:** Rename → MoveDoc, con otro proceso creando el destino entre Lstat y Rename.
- **Mecanismo:** verificar inexistencia y renombrar son pasos separados; en POSIX rename puede reemplazar el destino. No se generaliza este comportamiento a todas las plataformas.
- **Propuesta:** operación no-replace atómica soportada por plataforma, con pruebas de portabilidad; un mutex local no basta contra otros procesos.
- **Validación pendiente:** controlar el intercalado para crear destino tras Lstat y exigir rechazo sin alterar su contenido.

## 3. Recomendaciones de resiliencia

1. Priorizar C-002: reproducción disponible y consenso independiente. Incorporar test de fallo transitorio y reintento, tanto para Rename como para reconciliación.
2. Confirmar C-001 con SQLite aislado y datos sintéticos antes de corregir; revisar todos los caminos de actualización directa de metadatos.
3. Verificar C-003 con concurrencia controlada; evitar guardar campos de ciclo de vida al reconciliar un título.
4. Añadir pruebas de atomicidad multiproceso para operaciones de archivos antes de prometer no sobrescritura.
5. Para ampliar cobertura: auditar privacidad histórica completa, instaladores/actualizador, hooks, fronteras MCP, CI y dependencias en una ronda adicional. Los índices obsoletos impiden afirmar auditoría estructural exhaustiva.

No se aplicaron parches: las propuestas requieren diseño y validación de cierre; no se presenta código pretendidamente listo para producción sin haberlo implementado ni probado.
