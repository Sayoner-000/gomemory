# Arquitectura de gomemory

La descripción de capas, adaptadores, flujos de instalación y contratos de agentes está en [architecture.md](architecture.md). Se conserva esa ruta porque ya la enlazan otras guías del repositorio.

La [guía técnica](Guia-Tecnica_GOMEMORY-v2.40.html) resume la arquitectura completa en una sola página (HTML imprimible a PDF), y la [guía SDD](Guia-SDD_GOMEMORY-v2.40.html) explica cómo usar gomemory en un harness con Spec Kit. Las ediciones anteriores se conservan en [editions/](editions/).


## Expand (040)

`hook expand` adapta el payload del harness a `ExpandSignals`, consulta una vez el router puro `domain.RouteSupertools` y ejecuta sus acciones mediante `usecases.ExpandHook`. Las reglas se evalúan primero; después se agrega contexto y se ejecutan las observaciones silenciosas. El adaptador aplica los interruptores, deduplica sugerencias por episodio y renderiza el dialecto del harness sin aprobar herramientas.

Los puertos `RuleSourcePort` y `AgentConfigReaderPort` aíslan la lectura de reglas y configuración. `ObservationQueueRepository` y `ShieldStateRepository` conservan el estado de archivos; `InstinctRepository` y `InstinctGlobalRepository` aíslan las estadísticas y la promoción entre proyectos. El composition root construye todos los adaptadores.

La ruta ligera del hook no abre SQLite para permitir herramientas ni observar comandos. Solo abre el contenedor completo al registrar una activación o plegar observaciones. Las decisiones tienen presupuesto de 150 ms y fallo abierto; el registro se realiza después de emitir la decisión, con su propio límite. La inyección de instincts ocurre únicamente en `Builder`, con umbral 0,7 e interruptores aplicados.

Consulta [expand.md](expand.md) para comandos, formatos, permisos, límites y excepciones de cada harness.
