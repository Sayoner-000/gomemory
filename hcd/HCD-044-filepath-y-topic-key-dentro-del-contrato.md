# Filepath y topic_key dentro del contrato de privacidad

ID: HCD-044
Tipo: fix
Carril: full
Motivo: Privacidad, riesgo alto y decisiones interdependientes (rechazar o transformar al escribir, proteger lecturas históricas, saneamiento) — auditoría 2026-11-06 H-03

## Intención
Que `filepath` y `topic_key` de una memoria nunca guarden ni muestren texto marcado como privado, igual que ya pasa con el título, el contenido y el prompt originante.

## Reproducción
`insertMemory` (`adapters/secondary/persistence/memory.go:111` en la deduplicación y `:130` en la inserción) e `ImportMemory` (`adapters/secondary/persistence/memory.go:729`) escriben `m.Filepath` y `m.TopicKey` sin proteger. `GetMemoryByID` los devuelve tal cual (`finishScannedMemory` solo quita controles de terminal) y `renderMemoryDetail` (`adapters/primary/cli/cmd_mcp_render.go:44`) muestra el archivo en `get_memory`. El saneamiento (`adapters/secondary/persistence/privacy_maintenance.go:141`) solo mira título, contenido y prompt.
Programa de la auditoría: `docs/audits/evidence/2026-11-06-project/metadata.go.txt` → `filepath="<private>SYNTH-PATH-SECRET</private>" topic_key="<private>SYNTH-TOPIC-SECRET</private>"`.

## Alcance
Incluido: las tres escrituras de producción de los metadatos (inserción y deduplicación en `insertMemory`, que cubre MCP `save_memory`, CLI, TUI, hooks y semillas, más `ImportMemory`, que usa `ImportBundle`); la lectura de memorias (`finishScannedMemory`, que alimenta `get_memory`, listados, búsqueda y exportación) y la exportación (`ExportProject`); el saneamiento histórico (`readSanitizeCandidates` y `privacyTx.Apply`).
Excluido: otros campos de la memoria (ya protegidos); `source_review_id`, que solo lo escribe gomemory con ids de revisión; metadatos de otras tablas; cambiar el esquema de la base.

## ¿Qué pasa si no lo hacemos?
Un agente o una persona que pase una ruta o una clave de tópico con un bloque `<private>` (por ejemplo, al copiar un comando con un token) lo deja guardado en SQLite, visible en `get_memory`, en el bundle exportado y en el contexto. Además, `mem sanitize` no lo detecta, así que la promesa de privacidad no se cumple para esas filas.

## Alternativa más barata
Proteger solo la escritura nueva. Es más barata, pero deja visibles las filas que ya existen y `mem sanitize` seguiría sin verlas; la auditoría lo señala expresamente («rechazar nuevas entradas no limpia las existentes»). Se descarta como solución completa.

## Historias
### US1. Los metadatos nuevos no guardan texto privado
Como persona que guarda memorias por MCP, CLI o importación, quiero que una ruta o una clave de tópico con texto privado no se persista, para que lo marcado como `<private>` nunca llegue a la base.
- C-01: Dada una memoria con `filepath` que contiene un bloque privado, cuando se guarda por `insertMemory`, entonces el guardado falla con error de validación y la base no contiene el canario.
- C-02: Dada una memoria con `topic_key` que contiene un bloque privado, cuando se guarda por `insertMemory` (inserción o deduplicación sobre una memoria existente), entonces el guardado falla con error de validación, la memoria existente no cambia y la base no contiene el canario.
- C-03: Dado un bundle con metadatos privados, cuando se importa con `ImportBundle`, entonces la importación falla con error de validación y la base no contiene el canario.
- C-04: Dada una clave de tópico pública, cuando se guarda dos veces, entonces la segunda actualiza la misma memoria (el upsert sigue estable).

### US2. Las filas que ya existen no se muestran ni se quedan sin sanear
Como persona que ya tiene memorias guardadas antes de esta corrección, quiero que sus metadatos privados no se muestren y que `mem sanitize` los limpie, para que la privacidad valga también hacia atrás.
- C-05: Dada una fila histórica con metadatos privados, cuando se lee con `GetMemoryByID` o se exporta, entonces el resultado no contiene el canario.
- C-06: Dada una fila histórica con metadatos privados, cuando se ejecuta la vista previa del saneamiento, entonces aparece como candidata.
- C-07: Dada esa fila, cuando se aplica el saneamiento, entonces `filepath` queda con `ProtectText` aplicado, `topic_key` queda NULL, la base deja de contener el canario y la memoria conserva su identidad, su título y su contenido.

