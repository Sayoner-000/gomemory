# Informe ACR + ADM — estado actual de gomemory

**Fecha:** 2026-11-04, America/Bogota.  
**Revisión:** `4cfdd1cb0841f7d990135ea4125280d087319840` más el árbol de trabajo pendiente de HCD-025.  
**Estado inicial:** 8 archivos modificados y 11 archivos sin seguimiento.  
**Target congelado:** 1731 archivos versionados o no ignorados; SHA-256 del manifiesto ordenado de rutas y hashes: `2d9b5344103686cc8bad8f16b2e827887f54969a59176eb891fcb81b93bf1500`. Los archivos del informe y sus evidencias se añadieron después y no pertenecen al target.  
**Revisión ACR:** `acr_41c49fd2-7527-43f1-b870-be1c1f26ebfa`.  
**Modalidad:** revisión e informe; cero correcciones, commits, publicaciones o cambios en datos del usuario.

## 1. Resumen de salud del código

**Riesgo general: MEDIO.** Se confirmaron **5 hallazgos: 0 críticos, 0 altos, 5 medios y 0 bajos**. Los cinco corresponden a la nueva implementación pendiente de `mem optimize`. También se detectó una observación de formato que bloquea la comprobación de CI; se registra separadamente de los defectos funcionales.

La inspección fue del proyecto actual por superficies de riesgo, con mayor profundidad en HCD-025. La base existente aporta redacción de información privada, descargas con checksum, reemplazo atómico de binarios y pruebas de purga física. La compilación y la suite Go completa pasan. Estas comprobaciones no detectan los cinco escenarios adversos reproducidos durante esta revisión.

**Tecnologías:** Go 1.27.3, SQLite/FTS5 con modernc, MCP SDK y Bubble Tea/Lip Gloss en producción; TypeScript/Node para el instalador y plugin OpenCode; shell para instalación y mantenimiento. Python, Java y otros archivos detectados por el índice incluyen utilidades, pruebas y corpus; no constituyen evidencia de servicios de producción adicionales.

**Resultado del ledger ACR: `APPROVED`.** Su criterio permite hallazgos medios abiertos cuando no existen críticos/altos ni contradicciones graves pendientes. Este resultado no certifica ausencia de defectos, cierre de HCD-025, conformidad completa de CI ni preparación para publicar. Los cinco hallazgos permanecen **sin corregir**.

### Método, independencia y límites

- Dos subagentes revisores ejecutaron juicios iniciales separados sobre el mismo target, sin recibir los hallazgos del otro. Después realizaron cotejos focalizados y reproducciones independientes.
- Independencia efectiva **parcial**: contextos separados con el mismo modelo heredado. El ledger registra conservadoramente `degraded: same-model`; no se afirma diversidad de modelos.
- Una ronda inicial, cotejo posterior, cero rondas de corrección y cero rondas de reverificación de correcciones. Los IDs de este informe corresponden al ledger `C-001` a `C-005`; la numeración original de los revisores fue normalizada al registrarlos.
- El grafo estructural tenía generación `2026-11-04T11:46:05Z`, con watcher deshabilitado. Se comprobó cobertura de las rutas utilizadas. Los archivos nuevos de optimize estaban `not_tracked` y los proveedores modificados `metadata_changed`; las conclusiones sobre ellos proceden de lectura directa y ejecución de la fuente actual.
- No se revisaron línea por línea los 1731 archivos. Se inspeccionaron mantenimiento, persistencia, redacción privada, actualización, instalación, borrado y partes de hooks; el orquestador comprobó los workflows de CI. No se afirma cobertura exhaustiva de herramientas MCP, hooks, concurrencia o todas las rutas productivas.
- No se ejecutaron servicios remotos reales, reindexados del proyecto auditado, Windows real, análisis actual de CVE ni escáner de secretos. No se instalaron herramientas nuevas en el host. La revisión no garantiza ausencia de defectos fuera de estas superficies.
- El binario local `./mem` v2.45.2 fue retirado previamente; el global está en `/Users/josegomezj/.local/bin/mem`. La validación de la fuente utiliza `go build`, `go test` y programas temporales que importan los paquetes actuales, por lo que no atribuye al release global el comportamiento nuevo de HCD-025.

### Cobertura y validación

