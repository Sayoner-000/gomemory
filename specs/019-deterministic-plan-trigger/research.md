# Fase 0 — Investigación: activación determinista del modo plan atómico

Cada apartado sigue el formato **Decisión / Razón / Alternativas descartadas**, y separa de forma
explícita lo **verificado** de lo **pendiente de verificar en vivo**. La distinción no es
burocracia: la lección de campo n.º 2 del proyecto (una prueba vale lo que vale su fixture) y el bug
de la memoria 346 (tres mensajes que viajaban por un canal que el modelo nunca lee) nacieron
justamente de asumir capacidades del agente sin comprobarlas contra el sistema en ejecución.

## Estado de la evidencia

**Verificado en la documentación oficial de hooks** (`code.claude.com/docs/en/hooks`, consultada el
2026-08-17; la tabla de campos por evento venía truncada en la recuperación, de ahí los pendientes):

- `PreToolUse` admite `hookSpecificOutput.permissionDecision` con `permissionDecisionReason`.
- Las salidas de hook —`additionalContext`, `systemMessage` y stdout plano— están **topadas en
  10 000 caracteres**.
- `UserPromptSubmit` **no** recibe `permission_mode` en su payload: no hay forma de saber desde ese
  evento si la persona está en modo plan.

**Verificado en el código actual del repositorio**:

- `writeClaudeHooks` + `filterOutGomemoryHooks` (`adapters/primary/setup/claude_code_setup.go:290-379`)
  preservan las entradas de terceros: reconocen las propias por el comando (`hookCommandIsGomemory`)
  y solo filtran esas.
- El scope global (`runGlobalScopeSetup` → `setupClaudeGlobal`, `adapters/primary/cli/cmd_mcp_setup.go:130-254`)
  registra el servidor MCP en `~/.claude.json`, escribe el archivo de instrucciones de usuario y el
  envoltorio nativo del método — **pero no escribe hooks**. Los hooks solo se escriben a nivel
  proyecto, en `<root>/.claude/settings.json`.
- `DefaultBudget = 24000` caracteres para `get_context`
  (`adapters/secondary/persistence/settings.go:77`); el método atómico ocupa ~4,2 KB.
- El bloque de protocolo tiene marcador de **inicio** versionado y **ningún marcador de fin**;
  `composeAgentFile` (`adapters/primary/cli/cmd_install.go:288-315`) reemplaza con
  `out[:idx] + integration`, es decir descarta todo lo posterior.
- `AtomicPlanDisabled` ya existe como interruptor (`settings.go:60`).

**Verificado empíricamente en esta máquina**:

- Los dos `SessionStart` coexisten y **ambos** mensajes llegan al agente (el `CRITICAL` del brazo
  extensor y el contexto de gomemory).
- En un prompt que no es el primero de la sesión, `mem hook user-prompt-submit` emite **solo** el
  recordatorio de guardado: ni una palabra de modo plan.
- `EnterPlanMode` y `ExitPlanMode` existen como herramientas en esta versión de Claude Code.
- Ninguno de los cuatro archivos de instrucciones (`~/.claude/CLAUDE.md`, `~/CLAUDE.md`,
  `./CLAUDE.md`, `./AGENTS.md`) tiene un encabezado de nivel 2 después del bloque de protocolo: el
  borrado de contenido es un riesgo **latente**, todavía no materializado.

**Verificado en vivo el 2026-08-17** (T001-T004, protocolo en [quickstart.md](./quickstart.md) §1,
ejecutado dentro de esta misma sesión mediante las herramientas `EnterPlanMode`/`ExitPlanMode`, con
una sonda temporal registrada en `PreToolUse:ExitPlanMode` y `PostToolUse:EnterPlanMode`):