## Aclaraciones
- [x] Q1. Escritura: si filepath o topic_key traen texto privado, ¿se rechaza el guardado con error de validación, se protege el valor (redactar) o se descartan los metadatos y se guarda la memoria? → Rechazar (Recommended): error de validación y no se guarda nada; no cambia en silencio la identidad del topic_key ni provoca colisiones de upsert
- [x] Q2. Lectura histórica: las filas ya guardadas con metadatos privados, ¿se protegen al leerlas (get_memory, listados, búsqueda, exportación) o solo se limpian con mem sanitize? → Proteger al leer (Recommended): finishScannedMemory aplica ProtectText a filepath y topic_key; get_memory, listados, búsqueda y exportación nunca los muestran aunque no se haya saneado
- [x] Q3. Saneamiento: al aplicar mem sanitize sobre una fila con metadatos privados, ¿qué valor queda en filepath y en topic_key? → filepath protegido, topic_key NULL (Recommended): filepath = ProtectText(filepath); topic_key = NULL para evitar colisiones de upsert (idx_memories_topic no es único); identidad, título y contenido intactos

## Slices
### VS-01. Las escrituras rechazan metadatos privados
Tareas: 1, 2, 3
Criterios: C-01, C-02, C-03, C-04

### VS-02. Las filas históricas se protegen al leer y se sanean
Tareas: 4, 5
Criterios: C-05, C-06, C-07

## Checklist
- [x] 1. La reproducción ya no falla: el programa de la auditoría ya no muestra los canarios → `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/metadata.go.txt $d/main.go && ! (go run $d/main.go 2>&1 | grep -q SYNTH-)`
- [x] 2. `insertMemory` rechaza filepath o topic_key privados con `ErrValidation` en la inserción y en la deduplicación, sin tocar la memoria existente, y el upsert con clave pública sigue estable → `go test ./adapters/secondary/persistence/ -run TestMetadatosPrivados_SeRechazanAlGuardar -count=1`
- [x] 3. `ImportBundle` e `ImportMemory` rechazan metadatos privados antes de escribir → `go test ./application/usecases/ ./adapters/secondary/persistence/ -run 'TestImportBundle_RechazaMetadatosPrivados|TestImportMemory_RechazaMetadatosPrivados' -count=1`
- [x] 4. Leer o exportar una fila histórica con metadatos privados no devuelve los canarios → `go test ./adapters/secondary/persistence/ ./application/usecases/ -run 'TestMetadatosPrivados_HistoricosSeProtegenAlLeer|TestExportProject_ProtegeMetadatosHistoricos' -count=1`
- [x] 5. `mem sanitize` detecta las filas con metadatos privados y las deja con filepath protegido y topic_key NULL, sin tocar identidad, título ni contenido → `go test ./adapters/secondary/persistence/ -run TestPrivacyStore_SaneaMetadatosDeMemorias -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-06 07:21 -05:00 · tarea 1 · rojo observado · caducada · `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/metadata.go.txt $d/main.go && ! (go run $d/main.go 2>&1 | grep -q SYNTH-)` (salida 1)
- 2026-11-06 07:21 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/secondary/persistence/ -run TestMetadatosPrivados_SeRechazanAlGuardar -count=1` (salida 1)
      --- FAIL: TestMetadatosPrivados_SeRechazanAlGuardar (0.02s)
          --- FAIL: TestMetadatosPrivados_SeRechazanAlGuardar/filepath (0.00s)
              memory_metadata_privacy_test.go:42: InsertMemory = <nil>, quiero ErrValidation
          --- FAIL: TestMetadatosPrivados_SeRechazanAlGuardar/topic_key (0.00s)
              memory_metadata_privacy_test.go:42: InsertMemory = <nil>, quiero ErrValidation
          memory_metadata_privacy_test.go:59: dedup con filepath privado = <nil>, quiero ErrValidation
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.225s
      FAIL
- 2026-11-06 07:21 -05:00 · tarea 3 · rojo observado · caducada · `go test ./application/usecases/ ./adapters/secondary/persistence/ -run 'TestImportBundle_RechazaMetadatosPrivados|TestImportMemory_RechazaMetadatosPrivados' -count=1` (salida 1)
      --- FAIL: TestImportBundle_RechazaMetadatosPrivados (0.02s)
          portability_metadata_privacy_test.go:30: ImportBundle = {MemoriesImported:3 MemoriesSkipped:0 RelationsImported:0 RelationsSkipped:0} <nil>, quiero ErrValidation
      FAIL
      FAIL	mem/application/usecases	0.226s
      --- FAIL: TestImportMemory_RechazaMetadatosPrivados (0.02s)
          memory_metadata_privacy_test.go:85: ImportMemory = <nil>, quiero ErrValidation
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.223s
      FAIL
