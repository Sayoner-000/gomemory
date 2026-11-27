# Retirar ADR sync y anotación de impacto/hotspots inertes

ID: HCD-069
Tipo: technical
Carril: fast
Motivo: decisión ya tomada por la persona (memoria 2236) y alcance verificado con codegraph (2231, 2271); sin decisiones abiertas

## Intención
Que gomemory deje de cargar y ejecutar en cada turno y en cada guardado funciones sin efecto desde que se retiró codebase-memory-mcp (ADR sync y anotación de impacto/hotspots), conservando el refresco del snapshot de CodeGraph (MaybeRefresh/Refresh/Snapshot)


## Contexto y evidencia
- Decisión de la persona (memoria 2236, 2026-11-26): retirar ambas funciones en una unidad aparte de HCD-065.
- Verificado con codegraph (memoria 2231): ningún adaptador de producción implementa `ports.ADRSyncProvider`; `codegraphcli.Provider.ImpactFor` siempre devuelve `false` (`adapters/secondary/codegraph/codegraphcli/provider.go:76`) porque `codegraph status --json` no da fan-in. CodeGraph 1.6.4 no expone ADR ni hotspots.
- Corrección (memoria 2271): `MaybeRefresh` NO es inerte. `Refresh` ejecuta `codegraph init`, `index` o `sync` cuando el watcher no corre y alimenta la sección de arquitectura de `get_context`. Se conserva.
- `mem hcd history` no devuelve fichas sobre el tema; las decisiones previas están en las memorias citadas.

## Alcance
Incluido: retirar el ADR sync y la anotación de impacto/hotspots, con sus pruebas, ajustes y documentación.
- ADR sync: comando `mem adr-sync` (`adapters/primary/cli/cmd_adr_sync.go` y su entrada en `dispatcher.go` y en la ayuda), caso de uso `ImportADRs` (`application/usecases/import_adrs.go`) y su llamada en `hookTurnEnd`, puerto `ports.ADRSyncProvider` y `ports.ADRSyncRepository`, sus campos en `Container` y `cli.Deps`, `exportToADR`/`SetADRSync`/`SetAdrSyncEnabled`/`recordADRSyncAttempt` en `persistence/memory.go`, el repositorio `persistence/adr_sync.go`, `domain/adr_sync.go` y el documento ADR del dominio si solo lo usa esta función, y el ajuste `adr_sync_enabled`.
- Anotación de impacto/hotspots: `ImpactFor` del puerto `CodeGraphProvider` y sus implementaciones y dobles, `annotateImpact`/`SetCodeImpactProvider`/`codeImpactProvider` en `persistence/memory.go`, `domain.ImpactAnnotation` y `CodeImpactAnnotation`, el ajuste `code_impact_annotation_disabled`, la sección «Memoria conectada a código activo» de `get_context`, y `Clusters`/`Hotspots` (`CodeCluster`, `CodeHotspot`) de `CodeArchitecture`. Al implementar, el grafo mostró tres consumidores más de `ImpactFor`, igual de inertes y necesarios para quitarlo del puerto: las semillas de masa por hotspot (`ContextSeeds`/`DescribeSeeds`, usadas por `get_context` y `mem mass`), el refuerzo de prioridad de `mem pack` (`boostHotspotCandidates`) y la línea «Impacto» del contexto de edición (HCD-028: `EditContext.Impact`, `EditContextInput.Impact`). El comportamiento visible no cambia, porque `ImpactFor` siempre devolvía `false`.
- Las pruebas de lo retirado se retiran con ello; la documentación (README, `docs/`, ayuda del CLI) deja de anunciarlo.
- Un `settings.json` que aún contenga `adr_sync_enabled` o `code_impact_annotation_disabled` sigue cargando sin error.
- Tabla `adr_sync_records` (Q1, opción 1): se deja huérfana. No se borra ni se vacía, ningún código la vuelve a leer ni escribir y su migración de creación se conserva, para cumplir el esquema solo aditivo de la Constitución §25.1 y que un binario anterior siga abriendo la base. Se documenta como resto inerte.