| Comprobación | Resultado |
|---|---|
| `go build ./...` | Pasa |
| `go vet ./...` | Pasa |
| `go test ./... -count=1` | Pasa fuera del sandbox; incluye contratos e integración |
| Primer intento de suite Go dentro del sandbox | Limitación del entorno: `httptest` no puede abrir sockets locales; se repitió fuera del sandbox |
| `go test -race` en usecases, persistence, requestcompression, cli y setup | Pasa fuera del sandbox, sin carreras detectadas en los paquetes ejecutados; requestcompression no contiene tests |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Pasa: 66/66 |
| `npm --prefix installer run typecheck` | Pasa con dependencias ya presentes |
| `npm --prefix installer test` | Pasa: 24/24 |
| `bash scripts/test-install-uninstall.sh` | Pasa: solo retira binarios propios; entorno temporal |
| `bash scripts/test-purge-physical.sh` | Pasa: canario purgado ausente de DB/WAL y memoria superviviente consultable; binario compilado desde la fuente actual |
| `bash -n scripts/install.sh` | Pasa |
| Reproducciones adversas | Los cinco defectos reproducidos invocando funciones reales; C-001/C-002 además con `persistence.Open` y migraciones reales en almacenamiento temporal |
| `gofmt -l .` | Falla: lista `adapters/secondary/codegraph/codegraphcli/provider.go` y `domain/optimize.go`; el workflow test exige lista vacía |
| `git diff --check` | Falla: línea en blanco adicional al final de `codegraphcli/provider.go:297` |
| Integridad del target | Todos los archivos del manifiesto original conservan sus hashes al entregar el informe |
| `govulncheck`, `staticcheck`, `shellcheck`, `gitleaks` | No disponibles en PATH; no ejecutados ni instalados. La presencia de govulncheck en el workflow de release no sustituye una ejecución en esta revisión |
| Umbral combinado de cobertura y pruebas en Windows | No ejecutados en esta auditoría |

## 2. Hallazgos y fugas detectadas

| ID / ledger | Severidad | Estado | Problema |
|---|---|---|---|
| C-001 | Media | CONFIRMADO | Filtro de proyecto incorrecto al contar y borrar nodos |
| C-002 | Media | CONFIRMADO | El saneamiento deja nuevas aristas huérfanas |
| C-003 | Media | CONFIRMADO | `--dry-run --reindex-external` ejecuta indexado real |
| C-004 | Media | CONFIRMADO | Eliminación cruzada de snapshots válidos de proveedores |
| C-005 | Media | CONFIRMADO | Snapshot sin arquitectura provoca pánico |

No se demostró una nueva fuga de PII o credenciales en las superficies inspeccionadas. Esto no equivale a un escaneo exhaustivo de secretos o de todos los destinos de persistencia.

### C-001 — El filtro de proyecto solo restringe una rama del predicado

- **Categoría:** integridad y aislamiento lógico.
- **Ubicación:** `adapters/secondary/persistence/optimize_storage.go:66-72` y `:170-176`.
- **Ruta de alcance:** `mem optimize --yes --target=internal` → `CmdOptimize` → `OptimizeGraph.Execute` → `OptimizeStorage`.
- **Análisis:** se construye `A OR B AND project = ?`; por precedencia SQL equivale a `A OR (B AND project = ?)`. Un nodo cuyo `file_id` no existe satisface A independientemente de su clave de proyecto. El problema afecta tanto el conteo en simulación como el borrado real.
- **Límite de impacto:** el uso normal abre un archivo `mem.db` separado por proyecto. No se demostró una vía normal que mezcle claves; la afectación a otra clave exige que sus filas ya estén en la misma DB. Este hallazgo no implica acceso a los archivos físicos de otros proyectos.
- **Evidencia con esquema real:** nodo `other` con `file_id` inexistente; optimizar `target` devuelve `counted=1`, `other_nodes_remaining=0`, `err=<nil>`, con FTS y SQLite optimizados. Repro: `storage-real-schema.go.txt` del anexo.
- **Corrección mínima propuesta, no aplicada:** agrupar las dos ramas en SELECT y DELETE antes de añadir el filtro:

```sql
WHERE ((file_id != 0 AND file_id NOT IN (SELECT id FROM code_files))
    OR (file != '' AND file NOT IN
        (SELECT path FROM code_files WHERE project = code_nodes.project)))
  AND project = ?
```