- **V1 — CONFIRMADA POSITIVA.** `PreToolUse` dispara con matcher `ExitPlanMode` y `tool_input.plan`
  trae el texto **completo** del plan (2461 caracteres, coincidencia exacta con lo escrito). Dato
  adicional no anticipado: el esquema público de la herramienta `ExitPlanMode` **no declara** un
  parámetro `plan` (solo `allowedPrompts`, marcado obsoleto) — el runtime lo añade al payload del
  hook por fuera del esquema visible. También aparece `permission_mode: "plan"` en el payload y
  `tool_input.planFilePath` con la ruta del archivo de plan.
- **V2 — CONFIRMADA POSITIVA.** Con `permissionDecision: "deny"` + `permissionDecisionReason`, el
  motivo volvió al agente como un resultado `<error>` legible, sin abortar el turno; el turno
  continuó con normalidad, pidiendo redactar el plan otra vez.
- **V3 — CONFIRMADA POSITIVA.** `PostToolUse` con matcher `EnterPlanMode` acepta
  `hookSpecificOutput.additionalContext`: el texto de prueba apareció como system-reminder
  inmediatamente después de `EnterPlanMode`, antes de cualquier otra acción.
- **V4 — NO EVALUADA.** Entrar en modo plan por atajo de teclado no es reproducible desde una
  llamada a herramienta dentro de esta sesión. Queda sin bloquear nada: si resulta negativa, el
  degradado ya previsto (recordatorio por turno, US2) cubre el hueco igual.
- **Hallazgo adicional (no bloqueante para esta puerta, apuntado para investigar aparte):** los
  hooks de Claude Code se releen dinámicamente — **no hace falta reiniciar el agente** para que un
  cambio en `.claude/settings.json` surta efecto; se confirmó registrando la sonda a mitad de sesión
  y viéndola disparar en la siguiente llamada a herramienta. Esto corrige el prerrequisito de
  [quickstart.md](./quickstart.md) §1 ("reiniciar el agente"), que ya no es necesario.
- **Hallazgo adicional #2 (posible regresión de la feature 007, fuera de alcance de la 019):** el
  hook de producción ya existente `PostToolUse:ExitPlanMode → mem hook plan-approved` **no guardó
  ninguna memoria** durante el ciclo real de esta verificación, pese a que invocar el mismo
  subcomando a mano con un payload equivalente sí guarda correctamente (memoria de prueba `473`).
  Indicio de que `PostToolUse(ExitPlanMode)` podría no llevar `tool_input.plan` en esta versión de
  Claude Code, a diferencia de `PreToolUse` (V1, confirmado que sí lo lleva). No se investigó más a
  fondo para no gastar el ciclo de modo plan de esta verificación; queda como candidato a bug
  separado de la feature 007, a confirmar con una sonda dedicada en `PostToolUse:ExitPlanMode`.

**Consecuencia de diseño**: V1 y V2 confirmadas dejan en pie el mecanismo tal como está diseñado en
§1 y en [contracts/hook-plan-guard.md](./contracts/hook-plan-guard.md) — `PreToolUse(ExitPlanMode)`
con `permissionDecision: "deny"` es la traducción correcta a Claude Code del contrato neutral. No
hace falta replantear la Historia 1.

**T026 — validación de la Historia 1 contra el binario real, 2026-08-17**: tras implementar T017-T025
(heurística, motor de decisión, traductor de dialectos, registro en `claudeHookEvents` derivado de
`domain.KnownAgents`), se reinstaló `gomemory` en este mismo repositorio (`mem install .`) con un
binario recién compilado. `.claude/settings.json` (no versionado, gitignorado) quedó con
`PreToolUse:ExitPlanMode → mem hook plan-guard`, confirmado por lectura directa del archivo.

Los cinco escenarios de [quickstart.md](./quickstart.md) §2 se ejecutaron contra el binario real y
coincidieron exactamente con el diseño: prosa larga → `exit=1` con el motivo por stderr; repetir el
mismo plan → `exit=0` (episodio ya devuelto); tras `plan-approved` → `exit=1` de nuevo (episodio
reabierto); árbol → `exit=0`; plan trivial → `exit=0`; con `plan_guard_disabled=true` → `exit=0`
siempre.