Excluido: el refresco del snapshot de CodeGraph, las migraciones aplicadas, cualquier reimplementación y los hooks de Pi.
- `MaybeRefresh`, `Refresh`, `Snapshot`, `code-refresh` y la sección de arquitectura del grafo (totales y lenguajes) en `get_context`.
- Migraciones ya aplicadas: no se editan (§6).
- `MemoryRepository.Get` y `UpdateContent`: tienen otros llamadores (`cmd_get.go`, `cmd_mcp.go`, `consolidate_memories.go`, verificado con codegraph). Solo se corrigen sus comentarios, que hoy citan `ImportADRs`.
- `domain.StripImpactAnnotation` y su patrón: el gate de duplicados (`CheckNearDuplicates`) los usa para descontar notas de impacto ya guardadas en memorias antiguas, de cuando codebase-memory-mcp sí anotaba.
- Reimplementar ADR o hotspots sobre CodeGraph: solo si llega a exponer esas capacidades, en otra unidad.
- Los hooks en Pi (hallazgo de la memoria 2268): unidad aparte en pi-gomemory.

## ¿Qué pasa si no lo hacemos?
En ejecución, casi nada: `ImportADRs` no corre porque `ADRSyncProvider` es nil, y `annotateImpact` sale en cuanto `ImpactFor` devuelve `false`. El coste es de mantenimiento y de honestidad del producto: `mem adr-sync` y los ajustes `adr_sync_enabled` y `code_impact_annotation_disabled` prometen funciones que no hacen nada, el puerto `CodeGraphProvider` obliga a cada doble de prueba a implementar `ImpactFor`, y la documentación describe capacidades que no existen. Quien active `adr_sync_enabled` no obtiene nada ni recibe aviso.

## Alternativa más barata
Dejar el código y marcar `mem adr-sync` y los dos ajustes como obsoletos en la ayuda y la documentación. Cuesta poco, pero conserva el código muerto, el puerto ampliado y sus pruebas, y deja en el camino de cada guardado una llamada que no puede tener efecto. Se descarta porque la persona ya eligió retirarlo (memoria 2236).


## Aclaraciones
- [x] Q1. ¿La tabla adr_sync_records se borra con una migración, se vacía o se deja huérfana, dado que la Constitución §25.1 exige esquema solo aditivo? → 1

