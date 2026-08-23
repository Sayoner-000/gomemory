# Research — Feature 021: Instalación sin artefactos, reglas y constitución como memorias semilla

**Fecha**: 2026-08-23 · **Spec**: [spec.md](./spec.md)

Todo lo que sigue se verificó leyendo el código actual del repositorio, no de memoria.
Cada hallazgo incluye la ruta y la línea donde se comprobó.

---

## R1 — ¿De verdad el protocolo llega al agente sin `AGENTS.md`?

**Decisión**: Sí. Se puede retirar el bloque de los archivos de proyecto sin perder cobertura.

**Verificación**: `adapters/primary/cli/cmd_mcp.go:58` construye el servidor con
`&mcp.ServerOptions{Instructions: buildIntegrationBlock()}` — el MISMO texto que
`composeAgentFile` inyectaba en `AGENTS.md`. El comentario de las líneas 49-55 ya
declara la intención: «queda disponible para cualquier cliente MCP compatible sin
depender de un AGENTS.md/CLAUDE.md por proyecto». El archivo de proyecto era la
segunda copia, no la primera.

**Cobertura residual**: los agentes que no hablan MCP quedan cubiertos por el ámbito
de USUARIO, que `setup-mcp --scope global` sigue escribiendo vía
`InstallAtomicPlanGlobal` (`cmd_mcp_setup.go:188`). Esa ruta NO se toca en esta
feature — por eso `composeAgentFile`, `protocolStart` y `protocolEnd` se conservan
aunque `CmdInstall` deje de llamarlas.

**Alternativas descartadas**: (a) dejar de escribir solo en proyectos nuevos y
mantener los viejos — deja dos comportamientos conviviendo indefinidamente;
(b) mover las reglas de trabajo a `Instructions` del MCP — suma ~3.5 KB fijos a
cada sesión y las vuelve ineditables por la persona, justo lo contrario del objetivo.

---

## R2 — El `topic_key` ausente en `ListMemories` es un defecto, no un detalle

**Decisión**: Corregir la consulta **y** añadir un fetch dedicado. Son dos arreglos
distintos para dos fallos distintos; ninguno sustituye al otro.

### Fallo 1 — la columna que no viaja

`ListMemories` (`adapters/secondary/persistence/memory.go:488`) **no selecciona
`topic_key`**: su `SELECT` va de `id` a `updated_at` saltándose la columna, y el
`Scan` tampoco la lee. `domain.Memory.TopicKey` llega SIEMPRE vacío por esa vía. Su
hermana `ListAllMemories` (línea 527) **sí** la incluye — es una divergencia real
entre dos consultas que deberían proyectar lo mismo, no una decisión de diseño.

**Alcance verificado** (quién consume `List` y por tanto recibe la clave vacía):

| Consumidor | Sitio | ¿Le duele hoy? |
|---|---|---|
| Constructor de contexto | `build_context.go:189` | **Le dolería**: es el consumidor que esta feature añade |
| Tool MCP `list_memories` | `cmd_mcp.go:181` | Sí, en silencio: el campo se omite siempre |
| TUI | `tui.go` (7 sitios) | No lo muestra hoy |
| `mem list`, `mem project`, `mem compare`, huella | varios | No lo leen hoy |
| `mem migrate` | `cmd_migrate.go:37` | No: solo cuenta filas, no reinserta |

**Comprobado que NO está afectado**: `ConsolidateMemories`
(`consolidate_memories.go:50`) agrupa por `topic_key` pero lee con `ListAll`, que sí
trae la columna. La consolidación funciona. Es lo que mantiene el defecto **latente**
en vez de activo — y también lo que explica que nadie lo haya notado.

**Por qué se corrige aquí y no "algún día"**: esta feature sería el primer consumidor
en depender de esa columna por esa vía (FR-009 excluye la semilla de la sección de
preferencias comparando `TopicKey`). Sin el arreglo, la comparación daría siempre
falso y las reglas se emitirían **dos veces** — una vez íntegras y otra recortada en
preferencias. Un defecto latente que la feature activa deja de ser latente: pasa a
ser deuda que hay que pagar en el mismo cambio (FR-030).

