# Informe ACR + ADM: superficies de producción

Fecha: 2026-11-28. Revisión auditada: `85ad0e2ca5ce194dcb96aff92f2727716f1681cf` (árbol limpio, tree `0dd319c950c73042608ede1a9c17a1df95d12c82`).

ACR: `acr_0beb0563-bdf9-4c96-be98-0a1c4db0bae9`, revisión de seguimiento de `acr_93a32f92-751b-40bc-b106-69db03cd8d2e` (delta `c8295be..HEAD`, 58 archivos, 2 hallazgos heredados).

La auditoría fue de solo lectura: no modificó código de producción ni corrigió defectos, y no autorizó commit, push ni publicación. Los dos hallazgos se corrigieron después en HCD-071 (ver «Cierre»).

## 1. Resumen de salud del código

- Tecnologías: Go 1.27.4 en producción y pruebas; SQLite (`modernc`); TypeScript/Node en el plugin de OpenCode y el instalador; shell en `scripts/`; YAML en CI.
- Nivel de riesgo general: bajo para el delta auditado. La cobertura ACR quedó incompleta porque el revisor B no pudo ejecutarse. No hay defectos críticos, altos ni medios; lo encontrado es deuda de higiene tras la retirada de ADR sync.
- El delta conserva las defensas ya auditadas: redacción de privacidad antes de persistir (`ValidateMemoryMetadata` + `RedactPrivate`/`RedactSecrets` en `adapters/secondary/persistence/memory.go:61-80`), fallo cerrado en `verify --stale` (`application/usecases/hcd.go:410-417`), contrato de `CodeGraphProvider` alineado con la implementación (`application/ports/code_graph_provider.go:17-25`) y resolución de `latest` por redirección con validación de tag y respaldo autenticado (`scripts/install.sh`). La retirada de ADR sync (HCD-069) no dejó dependencias inyectadas vacías ni mensajes de locale huérfanos. Quedaron restos de texto, un helper de prueba sin uso y dos restos que HCD-069 decidió conservar (ver «Cierres heredados»).
- Resultado: 2 hallazgos (0 críticos, 0 altos, 0 medios, 1 bajo, 1 informativo). Ambos están confirmados en ADM por inspección directa del árbol, pero sin consenso ACR, porque solo hubo un revisor. Veredicto ACR: `INCOMPLETE`.

### Cobertura y validación

| Comprobación | Resultado |
|---|---|
| `git rev-parse HEAD` / tree / estado | `85ad0e2` / `0dd319c` / limpio; congelado antes de la revisión |
| `review_start` con `since_review_id` | Aceptado: delta de 58 archivos y 2 heredados |
| Revisores ACR | A: `success`, 2 hallazgos; B: `failure`, «Free usage limit reached» en dos intentos (mismo modelo y `deepseek-v4.1-flash-free`) |
| `go vet ./...` | Pasa |
| `go test ./...` | Pasa (sin fallos) |
| `go test -race ./domain ./application/usecases ./adapters/secondary/persistence ./adapters/secondary/hcdfiles` | Pasa (4 paquetes) |
| `go mod verify` | Pasa: todos los módulos verificados |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Pasa 68/68 |
| `npm --prefix installer run typecheck` / `npm --prefix installer test` | Pasa (typecheck limpio; 24/24) |
| `bash -n scripts/install.sh scripts/test-install-uninstall.sh` | Pasa (solo sintaxis) |
| `git diff --check` | Pasa (árbol limpio) |
| `govulncheck`, `gitleaks`, `staticcheck`, `semgrep`, `shellcheck` | No disponibles en PATH; no se instalaron |
| Windows real, escaneo exhaustivo de secretos, auditoría línea por línea | No ejecutados |

### Límites

El ACR se ejecutó como revisión de seguimiento sobre el delta desde `c8295be`. Los dos heredados se reabrieron porque sus archivos cambiaron y se verificaron en código, pero sin segundo revisor no hay consenso posible. El ledger marca la independencia como `degraded`: asignó el modelo `mimo-v2.6-flash-free` a los dos revisores, y el segundo intento con `deepseek-v4.1-flash-free` tampoco llegó a ejecutarse. Aunque B hubiera terminado con la asignación original, la independencia seguiría siendo `degraded` por compartir modelo con A, así que repetir la revisión sin cambiar esa asignación no daría un consenso real. La inspección ADM es dirigida por riesgo: no cubre todo el árbol ni todas las plataformas. Que las suites pasen no demuestra que falten los defectos descritos.

## 2. Hallazgos y fugas detectadas