- **Validación del cierre:** en una DB temporal con dos claves, ejecutar simulación y ejecución para una sola clave; las filas de la otra deben conservarse y quedar excluidas de los contadores. Probar además el alcance global sin filtro.

### C-002 — El orden de borrado crea residuos nuevos dentro de la misma operación

- **Categoría:** integridad referencial e idempotencia.
- **Ubicación:** `adapters/secondary/persistence/optimize_storage.go:157-177`; esquema de aristas en `adapters/secondary/persistence/db.go:403-414`.
- **Ruta de alcance:** mantenimiento interno desde CLI, o acción de optimización de la TUI que invoca el mismo caso de uso.
- **Análisis:** las aristas se purgan antes de borrar los nodos huérfanos. Una arista con ambos extremos todavía presentes supera la primera purga; el borrado posterior de uno de esos nodos la deja colgante. No existe FK/CASCADE que la retire automáticamente.
- **Evidencia con esquema real:** primera ejecución: `counted_nodes=1`, `counted_edges=0`, `dangling_edges=1`, `err=<nil>`. Segunda ejecución: `counted_edges=1`, `dangling_edges=0`. Repro: `storage-real-schema.go.txt`.
- **Corrección mínima propuesta, no aplicada:** dentro de la misma transacción, borrar nodos antes del saneamiento definitivo de aristas; actualizar la detección de dry-run para incluir aristas cuyos extremos se eliminarán y obtener los conteos de ejecución de las filas afectadas. El orden seguro es:

```text
BEGIN
detectar nodos candidatos y aristas que quedarán sin extremos
borrar nodos candidatos
borrar aristas sin extremos o sin archivo origen
obtener conteos reales y completar el mantenimiento
COMMIT
```

- **Validación del cierre:** una sola ejecución debe dejar cero aristas sin extremos; la segunda debe devolver cero elementos saneados. La simulación debe anticipar los mismos conteos sin modificar la DB.

### C-003 — La combinación de simulación y reindexado produce efectos externos

- **Categoría:** error lógico y contrato de simulación.
- **Ubicación:** `adapters/secondary/codegraph/codebasememory/provider.go:579-587` y `adapters/secondary/codegraph/codegraphcli/provider.go:284-292`; propagación de flags en `adapters/primary/cli/cmd_optimize.go:45`.
- **Ruta de alcance:** `mem optimize --dry-run --reindex-external` → ambos mantenedores.
- **Análisis:** el CLI promete que ningún dato se modifica, pero ambos adaptadores condicionan `IndexRepository` a `reindex` sin comprobar `!dryRun`. El índice externo puede reconstruirse y se consume trabajo de indexado durante una simulación.
- **Evidencia:** ejecutable espía pasado al adaptador real: `dry_run_reindex: err=<nil> reindexed=true command_executed=true`; otro revisor observó `cli index_repository {"mode":"full",...}`. No se reindexó el proyecto real.
- **Corrección mínima propuesta, no aplicada:** utilizar esta guarda en ambos adaptadores y comunicar la acción prevista durante simulación:

```go
if reindex && !dryRun && p.binPath != "" {
    // Mantener aquí el bloque existente de IndexRepository y su reporte.
}
```

- **Validación del cierre:** binario espía para cada proveedor; `Optimize(ctx, true, true)` no debe recibir indexado ni crear escrituras. La ejecución con `dryRun=false` debe seguir indexando y devolver `Reindexed=true`.

### C-004 — Cada proveedor borra la caché válida del otro

- **Categoría:** integridad de caché y coexistencia de proveedores.
- **Ubicación:** `adapters/secondary/codegraph/codebasememory/provider.go:548-555` y `adapters/secondary/codegraph/codegraphcli/provider.go:263-270`.
- **Ruta de alcance:** `mem optimize --yes` con mantenedores que comparten `memDir`.
- **Análisis:** ambos recorren `code_provider_snapshot*.json` y consideran residuo cualquier JSON cuyo campo `provider` no coincida con su propio nombre. Ese criterio incluye los snapshots sanos del otro proveedor. El diagnóstico posterior puede perder la arquitectura cacheada y requerir refresco.
- **Evidencia:** snapshot JSON válido de `codegraph` procesado por `codebase-memory-mcp`: `cleaned=1`, `snapshot_deleted=true`, `err=<nil>`. Los nombres reales de ambos proveedores usan ese prefijo.
- **Corrección mínima propuesta, no aplicada:** limitar la lectura y eliminación al snapshot propio obtenido por `p.snapshotPath()`. Para residuos de configuraciones anteriores, definir propiedad inequívoca antes de borrarlos; un proveedor ajeno reconocido debe conservarse. No basta con omitir el filtro de `provider`, porque se perdería la validación de identidad.
- **Validación del cierre:** crear los dos snapshots válidos y ejecutar ambos mantenedores: los dos deben sobrevivir y contabilizar cero residuos. Un snapshot propio corrupto sí debe detectarse y borrarse exclusivamente en ejecución real.

