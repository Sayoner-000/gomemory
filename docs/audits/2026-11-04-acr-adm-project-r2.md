# Informe ACR + ADM — segunda ronda del proyecto

**Fecha:** 2026-11-04, America/Bogota.  
**Revisión auditada:** `4cfdd1cb0841f7d990135ea4125280d087319840` más el árbol de trabajo actual de HCD-025 y sus correcciones. Árbol sucio.  
**Identidad del target:** 1737 archivos; digest SHA-256 del manifiesto ordenado de rutas y hashes: `0ee22e8a53dc1398e59a8f60e6b125f732603cafae23be4ed481e9691df020b2`.  
**Revisión ACR:** `acr_204d39cc-ded5-4dcc-b25e-461f6d775a40`.  
**Informe anterior:** [primera ronda](2026-11-04-acr-adm-project.md).  
**Modalidad:** solo lectura del código y datos existentes; entrega de informe y evidencias. No se corrigió código, cerró HCD-025, creó commits ni publicó nada.

## 1. Resumen de salud del código

**Riesgo general: MEDIO.** Los **cinco hallazgos anteriores están resueltos** en los escenarios verificados. Se confirmaron **tres hallazgos nuevos: 0 críticos, 0 altos, 2 medios y 1 bajo**. No hay sospechas ni contradicciones pendientes en el ledger de esta ronda.

Las correcciones previas restauran el aislamiento lógico del saneamiento, la eliminación de aristas en una sola pasada, el contrato de simulación y la convivencia de snapshots; también evitan los pánicos por arquitectura ausente. El formato y las comprobaciones mecánicas ejecutadas pasan. Persisten tres defectos de integridad o señalización que las suites actuales no detectan.

**Tecnologías:** Go 1.27.3, SQLite/FTS5, MCP SDK y Bubble Tea/Lip Gloss; TypeScript/Node para instalador y plugin OpenCode; shell para instalación y mantenimiento. Python y otros lenguajes presentes incluyen herramientas, pruebas y corpus; no se atribuye a todos uso en producción.

**Resultado del ledger ACR: `APPROVED`.** Significa que la ronda terminó sin hallazgos altos/críticos ni contradicciones graves. **Los tres hallazgos nuevos siguen abiertos**; el resultado no equivale a ausencia de defectos ni autorización de cierre o publicación.

### Alcance y método

Dos subagentes revisores inspeccionaron el mismo target con juicios iniciales separados, sin recibir los resultados del otro antes de entregarlos. Ambos encontraron los mismos tres comportamientos. Después se normalizó la severidad del valor `target` inválido a baja, por el impacto demostrado: éxito falso sin pérdida o modificación de datos. No hubo desacuerdo sobre el comportamiento.

Independencia efectiva **parcial**: contextos separados con el mismo modelo heredado. El ledger registra conservadoramente `degraded: same-model`. Se realizó una ronda inicial, cotejo posterior y cero rondas de corrección; las correcciones de la primera ronda ya estaban presentes al congelar este target.

El alcance fue el proyecto actual por riesgo, con profundidad en mantenimiento y sus correcciones. Desde el target de la primera ronda cambiaron seis archivos: los dos proveedores, sus pruebas de mantenimiento, `optimize_storage.go`, sus pruebas y `domain/optimize.go`. También se inspeccionaron el caso de uso, CLI, TUI, persistencia FTS y muestras de inserción/redacción privada. No es una revisión línea por línea de los 1737 archivos.

El grafo estructural conservaba generación `2026-11-04T11:46:05Z`, watcher deshabilitado. Las rutas nuevas de optimize estaban `not_tracked` y los proveedores/TUI modificados `metadata_changed`; se utilizó la fuente actual directamente. Se comprobó cobertura de las rutas de evidencia y se usó el grafo para símbolos existentes y relaciones. No se derivan conclusiones de ausencia del índice desactualizado.

### Cobertura y validación

