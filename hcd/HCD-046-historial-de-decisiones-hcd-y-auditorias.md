# Historial de decisiones HCD y auditorías en la memoria

ID: HCD-046
Tipo: feature
Carril: full
Motivo: Borra memorias (riesgo de datos), cambia el esquema y añade un contrato MCP, con decisiones interdependientes (alcance, consolidación, enlace por archivo, consulta)

## Intención
Que cada unidad HCD y cada auditoría tenga una ficha consultable en la memoria (búsqueda, hook de edición y `mem hcd history`) con intención, alternativa descartada, aclaraciones y estado, sin las memorias sueltas por paso.

## Alcance
Incluido: ficha HCD, ficha de auditoría, enlace archivo → ficha, backfill con consolidación y consulta, detallados así:
- **Ficha HCD**: memoria `decision` con `topic_key` `hcd/HCD-NNN` y `filepath` igual al documento de la unidad. Lleva título y estado, tipo, carril, fecha, intención, motivo, alcance, alternativa descartada, aclaraciones `Qn → respuesta`, revisiones aprobadas y archivos citados (`domain.CitedPaths`). Se actualiza con upsert en Start, Clarify (respuesta), Approve, Close, Cancel y Promote, y sustituye a las memorias sueltas de `HCD.recordDecision` (`application/usecases/hcd.go`).
- **Ficha de auditoría**: memoria `discovery` con `topic_key` `audit/<archivo>` por cada informe de `docs/audits/`. Lleva título, fecha, veredicto, tabla de hallazgos y estado de cierre si lo hay. El parser es tolerante: si un informe no tiene tabla reconocible, su ficha lleva solo título y resumen.
- **Enlace archivo → ficha**: tabla `memory_files`, migrada al abrir la base. `adapters/secondary/persistence/edit_context.go` devuelve también las fichas enlazadas al archivo que se edita.
- **Backfill**: `mem hcd index [--apply]` (sin `--apply` es solo vista previa), idempotente y por la vía inerte (sin sinapsis ni exportación ADR). Crea o actualiza todas las fichas y borra las memorias sueltas de pasos HCD, pero solo si su título coincide exactamente con un patrón de `recordDecision`. También indexa los documentos `hcd/HCD-*.md` que no tienen fila en el motor (Q7): la unidad toma id, título, tipo y carril del documento, y el estado de su sección Siguiente.
- **Consulta**: `mem hcd history [texto] [--file ruta] [--json]` y la tool MCP `hcd_history`.
Excluido: las specs de Spec Kit (`specs/`); otras memorias que mencionan una unidad (bugfix, discovery, checkpoints), que nunca se borran ni se reescriben; cambiar el formato del documento HCD o de los informes; indexar la evidencia (salidas de comandos).

## ¿Qué pasa si no lo hacemos?
De 45 unidades, las antiguas (HCD-001, HCD-012) no tienen ninguna memoria. Las recientes solo tienen líneas como «HCD-041: cerrada — Cerrada con aprobación sobre la revisión 0a94…», que no explican qué se decidió ni por qué, y ocupan de 4 a 6 entradas por unidad en «Decisiones Técnicas» del contexto. Para saber por qué se hizo algo hay que abrir y leer los documentos a mano, y los 10 informes de auditoría no se pueden buscar.

## Alternativa más barata
Solo un backfill que añada fichas sin tocar el resto. Es más barato, pero el ruido de las líneas sueltas sigue y crece con cada unidad, y el hook de edición no muestra nada porque busca por `filepath` exacto. Se descarta porque la persona eligió consolidar y consultar al editar.

## Historias
### US1. Cada unidad HCD tiene una ficha viva
Como agente que trabaja con HCD, quiero que cada unidad mantenga una única ficha en la memoria con lo decidido, para consultar el porqué sin abrir el documento.
- C-01: Dada una unidad recién iniciada, cuando se ejecuta Start, entonces existe una memoria `decision` con `topic_key` `hcd/HCD-NNN` que contiene intención y motivo del carril.
- C-02: Dada una unidad que pasa por aclaración, aprobación y cierre, cuando termina, entonces sigue habiendo una sola memoria para ella, con las aclaraciones `Qn → respuesta`, las revisiones aprobadas y el estado «cerrada», y no se creó ninguna memoria suelta por paso.
- C-03: Dado un documento con texto privado, cuando se escribe la ficha, entonces la ficha no contiene ese texto.

