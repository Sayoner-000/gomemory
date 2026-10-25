# Investigación: preparación Socrática → Spec Kit

**Fecha**: 2026-10-24. **Alcance**: diseño de la feature 042; sin implementación.

## Evidencia y límites de recuperación

- Se consultaron memoria, grafo propio y `codebase-memory-mcp`, con contraste de fuente actual mediante `get_code_snippet` y CodeGraph. Proyecto: `Users-josegomezj-home-rcw-gomemory`; generación observada: `2026-10-24T13:30:05Z`, estado `ready`, sin vigilancia automática activa.
- Las rutas de código citadas se comprobaron con `check_index_coverage`: sin problemas registrados y con metadatos coincidentes. Es una señal de mejor esfuerzo, no una prueba de cobertura exhaustiva. Las skills y configuraciones se leyeron directamente donde correspondía.
- El grafo externo representa varios métodos homónimos como `application.usecases.Apply`; su traza mezcla receptores. Se completó su paginación, pero las conclusiones sobre Socrática se apoyan en la fuente del archivo y en `get_symbol/list_dependencies("SocraticaFlow.Apply")`, que identifican la llamada al reductor y a `SocraticaNext`.
- La investigación de persistencia y distribución se delegó después de las rutas `DELEGATE` de Octopus. La transferencia acotó archivos, preguntas y salida; no se delegó la implementación.
- `setup-plan.sh --json` resolvió la feature correcta y devolvió `BRANCH` vacío. `git branch --show-current` confirmó `main`; el directorio se obtiene de `.specify/feature.json`, independientemente de la rama. No se modifica el script en esta feature.

## R1. Adoptar la base y extender el recorrido

**Decisión — Extend**: ampliar `domain.Reduce`, `RefinementFlow`, `SocraticaFlow` y la herramienta única `socratica_flow`. Añadir `ModeSpecify`, un agregado de preparación y operaciones específicas, sin un motor paralelo ni dependencias nuevas.

**Fundamento**: `domain/socratica.go:119-342` contiene el reductor puro; `application/usecases/socratica_flow.go:167-335` coordina transacciones, generación local y estado; `SocraticaNext`, en ese archivo:556-588, ya comunica el siguiente paso. `Start`:171 solo admite `refine/scrutiny`. `approve` en el reductor exige hoy únicamente una propuesta textual no vacía. La ampliación debe comprobar estructura y versión antes de autorizar.

**Alternativas**: Adopt sin cambios no comprueba historias ni bloqueos. Build con otra herramienta o máquina de estados duplicaría la autorización, privacidad y recuperación existentes. Se mantienen las fases generales; la etapa de preparación vive dentro del nuevo agregado.

## R2. Preparación por secciones y comprobaciones puras

**Decisión**: usar secciones `intent`, `scope` y `stories`; una operación `prepare` reemplaza una sección completa o comprueba una etapa. Exigir la revisión esperada para evitar sobrescrituras obsoletas. Gomemory calcula comprobaciones y texto final; el agente no envía un indicador de validez ni una propuesta libre como sustituto.

**Fundamento**: el reductor ya recibe reloj e identificadores desde fuera. Los nuevos validadores deben permanecer sin I/O. Se comprueban campos, declaraciones explícitas de listas vacías, identificadores, referencias y pendientes abiertos bloqueantes. Las condiciones textuales deben expresar un resultado observable; la corrección de negocio la confirma la persona. No se promete detectar automáticamente toda ambigüedad del lenguaje natural.

**Invalidación**: cada cambio material incrementa la revisión del contenido e invalida la sección modificada y las etapas posteriores, junto con revisión y autorización no consumidas. Cada comprobación conserva la huella de su proyección de entrada; una comprobación con huella obsoleta no habilita el avance. Un reemplazo idéntico es un no-op.

**Preguntas**: conservar una lista acumulativa de hasta tres preguntas por flujo, nunca reiniciarla al editar o retomar. En `specify`, terminar las respuestas vuelve a la preparación, sin disparar la reescritura libre ni un generador externo. Se permiten cero preguntas; después del límite solo edición y aportes voluntarios.

**Alternativas**: validar exclusivamente al aprobar oculta los faltantes durante el recorrido; un JSON libre con `ready=true` delegaría la compuerta al mismo agente que redacta.

## R3. Identidad entre revisión y entrega

**Decisión**: aplicar protección recursiva a los textos antes de comprobar y renderizar. Un renderizador puro produce el texto revisado, con orden de secciones fijo e identificadores de historias y criterios. Guardar su revisión y una huella SHA-256 de sus bytes exactos. Aprobar y confirmar vuelven a comprobar que el texto corresponde a la preparación vigente.