### Fallo 2 — la ventana de recencia

`Build()` pide `b.Lister.List(b.Project, 100)` (`build_context.go:189`) y la consulta
ordena por `created_at DESC`. La semilla se crea **una sola vez, al principio** de la
vida del proyecto. En cuanto se acumulen 100 memorias más recientes desaparece del
contexto: sin error, sin aviso, sin nada que lo delate salvo que alguien note que el
agente dejó de aplicar las reglas.

No es hipotético: la investigación de la feature 020 midió que **los checkpoints
automáticos son el 69% del corpus** de este mismo proyecto. Se generan solos, uno por
turno. Cien turnos de trabajo bastan para enterrar la semilla.

Subir el límite no arregla nada: solo mueve el umbral y encarece cada arranque de
sesión. El arreglo correcto es que la presencia de una memoria fijada **no dependa de
la recencia en absoluto** — de ahí `GetMemoryByTopicKey` y FR-031.

**Alternativas descartadas**: (a) subir el límite de 100 a 1000 — pospone el fallo y
engorda el contexto; (b) ordenar las fijadas primero en la consulta — mete lógica de
presentación en la capa de persistencia y sigue dependiendo del límite; (c) marcar la
semilla con `created_at` futuro — falsear un dato para ganarle a un `ORDER BY` es
exactamente el parche temporal que la constitución prohíbe (principio V.2).

## R3 — ¿El presupuesto de contexto trunca las reglas?

**Decisión**: Sí, hay que declararlas excepción explícita.

**Verificación**: `DefaultBudget = 24000` (`persistence/settings.go:90`), es decir
`Budget > 0` siempre en la práctica. Con techo, `acota()` (`build_context.go:146`)
recorta TODA memoria a `entryExtractChars = 200` y le adjunta `→ get_memory <id>`.
Las reglas de trabajo sembradas llegarían al agente como 200 caracteres y un puntero;
el agente tendría que decidir expandirlas para aplicarlas, que es precisamente el
fallo que la feature quiere evitar.

Ya existen dos excepciones con el mismo trato: los conflictos sin resolver y el
propio bloque de protocolo, según el comentario del campo `Budget`
(`build_context.go:103-107`): «protocolo (lo añade el llamador) y conflictos NUNCA se
recortan». La sección de reglas se suma a esa lista, no inventa un mecanismo nuevo.

**Contabilidad**: emitir contenido íntegro no rompe la invariante de la feature 020
(`rawChars >= finalChars`), porque esa línea base se deriva como
`discardedChars + len(salida)` (comentario en `build_context.go:109-116`): lo que se
emite entero simplemente no incrementa `discardedChars`.

**Excepción de la excepción**: en `IndexMode` la sección SÍ colapsa a puntero. Ese
modo es un índice puro por contrato (FR-032 de la feature 020) y `acota()` ya lo
resuelve antes de mirar el presupuesto (`build_context.go:147-153`).

---

## R4 — Los efectos secundarios de guardar: tres brechas, un solo cierre

**Decisión**: Sembrar por una vía de inserción **inerte**, no por la vía normal.

`InsertMemory` (`adapters/secondary/persistence/memory.go:39`) dispara cuatro efectos
pensados para el trabajo diario. Ninguno tiene sentido sobre una semilla creada por
la herramienta. Se auditaron uno por uno, y el resultado cambió el diseño: no basta
con constatar que hoy no hacen daño, porque dos de ellos **no hacen daño por
accidente** y el tercero **sí lo haría** en una configuración perfectamente válida.

### Auditoría