### US2. El historial existente se indexa y se consolida
Como persona que ya tiene unidades y auditorías, quiero un comando que indexe todo el historial y retire las líneas sueltas, para que el contexto muestre fichas y no ruido.
- C-04: Dadas unidades cerradas, canceladas y activas, cuando se ejecuta `mem hcd index --apply`, entonces cada una tiene su ficha; repetir el comando no crea ni cambia nada.
- C-05: Dadas memorias sueltas de pasos HCD («HCD-041: cerrada», «HCD-041: aclaración Q1») y una memoria bugfix titulada «HCD-041: …», cuando se aplica el índice, entonces las sueltas desaparecen y la bugfix se conserva.
- C-06: Dado `mem hcd index` sin `--apply`, cuando se ejecuta, entonces informa cuántas fichas crearía o actualizaría y cuántas memorias borraría, sin escribir nada.
- C-07: Dados los informes de `docs/audits/`, cuando se indexan, entonces cada informe tiene una ficha `audit/<archivo>` con sus hallazgos (ID, severidad y problema) cuando el informe tiene tabla, o con título y resumen si no la tiene.
- C-11: Dado un documento `hcd/HCD-NNN-*.md` cerrado sin fila en el motor, cuando se aplica el índice, entonces existe su ficha con estado «cerrada» y sus memorias sueltas de pasos se retiran.

### US3. El historial se consulta donde hace falta
Como agente o persona que va a cambiar código, quiero encontrar las decisiones que tocaron un archivo o un tema, para no deshacer algo decidido.
- C-08: Dada una ficha que cita `adapters/secondary/persistence/memory.go`, cuando el hook de edición prepara el contexto de ese archivo, entonces la ficha aparece entre sus memorias.
- C-09: Dado el historial indexado, cuando se ejecuta `mem hcd history topic_key`, entonces lista en orden cronológico las fichas que mencionan el texto, con id, fecha, estado y título; con `--file` filtra por archivo enlazado y con `--json` devuelve la misma información estructurada.
- C-10: Dado el servidor MCP, cuando un agente llama a `hcd_history` con texto o archivo, entonces recibe el mismo listado que la CLI.

## Aclaraciones
- [x] Q1. Alcance: ¿qué historial se indexa: solo unidades HCD, HCD y auditorías, o también las specs de Spec Kit? → HCD + auditorías: una ficha por unidad HCD y una por informe de docs/audits; las specs quedan fuera
- [x] Q2. Ruido: ¿qué se hace con las memorias sueltas por paso (carril, aclaración, aprobada, cerrada)? → Consolidar en la ficha (Recommended): cada paso actualiza una sola memoria por unidad (topic_key hcd/HCD-NNN); el backfill fusiona y borra las sueltas existentes
- [x] Q3. Consulta: ¿por dónde se consulta el historial? → Búsqueda de memoria, contexto al editar un archivo (hook de edición) y comando mem hcd history con tool MCP
- [x] Q4. Momento: ¿la ficha HCD se escribe (upsert) en cada paso del motor o solo al cerrar o cancelar la unidad? → En cada paso (Recommended): Start, respuesta de Clarify, Approve, Close, Cancel y Promote hacen upsert de la misma ficha
- [x] Q5. Enlace por archivo: ¿una tabla memory_files (memoria ↔ ruta) que consulta el hook de edición, o solo el texto «Archivos:» dentro de la ficha? → Tabla memory_files (Recommended): (proyecto, memoria, ruta) migrada al abrir; el hook de edición y mem hcd history --file la consultan
- [x] Q6. Borrado del backfill: ¿mem hcd index borra las memorias sueltas directamente o exige un paso de confirmación? → Dry-run + confirmación (Recommended): sin flags mem hcd index solo informa qué crearía y qué borraría; para escribir hace falta --apply, como mem sanitize
- [x] Q7. Unidades sin registro en el motor: HCD-030 y HCD-031 tienen documento en hcd/ pero no fila en hcd_units (creadas en otro worktree). ¿El backfill las indexa desde su documento o quedan fuera? → Indexar desde el documento (Recommended): el backfill recorre también hcd/HCD-*.md; una unidad sin fila toma id, título, tipo y carril del documento y el estado de su sección Siguiente; su ficha no lleva aprobaciones y sus memorias sueltas se consolidan igual