**Hallazgo operativo (no bloquea nada, documentado para quien reproduzca esto antes de un release):**
al intentar validar el ciclo **con el agente real** (`EnterPlanMode`/`ExitPlanMode` de esta misma
sesión) tras la reinstalación, el hook permitió un plan deliberadamente en prosa sin devolverlo. La
causa no es el mecanismo: `binRefFor` (`adapters/primary/cli/binref.go:45-70`) resuelve el comando del
hook con el primer `mem` que encuentra en `$PATH` — en esta máquina, el binario **publicado** v2.6.0
(`~/.local/bin/mem`), que no conoce el subcomando `plan-guard` y cae en el `default:` de `CmdHook`
(permitir en silencio). El binario recién compilado con esta feature nunca llegó a ejecutarse por esa
vía. Es un artefacto de **desarrollo pre-release**, no un defecto: tras publicar un release y actualizar
el `mem` global (con `mem update`, nunca copiando el binario a mano — preferencia ya registrada del
proyecto), el hook apuntará al binario correcto sin cambios. La validación contra el binario (arriba)
es la evidencia vigente de la Historia 1; la validación con agente real de T001-T004 (misma sesión, con
la sonda) ya había confirmado que el mecanismo de Claude Code en sí —`PreToolUse(ExitPlanMode)` trae
`tool_input.plan`, el `deny` llega legible— funciona. Combinadas, cubren extremo a extremo sin
necesidad de alterar el `mem` global de la máquina.

---

## 1. Dónde anclar el determinismo

**Decisión**: en el **borde de salida** del plan, enunciado como contrato neutral: *«antes de
presentar un plan, el agente invoca un comando con el texto del plan y respeta la decisión que
recibe»*. Claude Code es la **primera traducción** de ese contrato (`PreToolUse` con matcher
`ExitPlanMode`, respuesta `deny` con motivo), no su definición — ver §13. El borde de entrada queda
como mejor esfuerzo.

**Razón**: es el único punto del ciclo donde (a) existe una señal observable garantizada —el agente
tiene que llamar a esa herramienta para presentar el plan—, (b) el contenido a evaluar ya existe, y
(c) el mecanismo de respuesta está **documentado** (`permissionDecision`), en lugar de depender de un
campo cuya aceptación no está confirmada. Además invierte la carga: ya no hace falta que el agente
recuerde una instrucción de hace 40 000 tokens; el sistema simplemente no deja pasar lo que no
cumple.

**Alternativas descartadas**:

- *Solo inyectar en la entrada*: es lo que ya se intentó con el bootstrap del primer prompt, y su
  eficacia depende de la memoria del agente y de la competencia con otras directivas. Sigue en el
  plan, pero como calidad, no como garantía.
- *Subir el imperativo del texto de plan por encima del brazo extensor*: descartado en la
  especificación (guerra de volumen cuyo único modo de ganar es debilitar al vecino, contra INV-5).
- *Restringir herramientas de exploración durante el modo plan* (por ejemplo, gate propio sobre
  `Grep`/`Glob`): viola INV-3 y además ataca el instrumento equivocado: el problema no es que el
  agente explore, es la forma en que entrega.
- *Evaluar el plan después de aprobado* (`PostToolUse(ExitPlanMode)`, que ya existe): llega tarde por
  definición; la persona ya vio el plan. Se conserva para lo que sí sirve: capturar la decisión y
  cerrar el episodio.

## 2. Borde de entrada: `PostToolUse(EnterPlanMode)` con degradación declarada

**Decisión**: registrar `PostToolUse` con matcher `EnterPlanMode` → `mem hook plan-entered`, que
emite el documento de planificación recortado al presupuesto del canal en `additionalContext`. Si V3
resulta negativa, o si el agente entra en modo plan sin llamada a herramienta (V4), la cobertura la
da el **recordatorio de una línea en cada turno** (apartado 4).