## Checklist
- [x] 1. Retirar el caso de uso ImportADRs y su llamada en hookTurnEnd → `bash -c '! go doc ./application/usecases ImportADRs >/dev/null 2>&1 && go build ./...'`
- [x] 2. Retirar el comando mem adr-sync y su entrada en el dispatcher y la ayuda → `bash -c '! go doc ./adapters/primary/cli CmdADRSync >/dev/null 2>&1 && go build ./...'`
- [x] 3. Retirar exportToADR, SetADRSync y SetAdrSyncEnabled de la persistencia de memorias → `bash -c '! go doc ./adapters/secondary/persistence SetADRSync >/dev/null 2>&1 && ! go doc ./adapters/secondary/persistence SetAdrSyncEnabled >/dev/null 2>&1 && go build ./...'`
- [x] 4. Retirar los puertos ADRSyncProvider y ADRSyncRepository, su repositorio y sus campos en Container y Deps → `bash -c '! go doc ./application/ports ADRSyncProvider >/dev/null 2>&1 && ! go doc ./application/ports ADRSyncRepository >/dev/null 2>&1 && go build ./...'`
- [x] 5. Retirar el dominio ADR (ADRDocument y ADRSyncRecord) → `bash -c '! go doc ./domain ParseADRDocument >/dev/null 2>&1 && ! go doc ./domain ADRSyncRecord >/dev/null 2>&1 && go build ./...'`
- [x] 6. Retirar ImpactFor del puerto CodeGraphProvider y de sus implementaciones y dobles → `bash -c '! go doc ./application/ports CodeGraphProvider.ImpactFor >/dev/null 2>&1 && go vet ./...'`
- [x] 7. Retirar annotateImpact, SetCodeImpactProvider, ImpactAnnotation y CodeImpactAnnotation conservando StripImpactAnnotation → `bash -c '! go doc ./adapters/secondary/persistence SetCodeImpactProvider >/dev/null 2>&1 && ! go doc ./domain CodeImpactAnnotation >/dev/null 2>&1 && go test ./domain -run "^TestStripImpactAnnotation" -count=1'`
- [x] 8. Retirar la sección de hotspots de get_context y CodeCluster/CodeHotspot de CodeArchitecture → `bash -c '! go doc ./domain CodeHotspot >/dev/null 2>&1 && ! go doc ./domain CodeCluster >/dev/null 2>&1 && go test ./application/usecases -run "^TestBuild_ExternalGraphSection$" -count=1'`
- [x] 9. Retirar los ajustes adr_sync_enabled y code_impact_annotation_disabled manteniendo la carga de un settings.json que aún los tenga → `go test ./adapters/secondary/persistence -run '^TestHCD069_AjustesRetiradosSiguenCargando$' -count=1`
- [x] 10. Retirar de README y docs/ el comando adr-sync, sus flags y las filas de los dos ajustes, dejando solo la nota de retirada (las guías HTML versionadas y las auditorías son históricas y no se editan, como en HCD-044) → `bash -c '! grep -rIlE "mem adr-sync|--adr-sync|--code-impact-annotation|[|] .(adr_sync_enabled|code_impact_annotation_disabled). [|]" README.md docs/ --exclude="Guia-*" --exclude-dir=audits'`
- [x] 11. Regresión completa sin fallos → `bash -c 'go vet ./... && go test ./... -count=1'`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-27 06:16 -05:00 · tarea 1 · rojo observado · caducada · `bash -c '! go doc ./application/usecases ImportADRs >/dev/null 2>&1 && go build ./...'` (salida 1)
- 2026-11-27 06:16 -05:00 · tarea 2 · rojo observado · caducada · `bash -c '! go doc ./adapters/primary/cli CmdADRSync >/dev/null 2>&1 && go build ./...'` (salida 1)
- 2026-11-27 06:16 -05:00 · tarea 3 · rojo observado · caducada · `bash -c '! go doc ./adapters/secondary/persistence SetADRSync >/dev/null 2>&1 && ! go doc ./adapters/secondary/persistence SetAdrSyncEnabled >/dev/null 2>&1 && go build ./...'` (salida 1)
- 2026-11-27 06:16 -05:00 · tarea 4 · rojo observado · caducada · `bash -c '! go doc ./application/ports ADRSyncProvider >/dev/null 2>&1 && ! go doc ./application/ports ADRSyncRepository >/dev/null 2>&1 && go build ./...'` (salida 1)
- 2026-11-27 06:16 -05:00 · tarea 5 · rojo observado · caducada · `bash -c '! go doc ./domain ParseADRDocument >/dev/null 2>&1 && ! go doc ./domain ADRSyncRecord >/dev/null 2>&1 && go build ./...'` (salida 1)
- 2026-11-27 06:16 -05:00 · tarea 6 · rojo observado · caducada · `bash -c '! go doc ./application/ports CodeGraphProvider.ImpactFor >/dev/null 2>&1 && go vet ./...'` (salida 1)
- 2026-11-27 06:16 -05:00 · tarea 7 · rojo observado · caducada · `bash -c '! go doc ./adapters/secondary/persistence SetCodeImpactProvider >/dev/null 2>&1 && ! go doc ./domain CodeImpactAnnotation >/dev/null 2>&1 && go test ./domain -run "^TestStripImpactAnnotation" -count=1'` (salida 1)
- 2026-11-27 06:16 -05:00 · tarea 8 · rojo observado · caducada · `bash -c '! go doc ./domain CodeHotspot >/dev/null 2>&1 && ! go doc ./domain CodeCluster >/dev/null 2>&1 && go test ./application/usecases -run "^TestBuild_ExternalGraphSection$" -count=1'` (salida 1)
- 2026-11-27 06:16 -05:00 · tarea 10 · rojo observado · caducada · `bash -c '! grep -rIl -E "adr-sync|adr_sync_enabled|code_impact_annotation_disabled" README.md docs/'` (salida 1)
      docs/architecture.md
      docs/Guia-Tecnica_GOMEMORY-v2.56.html
      docs/CONFIGURATION.md
      docs/MANUAL.md
      docs/Guia-SDD_HCD_GOMEMORY-v2.56.html