### C-005 — Un snapshot estructuralmente incompleto provoca un pánico

- **Categoría:** disponibilidad y validación de datos persistidos.
- **Ubicación:** `adapters/secondary/codegraph/codebasememory/provider.go:573-574` y `adapters/secondary/codegraph/codegraphcli/provider.go:278-279`.
- **Ruta de alcance:** `mem optimize --dry-run` o `--yes` leyendo un snapshot incompleto.
- **Análisis:** un JSON válido puede declarar `available:true` sin `architecture`. Supera la validación actual y `Optimize` desreferencia `snap.Architecture`. Para codegraph se requiere además un `root_path` coincidente. La herramienta se cae durante una operación destinada a diagnosticar y sanear esos artefactos.
- **Evidencia:** ambos revisores invocaron el adaptador real; el cotejo de ambos proveedores produjo `runtime error: invalid memory address or nil pointer dereference`, incluso con `Optimize(ctx,true,false)`.
- **Corrección mínima propuesta, no aplicada:** hacer que `Snapshot` y la limpieza rechacen `Available && Architecture == nil`; proteger también el punto de uso:

```go
if snap.Available && snap.Architecture != nil {
    rep.StatusSummary = fmt.Sprintf("activo (%d nodos, %d aristas)",
        snap.Architecture.TotalNodes, snap.Architecture.TotalEdges)
} else {
    rep.StatusSummary = "disponible sin snapshot activo"
}
```

- **Validación del cierre:** snapshots incompletos propios para ambos proveedores, tanto simulación como ejecución, deben producir diagnóstico controlado sin pánico. La simulación conserva el archivo y la ejecución puede sanear únicamente el artefacto propio.

### Observación de formato — comprobación de CI pendiente

`gofmt -l .` lista `adapters/secondary/codegraph/codegraphcli/provider.go` y `domain/optimize.go`. El workflow `.github/workflows/test.yml` falla si la lista no está vacía. `git diff --check` detecta además una línea vacía adicional al final del proveedor codegraph. Cierre propuesto: aplicar `gofmt` en esos dos archivos y retirar el espacio final excedente; verificar lista vacía y salida 0 de `git diff --check`. No se aplicó porque el encargo fue auditar y entregar el informe.

## 3. Recomendaciones de resiliencia

1. Priorizar los contratos de no mutación en dry-run y propiedad de snapshots, junto con la guarda contra pánicos, antes de dar por cerrado el mantenimiento externo.
2. Completar el saneamiento interno en una sola transacción con un orden que preserve integridad y contadores basados en lo realmente modificado. Añadir las regresiones de C-001/C-002 sobre la DB creada por las migraciones reales.
3. Incorporar una matriz de pruebas de proveedores: ambos presentes, uno ausente, JSON corrupto, estructura incompleta y todas las combinaciones de simulación/reindexado. Utilizar ejecutables espía para verificar efectos, además de valores de retorno.
4. Mantener las comprobaciones existentes de checksum, permisos, redacción privada, purga física y carreras. Ejecutar govulncheck y el umbral combinado de cobertura antes de publicar; esta auditoría no los sustituye.

Los hallazgos no se han cerrado ni se ha modificado HCD-025. Cada propuesta debe implementarse y pasar su validación de cierre; solo entonces corresponde revisar el delta de corrección.

## 4. Evidencias y reproducción

Los programas y resultados se conservan en [evidence/2026-11-04-project](evidence/2026-11-04-project/README.md). Las pruebas usan temporales y datos sintéticos; el script de almacenamiento real configura su propio `GOMEMORY_DATA_HOME`. No se utilizaron memorias ni índices productivos para las reproducciones.

**VERDICT: APPROVED** — criterio ACR de severidad; cinco hallazgos medios abiertos y formato de CI pendiente, con los límites anteriores.