**Razón**: `PostToolUse` corre inmediatamente después de entrar en modo plan y antes de que el agente
redacte, que es exactamente la ventana útil. Elegirlo sobre `PreToolUse` para este propósito es
deliberado: `PreToolUse` está documentado para decidir permisos, no para inyectar contexto.

**Alternativas descartadas**:

- *`PreToolUse(EnterPlanMode)` con `permissionDecisionReason` como vehículo del contexto*: abusa de un
  campo de permisos para pasar 9 KB de documento; y "permitir con motivo" no es una semántica en la
  que convenga apoyarse.
- *Reinyectar el documento completo en cada turno*: coste de contexto inaceptable en sesiones largas
  (el documento ronda los 9 KB tras el recorte). De ahí que el fallback sea una línea, no el bloque.

## 3. Heurística de forma del plan

**Decisión**: función pura en `domain/plan_shape.go` que devuelve un veredicto de tres valores
—`Cumple`, `NoCumpleClaramente`, `NoAplica`— con **sesgo estructural a permitir**. Señales, todas
independientes del idioma:

1. **No aplica** si el plan es corto (por debajo de un umbral de líneas/caracteres): una solicitud
   trivial no necesita árbol (regla 5 del propio método).
2. **Cumple** si aparece cualquiera de: glifos de árbol (`├─`, `└─`, `│`), identificadores
   jerárquicos de dos o más niveles (`[1.2]`, `1.2.3`), o los marcadores del formato del método
   (`✓`, `⚠`, `dep:`, `∥`).
3. **No cumple claramente** solo si no hay **ninguna** de esas señales y el plan supera el umbral de
   tamaño.

La flecha de resultado verificable (`→`) **no** entra en la decisión: se usa únicamente para redactar
el motivo, porque exigirla produciría falsos bloqueos en planes válidos con otro formato.

**Razón**: el criterio de la especificación es "claramente no cumple" (FR-003), y el coste de un
falso bloqueo es mucho mayor que el de dejar pasar un plan mediocre. Una heurística estructural es
además comprobable con una tabla de casos, sin montar una sesión ni un agente — condición para que
la Historia 1 tenga pruebas de verdad y no un fixture complaciente.

**Alternativas descartadas**:

- *Palabras clave en español* ("subtarea", "objetivo"): rompe con proyectos en otro idioma y con el
  propio método traducido.
- *Pedirle a un modelo que juzgue el plan*: latencia y coste en el camino crítico de cada
  presentación, y no determinista, que es justo lo que se quiere eliminar.
- *Exigir el formato exacto del método*: convierte una guía en un molde y bloquea planes válidos.

## 4. Presupuesto del canal: el recorte es obligatorio, no defensivo

**Decisión**: `plan-entered` recorta a **9 500 caracteres** (margen de 500 sobre el tope de 10 000),
con prioridad estricta: el **método completo** primero, el historial después con lo que quede, y una
línea final indicando que el resto se recupera con `get_plan_context()`. Nunca se corta a mitad de
frase: el recorte cae en el último límite de párrafo o de línea que quepa.

**Razón**: no es una precaución teórica. `Budget` por defecto de `get_context` es **24 000**
caracteres y el método ocupa ~4 200: el documento de planificación **supera el canal por diseño**.
Sin recorte explícito, el propio agente recibiría un documento cortado por el runtime en un punto
arbitrario — que es la peor variante posible, porque el corte silencioso se lleva justamente el final
del método (la sección de autoverificación y el formato de salida).

**Alternativas descartadas**:

- *Bajar `Budget` global*: castiga a `get_context` en todos los demás usos por una restricción que es
  de un canal concreto.
- *Confiar en el truncamiento del runtime*: corta por el final, que es donde vive el formato de
  salida; equivale a inyectar el método sin su conclusión.

## 5. Actualizar el bloque de protocolo sin destruir contenido

**Decisión**: a partir de la versión **v8** el bloque se delimita con marcador de inicio **y de
fin**. Para los bloques legados (v1..v7, que no tienen fin) se aplica una regla de límite explícita:
el bloque termina en el **siguiente encabezado de nivel 2** o, si no hay ninguno, al final del
archivo. Verificado que la regla es exacta para el bloque actual: todos sus subtítulos son de nivel
3, así que un nivel 2 posterior siempre es contenido ajeno.