- 2026-11-27 06:17 -05:00 · tarea 1 · verde · caducada · `bash -c '! go doc ./application/usecases ImportADRs >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:18 -05:00 · tarea 2 · verde · caducada · `bash -c '! go doc ./adapters/primary/cli CmdADRSync >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:20 -05:00 · tarea 3 · verde · caducada · `bash -c '! go doc ./adapters/secondary/persistence SetADRSync >/dev/null 2>&1 && ! go doc ./adapters/secondary/persistence SetAdrSyncEnabled >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:21 -05:00 · tarea 4 · verde · caducada · `bash -c '! go doc ./application/ports ADRSyncProvider >/dev/null 2>&1 && ! go doc ./application/ports ADRSyncRepository >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:22 -05:00 · tarea 5 · verde · caducada · `bash -c '! go doc ./domain ParseADRDocument >/dev/null 2>&1 && ! go doc ./domain ADRSyncRecord >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:29 -05:00 · tarea 6 · verde · caducada · `bash -c '! go doc ./application/ports CodeGraphProvider.ImpactFor >/dev/null 2>&1 && go vet ./...'` (salida 0)
- 2026-11-27 06:29 -05:00 · tarea 7 · verde · caducada · `bash -c '! go doc ./adapters/secondary/persistence SetCodeImpactProvider >/dev/null 2>&1 && ! go doc ./domain CodeImpactAnnotation >/dev/null 2>&1 && go test ./domain -run "^TestStripImpactAnnotation" -count=1'` (salida 0)
      ok  	mem/domain	0.770s
- 2026-11-27 06:30 -05:00 · tarea 8 · verde · caducada · `bash -c '! go doc ./domain CodeHotspot >/dev/null 2>&1 && ! go doc ./domain CodeCluster >/dev/null 2>&1 && go test ./application/usecases -run "^TestBuild_ExternalGraphSection$" -count=1'` (salida 0)
      ok  	mem/application/usecases	0.220s
- 2026-11-27 06:30 -05:00 · tarea 9 · rojo observado · caducada · `go test ./adapters/secondary/persistence -run '^TestHCD069_AjustesRetiradosSiguenCargando$' -count=1` (salida 1)
                "auto_approve": false,
                "auto_approve_tools": null,
                "adr_sync_enabled": true,
                "code_impact_annotation_disabled": true,
                "budget": 24000,
                "compact_threshold": 48000,
                "dedup_window_days": 7,
                "review_max_fix_rounds": 2,
                "review_auto_fix_severities": [
                  "CRITICAL",
                  "HIGH"
                ],
                "expand_compact_threshold": 50,
                "context_default_budget": 4000,
                "context_min_relevance": 0.65,
                "context_max_items": 20
              }
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.186s
      FAIL