| Comprobación | Resultado en esta ronda |
|---|---|
| `go build ./...` | Pasa |
| `go vet ./...` | Pasa |
| `go test ./... -count=1` | Pasa, incluidos contratos e integración |
| `go test -race ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli ./adapters/primary/setup` | Pasa; sin carreras detectadas en estas ejecuciones. Usecases/setup reutilizan resultados válidos de caché |
| Sockets de pruebas Go | Suite y detector de carreras ejecutados fuera del sandbox por la restricción de sockets locales comprobada en la primera ronda |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Pasa: 66/66 |
| `npm --prefix installer run typecheck` | Pasa, con dependencias existentes |
| `npm --prefix installer test` | Pasa: 24/24 |
| `bash scripts/test-install-uninstall.sh` | Pasa, entorno temporal |
| `bash scripts/test-purge-physical.sh` | Pasa, binario actual y DB/WAL temporales; canario eliminado y memoria superviviente consultable |
| `bash -n scripts/install.sh` | Pasa |
| `gofmt -l .` | Pasa, lista vacía |
| `git diff --check` | Pasa |
| Reproducciones anteriores | Los cinco defectos dejan de manifestarse; ambos revisores verificaron sus cierres |
| Reproducciones nuevas | Dos programas independientes y una comprobación adicional del CLI compilado confirman los tres defectos |
| Integridad del target | Los archivos congelados conservan sus hashes al entregar; solo se añadieron este informe y sus evidencias |
| `govulncheck`, `staticcheck`, `shellcheck`, `gitleaks` | No disponibles en PATH; no instalados ni ejecutados |
| CVE actuales, escaneo exhaustivo de secretos, cobertura combinada, Windows real y proveedores remotos | No ejecutados |

### Estado de los hallazgos de la primera ronda

Los IDs de esta tabla pertenecen a la revisión anterior `acr_41c49fd2-7527-43f1-b870-be1c1f26ebfa`, no al ledger nuevo.

| ID anterior | Estado | Evidencia actual |
|---|---|---|
| C-001 — filtro SQL de proyecto | RESOLVED | Esquema real: `counted=0`, `other_nodes_remaining=1`; la otra clave permanece |
| C-002 — aristas residuales | RESOLVED | Primera pasada: 1 nodo y 1 arista eliminados, `dangling_edges=0`; segunda pasada: 0 aristas eliminadas |
| C-003 — dry-run reindexaba | RESOLVED | Ejecutable espía: `reindexed=false`, `command_executed=false` |
| C-004 — snapshots cruzados | RESOLVED | Snapshot válido ajeno: `cleaned=0`, `snapshot_deleted=false` |
| C-005 — pánico por arquitectura nil | RESOLVED | Ambos adaptadores manejan el snapshot incompleto sin pánico |
| Observación de formato | RESOLVED | `gofmt -l .` vacío y `git diff --check` con salida 0 |

Estos cierres describen los defectos originales. La fila residual FTS de R2-01 y los errores de borrado de R2-03 tienen causas diferentes y no se presentan como reapariciones de los cinco defectos anteriores.

## 2. Hallazgos y fugas detectadas

| Informe / ledger nuevo | Severidad | Estado | Problema |
|---|---|---|---|
| R2-01 / C-001 | Media | CONFIRMADO | Documentos FTS de nodos borrados permanecen en `code_search` |
| R2-02 / C-002 | Baja | CONFIRMADO | `--target` inválido produce éxito sin mantenimiento |
| R2-03 / C-003 | Media | CONFIRMADO | Fallos de borrado de snapshots se cuentan como saneamientos exitosos |

No se demostró una nueva fuga de PII o credenciales. La inspección no constituye un escaneo exhaustivo de todos los destinos de datos o secretos.

### R2-01 — La purga de nodos no retira sus documentos FTS