| Efecto | Estado hoy | ¿Por qué es una brecha? |
|---|---|---|
| `RedactPrivate` + `RedactSecrets` (líneas 40-41) | **Inocuo, comprobado**: los 6 patrones son específicos de proveedor (`AKIA…`, `ghp_…`, `sk-ant-…`, `xox…`, JWT, PEM — `domain/redact.go:20-30`) y se corrieron contra las dos plantillas: **0 coincidencias en ambas** | Inocuo *hoy*. Nada impide que una edición futura de una plantilla introduzca una cadena que matchee, y la semilla se guardaría mutilada **en silencio**: sin error, sin diferencia visible salvo un `[REDACTED:…]` perdido en 635 líneas |
| `annotateImpact` (línea 55) | Inocuo: solo actúa si la memoria trae `Filepath`, y las semillas no lo llevan | Depende de que nadie añada un `Filepath` "para dar contexto". Es una precondición implícita, no una garantía |
| `formSynapse` (línea 78) | Inocuo **por accidente**: retorna de inmediato si `sessionID` está vacío (línea 389), y las semillas se insertan sin sesión | La inercia depende de un dato que parece incidental. Quien más adelante asocie la siembra a la sesión activa —un cambio razonable en apariencia— reactivaría el enlace sin darse cuenta |
| `exportToADR` (línea 79) | **BRECHA ACTIVA** | Ver abajo |

### La brecha activa: `exportToADR`

El guard es `if adrSyncProvider == nil || adrSyncRepo == nil || !settingsAdrSyncEnabled`
(línea 247), y `AdrSyncEnabled` no figura en `DefaultSettings()` (`settings.go:103`),
o sea `false` salvo opt-in. Esa es la lectura tranquilizadora, y es incompleta.

`adrSectionForType` (línea 230) mapea **`domain.Architecture` → `ADRSectionArchitecture`,
exportable**. La semilla de la constitución es precisamente de tipo `architecture`.
Para una persona que activó `adr_sync_enabled` —una capacidad soportada del
producto—, instalar gomemory **publicaría las 635 líneas de la constitución en su
documento ADR externo**, sin haberlo pedido. Y no de forma diferida: `exportToADR`
corre **síncrono** dentro de `InsertMemory`, con un timeout de 4 segundos (comentario
de la línea 216), así que además se nota.

Un efecto que solo es inofensivo mientras una opción esté apagada no es inofensivo:
es una bomba con temporizador de configuración.

### El cierre: una sola vía, tres brechas

Las tres se cierran con el mismo mecanismo — sembrar a través de una inserción que
**no ejecuta los canales laterales**:

```
insertMemory(db, m, opts)            <- núcleo compartido: INSERT + índice FTS
├─ InsertMemory(db, m)               = insertMemory(db, m, {})            (sin cambios)
└─ InsertSeedMemory(db, m)           = insertMemory(db, m, {inerte:true}) (NUEVO)
                                        omite formSynapse y exportToADR
```

Un núcleo, dos puertas. No duplica la lógica de inserción ni toca el camino existente.

| Brecha | Antes | Después |
|---|---|---|
| Sinapsis espuria | Inocua *porque* `SessionID` va vacío | Inocua **por construcción**: la vía inerte no llama a `formSynapse`, pase lo que pase con la sesión |
| Publicación externa | Depende de que `adr_sync_enabled` esté apagado | Nunca ocurre, ni con la opción encendida (FR-034) |
| Texto mutilado | Depende de que las plantillas no contengan cadenas parecidas a secretos | Test guardián: lo guardado debe ser idéntico carácter por carácter al origen (FR-032). Si una edición futura rompe la igualdad, el test lo grita en CI |

**Alternativas descartadas**: (a) confiar en los defaults y documentarlo — deja el
daño a una casilla de configuración de distancia; (b) `domain.Memory.SkipADRExport
bool` — mete una preocupación de adaptador dentro de la entidad de dominio, prohibido
por el principio I; (c) sembrar la constitución con un tipo no exportable (p. ej.
`learning`) — falsea la clasificación para esquivar un efecto, y `architecture` es la
clasificación correcta y ya aprobada.

## R5 — ¿Cómo se evita pisar una semilla que la persona editó?