## Slices
### VS-01. Cada unidad HCD mantiene una ficha viva
Tareas: 1, 2
Criterios: C-01, C-02, C-03

### VS-02. La ficha aparece al editar los archivos que cita
Tareas: 3
Criterios: C-08

### VS-03. Cada auditoría tiene su ficha
Tareas: 4
Criterios: C-07

### VS-04. El historial existente se indexa y se consolida
Tareas: 5, 6
Criterios: C-04, C-05, C-06, C-11

### VS-05. El historial se consulta por CLI y MCP
Tareas: 7, 8
Criterios: C-09, C-10

### VS-06. Verificación real y regresión
Tareas: 9, 10
Criterios: C-04, C-08, C-09

## Checklist
- [x] 1. `domain.RenderHCDDigest` arma título y contenido de la ficha (intención, motivo, alcance, alternativa, aclaraciones, aprobaciones y archivos, con tope de tamaño) y `domain.IsHCDStepMemoryTitle` reconoce solo los títulos exactos de los pasos → `go test ./domain/ -run 'TestRenderHCDDigest|TestIsHCDStepMemoryTitle' -count=1`
- [x] 2. El caso de uso HCD hace upsert de una sola ficha `hcd/HCD-NNN` en Start, Clarify, Approve, Close, Cancel y Promote, sin memorias sueltas ni texto privado → `go test ./application/usecases/ -run TestHCDDigest_ -count=1`
- [x] 3. La tabla `memory_files` se migra, se rellena al guardar una ficha, se limpia al borrar la memoria, y `ListMemoriesByFilepath` devuelve las fichas enlazadas; el contexto de edición pone las fichas de auditoría después de las decisiones → `go test ./adapters/secondary/persistence/ ./application/usecases/ -run 'TestDecisionHistory_|TestListMemoriesByFilepath_IncluyeEnlaces|TestEditContext_PriorizaDecisionesSobreAuditorias' -count=1`
- [x] 4. `domain.ParseAuditReport` extrae título, fecha, veredicto, hallazgos y estado de cierre de los informes reales, y tolera un informe sin tabla → `go test ./domain/ -run TestParseAuditReport -count=1`
- [x] 5. El caso de uso `IndexHistory` da una vista previa sin escribir, aplica con upsert inerte, es idempotente y solo borra memorias de pasos (sobrevive una bugfix «HCD-041: …») → `go test ./application/usecases/ -run TestIndexHistory_ -count=1`
- [x] 6. `mem hcd index` muestra la vista previa y `mem hcd index --apply` escribe, en texto y en `--json` → `go test ./adapters/primary/cli/hcd/ -run TestExecuteIndex -count=1`
- [x] 7. `mem hcd history [texto] [--file ruta] [--json]` lista las fichas en orden cronológico con id, fecha, estado y título → `go test ./adapters/primary/cli/hcd/ -run TestExecuteHistory -count=1`
- [x] 8. La tool MCP `hcd_history` está registrada y devuelve el mismo listado que la CLI → `go test ./adapters/primary/cli/ -run 'TestMCP_HCDHistory|TestPublishedSchemasHaveNoNullUnions' -count=1`
- [x] 9. En el almacén real, `mem hcd index --apply` deja una ficha por unidad y por auditoría, y `history --file adapters/secondary/persistence/memory.go` lista HCD-044 → `go run ./infrastructure hcd history --file adapters/secondary/persistence/memory.go | grep -q HCD-044`
- [x] 10. La suite completa sigue en verde → `go vet ./... && go test ./... -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-06 19:37 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain/ -run 'TestRenderHCDDigest|TestIsHCDStepMemoryTitle' -count=1` (salida 1)
      # mem/domain [mem/domain.test]
      domain/hcd_digest_test.go:64:7: undefined: RenderHCDDigest
      domain/hcd_digest_test.go:91:5: undefined: HCDDigestTopic
      domain/hcd_digest_test.go:92:26: undefined: HCDDigestTopic
      domain/hcd_digest_test.go:97:12: undefined: RenderHCDDigest
      domain/hcd_digest_test.go:97:72: undefined: HCDDigestMaxContent
      domain/hcd_digest_test.go:118:13: undefined: IsHCDStepMemoryTitle
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-06 19:38 -05:00 · tarea 2 · rojo observado · caducada · `go test ./application/usecases/ -run TestHCDDigest_ -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd_test.go:87:24: cannot use d (variable of type *hcdDecisions) as usecases.HCDDecisionSink value in argument to uc.ConfigureDecisions: *hcdDecisions does not implement usecases.HCDDecisionSink (missing method Insert)
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-06 19:40 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/secondary/persistence/ -run 'TestDecisionHistory_|TestListMemoriesByFilepath_IncluyeEnlaces' -count=1` (salida 1)
      # mem/adapters/secondary/persistence [mem/adapters/secondary/persistence.test]
      adapters/secondary/persistence/decision_history_test.go:37:10: undefined: NewDecisionHistoryRepository
      adapters/secondary/persistence/decision_history_test.go:60:13: undefined: NewDecisionHistoryRepository
      adapters/secondary/persistence/decision_history_test.go:71:13: undefined: NewDecisionHistoryRepository
      adapters/secondary/persistence/decision_history_test.go:86:10: undefined: NewDecisionHistoryRepository
      FAIL	mem/adapters/secondary/persistence [build failed]
      FAIL