**Fundamento**: `SocraticaPromptHash`, `domain/socratica.go:343-346`, recorta espacios; sirve al reconocimiento actual de procedencia, pero no prueba identidad carácter a carácter. La huella exacta de revisión será distinta de esa huella normalizada. `ProtectText` aplicado únicamente al persistir podría cambiar lo que la persona ya revisó; por eso la protección precede a la revisión.

**Compatibilidad**: rechazar para `specify` las rutas de propuesta libre `rewrite`, `edit` textual y `send_original`; las ediciones se hacen con `prepare`. La ruta genérica de ejecución local tampoco puede eludir la preparación. `refine/scrutiny` conservan sus operaciones actuales.

**Alternativas**: reescribir después de aprobar rompe la identidad; comparar solo textos normalizados acepta diferencias no revisadas.

## R4. Confirmación idempotente y concesión de ejecución

**Decisión**: conservar `approve → ack`, añadiendo a la respuesta de `specify` un `execution_granted` que solo es verdadero en la primera transición confirmada. Extender el resultado interno del repositorio para distinguir ese caso, sin cambiar las salidas previas de los otros modos.

**Evidencia**: `adapters/secondary/persistence/socratica.go:310-337` devuelve `acked=true, already_dispatched=false` tanto al confirmar por primera vez como al repetir el mismo nonce. El `UPDATE` de la primera transición exige `phase='approved' AND dispatch_ack=0`; `RowsAffected==1` distingue la primera concesión. La concesión no se devuelve hasta confirmar el UnitOfWork. El reductor y el adaptador también deben transportarla sin almacenarla como un permiso recuperable.

**Pérdida de respuesta**: repetir el nonce recupera el acuse, pero no una nueva concesión. Si se perdió la primera respuesta, no se ejecuta desde el replay: se muestra autorización consumida sin resultado confirmado y se exige reconciliación explícita. No se promete ejecución exactamente una vez ante una caída del receptor.

**Alternativas descartadas**: `acked && !already_dispatched` no distingue el replay. `ConsumeDispatch` no debe reutilizarse: `SocraticaFlow.ClassifyPrompt`:526-532 lo usa como marca de procedencia para el hook. Se mantiene ese cometido intacto.

## R5. Persistencia aditiva y revisión conservada

**Decisión**: añadir `preparation_json` nullable a `socratica_flows` y `socratica_history`, y `payload_policy` a flujos, con valor predeterminado `persistent`. Ampliar las estructuras existentes, el mapeo explícito de columnas y el historial; los registros antiguos conservan preparación nula.

**Fundamento**: `socratica.go:176-244` mapea los flujos; `SaveTransition`:269 controla fase, revisión y fecha; las operaciones de historial y borrado ya delimitan el ciclo de vida. El catálogo observado en `adapters/secondary/persistence/db.go:193-241` tiene versión 2. La implementación elevará a 3 si esa sigue siendo la versión anterior al implementar, actualizará el golden y añadirá las columnas mediante el mecanismo aditivo existente, dentro de su transacción.

**Privacidad retroactiva**: ampliar inventarios en `socratica_sanitize.go` y `domain/privacy_maintenance.go`, así como `RedactFlow`. Si la depuración cambia una preparación no enviada, invalida comprobaciones, revisión y autorización anterior; si ya fue enviada, conserva solo la evidencia de consumo que corresponda, sin habilitar reenvío. `approve` y `ack` comprueban las huellas vigentes incluso si otra operación modificó el almacenamiento.

**Alternativas**: tablas separadas por historia/criterio añaden relaciones de borrado innecesarias para un agregado que se actualiza como unidad. Cambiar solo `CREATE TABLE IF NOT EXISTS` no migra bases existentes. Los campos nuevos no se guardan como memorias generales.

## R6. Modo privado y pendiente único entre procesos

**Decisión**: para `specify` privado, mantener textos, preparación y revisión en un depósito efímero por instancia MCP. Conservar en la fila SQL solo la reserva y metadatos no textuales: identidad, proyecto, cliente, modo, política, fases, revisiones, fechas y marcas de entrega. La política se fija al crear el flujo y no cambia al alternar ajustes posteriormente.

**Fundamento**: actualmente `Start` persiste el flujo incluso en privado; `Private` suprime historial y redacta al confirmar (`socratica_flow.go:189,276,290-300`). El nuevo recorrido requiere que su preparación y revisión sean efímeras. Una reserva SQL sin texto conserva el índice único de pendiente por proyecto/agente y la evidencia de autorización consumida entre procesos, sin persistir el contenido privado.

**Diseño**: un adaptador coordinado implementa los ports existentes y separa payload de metadatos. Prepara los cambios del depósito bajo mutex, confirma la transacción de metadatos y después publica el payload antes de responder. Un rollback no publica cambios. Una caída puede perder el payload, lo que es aceptable para un flujo privado y debe declararse. No se guardan payloads en archivos temporales, locks ni logs.

