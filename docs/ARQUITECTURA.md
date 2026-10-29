# Arquitectura de gomemory

La descripción de capas, adaptadores, flujos de instalación y contratos de agentes está en [architecture.md](architecture.md). Se conserva esa ruta porque ya la enlazan otras guías del repositorio.

La [guía técnica](Guia-Tecnica_GOMEMORY-v2.40.html) resume la arquitectura completa en una sola página (HTML imprimible a PDF), y la [guía SDD_HCD](Guia-SDD_HCD_GOMEMORY-v2.40.html) explica cómo usar gomemory en un harness con Spec Kit y con HCD, y cómo conviven los dos. Las ediciones anteriores se conservan en [editions/](editions/).


## Expand (040)

`hook expand` adapta el payload del harness a `ExpandSignals`, consulta una vez el router puro `domain.RouteSupertools` y ejecuta sus acciones mediante `usecases.ExpandHook`. Las reglas se evalúan primero; después se agrega contexto y se ejecutan las observaciones silenciosas. El adaptador aplica los interruptores, deduplica sugerencias por episodio y renderiza el dialecto del harness sin aprobar herramientas.

Los puertos `RuleSourcePort` y `AgentConfigReaderPort` aíslan la lectura de reglas y configuración. `ObservationQueueRepository` y `ShieldStateRepository` conservan el estado de archivos; `InstinctRepository` y `InstinctGlobalRepository` aíslan las estadísticas y la promoción entre proyectos. El composition root construye todos los adaptadores.

La ruta ligera del hook no abre SQLite para permitir herramientas ni observar comandos. Solo abre el contenedor completo al registrar una activación o plegar observaciones. Las decisiones tienen presupuesto de 150 ms y fallo abierto; el registro se realiza después de emitir la decisión, con su propio límite. La inyección de instincts ocurre únicamente en `Builder`, con umbral 0,7 e interruptores aplicados.

Consulta [expand.md](expand.md) para comandos, formatos, permisos, límites y excepciones de cada harness.

## HCD (045)

HCD conduce una unidad de trabajo mediante las skills `/hcd` o `$hcd` y `hcd-*`. Cada skill llama al motor interno `mem hcd`; el motor aplica las precondiciones, revisiones e idempotencia antes de cambiar el estado. El dominio (`domain/hcd*.go`) define estados, atomicidad y vigencia de evidencia. Los casos de uso (`application/usecases/hcd_*.go`) coordinan el flujo. Los adaptadores CLI, SQLite y `hcdfiles` exponen comandos, persistencia y artefactos; `infrastructure/container.go` los conecta.

El contenido redactado tiene su fuente de verdad en `hcd/features/{NNN}-{slug}/definition.md` y `delivery.md`. Las tablas SQLite `hcd_*` guardan el estado de control, las revisiones, operaciones, decisiones, ejecuciones y evidencia. `verification.md` es un informe generado a partir de esas fuentes y se regenera al verificar; no es una entrada editable del protocolo. Las decisiones aceptadas se reflejan además en la memoria persistente después de confirmar la transacción de control.

Los puertos mantienen separados esos recursos: `HCDRepository` lee y escribe el estado de control; `HCDUnitOfWork` agrupa sus mutaciones en una transacción; `HCDArtifactRepository` lee, escribe y calcula las huellas de los archivos; `HCDContextRepository` obtiene la unidad activa para el contexto de arranque. `HCDContextDetailRepository` proporciona las aclaraciones, decisiones y desvíos que acompañan esa unidad.

`hcd_focus` señala una sola unidad activa por proyecto. El hook `turn-end` registra un heartbeat con sesión, cliente y archivos observados, incluso si el turno no produjo archivos. Los desvíos se calculan con ese registro y con las ejecuciones. El constructor del contexto agrega la sección «HCD activo» cuando hay una unidad enfocada; el delta de sesión la deduplica. El heartbeat usa un tiempo límite configurable y, si falla, emite un aviso sin interrumpir el turno.