- 2026-11-27 06:31 -05:00 · tarea 9 · verde · caducada · `go test ./adapters/secondary/persistence -run '^TestHCD069_AjustesRetiradosSiguenCargando$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.314s
- 2026-11-27 06:33 -05:00 · tarea 10 · rojo observado · caducada · `bash -c '! grep -rIlE "mem adr-sync|--adr-sync|--code-impact-annotation|[|] .(adr_sync_enabled|code_impact_annotation_disabled). [|]" README.md docs/ --exclude="Guia-*" --exclude-dir=audits'` (salida 1)
      docs/architecture.md
      docs/CONFIGURATION.md
      docs/MANUAL.md
- 2026-11-27 06:33 -05:00 · tarea 10 · verde · caducada · `bash -c '! grep -rIlE "mem adr-sync|--adr-sync|--code-impact-annotation|[|] .(adr_sync_enabled|code_impact_annotation_disabled). [|]" README.md docs/ --exclude="Guia-*" --exclude-dir=audits'` (salida 0)
- 2026-11-27 06:34 -05:00 · tarea 11 · falló · caducada · `bash -c 'go vet ./... && go test ./... -count=1'` (salida 1)
        ○ .claude/settings.json: no encontrado
        ○ .claude/settings.json: sin permisos de gomemory
        ○ opencode.json: sin permisos de gomemory
        ○ envoltorios nativos: no encontrados
      Resumen:
        ✓ Integración de gomemory en /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCmdUninstallAcceptsYesFlagInAnyPosition3386317886/003
        ✓ /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCmdUninstallAcceptsYesFlagInAnyPosition3386317886/003/mem (copia v2.60.1)
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ Memoria 1 eliminada
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ 1 memoria(s) eliminada(s) de el proyecto "001-85d8fd0bca12ed2f".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ 1 memoria(s) eliminada(s) por garbage collection de el proyecto "001-85d8fd0bca12ed2f".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      FAIL
      FAIL	mem/tests/contract	81.626s
      ok  	mem/tests/integration	40.182s
      ?   	mem/version	[no test files]
      FAIL
- 2026-11-27 06:37 -05:00 · tarea 11 · verde · caducada · `bash -c 'go vet ./... && go test ./... -count=1'` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	7.645s
      ok  	mem/adapters/secondary/hostbind	0.249s
      ok  	mem/adapters/secondary/i18n	0.113s
      ok  	mem/adapters/secondary/launcher	0.528s
      ok  	mem/adapters/secondary/persistence	28.267s
      ok  	mem/adapters/secondary/release	0.350s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.204s
      ok  	mem/adapters/secondary/speckit	0.116s
      ok  	mem/adapters/secondary/tokens	0.108s
      ok  	mem/adapters/secondary/usage	0.104s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	27.602s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.283s
      ok  	mem/domain/i18n	0.094s
      ok  	mem/infrastructure	0.979s
      ok  	mem/tests/contract	83.851s
      ok  	mem/tests/integration	42.627s
      ?   	mem/version	[no test files]
- 2026-11-27 06:39 -05:00 · tarea 1 · verde · caducada · `bash -c '! go doc ./application/usecases ImportADRs >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:39 -05:00 · tarea 2 · verde · caducada · `bash -c '! go doc ./adapters/primary/cli CmdADRSync >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:39 -05:00 · tarea 3 · verde · caducada · `bash -c '! go doc ./adapters/secondary/persistence SetADRSync >/dev/null 2>&1 && ! go doc ./adapters/secondary/persistence SetAdrSyncEnabled >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:39 -05:00 · tarea 4 · verde · caducada · `bash -c '! go doc ./application/ports ADRSyncProvider >/dev/null 2>&1 && ! go doc ./application/ports ADRSyncRepository >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:39 -05:00 · tarea 5 · verde · caducada · `bash -c '! go doc ./domain ParseADRDocument >/dev/null 2>&1 && ! go doc ./domain ADRSyncRecord >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:39 -05:00 · tarea 6 · verde · caducada · `bash -c '! go doc ./application/ports CodeGraphProvider.ImpactFor >/dev/null 2>&1 && go vet ./...'` (salida 0)
- 2026-11-27 06:39 -05:00 · tarea 7 · verde · caducada · `bash -c '! go doc ./adapters/secondary/persistence SetCodeImpactProvider >/dev/null 2>&1 && ! go doc ./domain CodeImpactAnnotation >/dev/null 2>&1 && go test ./domain -run "^TestStripImpactAnnotation" -count=1'` (salida 0)
      ok  	mem/domain	0.127s
- 2026-11-27 06:39 -05:00 · tarea 8 · verde · caducada · `bash -c '! go doc ./domain CodeHotspot >/dev/null 2>&1 && ! go doc ./domain CodeCluster >/dev/null 2>&1 && go test ./application/usecases -run "^TestBuild_ExternalGraphSection$" -count=1'` (salida 0)
      ok  	mem/application/usecases	0.209s
