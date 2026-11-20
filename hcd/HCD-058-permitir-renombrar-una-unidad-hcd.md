# Permitir renombrar una unidad HCD

ID: HCD-058
Tipo: fix
Carril: fast
Motivo: Renombrar un HCD es imposible: no hay comando, editar el título no se refleja en el motor ni en la memoria y renombrar el archivo bloquea la unidad (open …: no such file or directory)

## Intención
mem hcd rename y la tool MCP hcd_rename cambian título, slug, DocPath y archivo (también en unidades cerradas); la edición manual del título o del nombre del archivo se sincroniza; la ficha del historial y el grafo se actualizan al instante

## Reproducción
En un proyecto sandbox (`HOME` temporal, `mem` v2.54.x): `mem hcd start --kind fix --lane fast --title "Titulo original" …` crea `hcd/HCD-001-titulo-original.md`.

1. `mem hcd rename --title x` → `comando desconocido: rename`. No hay vía CLI ni MCP para cambiar el título.
2. Editar a mano la línea `# Titulo nuevo` del documento → `mem hcd show` sigue mostrando la unidad con el título y la ruta originales; la ficha del historial (`RenderHCDDigest` usa `Unit.Title`) conserva el nombre viejo.
3. `mv hcd/HCD-001-titulo-original.md hcd/HCD-001-titulo-nuevo.md` → `mem hcd show` falla con `open …/hcd/HCD-001-titulo-original.md: no such file or directory`: la unidad queda bloqueada.

Causa raíz: `HCD.Start` fija `Title`, `Slug` y `DocPath` una sola vez; ningún caso de uso los cambia, `HCDStore.Save` no persiste `slug` y `Show` lee `DocPath` sin buscar el documento por su ID.

## Alcance
Incluido: caso de uso `HCD.Rename` y comando `mem hcd rename [unidad] --title T` (vía manual y vía IA por CLI) más la tool MCP `hcd_rename`; renombra título, slug, `DocPath` y el archivo `hcd/<ID>-<slug>.md`, y reescribe la línea `# título`. Funciona en unidades activas, cerradas y canceladas, sin cambiar el foco ni el estado. Conserva aprobaciones y evidencia (el título no forma parte de `definition_revision`). Actualiza al instante la ficha del historial (mismo `topic_key`, nuevo título y `filepath`) y reapunta a la nueva ruta las memorias y enlaces `memory_files` que citaban la ruta vieja. Reconciliación de la edición manual: si el documento no está en `DocPath`, el motor lo localiza por `hcd/<ID>-*.md` y adopta esa ruta; si la línea `# título` difiere del título registrado, adopta el del documento. Actualizar las skills `hcd` y `hcd-define` para que la IA use el comando.
Excluido: cambiar ID, número, tipo o carril; renombrar el archivo cuando solo se edita la línea `# título` a mano (se sincroniza el título; el archivo se alinea con `mem hcd rename`); reindexar grafos externos (codegraph y codebase-memory vigilan el árbol por su cuenta).

## ¿Qué pasa si no lo hacemos?
Un título mal puesto queda para siempre en el motor, en el contexto de arranque y en el historial de decisiones. Quien intenta corregirlo a mano renombrando el archivo deja la unidad inutilizable y debe restaurar el nombre exacto para recuperarla.

## Alternativa más barata
Documentar que el título no se puede cambiar o que hay que editar SQLite a mano. No resuelve el bloqueo por renombrar el archivo ni deja la memoria coherente, y contradice el pedido explícito de la persona.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → re-ejecutar la reproducción en sandbox con el binario nuevo
- [x] 2. `HCD.Rename` cambia título, slug, `DocPath`, archivo y encabezado, también en unidades cerradas, y conserva aprobaciones → `go test ./application/usecases -run TestHCDRename`
- [x] 3. La edición manual (archivo movido o `# título` editado) se reconcilia sin bloquear → `go test ./application/usecases -run TestHCDReconcile`
- [x] 4. La ficha y las memorias que citan la ruta vieja se reapuntan a la nueva → `go test ./adapters/secondary/persistence -run TestRetargetPath`
- [x] 5. `mem hcd rename` y la tool MCP `hcd_rename` renombran la unidad → `go test ./adapters/primary/cli/... -run 'TestHCDRenameCLI|TestHCDRenameMCP'`
- [x] 6. Las skills `hcd` y `hcd-define` documentan el comando → `grep -q 'mem hcd rename' infrastructure/templates/hcd/hcd/SKILL.md infrastructure/templates/hcd/hcd-define/SKILL.md`
- [x] 7. La suite completa sigue verde → `go test ./...`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-20 18:18 -05:00 · tarea 1 · rojo observado · caducada · `bash /tmp/hcd058-repro.sh mem` (salida 2)
      comando desconocido: rename
- 2026-11-20 18:30 -05:00 · tarea 2 · verde · vigente · `go test ./application/usecases -run TestHCDRename` (salida 0)
      ok  	mem/application/usecases	1.773s
- 2026-11-20 18:30 -05:00 · tarea 1 · verde · vigente · `go build -o /tmp/mem-hcd058 ./infrastructure && bash /tmp/hcd058-repro.sh /tmp/mem-hcd058` (salida 0)
      OK: renombrado por comando, por archivo y por título
- 2026-11-20 18:30 -05:00 · tarea 6 · verde · vigente · `grep -q 'mem hcd rename' infrastructure/templates/hcd/hcd/SKILL.md infrastructure/templates/hcd/hcd-define/SKILL.md` (salida 0)
- 2026-11-20 18:30 -05:00 · tarea 3 · verde · vigente · `go test ./application/usecases -run TestHCDReconcile` (salida 0)
      ok  	mem/application/usecases	1.090s
- 2026-11-20 18:30 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/cli/... -run 'TestHCDRenameCLI|TestHCDRenameMCP'` (salida 0)
      ok  	mem/adapters/primary/cli	0.720s
      ok  	mem/adapters/primary/cli/clikit	0.196s [no tests to run]
      ok  	mem/adapters/primary/cli/hcd	(cached)
      ok  	mem/adapters/primary/cli/review	0.209s [no tests to run]
      ok  	mem/adapters/primary/cli/socratica	0.285s [no tests to run]
- 2026-11-20 18:30 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/secondary/persistence -run TestRetargetPath` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.268s
- 2026-11-20 18:30 -05:00 · tarea 7 · verde · vigente · `go test ./...` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	(cached)
      ok  	mem/adapters/secondary/hostbind	(cached)
      ok  	mem/adapters/secondary/i18n	(cached)
      ok  	mem/adapters/secondary/launcher	(cached)
      ok  	mem/adapters/secondary/persistence	(cached)
      ok  	mem/adapters/secondary/release	(cached)
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	(cached)
      ok  	mem/adapters/secondary/speckit	(cached)
      ok  	mem/adapters/secondary/tokens	(cached)
      ok  	mem/adapters/secondary/usage	(cached)
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	(cached)
      ?   	mem/assets	[no test files]
      ok  	mem/domain	(cached)
      ok  	mem/domain/i18n	(cached)
      ok  	mem/infrastructure	0.757s
      ok  	mem/tests/contract	111.635s
      ok  	mem/tests/integration	(cached)
      ?   	mem/version	[no test files]

## Siguiente
Cerrada con aprobación sobre la revisión dd98fd27f696.