**Razón**: es la migración mínima que convierte una heurística en un contrato: se usa una sola vez,
para pasar de v7 a v8, y desde ahí el límite es explícito para siempre. Hoy el riesgo es latente (no
hay nivel 2 después del bloque en ninguno de los cuatro archivos revisados), lo que hace de esto un
arreglo preventivo barato en vez de una recuperación de datos.

**Alternativas descartadas**:

- *Seguir truncando y avisar en la salida del instalador*: trasladar a la persona el coste de un
  defecto conocido.
- *Reconocer el bloque legado por su texto conocido*: se rompe con cualquier edición manual, y la
  gente edita estos archivos.
- *Insertar el bloque nuevo dejando el viejo*: duplica instrucciones contradictorias, que es peor que
  el borrado.

## 6. Reconciliación con la regla "nunca interrumpir el modo plan" (feature 013, FR-034)

**Decisión**: la regla se mantiene tal cual para todo lo **ambiental** —proyecto sin memoria, canal
caído, error interno, evaluación dudosa: siempre permitir y siempre salir con código 0— y se
distingue de la **devolución deliberada** del `plan-guard`, que es una decisión del protocolo,
declarada con su motivo, limitada a una vez por episodio y apagable.

**Razón**: aquella regla protegía al usuario de que un fallo de la memoria le rompiera el flujo de
trabajo. Un plan devuelto con el motivo "esto no es un árbol de tareas verificables, rehazlo" no es
un fallo: es la funcionalidad. Confundir ambos casos habría dejado la feature sin su único mecanismo
determinista.

**Alternativas descartadas**:

- *Solo advertir sin bloquear*: es lo que ya existe (texto en el contexto) y es exactamente lo que
  falló.
- *Bloquear siempre hasta que cumpla*: bucles de reintento y una persona secuestrada por una
  heurística. De ahí el límite de una devolución por episodio.

## 7. El canal de hooks a nivel usuario no existe hoy

**Decisión**: el ámbito global debe escribir **también** los hooks, en `~/.claude/settings.json`,
preservando toda entrada ajena. Es requisito para que las Historias 1 y 4 se cumplan juntas.

**Razón**: hallazgo del recorrido — el scope global registra el servidor MCP, el archivo de
instrucciones y el envoltorio del método, pero **no** hooks. Que hoy existan hooks de gomemory en
`~/.claude/settings.json` es consecuencia del accidente de haber instalado `$HOME` como proyecto
(mismas marcas de tiempo en `$HOME/CLAUDE.md`, `$HOME/AGENTS.md`, `$HOME/.mcp.json` y el envoltorio
del método). Sin este cambio, un proyecto nuevo tendría el texto pero no el determinismo, y FR-016
quedaría incumplido.

**Alternativas descartadas**:

- *Depender de la instalación accidental de `$HOME`*: la especificación lo prohíbe (FR-016) y se
  rompería en cuanto alguien limpie su directorio personal.
- *Exigir instalación por proyecto*: contradice "habilitar una vez cubre todos los proyectos".

## 8. Duplicación de entradas al reinstalar: el riesgo real

**Decisión**: registrar los subcomandos nuevos (`plan-entered`, `plan-guard`) en
`hookCommandIsGomemory` **en la misma tarea** que los añade a `claudeHookEvents`, y cubrirlo con una
prueba de doble instalación.

**Razón**: el filtro de idempotencia reconoce las entradas propias por substring del comando. Un
subcomando no registrado ahí no se filtra, así que **cada reinstalación añade otra copia** — el mismo
tropiezo que la feature 007 tuvo que corregir con `plan-approved` (T004). Es el único riesgo real de
convivencia que encontré: la fusión ya preserva lo ajeno, así que el vecino no está en peligro; el
que se rompe es uno mismo.