- 2026-11-27 06:39 -05:00 · tarea 9 · verde · caducada · `go test ./adapters/secondary/persistence -run '^TestHCD069_AjustesRetiradosSiguenCargando$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.168s
- 2026-11-27 06:39 -05:00 · tarea 10 · verde · caducada · `bash -c '! grep -rIlE "mem adr-sync|--adr-sync|--code-impact-annotation|[|] .(adr_sync_enabled|code_impact_annotation_disabled). [|]" README.md docs/ --exclude="Guia-*" --exclude-dir=audits'` (salida 0)
- 2026-11-27 06:40 -05:00 · tarea 1 · verde · vigente · `bash -c '! go doc ./application/usecases ImportADRs >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:40 -05:00 · tarea 2 · verde · vigente · `bash -c '! go doc ./adapters/primary/cli CmdADRSync >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:40 -05:00 · tarea 3 · verde · vigente · `bash -c '! go doc ./adapters/secondary/persistence SetADRSync >/dev/null 2>&1 && ! go doc ./adapters/secondary/persistence SetAdrSyncEnabled >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:40 -05:00 · tarea 4 · verde · vigente · `bash -c '! go doc ./application/ports ADRSyncProvider >/dev/null 2>&1 && ! go doc ./application/ports ADRSyncRepository >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:40 -05:00 · tarea 5 · verde · vigente · `bash -c '! go doc ./domain ParseADRDocument >/dev/null 2>&1 && ! go doc ./domain ADRSyncRecord >/dev/null 2>&1 && go build ./...'` (salida 0)
- 2026-11-27 06:40 -05:00 · tarea 6 · verde · vigente · `bash -c '! go doc ./application/ports CodeGraphProvider.ImpactFor >/dev/null 2>&1 && go vet ./...'` (salida 0)
- 2026-11-27 06:40 -05:00 · tarea 7 · verde · vigente · `bash -c '! go doc ./adapters/secondary/persistence SetCodeImpactProvider >/dev/null 2>&1 && ! go doc ./domain CodeImpactAnnotation >/dev/null 2>&1 && go test ./domain -run "^TestStripImpactAnnotation" -count=1'` (salida 0)
      ok  	mem/domain	0.318s
- 2026-11-27 06:40 -05:00 · tarea 8 · verde · vigente · `bash -c '! go doc ./domain CodeHotspot >/dev/null 2>&1 && ! go doc ./domain CodeCluster >/dev/null 2>&1 && go test ./application/usecases -run "^TestBuild_ExternalGraphSection$" -count=1'` (salida 0)
      ok  	mem/application/usecases	0.231s
- 2026-11-27 06:40 -05:00 · tarea 9 · verde · vigente · `go test ./adapters/secondary/persistence -run '^TestHCD069_AjustesRetiradosSiguenCargando$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.190s
- 2026-11-27 06:40 -05:00 · tarea 10 · verde · vigente · `bash -c '! grep -rIlE "mem adr-sync|--adr-sync|--code-impact-annotation|[|] .(adr_sync_enabled|code_impact_annotation_disabled). [|]" README.md docs/ --exclude="Guia-*" --exclude-dir=audits'` (salida 0)
- 2026-11-27 06:40 -05:00 · tarea 11 · verde · vigente · `bash -c 'go vet ./... && go test ./... -count=1'` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	5.482s
      ok  	mem/adapters/secondary/hostbind	0.264s
      ok  	mem/adapters/secondary/i18n	0.127s
      ok  	mem/adapters/secondary/launcher	0.553s
      ok  	mem/adapters/secondary/persistence	27.367s
      ok  	mem/adapters/secondary/release	0.363s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.172s
      ok  	mem/adapters/secondary/speckit	0.133s
      ok  	mem/adapters/secondary/tokens	0.120s
      ok  	mem/adapters/secondary/usage	0.112s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	25.604s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.217s
      ok  	mem/domain/i18n	0.118s
      ok  	mem/infrastructure	0.797s
      ok  	mem/tests/contract	85.176s
      ok  	mem/tests/integration	39.902s
      ?   	mem/version	[no test files]

## Siguiente
Cerrada con aprobación sobre la revisión 750956499a8f.
