# Feature Specification: Mantenimiento de Memoria (Purga, Compactación y Garbage Collector)

**Feature Branch**: `003-memory-maintenance`

**Created**: 2026-06-22

**Status**: Draft

**Input**: User description: "agregemos un feature en TUI o en este proyecto para uninstall o purgar la memoria en caso que crezca muchisimos y como compactar y hacer el garbage collector"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Purgar memoria cuando crece demasiado (Priority: P1)

Como usuario de gomemory, cuando el almacén de memorias crece demasiado (muchos registros o un archivo `.memory/mem.db` muy pesado), quiero poder borrar memorias de forma deliberada —todas, o filtradas por proyecto/tipo/antigüedad— para recuperar control sobre el tamaño del almacén, sin tener que editar la base de datos a mano.

**Why this priority**: Es el dolor que origina el feature: sin una vía segura de purga, la única opción hoy es borrar manualmente `.memory/mem.db`, lo cual es arriesgado (pérdida total) y no está soportado por ningún comando.

**Independent Test**: Se puede probar de forma aislada guardando varias memorias de prueba, ejecutando el comando/acción de purga con un filtro (p. ej. por proyecto), confirmando la operación, y verificando que solo las memorias dentro del alcance fueron eliminadas y las demás permanecen intactas.

**Acceptance Scenarios**:

1. **Given** un proyecto con memorias guardadas, **When** el usuario ejecuta la purga sobre ese proyecto y confirma la operación, **Then** todas las memorias de ese proyecto se eliminan y las de otros proyectos permanecen sin cambios.
2. **Given** una purga en curso, **When** el usuario no confirma explícitamente (cancela o no responde "sí"), **Then** no se elimina ninguna memoria.
3. **Given** memorias con relaciones (`memory_relations`) entre sí, **When** una de las memorias relacionadas se purga, **Then** las relaciones que la referencian también se eliminan (sin registros huérfanos).
4. **Given** un proyecto sin memorias, **When** el usuario ejecuta la purga sobre ese proyecto, **Then** el sistema informa que no había nada que borrar, sin error.

---

### User Story 2 — Compactar el almacenamiento para recuperar espacio en disco (Priority: P2)

Como usuario de gomemory, después de borrar memorias (por purga o por garbage collection) quiero poder compactar la base de datos para que el espacio liberado se refleje realmente en el tamaño del archivo en disco, en vez de quedar un archivo inflado con espacio muerto.

**Why this priority**: Sin compactación, purgar o hacer GC reduce el conteo de registros pero no el tamaño real en disco, dejando al usuario con la falsa sensación de que "ya liberó espacio".

**Independent Test**: Se puede probar de forma aislada guardando memorias, borrando una parte significativa, ejecutando "compactar", y comparando el tamaño del archivo `.memory/mem.db` antes y después.

**Acceptance Scenarios**:

1. **Given** una base de datos con espacio liberado por borrados previos, **When** el usuario ejecuta la compactación, **Then** el tamaño en disco del archivo de memoria se reduce y el sistema reporta el tamaño antes y después.
2. **Given** una base de datos ya compacta (sin espacio muerto que reclamar), **When** el usuario ejecuta la compactación, **Then** la operación se completa sin error y reporta que no había espacio significativo que liberar.
3. **Given** una compactación en curso, **When** se completa, **Then** ninguna memoria ni relación sobreviviente se pierde ni se modifica (la compactación nunca borra datos).

---

### User Story 3 — Garbage collection de memorias antiguas (Priority: P3)

Como usuario de gomemory, quiero que existan memorias "viejas" o de bajo valor (según antigüedad u otro criterio) se puedan limpiar mediante un mecanismo de garbage collection, para que el almacén se mantenga saludable con el tiempo sin depender de que yo recuerde purgar manualmente.

**Why this priority**: Es la capa de mantenimiento continuo; sin ella, el usuario siempre depende de acordarse de purgar/compactar manualmente cuando ya el problema es grande.

**Independent Test**: Se puede probar de forma aislada guardando memorias con distintas fechas de creación, ejecutando el garbage collector con un umbral de antigüedad determinado, y verificando que solo las memorias más viejas que el umbral fueron eliminadas.

**Acceptance Scenarios**:

1. **Given** memorias más antiguas que el umbral de retención configurado, **When** se ejecuta el garbage collector, **Then** esas memorias (y sus relaciones) se eliminan, mientras que las memorias dentro del umbral permanecen.
2. **Given** un almacén donde ninguna memoria supera el umbral de retención, **When** se ejecuta el garbage collector, **Then** no se elimina nada y el sistema lo reporta explícitamente.
3. **Given** que el garbage collector eliminó memorias, **When** termina la ejecución, **Then** el usuario recibe un resumen de cuántas memorias se eliminaron y de qué proyecto(s).

