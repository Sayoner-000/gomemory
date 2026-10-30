# Arquitectura de gomemory

La descripción de capas, adaptadores, flujos de instalación y contratos de agentes está en [architecture.md](architecture.md). Se conserva esa ruta porque ya la enlazan otras guías del repositorio.

La [guía técnica](Guia-Tecnica_GOMEMORY-v2.42.html) resume la arquitectura completa en una sola página (HTML imprimible a PDF), y la [guía SDD_HCD](Guia-SDD_HCD_GOMEMORY-v2.42.html) explica cómo usar gomemory en un harness con Spec Kit y con HCD, y cómo conviven los dos. El repositorio conserva solo las guías vigentes.


## Expand (040)

`hook expand` adapta el payload del harness a `ExpandSignals`, consulta una vez el router puro `domain.RouteSupertools` y ejecuta sus acciones mediante `usecases.ExpandHook`. Las reglas se evalúan primero; después se agrega contexto y se ejecutan las observaciones silenciosas. El adaptador aplica los interruptores, deduplica sugerencias por episodio y renderiza el dialecto del harness sin aprobar herramientas.

Los puertos `RuleSourcePort` y `AgentConfigReaderPort` aíslan la lectura de reglas y configuración. `ObservationQueueRepository` y `ShieldStateRepository` conservan el estado de archivos; `InstinctRepository` y `InstinctGlobalRepository` aíslan las estadísticas y la promoción entre proyectos. El composition root construye todos los adaptadores.

La ruta ligera del hook no abre SQLite para permitir herramientas ni observar comandos. Solo abre el contenedor completo al registrar una activación o plegar observaciones. Las decisiones tienen presupuesto de 150 ms y fallo abierto; el registro se realiza después de emitir la decisión, con su propio límite. La inyección de instincts ocurre únicamente en `Builder`, con umbral 0,7 e interruptores aplicados.

Consulta [expand.md](expand.md) para comandos, formatos, permisos, límites y excepciones de cada harness.

## HCD (045)

HCD conduce una unidad de trabajo por su ceremonia: `define → clarify → approve → slice → tasks → implement → verify → close` en el carril completo, o la Vía rápida (`define → tasks → implement → verify → close`, sin `approve` ni `slice`). La persona la recorre con las skills `/hcd` o `$hcd` y `hcd-*`; cada skill llama al motor `mem hcd`, que es el único que registra aclaraciones, aprobaciones, evidencia y cierres.

- **Dominio** (`domain/hcd.go`, `hcd_doc.go`, `hcd_stage.go`): `ParseUnitDoc` lee el documento de la unidad; `domain.HCDState` deriva el paso, la siguiente skill, el gate de evidencia, el gate TDD (`RedMissing`) y `Analyze`. El paso nunca se guarda.
- **Revisiones**: la de la definición (todas sus secciones obligatorias: Intención, Reproducción, Alcance, «¿Qué pasa si no lo hacemos?», «Alternativa más barata», Historias y Aclaraciones) es la que aprueba `approve`; la de trabajo (la definición más Slices y Checklist, sin casillas) es la que usan la evidencia y el cierre.
- **Caso de uso** (`application/usecases/hcd.go`): `Start`, `Show`, `Clarify`, `Approve`, `Verify`, `Analyze`, `Close`, `Cancel`, `Promote` y `ActiveHCDContext`. Las decisiones se reflejan como memorias `decision`.
- **Puertos** (`application/ports/hcd.go`): `HCDStore` (unidades, evidencia y aprobaciones), `HCDWorkspace` (documento, huella y ejecución) y `HCDContextRepository` (sección «HCD activo» del contexto de arranque).
- **Adaptadores**: `persistence.HCDStore` sobre las tablas `hcd_units`, `hcd_evidence` y `hcd_approvals` (ADR 0003); `hcdfiles.Workspace`, que solo escribe bajo `hcd/` sin seguir symlinks, calcula la huella (`HEAD` más los archivos cambiados y sin rastrear, excluyendo `hcd/`) y ejecuta las comprobaciones con tiempo límite; `cli/hcd`, el motor con salida `--json`; y el formulario «Especificación» de la TUI.
- **Normas transversales**: el caso de uso resuelve la Constitución y el Playbook vigentes (`ConfigureNorms`, sobre `ResolvePinnedDoc`). Cada aprobación guarda sus huellas; si cambian tras aprobar la definición, `HCDState.NormsChanged` hace que `analyze` lo reporte y que el cierre exija reaprobar.
- **Hook**: en Claude Code, `mem hook expand pre-tool` avisa una vez por sesión, sin bloquear, al editar código sin unidad activa. Solo consulta la tabla de unidades, para caber en el plazo del hook.

La única fuente de contenido es el documento `hcd/HCD-NNN-<slug>.md`, que viaja en el PR. SQLite guarda solo el control: qué unidad está activa, qué se observó y qué aprobó la persona.