- 2026-11-06 19:41 -05:00 · tarea 4 · rojo observado · caducada · `go test ./domain/ -run TestParseAuditReport -count=1` (salida 1)
      # mem/domain [mem/domain.test]
      domain/audit_digest_test.go:20:20: undefined: AuditReport
      domain/audit_digest_test.go:20:45: undefined: AuditFinding
      domain/audit_digest_test.go:26:9: undefined: AuditFinding
      domain/audit_digest_test.go:31:8: undefined: ParseAuditReport
      domain/audit_digest_test.go:44:8: undefined: ParseAuditReport
      domain/audit_digest_test.go:58:8: undefined: RenderAuditDigest
      domain/audit_digest_test.go:75:8: undefined: ParseAuditReport
      domain/audit_digest_test.go:76:8: undefined: RenderAuditDigest
      domain/audit_digest_test.go:81:5: undefined: AuditDigestTopic
      domain/audit_digest_test.go:82:26: undefined: AuditDigestTopic
      domain/audit_digest_test.go:82:26: too many errors
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-06 19:43 -05:00 · tarea 5 · rojo observado · caducada · `go test ./application/usecases/ -run TestIndexHistory_ -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/index_history_test.go:57:18: undefined: usecases.IndexHistory
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-06 19:45 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/cli/hcd/ -run TestExecuteIndex -count=1` (salida 1)
      # mem/adapters/primary/cli/hcd [mem/adapters/primary/cli/hcd.test]
      adapters/primary/cli/hcd/history_test.go:40:7: deps.HistoryIndex undefined (type *clikit.Deps has no field or method HistoryIndex)
      adapters/primary/cli/hcd/history_test.go:47:7: deps.HistoryQuery undefined (type *clikit.Deps has no field or method HistoryQuery)
      adapters/primary/cli/hcd/history_test.go:47:32: undefined: usecases.HistoryQuery
      FAIL	mem/adapters/primary/cli/hcd [build failed]
      FAIL
- 2026-11-06 19:45 -05:00 · tarea 7 · rojo observado · caducada · `go test ./adapters/primary/cli/hcd/ -run TestExecuteHistory -count=1` (salida 1)
      # mem/adapters/primary/cli/hcd [mem/adapters/primary/cli/hcd.test]
      adapters/primary/cli/hcd/history_test.go:40:7: deps.HistoryIndex undefined (type *clikit.Deps has no field or method HistoryIndex)
      adapters/primary/cli/hcd/history_test.go:47:7: deps.HistoryQuery undefined (type *clikit.Deps has no field or method HistoryQuery)
      adapters/primary/cli/hcd/history_test.go:47:32: undefined: usecases.HistoryQuery
      FAIL	mem/adapters/primary/cli/hcd [build failed]
      FAIL