**Alternativas descartadas**:

- *Reconocer las entradas propias por un campo marcador nuevo en el JSON*: rompería la limpieza de
  las instalaciones ya existentes, que no lo llevan.

## 9. Idempotencia por episodio: máquina de estados con los hooks disponibles

**Decisión**: un contador por sesión en un marcador de archivo bajo `.memory/`. `plan-entered` lo
pone a cero (empieza un episodio); `plan-guard` solo devuelve el plan si el contador está a cero, y
lo incrementa; `plan-approved` lo pone a cero (cierra el episodio). Si `plan-entered` no está
disponible (V3/V4 negativas), el ciclo sigue funcionando con `plan-guard` + `plan-approved`.

**Razón**: cubre la garantía "como máximo una devolución por episodio" con los eventos que ya
existen, sin inventar un identificador de episodio que el agente no expone. El sesgo del caso
degradado es el correcto: si la persona rechaza el plan y no hay aprobación que reinicie el contador,
el siguiente plan **pasa sin evaluar** — se peca de permisivo, nunca de bloqueante.

**Alternativas descartadas**:

- *Guardar el estado en SQLite*: escritura en el camino crítico del hook sin ganancia; contradice el
  objetivo de < 50 ms.
- *Hash del texto del plan como identidad de episodio*: dos intentos distintos del mismo episodio son
  textos distintos, así que permitiría devolver dos veces.

## 10. Redacción compuesta: cómo se enuncian juntas exploración y forma

**Decisión**: en el texto propio de gomemory (bloque de protocolo, instrucciones MCP, plugin de
OpenCode y recordatorio por turno), las dos capacidades se enuncian como **pasos de una misma
secuencia**, con el grafo nombrado como el instrumento del paso de exploración:

> Para explorar el código usa las herramientas del grafo; para entregar el plan usa el árbol de
> tareas atómicas. Lo que descubras con el grafo alimenta las hojas del árbol.