**Sin payload**: devolver `private_state_unavailable`; no reconstruirlo desde parámetros recibidos, ni admitir aprobación, confirmación o reenvío. Se puede consultar o cancelar la reserva y empezar otro flujo mediante una decisión explícita. No se recupera un original privado perdido. No se escribe historial privado.

**Alternativas**: un depósito solo en memoria sin reserva duradera permitiría dos pendientes simultáneos en procesos distintos; guardar el agregado privado en SQL conservaría contenido que se decidió mantener efímero. No se altera el comportamiento privado histórico de `refine/scrutiny`.

## R7. Traspaso desde el agente y resultado observable

**Decisión**: la skill comprueba que puede cargar e invocar `speckit-specify` en el proyecto antes de `ack`; un binario `specify` disponible por sí solo no certifica esa capacidad. Si falta, conserva la preparación y la aprobación vigente, sin confirmar. Tras la primera concesión entrega exactamente `approved_prompt` como argumento de la skill, sin lanzar un shell construido a partir del texto ni ejecutar la necesidad descrita.

**Resultado**: añadir `action=report` al mismo contrato para registrar un resultado comunicado por el agente, con `dispatch_id`, estado y rutas relativas de los artefactos. No concede permisos ni genera archivos. Las rutas son información comunicada por el agente, no evidencia de inspección semántica por el backend. La aceptación real debe abrir los artefactos y verificar alcance y trazabilidad; un estado `dispatched` no equivale a generación completada.

**Alternativas**: introducir un ejecutor externo o instalar Spec Kit automáticamente amplía el alcance. Consumir autorización antes de comprobar disponibilidad desperdicia una aprobación válida. Inferir éxito del acuse confunde autorización con resultado.

## R8. Distribución, terminal y evidencia por agente

**Decisión**: ampliar `infrastructure/templates/socratica/SKILL.md` y sincronizar `skills/socratica/SKILL.md`; añadir un wrapper fino `commands/socratica-hu.md` y destinos Claude/OpenCode en `socraticaWrapperTargets`. Codex usa su invocación de skill y lenguaje natural.

**Evidencia**: `adapters/primary/setup/agent_skills_setup.go:89` instala bajo raíces existentes y compara contenido; `:264` enumera wrappers. `adapters/primary/cli/cmd_mcp_setup.go:270-272` pasa las plantillas. `global_artifacts.go:18` deriva el inventario de comandos de esos destinos. La paridad canónica/publicada existe hoy, pero la prueba genérica de paridad todavía no incluye Socrática.

**Terminal/TUI**: `cmd_socratica.go:908` retoma pendientes mediante `drive`, y `tui/socratica.go:481` ofrece acciones por fase sin discriminar modo. Añadir reconocimiento de `specify` antes de esas rutas: mostrar preparación y una instrucción para retomar desde el agente; bloquear generación, edición textual y ejecución locales genéricas del nuevo modo.

**Verificación**: reutilizar pruebas de instalación, skills, MCP, CLI/TUI, dominio y repositorio. `TestRealSocraticaHarness`, `adapters/secondary/generator/harness_test.go:244`, solo verifica preguntas y reescritura de `refine` con binarios reales opt-in: no prueba descubrimiento del alias, preflight ni generación con Spec Kit. Se requieren evidencias separadas de recorrido completo en Claude Code, Codex y OpenCode; no se certifican capacidades a partir de la presencia del binario.

## R9. Constitución y dependencias

**Decisión — Adopt**: mantener Go 1.27.0, toolchain 1.27.3 y las dependencias fijadas en `go.mod`, SQLite local, MCP y TUI existentes. Las nuevas reglas de dominio usan biblioteca estándar. No se añade ni actualiza una dependencia, servidor MCP o proveedor de modelos.

**Fuentes constitucionales**: `mem constitution` entrega la base 2.0.0; `.specify/memory/constitution.md` contiene la versión de proyecto 2.1.0 y la excepción §25.1 aprobada el 2026-10-23, revisable el 2027-01-23, con ADR 0001. Se aplican ambas: base y enmienda local. No se sincroniza la base sobre el archivo local porque borraría esa extensión.

**Búsqueda previa**: se revisaron las decisiones existentes, símbolos, skills instaladas `socratica/speckit-specify` y sus contratos locales antes de decidir. La integración reutiliza esas capacidades; no hay una dependencia externa candidata que requiera selección o documentación externa nueva.

**Cierre de investigación**: decisiones de diseño resueltas; sin aclaraciones pendientes. La evidencia de ejecución del nuevo recorrido y las comprobaciones de migración son trabajo de implementación y aceptación, no resultados ya obtenidos por esta planificación.