**Decisión**: Comprobar-y-omitir (`create-if-missing`), nunca upsert.

**Verificación**: `findDuplicateTx` (`memory.go:345`) hace que un `Insert` con un
`TopicKey` ya existente **actualice** la fila previa (`UPDATE memories SET content =
…`, línea 74). Si `SeedDefaults` insertara sin comprobar, cada `mem install` con un
binario nuevo reescribiría el texto que la persona editó — el escenario 3 de la
Historia 1 fallaría.

Por eso `SeedDefaults` consulta primero con `GetMemoryByTopicKey` y solo inserta
cuando no hay nada. El precio aceptado: si una versión futura mejora el texto de las
plantillas, los proyectos existentes conservan el suyo. Es la decisión correcta —
la semilla pasa a ser propiedad de la persona en cuanto existe.

---

## R6 — ¿Dónde se siembra para que no dependa de `mem install`?

**Decisión**: Dos puntos de entrada — `CmdInstall` y el arranque del servidor MCP.

**Verificación**: `CmdMCP` (`cmd_mcp.go:23`) ya hace exactamente este tipo de trabajo
oportunista antes de servir: arranca la sesión si no hay ninguna activa (líneas
30-36), etiquetado como *best-effort, no debe romper el server*. La siembra encaja en
ese mismo hueco, con el mismo criterio: errores registrados, nunca fatales.

Coste en estado estable: dos `SELECT … WHERE topic_key = ? LIMIT 1` por arranque,
sobre el índice parcial `idx_memories_topic` que ya existe
(`persistence/db.go:225`). Cero escrituras a partir del segundo arranque.

Esto es lo que hace cierta la frase del usuario «desde la última versión se agregan
de manera automática a la memoria»: desde v1.9 mucha gente usa
`setup-mcp --scope global` y **nunca** ejecuta `mem install`.

---

## R7 — ¿Dónde queda el respaldo de los archivos que se borran?

**Decisión**: `<proyecto>/.memory/backups/agent-files/`.

**Verificación**: `.memory/` es el directorio de estado por proyecto
(`MemDir = ".memory"`, `persistence/db.go:12`) y sigue existiendo aunque la base de
datos sea global — ahí viven `settings.json` y la huella de contexto
(`activation_inspect.go:181` lo lee). Está en `.gitignore` (línea 2), así que el
respaldo no se cuela en un commit por accidente.

**Contradicción aparente y su resolución**: la feature busca *no crear nada* en el
proyecto, y el respaldo crea un directorio. No hay contradicción: `.memory/` no es un
artefacto nuevo (ya existe o se crea igual al usar la memoria), está ignorado por git,
y solo se puebla cuando hay algo que respaldar. Un proyecto limpio nunca ve ese
directorio de respaldos.

---

## R8 — ¿Qué formato necesita el envoltorio de `/constitution`?

**Decisión**: Clonar el patrón ya probado de `InstallAtomicPlanWrappers`.

**Verificación**: `adapters/primary/setup/atomic_plan_setup.go:22-36` define los dos
destinos que cada agente descubre solo: `.claude/skills/<nombre>/SKILL.md` con
frontmatter `name` + `description`, y `.opencode/commands/<nombre>.md` con
frontmatter `description`. En Claude Code una skill llamada `constitution` se invoca
como `/constitution`, que es exactamente lo pedido, y no colisiona con
`/speckit-constitution` (nombre distinto).

**Regla heredada, y el motivo por el que importa aquí**: el comentario de las líneas
38-49 explica que los envoltorios se **regeneran en cada instalación desde la fuente
embebida** para que «no haya copia editable» que pueda divergir. El paso 4b que esta
feature elimina cometía justo ese error con la constitución: copiaba 635 líneas a la
raíz y ahí quedaban, congeladas. Por eso el envoltorio nuevo **no lleva el texto de
la constitución**: lleva la instrucción de resolverla desde la memoria en el momento
de la invocación.

---