- 2026-11-06 07:21 -05:00 · tarea 4 · rojo observado · caducada · `go test ./adapters/secondary/persistence/ ./application/usecases/ -run 'TestMetadatosPrivados_HistoricosSeProtegenAlLeer|TestExportProject_ProtegeMetadatosHistoricos' -count=1` (salida 1)
                "version": 2,
                "exported_at": "2026-11-06T12:21:34Z",
                "source_project": "src",
                "memories": [
                  {
                    "ref_id": 1,
                    "type": "decision",
                    "title": "A",
                    "content": "cuerpo",
                    "filepath": "src/\u003cprivate\u003eSYNTH-PATH-0044\u003c/private\u003ea.go",
                    "topic_key": "\u003cprivate\u003eSYNTH-TOPIC-0044\u003c/private\u003e",
                    "created_at": "2026-11-06 07:21:34",
                    "updated_at": "2026-11-06 07:21:34"
                  }
                ],
                "relations": null
              }
      FAIL
      FAIL	mem/application/usecases	0.201s
      FAIL
- 2026-11-06 07:21 -05:00 · tarea 5 · rojo observado · caducada · `go test ./adapters/secondary/persistence/ -run TestPrivacyStore_SaneaMetadatosDeMemorias -count=1` (salida 1)
      --- FAIL: TestPrivacyStore_SaneaMetadatosDeMemorias (0.01s)
          memory_metadata_privacy_test.go:141: candidatos = [], quiero [memory_filepath:20 memory_topic_key:20]
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.200s
      FAIL
- 2026-11-06 07:22 -05:00 · tarea 1 · verde · caducada · `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/metadata.go.txt $d/main.go && ! (go run $d/main.go 2>&1 | grep -q SYNTH-)` (salida 0)
- 2026-11-06 07:22 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/persistence/ -run TestMetadatosPrivados_SeRechazanAlGuardar -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.209s
- 2026-11-06 07:22 -05:00 · tarea 3 · verde · caducada · `go test ./application/usecases/ ./adapters/secondary/persistence/ -run 'TestImportBundle_RechazaMetadatosPrivados|TestImportMemory_RechazaMetadatosPrivados' -count=1` (salida 0)
      ok  	mem/application/usecases	0.206s
      ok  	mem/adapters/secondary/persistence	0.195s
- 2026-11-06 07:22 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/persistence/ ./application/usecases/ -run 'TestMetadatosPrivados_HistoricosSeProtegenAlLeer|TestExportProject_ProtegeMetadatosHistoricos' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.176s
      ok  	mem/application/usecases	0.178s
- 2026-11-06 07:22 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/secondary/persistence/ -run TestPrivacyStore_SaneaMetadatosDeMemorias -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.166s
- 2026-11-06 07:28 -05:00 · tarea 1 · verde · caducada · `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/metadata.go.txt $d/main.go && ! (go run $d/main.go 2>&1 | grep -q SYNTH-)` (salida 0)
- 2026-11-06 07:28 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/persistence/ -run TestMetadatosPrivados_SeRechazanAlGuardar -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.238s
- 2026-11-06 07:28 -05:00 · tarea 3 · verde · caducada · `go test ./application/usecases/ ./adapters/secondary/persistence/ -run 'TestImportBundle_RechazaMetadatosPrivados|TestImportMemory_RechazaMetadatosPrivados' -count=1` (salida 0)
      ok  	mem/application/usecases	0.189s
      ok  	mem/adapters/secondary/persistence	0.185s
- 2026-11-06 07:28 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/persistence/ ./application/usecases/ -run 'TestMetadatosPrivados_HistoricosSeProtegenAlLeer|TestExportProject_ProtegeMetadatosHistoricos' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.176s
      ok  	mem/application/usecases	0.178s
- 2026-11-06 07:28 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/secondary/persistence/ -run TestPrivacyStore_SaneaMetadatosDeMemorias -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.178s
- 2026-11-06 07:30 -05:00 · tarea 1 · verde · vigente · `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/metadata.go.txt $d/main.go && ! (go run $d/main.go 2>&1 | grep -q SYNTH-)` (salida 0)
- 2026-11-06 07:30 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/secondary/persistence/ -run TestMetadatosPrivados_SeRechazanAlGuardar -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.197s
- 2026-11-06 07:30 -05:00 · tarea 3 · verde · vigente · `go test ./application/usecases/ ./adapters/secondary/persistence/ -run 'TestImportBundle_RechazaMetadatosPrivados|TestImportMemory_RechazaMetadatosPrivados' -count=1` (salida 0)
      ok  	mem/application/usecases	0.199s
      ok  	mem/adapters/secondary/persistence	0.182s
- 2026-11-06 07:30 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/secondary/persistence/ ./application/usecases/ -run 'TestMetadatosPrivados_HistoricosSeProtegenAlLeer|TestExportProject_ProtegeMetadatosHistoricos' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.166s
      ok  	mem/application/usecases	0.169s
- 2026-11-06 07:30 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/secondary/persistence/ -run TestPrivacyStore_SaneaMetadatosDeMemorias -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.485s

## Siguiente
Cerrada con aprobación sobre la revisión 2db1263e809f.