| ID | Severidad | Estado ADM / ACR | Problema |
|---|---|---|---|
| H-01 | Baja | CONFIRMADO / no adjudicado (A-001) | Helper de prueba `adrProviderEspia` huérfano tras HCD-069 |
| H-02 | Informativa | CONFIRMADO / no adjudicado (A-002) | Seis comentarios apuntan a símbolos o claves eliminados en el delta (A reportó cinco; el sexto apareció al revisar el informe) |

Los dos estados difieren a propósito. En ADM ambos hallazgos están demostrados por inspección directa del árbol. En ACR solo los reportó el revisor A, y como B no pudo ejecutarse, el ledger no produjo clasificación de consenso.

### H-01: helper de prueba sin llamador tras la retirada de ADR sync

- Categoría: higiene, código muerto.
- Severidad: baja. Estado: CONFIRMADO (ADM), no adjudicado (ACR A-001).
- Ubicación: `adapters/secondary/persistence/memory_seed_test.go:233-247` (`adrProviderEspia` y sus métodos `Name`, `GetDocument`, `UpdateDocument`).
- Ruta de alcance: ninguna en producción; se compila con `go test ./...` en el paquete `adapters/secondary/persistence`.
- Análisis: el único consumidor, `TestInsertSeedMemory_NoExportaADR`, se eliminó en `f8868e2` (HCD-069) y el tipo auxiliar quedó intacto. Su comentario dice que implementa `ports.ADRSyncProvider`, un puerto borrado en el mismo delta, así que describe una capacidad retirada. No afecta a la ejecución, pero confunde a quien mantenga el archivo, y `go vet` no lo detecta porque en Go un tipo sin referencias es legal.
- Evidencia: la búsqueda sobre `*.go` solo devuelve las 5 líneas de definición y comentario del propio archivo (233, 235, 237, 239, 244), sin instanciaciones. `git diff c8295be..85ad0e2 -- adapters/secondary/persistence/memory_seed_test.go` confirma que se eliminó la prueba que lo usaba y no el tipo. Verificado en el árbol congelado.
- Corrección mínima propuesta: borrar el bloque completo `233-247`, incluido el comentario que nombra al puerto inexistente.
- Validación del cierre: la búsqueda de `adrProviderEspia` en `*.go` debe devolver 0 coincidencias y `go test ./adapters/secondary/persistence` debe seguir pasando.

### H-02: comentarios que nombran símbolos eliminados

- Categoría: deuda de documentación interna.
- Severidad: informativa. Estado: CONFIRMADO (ADM), no adjudicado (ACR A-002).
- Ubicación: `infrastructure/container.go:193`, `adapters/primary/cli/seeds.go:37`, `adapters/secondary/persistence/settings.go:52` y `:135`, `application/ports/settings_repository.go:118`, `adapters/secondary/persistence/settings_test.go:525`.
- Ruta de alcance: ninguna en ejecución; afecta a quien lee el código.
- Análisis: la retirada mecánica de ADR sync y de la anotación de impacto dejó cinco comentarios que todavía nombran `adrSyncProvider`, `ADRSyncProvider`, `CodeImpactAnnotationDisabled` y `AdrSyncEnabled`. Un sexto (`settings_test.go:525`, «es opt-in, como adr_sync_enabled») usa como comparación la clave de settings retirada. Ninguno de esos símbolos sigue definido y la clave ya no existe. Estorba al navegar el código, sin efecto en ejecución ni en contratos.
- Evidencia: la búsqueda sobre `*.go` de los cuatro identificadores devuelve solo esos cinco comentarios, sin definiciones ni usos. La búsqueda original no incluyó los nombres de clave de settings (`adr_sync_enabled`, `code_impact_annotation_disabled`); al añadirlos aparece el sexto comentario. Las demás coincidencias de esas claves, en `settings_test.go:121-143`, son la prueba de HCD-069 que comprueba que las claves retiradas se ignoran. Verificado en el árbol congelado.
- Corrección mínima propuesta: eliminar o reescribir cada comentario para que remita a un patrón vigente, sin tocar código.
- Validación del cierre: la búsqueda de los cuatro identificadores debe devolver 0 coincidencias, `adr_sync_enabled` solo debe aparecer en la prueba de claves retiradas de `settings_test.go` y el diff solo debe tocar líneas de comentario.

### Cierres heredados y verificación de regresiones