- **Categoría:** integridad y retención residual de índices.
- **Severidad/estado:** media, CONFIRMADO por ambos revisores.
- **Ubicación:** `adapters/secondary/persistence/optimize_storage.go:180-235`; referencia de limpieza existente: `adapters/secondary/persistence/code_graph.go:142`.
- **Ruta de alcance:** `mem optimize --yes --target=internal` o mantenimiento TUI → `OptimizeGraph.Execute` → `OptimizeStorage`.
- **Análisis:** el borrado directo de `code_nodes` evita la limpieza de `code_search` que realiza `deleteNodesByIDs` en el mantenimiento habitual. `INSERT ... VALUES ('optimize')` compacta segmentos FTS, pero no elimina documentos que han perdido su nodo. El proceso declara `FTSOptimized=true` con referencias y texto residuales.
- **Evidencia:** DB abierta con `persistence.Open`, migraciones y FTS reales, datos sintéticos: `nodes_cleaned=1`, `live_nodes=0`, `orphan_fts_rows=1`, `fts_optimized=true`, `err=<nil>`. Ambos revisores construyeron y ejecutaron una reproducción propia.
- **Límite de impacto:** la búsqueda pública hace `JOIN code_nodes`, por lo que no se demostró que devuelva esa fila obsoleta. Tampoco se demostró fuga de secretos: el defecto probado es retención e incoherencia del índice.
- **Corrección propuesta, no aplicada:** retirar los documentos de todos los nodos candidatos, incluidos paquetes, dentro de la misma transacción. Sanear además los documentos antiguos sin nodo. Cuando `code_search` exista, la limpieza de documentos desvinculados de la DB puede realizarse después de borrar los nodos y antes de optimizar FTS:

```sql
DELETE FROM code_search
WHERE node_id NOT IN (SELECT id FROM code_nodes);
```

La integración debe manejar el error de esta operación y conservar el soporte de builds sin FTS. Esta sentencia retira únicamente documentos sin nodo vivo; no sustituye la eliminación de nodos ni reconstruye filas FTS que falten.

- **Validación del cierre:** sobre el esquema real, crear un nodo candidato a purga, un documento ya huérfano y un nodo vivo con su documento. Una ejecución debe dejar cero documentos sin nodo, conservar la búsqueda del nodo vivo y ser idempotente. La simulación debe conservar todas las filas.

### R2-02 — Un ámbito desconocido devuelve éxito sin hacer trabajo

- **Categoría:** éxito falso y validación de entrada.
- **Severidad/estado:** baja, CONFIRMADO por ambos revisores.
- **Ubicación:** `application/usecases/optimize_graph.go:42-69` y `adapters/primary/cli/cmd_optimize.go:19,81-84`.
- **Ruta de alcance:** `mem optimize --yes --target=interanl`.
- **Análisis:** el flag acepta cualquier cadena. El caso de uso solo entra en las ramas reconocidas y termina con un reporte vacío y error nil para las demás. El CLI interpreta el reporte vacío como ausencia de residuos y anuncia que todo estaba optimizado.
- **Evidencia:** ambos revisores observaron cero llamadas al mantenimiento con un valor inválido. Adicionalmente se compiló un binario desde la fuente auditada y se ejecutó en un proyecto y almacén temporales:

```text
invalid-target exit=0
✓ El almacén y grafos ya estaban optimizados — nada que sanear.
```

- **Corrección propuesta, no aplicada:** validar la lista cerrada al entrar al caso de uso, antes de cualquier efecto; el CLI debe propagar ese error con salida distinta de cero:

```go
switch target {
case "all", "internal", "external":
default:
    return out, fmt.Errorf("ámbito de optimización inválido %q: use all, internal o external", target)
}
```

Este bloque se ubica después de normalizar el valor vacío a `all`.

- **Validación del cierre:** entrada inválida produce error y código CLI no cero, sin llamadas a repositorios ni proveedores. Vacío, `all`, `internal` y `external` conservan su selección de dependencias y comportamiento.