## R9 — ¿Qué se rompe al dejar de escribir los archivos?

**Decisión**: Un solo punto, ya identificado, con arreglo acotado.

**Verificación**: `inspectInstructions` (`setup/activation_inspect.go:196`) recorre
`claudeAgentFiles` buscando el marcador de versión del protocolo y, si no lo
encuentra, devuelve `StateMissing`. En cuanto `CmdInstall` deje de escribir esos
archivos, `mem doctor`/el informe de activación reportaría una falla en ámbito de
PROYECTO para todos los agentes — una falla falsa, porque el protocolo sí llega.

El ámbito de USUARIO no se toca: ahí `setup-mcp --scope global` sigue escribiendo y
la comprobación sigue siendo válida.

**Comprobado que NO se rompe**: `cmd_doctor.go` (96 líneas) no inspecciona archivos
de agente; `tests/contract/integration_block_test.go` y
`cmd_install_protocol_test.go` prueban `buildIntegrationBlock()` y
`composeAgentFile()`, funciones que sobreviven intactas.

---

## R10 — ¿Hace falta tocar `mem update`?

**Decisión**: No. La limpieza en `CmdInstall` lo cubre.

**Verificación**: `CmdUpdate` (`cmd_update.go:116`) termina delegando:
`runIn(root, self, "install", root)`. Todo lo que se añada a `CmdInstall` —siembra,
limpieza, mensaje— se ejecuta también en la actualización, sin una línea extra y sin
riesgo de que las dos rutas diverjan.

---

## R11 — ¿Dónde se conecta el nuevo puerto sin romper la arquitectura hexagonal?

**Decisión**: Campo opcional en el `Builder`, asignado en el composition root.

**Verificación**: `infrastructure/container.go:69` crea el builder con
`usecases.New(memRepo, sessRepo, relRepo, root, project)` y a continuación le asigna
los colaboradores opcionales uno por uno: `contextBuilder.Graph` (línea 70),
`contextBuilder.Counter` y `contextBuilder.Recorder` (líneas 129-130). El campo
`Topics` sigue el mismo patrón: opcional, `nil`-safe, asignado en el único lugar
donde se hace el wiring. No hay que cambiar la firma de `New` ni tocar los tests que
la usan.

---

## Resumen de decisiones

| # | Decisión | Motivo en una línea |
|---|---|---|
| R1 | Retirar el bloque de protocolo de los archivos de proyecto | Ya viaja en `Instructions` del MCP; era copia duplicada |
| R2 | Corregir `ListMemories` **y** añadir `GetMemoryByTopicKey` | Defecto latente real (columna que no viaja) + fallo por ventana de recencia; la feature activaría el primero |
| R3 | Sección de reglas exenta de `acota()`/`fits()`, salvo en modo índice | Con `Budget=24000` llegarían recortadas a 200 caracteres |
| R4 | Sembrar por una vía de inserción **inerte** | `exportToADR` publicaría la constitución fuera con `adr_sync_enabled=true`; dos brechas más eran inocuas solo por accidente |
| R5 | Comprobar-y-omitir, nunca upsert | `Insert` con `topic_key` existente hace `UPDATE` y pisaría la edición |
| R6 | Sembrar en `CmdInstall` y al arrancar el MCP | Desde v1.9 mucha gente nunca ejecuta `install` |
| R7 | Respaldo en `.memory/backups/agent-files/` | Directorio de estado ya existente e ignorado por git |
| R8 | Envoltorio sin copia del texto, resuelto en la invocación | Evita repetir el error de la copia congelada del paso 4b |
| R9 | Canal de instrucciones de proyecto → `not_applicable` | Si no, el diagnóstico reporta una falla falsa |
| R10 | No tocar `mem update` | Delega en `install`; heredaría todo |
| R11 | `Topics` como campo opcional del `Builder` | Mismo patrón que `Graph`/`Counter`/`Recorder` |

**NEEDS CLARIFICATION pendientes**: ninguno.
