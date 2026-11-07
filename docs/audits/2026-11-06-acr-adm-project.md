# Informe ACR + ADM del proyecto

Fecha: 2026-11-06, America/Bogota. Revisión: `1f3daaede2b0e54a3408858c673770e23bc74338`. Árbol limpio al congelar; 1817 archivos versionados. Solo se añaden este informe y reproducciones sintéticas. No se modificó código ni se corrigieron defectos.

## 1. Resumen de salud del código

**Riesgo general: ALTO.** Tres defectos reproducidos: **0 críticos, 2 altos, 1 medio, 0 bajos**. Los dos altos afectan la protección de contenido marcado como privado; el medio permite exceder el plazo de verificaciones HCD. No se usaron secretos reales.

El proyecto tiene pruebas amplias, comprobación de carreras, descargas con checksum, acciones CI fijadas por SHA y protección de texto en múltiples fronteras. Las comprobaciones ejecutadas pasan, pero no cubren los tres escenarios descritos.

Tecnologías: producción en Go 1.27.3, SQLite/FTS, MCP y TUI; TypeScript/Node en instalador, JavaScript en OpenCode y shell en instalación/mantenimiento. Python y otros lenguajes también aparecen en herramientas, pruebas y corpus; no se atribuye a todos uso productivo.

### Método ACR y alcance

Revisión `acr_8e552a63-8a6e-4369-8531-37b41317cb21`. Dos revisores del mismo modelo heredado, con contextos aislados y juicio inicial separado. Independencia efectiva parcial; el ledger la registra conservadoramente como `degraded: same-model`. Una ronda inicial y cero rondas de corrección.

**Ledger: 0 CONFIRMED, 3 SUSPECT, 0 contradicciones.** A descubrió H-01 y H-02; B descubrió H-03. El orquestador volvió a ejecutar las tres reproducciones con el mismo resultado. Esto confirma los comportamientos a efectos ADM, pero no equivale a descubrimiento independiente por ambos revisores: se mantienen como SUSPECT en ACR.

**Resultado formal del ledger: APPROVED.** Su regla no bloquea por SUSPECT. Este resultado no demuestra que los dos riesgos altos estén cerrados ni que el proyecto sea seguro para publicar. Los tres defectos siguen abiertos y necesitan corrección y verificación posterior.

Se auditó el repositorio completo como alcance, con inspección por riesgo, no línea por línea de sus 1817 archivos. Se priorizaron privacidad, persistencia, originales, portabilidad, HCD, MCP/hooks, actualización, instaladores y CI. Se consultó el informe del 2026-11-04; no se revalidaron exhaustivamente sus cierres.

### Cobertura y validación