---

### User Story 4 — Desinstalación completa de la herramienta (Priority: P4)

Como usuario de gomemory, quiero poder desinstalar la herramienta de un proyecto —removiendo el binario `mem`, los hooks, las entradas en CLAUDE.md/AGENTS.md y el registro MCP, además de los datos— como reverso exacto de `mem install`, para casos donde quiero quitar gomemory del proyecto por completo y no solo vaciar su memoria.

**Why this priority**: Es la acción más drástica y menos frecuente: a diferencia de la purga (que se usa para controlar el crecimiento sin dejar de usar la herramienta), la desinstalación completa es para cuando ya no se quiere usar gomemory en ese proyecto. Por eso va después de purga/compactación/GC, que son el mantenimiento recurrente.

**Independent Test**: Se puede probar de forma aislada instalando gomemory en un directorio de prueba (`mem install`), guardando memorias, ejecutando la desinstalación completa, y verificando que el binario, los hooks, las entradas en CLAUDE.md/AGENTS.md, el registro MCP y el directorio `.memory/` ya no existen en ese directorio.

**Acceptance Scenarios**:

1. **Given** un proyecto con gomemory instalado (binario, hooks, entradas en CLAUDE.md/AGENTS.md, registro MCP) y memorias guardadas, **When** el usuario ejecuta la desinstalación completa y confirma, **Then** se eliminan el binario `mem`, los hooks, las entradas agregadas en CLAUDE.md/AGENTS.md, el registro MCP y el directorio `.memory/` (datos incluidos).
2. **Given** una desinstalación completa en curso, **When** el usuario no confirma explícitamente, **Then** no se elimina nada (ni binario, ni hooks, ni datos).
3. **Given** un proyecto donde gomemory nunca fue instalado mediante `mem install` (p. ej. solo se usó `.memory/` manualmente), **When** se ejecuta la desinstalación completa, **Then** el sistema elimina lo que encuentra (datos, `.memory/`) e informa qué componentes de la instalación (binario/hooks/MCP) no estaban presentes, sin fallar.

---

### Edge Cases

- ¿Qué pasa si se solicita una purga, compactación o GC mientras la TUI y la CLI tienen el archivo `.memory/mem.db` abierto al mismo tiempo (lock/WAL en uso)?
- ¿Qué pasa si el usuario purga el proyecto sobre el que está parado actualmente en una sesión activa (memorias de la sesión en curso)?
- ¿Qué pasa si se cancela el proceso (Ctrl+C) a mitad de una purga, compactación o desinstalación completa?
- ¿Qué pasa si el garbage collector se ejecuta sobre un proyecto que no existe o ya no tiene memorias?
- ¿Qué pasa si el usuario pide compactar un archivo que no existe todavía (`.memory/` nunca inicializado)?
- ¿Qué pasa si se pide una desinstalación completa pero el binario `mem` que se está ejecutando es el mismo que hay que borrar (auto-eliminación en uso)?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE proveer una forma de eliminar memorias permanentemente (purga), accesible tanto desde la CLI como desde la TUI.
- **FR-002**: El sistema DEBE exigir una confirmación explícita del usuario antes de ejecutar cualquier purga, para evitar pérdida accidental de datos.
- **FR-003**: El sistema DEBE permitir acotar la purga por proyecto, y opcionalmente por tipo de memoria y/o antigüedad, en vez de forzar siempre un "borrar todo". Si el usuario no especifica alcance, la purga DEBE limitarse por defecto al proyecto actual; purgar todos los proyectos a la vez DEBE requerir una bandera o confirmación adicional explícita (p. ej. `--all`).
- **FR-004**: El sistema DEBE eliminar, junto con cada memoria borrada (por purga o por GC), cualquier registro de `memory_relations` que la referencie, para no dejar relaciones huérfanas.
- **FR-005**: El sistema DEBE garantizar que una purga acotada a un proyecto nunca elimine memorias de otro proyecto.
- **FR-006**: El sistema DEBE proveer una operación de "compactar" que recupere el espacio en disco liberado por borrados previos, sin eliminar ninguna memoria existente.
- **FR-007**: El sistema DEBE reportar el tamaño del almacenamiento antes y después de compactar, para que el usuario pueda verificar que la operación tuvo efecto.
- **FR-008**: El sistema DEBE permitir consultar en cualquier momento el tamaño actual del almacenamiento y la cantidad de memorias guardadas, para decidir si conviene purgar, compactar o esperar.
- **FR-009**: El sistema DEBE proveer un mecanismo de garbage collection que elimine memorias más antiguas que un umbral de retención. El GC se dispara únicamente a demanda (comando CLI o acción explícita en la TUI); el sistema NO DEBE ejecutar GC automáticamente en segundo plano (ni al iniciar sesión, ni al guardar, ni por tamaño del almacén).
- **FR-010**: La TUI DEBE exponer el tamaño/cantidad de memorias y dar acceso directo a las acciones de purga, compactación y garbage collection, para que el usuario note el crecimiento sin salir de la herramienta.
- **FR-011**: El sistema DEBE informar un resumen (cuántas memorias se eliminaron, de qué proyecto(s), espacio recuperado) al finalizar cualquier operación de purga o garbage collection.
- **FR-012**: El sistema DEBE ofrecer dos acciones distintas y claramente diferenciadas: (a) "purgar" — vaciar datos de memoria (FR-001 a FR-005), dejando la herramienta instalada, y (b) "desinstalar" — reverso completo de `mem install`, que además remueve el binario `mem`, los hooks registrados, las entradas agregadas en CLAUDE.md/AGENTS.md y el registro MCP, junto con el directorio `.memory/`.
- **FR-013**: La desinstalación completa (FR-012b) DEBE exigir confirmación explícita del usuario antes de ejecutarse, igual que la purga (FR-002), dado que es una operación irreversible y de mayor alcance.
- **FR-014**: La desinstalación completa DEBE poder ejecutarse de forma segura aunque el binario `mem` en uso sea el mismo que debe eliminarse (mismo mecanismo ya usado por `mem install` para auto-reemplazo de binario).