Se elimina del párrafo del grafo la reclamación explícita del modo plan ("independientemente de la
tarea: chat, plan, resumen") **solo** en lo que respecta a la forma de la salida: la preferencia por
el grafo frente a leer archivos a mano se mantiene intacta para la exploración, que es su papel.

**Razón**: la competencia la crea el texto propio, no el brazo extensor, así que se corrige en el
texto propio (INV-1). El extensor sale reforzado: pasa de rival a instrumento nombrado del plan.

**Alternativas descartadas**:

- *Añadir "excepto en modo plan" al párrafo del grafo*: debilitaría al extensor exactamente donde más
  útil es (explorar para fundamentar un plan). Contra INV-5.
- *Dejar el texto como está y compensar con el guard*: el guard arregla la forma, no la calidad; sin
  la redacción compuesta el agente sigue eligiendo entre dos órdenes que suenan rivales.

## 11. OpenCode: qué se puede igualar y qué se declara

**Decisión**: OpenCode recibe la paridad de **texto** (instrucción compuesta, ya inyectada en cada
turno por su plugin) y **no** la devolución del borde de salida, porque su ciclo no ofrece un punto
de decisión antes de presentar el plan. El reporte de cobertura lo declara como degradación
explícita, no como cobertura completa. Su plugin pasa además a ser la **implementación de referencia**
del contrato neutral de §13: si mañana OpenCode ofrece un punto de decisión antes de presentar el
plan, sube al nivel 1 implementando el contrato, sin cambios en gomemory.

**Razón**: FR-017 exige declarar las diferencias en lugar de aparentar paridad. Inventar una
devolución simulada en OpenCode (por ejemplo, corregir en el turno siguiente) daría la ilusión de
garantía sin la garantía.

**Alternativas descartadas**:

- *Evaluar el plan en `handleTurnEnd` e inyectar la corrección para el turno siguiente*: la persona ya
  vio el plan; es exactamente el "llega tarde" del apartado 1.

## 12. Dónde vive el reporte de cobertura

**Decisión**: subcomando nuevo `mem doctor`, con `--json` para consumo por el script de regresión y
`--strict` para terminar con código distinto de cero cuando hay problemas (uso en CI). La lógica de
composición vive en un caso de uso; el adaptador de setup aporta la inspección de rutas.

**Razón**: mantiene el script de regresión **fino** (compara JSON, no reimplementa reglas) y deja la
lógica donde puede probarse en Go con cobertura. `mem doctor` es además el nombre que la gente busca
para "¿está bien instalado esto?".

**Alternativas descartadas**:

- *Ampliar `mem settings`*: mezcla preferencias con diagnóstico.
- *Dejar toda la lógica en el script de shell*: sin pruebas unitarias y duplicando reglas que ya
  viven en Go; es la clase de sitio donde una regresión pasa desapercibida.
- *Ampliar `mem install --check`*: acopla el diagnóstico a la instalación, y se quiere poder
  diagnosticar sin instalar nada.

## 13. Neutralidad de agente: dialectos, niveles y registro único

**Decisión**: la capacidad se define en tres capas, y ningún agente ocupa el centro.

**13.1 — Un motor, varios dialectos.** El motor de decisión (heurística, estado de episodio,
presupuesto) es único y se invoca por una sola superficie de línea de comandos. Lo único que cambia
por agente es el **dialecto** de entrada y de salida:

| Dialecto | Entrada | Salida de «devolver el plan» | Para quién |
|---|---|---|---|
| `neutral` (por defecto) | `{"plan":"…"}` por stdin, o texto plano | código de salida ≠ 0 y motivo por stderr | cualquier agente, incluidos los que no existen todavía |
| `json` | igual que neutral | objeto JSON con el motivo en un campo declarado | agentes que leen JSON de stdout |
| `claude` | `tool_input.plan` | `hookSpecificOutput.permissionDecision: "deny"` + motivo | Claude Code |
| `text` | igual que neutral | motivo por stdout, código 0 | agentes que solo inyectan stdout como contexto (patrón del plugin de OpenCode) |

La selección es automática a partir de lo que el agente envía (si el payload trae la forma de un
dialecto conocido, se responde en ese dialecto) y se puede forzar con una bandera explícita. **Si no
se puede determinar, se responde en `neutral`** — nunca en el dialecto de un agente concreto.

El código de salida deja de ser «siempre 0» de forma absoluta: es 0 en los dialectos que transportan
la decisión en la salida, y distinto de 0 **solo** en el dialecto que por contrato usa el código como
vehículo. Sigue siendo cierto lo que importaba: ningún fallo ambiental produce jamás un bloqueo.

**13.2 — Tres niveles de activación, declarados por agente.**

| Nivel | Qué exige del agente | Qué obtiene |
|---|---|---|
| 1 — determinista | poder invocar un comando antes de presentar el plan y respetar su decisión | la garantía completa (un plan sin forma no llega a la persona) |
| 2 — inyección en la entrada | poder inyectar contexto al entrar en modo plan | el método y el historial disponibles antes de redactar |
| 3 — piso textual | poder leer un archivo de instrucciones | protocolo, recordatorio por turno y envoltorio nativo |

**Todo agente tiene el nivel 3.** Los niveles 1 y 2 son capacidades del agente, no favores de
gomemory: se declaran, se reportan y se degradan con honestidad. Lo que la especificación prohíbe no
es que un agente tenga menos, es que gomemory lo esconda o que defina la capacidad en el formato de
uno solo.

**13.3 — Registro único de capacidades.** Una tabla de dominio declara, por agente: dialecto,
niveles soportados, ámbitos disponibles (proyecto/usuario), rutas de sus archivos de instrucciones y
formato de su envoltorio nativo. Añadir un agente es añadir una fila. El reporte de estado y la
verificación de regresión se alimentan de esa tabla, así que un agente nuevo aparece en los dos sin
tocarlos.

**Alcance declarado**: las tablas por agente que ya existen dispersas —agentes con ámbito de usuario,
envoltorios nativos del método, archivos de instrucciones reconocidos— **no** se migran en esta
feature. El registro nace como fuente única para lo nuevo (niveles, dialectos, reporte) y la
unificación del resto queda como trabajo posterior explícito. Migrar todo aquí convertiría una
feature de determinismo en un refactor del instalador, y la constitución pide impacto mínimo.

**13.4 — Contrato publicado para integradores.** Se publica
[contracts/agent-integration.md](./contracts/agent-integration.md): qué comando invocar, en qué
momento, qué enviar y cómo interpretar la respuesta, con un ejemplo mínimo ejecutable. El plugin de
OpenCode pasa a ser la **implementación de referencia** de ese contrato, no un caso especial.

**Razón**: el objetivo declarado desde la feature 013 es que cualquier agente quede cubierto, «los de
hoy y los que aparezcan después». Eso solo es verdad si la capacidad está definida fuera de todo
agente y publicada para que un tercero la implemente. Anclarla en el formato de Claude Code habría
reproducido, en el mecanismo determinista, la misma asimetría que la feature venía a corregir — con
el agravante de que esta vez sería estructural y no de redacción.

**Alternativas descartadas**:

- *Implementar primero para Claude Code y «generalizar después»*: el «después» no llega, y el diseño
  queda con la forma del primer agente. Es exactamente cómo nació la asimetría del punto 3 del
  problema.
- *Un adaptador de código por agente*: cada agente nuevo exigiría una versión de gomemory. El
  contrato publicado invierte la dependencia: el agente se adapta a gomemory sin que gomemory lo
  conozca.
- *Solo el dialecto neutral, sin traducciones*: obligaría a Claude Code a usar el código de salida
  cuando tiene un mecanismo más expresivo y mejor documentado. Traducir es baratísimo y no cuesta
  neutralidad, siempre que el neutral sea el que manda por defecto.
- *Declarar capacidades en un archivo de configuración editable por la persona*: invita a declarar
  capacidades que el agente no tiene, y el fallo resultante sería silencioso y confuso.

## Resumen de decisiones

| # | Decisión | Riesgo si la verificación en vivo falla |
|---|---|---|
| 1 | Determinismo en `PreToolUse(ExitPlanMode)` con `deny` + motivo | V1/V2 negativas → la feature pierde su garantía; el fallback es la paridad textual por turno y se declara en el reporte |
| 2 | Entrada por `PostToolUse(EnterPlanMode)`, mejor esfuerzo | V3/V4 negativas → cubre el recordatorio de una línea por turno; ninguna otra historia se afecta |
| 3 | Heurística estructural pura, sesgada a permitir | Ninguno: es texto puro y se prueba con tabla de casos |
| 4 | Recorte a 9 500 con prioridad método > historial | Ninguno: la restricción está documentada y medida |
| 5 | Marcador de fin en v8 + regla de nivel 2 para legados | Ninguno: verificado contra los cuatro archivos reales |
| 6 | Devolución deliberada ≠ interrupción ambiental | Ninguno: reconciliación de criterio, no de mecanismo |
| 7 | El ámbito global también escribe hooks | Ninguno: rutas y fusión ya verificadas en el código |
| 8 | Registrar los subcomandos nuevos en el filtro de idempotencia | Ninguno, si se hace en la misma tarea |
| 9 | Contador de episodio en marcador de archivo | Ninguno: degrada a permisivo |
| 10 | Redacción compuesta grafo → árbol | Ninguno: es texto propio |
| 11 | OpenCode: paridad textual, degradación declarada | Ninguno: es lo que se declara |
| 12 | `mem doctor [--json] [--strict]` | Ninguno |
| 13 | Contrato neutral + dialectos + registro único de capacidades + contrato publicado para integradores | Ninguno: es diseño propio y no depende de ninguna capacidad externa. Si V1/V2 fallan, el nivel 1 queda sin traducción para Claude Code, pero el contrato sigue disponible para cualquier otro agente que sí pueda cumplirlo |