- 2026-11-06 19:47 -05:00 · tarea 8 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestMCP_HCDHistory|TestPublishedSchemasHaveNoNullUnions' -count=1` (salida 1)
      panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
      [signal SIGSEGV: segmentation violation code=0x2 addr=0x8 pc=0x105165710]
      goroutine 50 [running]:
      testing.tRunner.func1.2({0x105dd6448, 0x105f69860})
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2125 +0x1a0
      testing.tRunner.func1()
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2128 +0x2c8
      panic({0x105dd6448?, 0x105f69860?})
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/runtime/panic.go:859 +0x120
      mem/adapters/primary/cli.toolText(...)
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_session_summary_test.go:40
      mem/adapters/primary/cli.TestMCP_HCDHistory(0x430ced7dd688)
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_history_tools_test.go:42 +0x4c0
      testing.tRunner(0x430ced7dd688, 0x105e9b270)
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2195 +0xc4
      created by testing.(*T).Run in goroutine 1
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2260 +0x3b8
      FAIL	mem/adapters/primary/cli	0.474s
      FAIL
- 2026-11-06 19:57 -05:00 · tarea 1 · verde · vigente · `go test ./domain/ -run 'TestRenderHCDDigest|TestIsHCDStepMemoryTitle' -count=1` (salida 0)
      ok  	mem/domain	0.148s
- 2026-11-06 19:57 -05:00 · tarea 2 · verde · vigente · `go test ./application/usecases/ -run TestHCDDigest_ -count=1` (salida 0)
      ok  	mem/application/usecases	0.737s
- 2026-11-06 19:57 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/secondary/persistence/ ./application/usecases/ -run 'TestDecisionHistory_|TestListMemoriesByFilepath_IncluyeEnlaces|TestEditContext_PriorizaDecisionesSobreAuditorias' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.252s
      ok  	mem/application/usecases	0.186s
- 2026-11-06 19:57 -05:00 · tarea 4 · verde · vigente · `go test ./domain/ -run TestParseAuditReport -count=1` (salida 0)
      ok  	mem/domain	0.126s
- 2026-11-06 19:57 -05:00 · tarea 5 · verde · vigente · `go test ./application/usecases/ -run TestIndexHistory_ -count=1` (salida 0)
      ok  	mem/application/usecases	0.372s
- 2026-11-06 19:57 -05:00 · tarea 6 · verde · vigente · `go test ./adapters/primary/cli/hcd/ -run TestExecuteIndex -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.450s
- 2026-11-06 19:57 -05:00 · tarea 7 · verde · vigente · `go test ./adapters/primary/cli/hcd/ -run TestExecuteHistory -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.290s
- 2026-11-06 19:57 -05:00 · tarea 8 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestMCP_HCDHistory|TestPublishedSchemasHaveNoNullUnions' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.456s
- 2026-11-06 19:57 -05:00 · tarea 9 · verde · vigente · `go run ./infrastructure hcd history --file adapters/secondary/persistence/memory.go | grep -q HCD-044` (salida 0)
- 2026-11-06 19:57 -05:00 · tarea 10 · verde · vigente · `go vet ./... && go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/generator	1.244s
      ok  	mem/adapters/secondary/gitdiff	0.718s
      ok  	mem/adapters/secondary/hcdfiles	3.293s
      ok  	mem/adapters/secondary/hostbind	0.182s
      ok  	mem/adapters/secondary/launcher	0.632s
      ok  	mem/adapters/secondary/persistence	32.200s
      ok  	mem/adapters/secondary/release	0.391s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.232s
      ok  	mem/adapters/secondary/speckit	0.132s
      ok  	mem/adapters/secondary/tokens	0.157s
      ok  	mem/adapters/secondary/usage	0.147s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	14.989s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.204s
      ok  	mem/infrastructure	0.445s
      ok  	mem/tests/contract	96.480s
      ok  	mem/tests/integration	48.428s
      ?   	mem/version	[no test files]

## Siguiente
Cerrada con aprobación sobre la revisión 7eb79f5f1bc6.