| Comprobación | Resultado |
|---|---|
| `go build ./...` | Pasa |
| `go vet ./...` | Pasa |
| `go test ./...` | Pasa; incluye resultados de caché, contratos 92.2 s |
| `go test -race ./domain/... ./application/... ./adapters/secondary/persistence/...` | Pasa; persistencia 139.9 s |
| `go mod verify` | Pasa: todos los módulos verificados |
| OpenCode: `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Pasa 67/67 |
| `npm --prefix installer run typecheck` | Pasa, dependencias existentes |
| `npm --prefix installer test` | Pasa 24/24 |
| `bash -n` sobre install.sh, coverage.sh y test-install-uninstall.sh | Pasa; solo sintaxis |
| `git diff --check` | Pasa antes de añadir el informe |
| Tres reproducciones sintéticas | Comportamiento reproducido por revisor y orquestador |
| Grafo | Generación 2026-11-06T11:48:26Z, watcher deshabilitado; cobertura consultada para evidencia, fuente actual contrastada |
| `govulncheck`, `staticcheck`, `shellcheck`, `gitleaks`, `semgrep` | No disponibles en PATH; no instalados |
| CVE actuales, escaneo exhaustivo de secretos, Windows real, agentes remotos, cobertura combinada | No ejecutados |

Inspección profunda: `domain/redact.go`; persistencia `memory.go`, `privacy_maintenance.go`, `relation.go`, `compression_originals.go`, `automatic_originals.go`, `instincts.go`, `hcd.go`, `db.go`; casos de uso `hcd.go`, `save_gate.go`, `portability.go`, `retrieve_original.go`; `hcdfiles/workspace.go`, `generator/exec.go`; MCP `cmd_mcp.go` y `cmd_mcp_render.go`; `cmd_update.go`; instalador `bootstrap.ts` y `core.ts`; `scripts/install.sh`; workflows test y release.

Inspección focalizada adicional: sesiones, repositorios, globalstore, hooks/conversación, socrática, tipos de dominio, shield/expandstate, CLI save/HCD, consola/TUI, proveedores de grafo, wiring y pruebas HCD. Los recorridos dinámicos por interfaces se contrastaron en fuente; no se interpretó una arista ausente como prueba de ausencia. El índice tiene huecos generales: su cobertura sin incidencias registradas para las rutas citadas no demuestra exhaustividad.

## 2. Hallazgos y fugas detectadas

| ID / origen / ledger | Severidad | Estado ADM / ACR | Problema |
|---|---|---|---|
| H-01 / A-001 / C-001 | Alta | CONFIRMADO / SUSPECT | Recortar antes de redactar pierde el límite privado |
| H-02 / A-002 / C-002 | Media | CONFIRMADO / SUSPECT | Timeout HCD espera pipes heredados |
| H-03 / B-001 / C-003 | Alta | CONFIRMADO / SUSPECT | Metadatos de memoria conservan contenido privado |

### H-01 — Evidencia HCD expone texto privado al recortarlo

- **Categoría:** privacidad e integridad del procesamiento.
- **Ubicación:** `adapters/secondary/hcdfiles/workspace.go:160,190`; `application/usecases/hcd.go:305-313`; persistencia `hcd.go:103`.
- **Ruta:** `mem hcd verify` → `HCD.Verify` → `Workspace.Run` → `tail` → `ProtectText` → evidencia DB, documento HCD y respuesta CLI.
- **Causa e impacto:** Run conserva solo 2000 bytes y 20 líneas antes de proteger el texto. Si la apertura privada queda fuera del fragmento, ProtectText no puede reconocer que el contenido restante es privado. La persistencia y publicación posteriores se comprobaron en fuente; la reproducción ejecuta Run y ProtectText reales.
- **Evidencia:** [programa sintético](evidence/2026-11-06-project/private-tail.go.txt). `excerpt_bytes=2000 contains_private_marker=false protected_leaks_synthetic_private=true err=<nil>`.
- **Corrección propuesta, no aplicada:** proteger la salida completa antes de cualquier recorte, conservando la defensa adicional de Verify. Sustitución mínima en workspace.go:

```go
excerpt := tail(domain.ProtectText(out.String()))
```

Requiere importar `mem/domain`. Si se cambia la captura a un buffer acotado, la redacción debe conservar estado entre fragmentos antes de descartar datos.

- **Validación del cierre:** canarios privados con apertura anterior a 2000 bytes y a 20 líneas, etiquetas incompletas/anidadas y salida pública alrededor. Comprobar ausencia del canario en Excerpt, SQLite, documento y JSON CLI; comprobar conservación de texto público. La prueba debe fallar con la implementación actual.

### H-02 — El plazo de una verificación no limita la espera de descendientes

- **Categoría:** disponibilidad y ciclo de vida de recursos.
- **Ubicación:** `adapters/secondary/hcdfiles/workspace.go:152-159`.
- **Ruta:** `mem hcd verify` → shell de la comprobación → proceso descendiente que hereda stdout/stderr.
- **Causa e impacto:** CommandContext mata el shell al vencer el plazo; Run continúa esperando los pipes que conserva un descendiente. No configura WaitDelay ni cancelación del árbol de procesos. Un hijo sin terminación puede mantener la espera indefinidamente.
- **Evidencia:** [programa sintético](evidence/2026-11-06-project/timeout.go.txt). Con timeout 100 ms y `sleep 2 & wait`: `elapsed=2.011s exit=-1`, sin error de infraestructura y con aviso de tiempo agotado.
- **Corrección propuesta, no aplicada:** limitar espera de pipes antes de Run; el siguiente bloque limita esa espera, pero debe acompañarse de cancelación del árbol mediante grupos de procesos Unix y Job Objects Windows:

```go
cmd.WaitDelay = 100 * time.Millisecond
```

El margen definitivo debe formar parte del contrato. `adapters/secondary/generator/exec.go` ofrece un precedente de WaitDelay. WaitDelay por sí solo no demuestra que los hijos hayan terminado.

- **Validación del cierre:** hijo de varios segundos y timeout 100 ms; retorno dentro del plazo más margen documentado. Un hijo que intenta escribir un canario después del timeout debe haber terminado antes. Ejecutar Unix y Windows, conservando exit=-1 para timeout.

### H-03 — Filepath y topic_key quedan fuera de la protección privada

- **Categoría:** privacidad y retención de datos.
- **Ubicación:** `adapters/secondary/persistence/memory.go:114,132,731,871`; `privacy_maintenance.go:141`; `adapters/primary/cli/cmd_mcp_render.go:44`.
- **Ruta:** MCP `save_memory` acepta filepath/topic_key → SaveWithGate → Insert → insertMemory. Deduplicación e importación también escriben esos metadatos. `get_memory` lee y renderiza filepath.
- **Causa e impacto:** título, contenido y prompt se protegen; filepath y topic_key se escriben directamente. La lectura solo quita controles de terminal. El saneamiento selecciona title/content/origin_prompt, por lo que no detecta esos metadatos históricos.
- **Evidencia:** [programa sintético](evidence/2026-11-06-project/metadata.go.txt), SQLite en memoria con InsertSeedMemory, que comparte insertMemory con producción, y GetMemoryByID: ambos campos conservan íntegros sus canarios dentro de etiquetas privadas. La llegada desde MCP y renderizado se comprobaron en fuente.
- **Corrección propuesta, no aplicada:** rechazar metadatos que contengan texto protegido en lugar de transformar silenciosamente topic_key, evitando colisiones de identidad. Validación común antes de escrituras de inserción, deduplicación e importación:

```go
for _, value := range []string{m.Filepath, m.TopicKey} {
    if domain.ProtectText(value) != value {
        return 0, fmt.Errorf("metadatos de memoria contienen datos privados")
    }
}
```

El retorno mostrado corresponde a funciones `(int64, error)`; adaptar el retorno en otras firmas. Proteger además lecturas históricas y ampliar selección/corrección del saneamiento; rechazar nuevas entradas no limpia las existentes.

- **Validación del cierre:** canarios en ambos metadatos, por inserción, deduplicación e importación; ausencia en SQL, get_memory y exportación. Inyectar filas históricas para probar lectura protegida y saneamiento. Verificar que claves públicas mantienen el upsert estable y que no se generan colisiones.

## 3. Recomendaciones de resiliencia

1. Proteger antes de transformaciones que pierdan delimitadores, y probar los destinos finales de privacidad, no solo la función de redacción.
2. Incluir metadatos en el contrato de privacidad y saneamiento; distinguir texto presentable de claves de identidad.
3. Dar a las ejecuciones externas un presupuesto completo: proceso principal, descendientes y espera de pipes. Probar efectos posteriores al timeout.
4. Incorporar las tres regresiones y ejecutar análisis de vulnerabilidades/secretos con herramientas disponibles en CI. No inferir ausencia de fugas de las suites verdes.

## 4. Evidencias y límites

Para repetir cada programa: copiar el archivo `.go.txt` a una ruta temporal con extensión `.go` y ejecutar `go run /ruta/temporal.go` desde la raíz del repositorio. Solo usan datos sintéticos; el caso de timeout espera aproximadamente dos segundos.

Los tres problemas están abiertos. No hubo correcciones ni rejuicio de correcciones. La inspección no garantiza ausencia de otros defectos, no cubre todas las escrituras ni plataformas y no valida instalaciones remotas. No se instalaron dependencias nuevas; el build del instalador usó las ya existentes. Solo se entregan documentación y reproducciones.

**VERDICT: APPROVED** según el ledger ACR, con tres SUSPECT; **riesgo ADM ALTO y dos fugas reproducidas pendientes de cierre**.

## 5. Estado de cierre (actualizado 2026-11-06)

Los tres hallazgos se corrigieron en unidades HCD, con rojo observado antes del cambio, verde después y aprobación citada al cerrar. Las reproducciones sintéticas de `evidence/` ya no muestran el defecto.

| Hallazgo | Unidad | Corrección | Comprobación |
|---|---|---|---|
| H-01 | HCD-043 (ceremonia completa) | `Workspace.Run` aplica `ProtectText` a la salida completa antes de recortarla; `HCD.Verify` mantiene la segunda protección. | `TestRunProtegeLaSalidaAntesDeRecortar`, `TestHCDVerifyNoGuardaTextoPrivadoRecortado`; `private-tail` → `protected_leaks_synthetic_private=false`. |
| H-03 | HCD-044 (ceremonia completa) | `domain.ValidateMemoryMetadata` rechaza filepath o topic_key protegidos en la inserción, la deduplicación, `ImportMemory` e `ImportBundle`; la lectura los protege en filas históricas; `mem sanitize` añade `memory_filepath` (protegido) y `memory_topic_key` (NULL). | `TestMetadatosPrivados_*`, `TestImportMemory_RechazaMetadatosPrivados`, `TestImportBundle_RechazaMetadatosPrivados`, `TestExportProject_ProtegeMetadatosHistoricos`, `TestPrivacyStore_SaneaMetadatosDeMemorias`; `metadata` ya no muestra los canarios. |
| H-02 | HCD-045 (Vía rápida) | La comprobación corre en su propio grupo de procesos (Unix: `Setpgid` + `SIGKILL` al grupo; Windows: `taskkill /T /F`) y `WaitDelay` de 1 s acota la espera: `Run` vuelve como mucho en plazo + 1 s. | `TestRunTerminaLosDescendientesAlVencerElPlazo`; `timeout` → `elapsed` en ms con `exit=-1`; `GOOS=windows` build y vet. |

Validación conjunta: `go vet ./...` y `go test ./...` en verde; `go test -race` en persistencia y dominio en verde. Pendiente: el comportamiento de H-02 en Windows real solo se comprueba en el job Windows del CI. Las herramientas de la sección de cobertura que no estaban disponibles (`govulncheck`, `gitleaks`, etc.) siguen sin ejecutarse.