### Key Entities *(include if feature involves data)*

- **Memory**: entidad existente (proyecto, tipo, contenido, fecha de creación, etc.); es el objeto principal sobre el que actúan purga y garbage collection.
- **Memory Relation**: vínculo entre dos memorias; debe limpiarse cuando cualquiera de los dos lados es eliminado.
- **Retention Policy**: criterio (antigüedad, tipo, proyecto) que determina qué elimina el garbage collector.
- **Storage Report**: snapshot de tamaño en disco y cantidad de memorias, usado para decidir y para verificar el efecto de purga/compactación.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un usuario puede purgar las memorias dentro de un alcance elegido en menos de 30 segundos, con una sola confirmación explícita.
- **SC-002**: Tras borrar una porción significativa de memorias y compactar, el tamaño en disco del archivo de memoria se reduce de forma medible (no solo el conteo de registros).
- **SC-003**: Un usuario puede conocer el tamaño/cantidad actual del almacenamiento en menos de 5 segundos sin salir de la CLI ni de la TUI.
- **SC-004**: El garbage collector elimina únicamente memorias más antiguas que el umbral configurado — cero eliminaciones de memorias recientes en pruebas de validación.
- **SC-005**: El 100% de las operaciones de purga y de desinstalación completa exigen y respetan una confirmación explícita del usuario antes de borrar — cero operaciones destructivas silenciosas o accidentales.
- **SC-006**: Tras una desinstalación completa, no quedan rastros de la instalación en el proyecto (binario, hooks, entradas en CLAUDE.md/AGENTS.md, registro MCP, directorio `.memory/`), verificable con una sola inspección del directorio del proyecto.

## Assumptions

- El backend de almacenamiento sigue siendo el SQLite existente (`.memory/mem.db`); las operaciones de mantenimiento actúan sobre ese archivo.
- "Compactar" es una operación no destructiva, distinta de la purga: nunca elimina memorias, solo recupera espacio.
- La confirmación de purga/desinstalación puede satisfacerse con un prompt interactivo (CLI) o una pantalla de confirmación (TUI); puede existir una bandera para uso no interactivo/scripts, pero sigue requiriendo una confirmación explícita en algún punto del flujo (p. ej. al pasar la bandera).
- Las operaciones de mantenimiento (purga, compactación, GC, desinstalación) requieren acceso exclusivo o seguro al archivo de base de datos, reutilizando el mismo mecanismo de conexión (WAL + busy timeout) ya usado por el resto del sistema.
- El garbage collector es exclusivamente a demanda (sin disparo automático); el umbral de retención por defecto es de 90 días, configurable por el usuario al invocar la acción.
- La purga, sin alcance explícito, afecta solo al proyecto actual; afectar todos los proyectos requiere pasar una bandera/acción explícita adicional (p. ej. `--all`).
- "Desinstalar" es una acción separada y más drástica que "purgar": además de los datos, remueve la instalación de la herramienta (binario, hooks, entradas en CLAUDE.md/AGENTS.md, registro MCP), como reverso de `mem install`.