### R2-03 — Los borrados fallidos se presentan como completados

- **Categoría:** éxito falso y manejo de errores de filesystem.
- **Severidad/estado:** media, CONFIRMADO por ambos revisores.
- **Ubicación:** `adapters/secondary/codegraph/codebasememory/provider.go:558-585` y `adapters/secondary/codegraph/codegraphcli/provider.go:270-283`.
- **Ruta de alcance:** `mem optimize --yes --target=external` → mantenedor de proveedor → limpieza de un snapshot propio corrupto.
- **Análisis:** se incrementa `SnapshotsCleaned` antes de intentar borrar el archivo y se descarta el error de `os.Remove`. Un fallo de permisos deja intacto el artefacto, pero se devuelve error nil y el CLI informa que fue purgado. El mismo patrón también aparece en la eliminación del lock de codebase.
- **Evidencia dinámica:** dos reproducciones independientes con el adaptador real de codebase, archivo propio corrupto y directorio temporal con modo `0500`: `err=<nil>`, `cleaned=1`, `file_remains=true`. Se comprobó en fuente el mismo patrón en codegraph. La reproducción de permisos requiere usuario sin privilegios de root.
- **Corrección propuesta, no aplicada:** en simulación contar candidatos; en ejecución, contar únicamente borrados exitosos y devolver un error contextual o un reporte parcial explícito. Una ausencia debida a otra limpieza concurrente puede tratarse sin contar una eliminación propia:

```go
if dryRun {
    rep.SnapshotsCleaned++
} else if err := os.Remove(path); err != nil {
    if !os.IsNotExist(err) {
        return rep, fmt.Errorf("eliminar residuo %q: %w", path, err)
    }
} else {
    rep.SnapshotsCleaned++
}
```

Aplicar la misma política a todos los puntos de eliminación de snapshots y locks de ambos proveedores.

- **Validación del cierre:** directorio sin permiso de escritura: el archivo permanece, se obtiene error explícito o estado parcial y cero elementos eliminados. Restaurados los permisos, se elimina el archivo y se cuenta uno. Verificar ambos proveedores y que la simulación no borre.

## 3. Recomendaciones de resiliencia

1. Mantener juntos los borrados de entidad, relaciones y representaciones secundarias. La nueva regresión debe comprobar el contenido de `code_search`, además de contar nodos y aristas.
2. Validar enumeraciones en el caso de uso, para que los adaptadores CLI/TUI/MCP compartan el mismo contrato y una entrada desconocida no se convierta en éxito.
3. Distinguir candidatos, eliminaciones realizadas y fallos parciales. Usar permisos restringidos y ejecutables espía en las pruebas para comprobar efectos reales y errores, además del reporte devuelto.
4. Conservar las pruebas de cierre de la primera ronda; ejecutar las comprobaciones de vulnerabilidades y cobertura no disponibles en esta auditoría antes de publicar.

## 4. Evidencias, límites y entrega

Programas de ambos revisores, resultados de cierres y logs actuales: [evidence/2026-11-04-project-r2](evidence/2026-11-04-project-r2/README.md). Las reproducciones utilizan datos sintéticos y almacenes temporales. El binario global v2.45.2 no se utilizó para atribuirle el comportamiento pendiente de HCD-025; el caso CLI se ejecutó con un binario compilado desde la fuente actual.

La inspección no fue exhaustiva de privacidad, hooks, MCP, CI/CD ni todos los archivos productivos. No se ejecutaron agentes remotos, Windows real, escaneo de secretos, CVE actuales ni cobertura combinada. No se instalaron dependencias adicionales. Las comprobaciones verdes tienen el alcance de las rutas ejecutadas y no garantizan ausencia de defectos.

**VERDICT: APPROVED** — cinco cierres anteriores verificados; dos hallazgos medios y uno bajo nuevos, pendientes de corrección.