- 443 (MEDIUM, `verify --stale`): no se reproduce. `application/usecases/hcd.go:410-417` falla cerrado antes de ejecutar ningún comando cuando hay tareas caducadas con evidencia manual, y el mensaje nombra esas tareas. Lo cubre `application/usecases/hcd070_stale_mixed_test.go`. El revisor A confirmó que no reaparece.
- 444 (LOW, contrato `CodeGraphProvider`): no se reproduce. `application/ports/code_graph_provider.go:17-25` describe ahora el refresco detached con `init`/`index` (plazo de 10 min), sondeo de 3 s, sync de 30 s, reescritura del snapshot y `GOMEMORY_NO_CODE_REFRESH`, igual que hace `adapters/secondary/codegraph/codegraphcli/provider.go:139-199`. El revisor A confirmó que no reaparece.
- 2026-11-06 HCD-043/044/045: siguen vigentes. `ProtectText` previo al recorte y el grupo de procesos con `WaitDelay` están fuera del delta y no cambiaron. `ValidateMemoryMetadata` se ejecuta antes de cualquier dedup o INSERT en `adapters/secondary/persistence/memory.go:61-80`, con redacción de título, contenido y provenance en las líneas 66-77.
- 2026-11-21 (C-001 a C-004): no se volvieron a adjudicar; sus archivos no cambiaron en este delta.
- HCD-069 (retirada de ADR sync y de la anotación de impacto): no quedan dependencias inyectadas vacías en `infrastructure/container.go` ni claves de locale huérfanas. Además del helper y los comentarios descritos arriba, hay dos restos que se conservan a propósito y no son deuda:
  - La tabla `adr_sync_records` y sus índices se siguen creando en `adapters/secondary/persistence/db.go:439-454`. HCD-069 lo decidió en su Q1 (opción 1): la tabla queda huérfana para cumplir el esquema solo aditivo de la Constitución §25.1 y para que un binario anterior siga abriendo la base. Su FK a `memories(id)` no bloquea borrados porque las conexiones abren con `foreign_keys = 0` (`db_pragmas_test.go:14`).
  - `domain/impact_annotation.go` (`StripImpactAnnotation`) se conserva para quitar la nota de impacto que guardan las memorias antiguas. La usa `application/usecases/save_gate.go:68` y `:85`.

## 3. Recomendaciones de resiliencia

1. Borrar el helper muerto y los seis comentarios obsoletos en un cambio pequeño de higiene. `staticcheck` (U1000) habría detectado el primero, pero no está disponible localmente.
2. Repetir la revisión ACR de seguimiento cuando haya cuota, con modelos distintos para A y B y con ambos llegando a ejecutarse. Con el mismo modelo la independencia sigue en `degraded` aunque B termine.
3. Añadir un escáner de secretos al CI de release. `govulncheck` ya corre en CI (`.github/workflows/release.yml:12-22`, `golang/govulncheck-action` fijado por SHA); solo faltaba en el PATH local.

Una versión anterior de este informe recomendaba limitar `GITHUB_TOKEN` y fijar las acciones por SHA. Se retiró porque las acciones de `test.yml` y `release.yml` ya están fijadas por SHA, y el `GITHUB_TOKEN` de `test.yml:47` solo existe en el paso de `install.sh`, documentado como respaldo por la API.

## Cierre

HCD-071 corrigió H-01 y H-02 el 2026-11-28 (revisión de cierre `5aa31e1fd3c2`). Borró `adrProviderEspia` con el import `context` que quedaba sin uso, y reescribió los seis comentarios para que remitan a ajustes vigentes (`ports.MemoryTopicQuerier`, `SpeckitContextDisabled`, `CompactAgentNotice`). Los cinco identificadores ya no aparecen en `*.go`; `go vet ./...` y `go test ./... -count=1` pasan. La recomendación 1 queda cumplida. La 2 y la 3 siguen abiertas.

## ACR: resultado del ledger

```
Target:       repository @ 85ad0e2ca5ce194dcb96aff92f2727716f1681cf
Digest:       0dd319c950c73042608ede1a9c17a1df95d12c82
Review:       acr_0beb0563-bdf9-4c96-be98-0a1c4db0bae9
Since:        acr_93a32f92-751b-40bc-b106-69db03cd8d2e
Since files:  58 (delta git), 2 inherited reabiertos
Independence: degraded (mismo modelo asignado; B nunca produjo resultados)
Rounds:       initial 1 · fix 0 · rejudgment 0
Reviewers:    A success (2 hallazgos) · B failure (límite de uso free, 2 intentos)
Confirmed:    0    Suspect: 0 en ledger    Contradicciones: 0    Info: 0
Correcciones: ninguna (solo lectura)    Re-judgment: n/a
VERDICT: INCOMPLETE
```

`INCOMPLETE` significa que no se cumplió el protocolo de dos revisores. No equivale a una aprobación ni autoriza commit, push, merge, despliegue o publicación. Los dos hallazgos de A quedan registrados sin clasificación de consenso.
