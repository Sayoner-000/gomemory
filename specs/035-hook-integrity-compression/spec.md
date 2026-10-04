# Feature Specification: Integridad de los hooks, avisos por conversación y compresión que no destruye

**Feature Branch**: `035-hook-integrity-compression`

**Created**: 2026-10-04

**Status**: Implemented

**Input**: Descripción de la persona: «siento que todavía gomemory no genera una compresión adecuada en codex, opencode y claude. Adicional el recordatorio de checkpoint se queda pegado y trae recordatorio de otras sesiones una vez cerrada la conversación y debería ser más coherente ese disparador». Ampliación: «con claude code, también se degradan mucho los hooks de gomemory, perdiendo la invocación a las tools y tomas decisiones no acertadas en el camino… blindemos correctamente estos issues». Decisión tomada: un solo spec con el alcance «recordatorios + compresión» **más** el blindaje de los hooks en Claude Code.

## Contexto del problema *(no normativo — fundamenta el porqué)*

Evidencia medida el 2026-10-04 sobre el binario instalado (v2.27.1), en una sesión real de Claude Code en este repositorio:

**Hooks degradados en Claude Code**
- Los hooks de gomemory están registrados a la vez en el ámbito de usuario (`~/.claude/settings.json`) y en el de proyecto (`.claude/settings.json`), así que **cada evento corre dos veces**. En la sesión llegaron duplicados el recordatorio de modo plan y la regla de Octopus. `mem doctor` muestra ambos registros como sanos.
- La salida del arranque de sesión midió 17 777 caracteres y la del primer prompt en modo plan 12 944. El tope del canal de Claude Code es 10 000 caracteres: el host guardó ambas en un archivo y al modelo solo le mostró una vista previa de unos 2 KB. Se perdieron el protocolo, las reglas, el método de plan y la memoria del proyecto; solo llegó la primera línea, la de carga de tools.
- El hook de salidas de herramientas degradó lo que el agente había pedido:
  - un listado de código Go (vía `sed`) llegó sin indentación y con sentencias reemplazadas por «1 frases omitidas»;
  - una búsqueda en el grafo de código llegó con «omitidos 23 de 30 elementos», lo que obligó al agente a buscar por otra vía;
  - un diagnóstico de `mem doctor` llegó con una línea omitida.
- El recordatorio de modo plan y la regla de delegación se inyectan en **cada** turno (más de 1 300 caracteres por turno, el doble con el registro duplicado).

**Avisos que cruzan de conversación**
- El estado por turno (huella de contexto, debounces, aviso pendiente al agente) es por proyecto y ningún inicio de conversación lo reinicia de forma coherente.
- En Codex, el fin de turno deja un «aviso de compactación pendiente» que el primer prompt de la conversación siguiente no consume; el segundo prompt lo entrega en una conversación que no lo generó.
- Codex no tiene evento de fin de sesión y OpenCode solo cierra al terminar el proceso: la conversación nueva reutiliza la sesión anterior, cuya antigüedad hace saltar el recordatorio de guardado de inmediato.
- El contador de huella contenía `5078217904191461789312457`: quedó corrupto por escrituras concurrentes de varios agentes, se lee como 0 y el aviso de compactación deja de funcionar.

**Compresión con poco rendimiento**
- `mem pack savings`: 276 usos con 0,3 % de ahorro, todos contados como «degradación»; solo JSON rinde (81,5 %). Las salidas típicas de herramientas (búsquedas por texto, listados de archivos, salida de tests) no tienen un compresor adecuado.
- Codex no permite reescribir salidas de herramientas (v2.27.1): ahí solo comprime lo que gomemory genera en origen.

## Clarifications

### Session 2026-10-04

- Q: ¿Qué pasa con las instalaciones que ya tienen hooks duplicados? → A: Limpieza automática en `mem update` y `mem install`, y aviso en el arranque de sesión hasta que se corrija (nunca se reescribe la configuración del host durante `session-start`). La limpieza conserva el ámbito global y quita del proyecto lo duplicado (ver FR-001).
- Q: ¿El recordatorio de modo plan deja de emitirse en cada turno (decisión de la feature 019)? → A: Se diferencia por señal: en Claude Code (con señal de entrada a plan observable) solo al inicio, tras compactar o al entrar en plan; en Codex y OpenCode se mantiene en cada turno (019 vigente para ellos). La regla de Octopus, en todos los agentes, solo al inicio, tras compactar o cuando cambia su activación.
- Q: ¿Las protecciones (duplicados descartados, recortes por presupuesto, exclusiones de compresión) dejan rastro consultable? → A: Sí. Se registran en el registro de canales existente (feature 024) y `mem doctor` muestra los conteos de los últimos 7 días.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - El agente recibe completas y una sola vez las instrucciones de gomemory en Claude Code (Priority: P1)

Una persona abre Claude Code en un proyecto con gomemory. El agente recibe, una sola vez y completas, la carga de tools, el protocolo de memoria y el contexto que quepa. No recibe vistas previas truncadas ni duplicados, así que llama a las tools correctas desde el primer turno.

**Why this priority**: es la causa directa de que el agente «pierda la invocación a las tools y tome decisiones no acertadas». Sin esto, el resto de la memoria no llega al modelo.

**Independent Test**: abrir una conversación nueva en modo plan sobre este repositorio con la instalación vigente y comprobar que ninguna salida de gomemory se trunca ni aparece duplicada, y que el agente llama a las tools de memoria en su primer turno.

**Acceptance Scenarios**:

1. **Given** hooks de gomemory registrados en usuario y proyecto, **When** la persona ejecuta `mem install` o `mem update`, **Then** cada subcomando queda registrado una sola vez (el global se conserva; el proyecto solo añade lo que el global no cubre) y los hooks ajenos quedan intactos.
2. **Given** hooks duplicados y ninguna reinstalación ni actualización todavía, **When** arranca una sesión, **Then** la persona ve un aviso (no el modelo) con el comando que lo corrige, la configuración del host no se modifica y el aviso deja de aparecer en cuanto se corrige la duplicación.
3. **Given** un registro duplicado que sobrevivió por cualquier motivo, **When** se dispara un evento, **Then** solo la primera invocación produce salida y efectos, y `mem doctor` advierte la duplicación con su remedio.
4. **Given** un proyecto con memoria abundante, **When** arranca la sesión o se envía el primer prompt (también en modo plan), **Then** cada salida cabe en el tope del canal, las instrucciones críticas van completas y primero, y lo recortado se señala con una indicación para pedir el resto.
5. **Given** una sesión en curso, **When** la persona envía prompts sucesivos, **Then** en Claude Code los recordatorios de modo plan y de delegación no se repiten en cada turno: aparecen al inicio, tras compactar, al entrar en plan o cuando cambia la activación de la delegación. En Codex y OpenCode el de modo plan sigue en cada turno.

---

### User Story 2 - La compresión de salidas nunca destruye lo que el agente pidió (Priority: P1)

El agente lee código, busca en el grafo o consulta un diagnóstico. Lo que recibe es exactamente lo que la herramienta devolvió, o bien un resumen de volumen redundante que conserva literal todo lo relevante y deja una referencia para recuperar el original.

**Why this priority**: una salida alterada hace que el agente edite sobre texto falso o descarte resultados reales. Es pérdida de corrección, peor que no ahorrar.

**Independent Test**: repetir con el hook activo las tres salidas reales de la evidencia (listado de código Go, búsqueda de 30 resultados, diagnóstico) y compararlas byte a byte con la salida sin hook.

**Acceptance Scenarios**:

1. **Given** una salida de herramienta que contiene código (con o sin cabecera de paquete), **When** pasa por la compresión de salidas, **Then** cada línea conservada sale idéntica, indentación incluida, y ninguna sentencia se sustituye por una marca de omisión.
2. **Given** resultados de búsqueda o de consulta exacta de tamaño moderado (hasta 50 elementos), **When** pasan por la compresión, **Then** llegan completos; una búsqueda por patrón con cientos de coincidencias solo se resume como listado (US4).
3. **Given** texto en prosa en una salida de herramienta, **When** pasa por la compresión, **Then** se entrega sin cambios.
4. **Given** una salida pequeña, **When** está por debajo del umbral mínimo de compresión de salidas, **Then** se entrega sin cambios.

---

### User Story 3 - Ningún aviso cruza de una conversación a otra (Priority: P2)

La persona cierra una conversación en Claude, Codex u OpenCode y abre otra. La conversación nueva empieza limpia: sin el aviso de compactación de la anterior ni recordatorios de guardado basados en el reloj de la sesión vieja.

**Why this priority**: es el síntoma del «checkpoint pegado». Genera ruido y empuja al agente a guardar cosas que no corresponden, pero no rompe la corrección de las salidas.

**Independent Test**: en Codex, provocar el aviso de compactación pendiente, cerrar la conversación, abrir otra y enviar tres prompts; ninguno debe traer el aviso.

**Acceptance Scenarios**:

1. **Given** un aviso pendiente dejado por la conversación A, **When** empieza la conversación B, **Then** B nunca recibe ese aviso.
2. **Given** un aviso pendiente de la conversación en curso, **When** el siguiente prompt es el primero tras el arranque, **Then** el aviso se entrega en ese prompt y una sola vez.
3. **Given** una conversación nueva que reutiliza una sesión abierta hace horas, **When** pasan los primeros minutos, **Then** el recordatorio de guardado mide desde el inicio de la conversación, no desde el de la sesión.
4. **Given** una sesión sin actividad durante más del umbral de inactividad, **When** empieza una conversación nueva, **Then** la sesión vieja se cierra con un resumen automático y su respaldo, y se abre una nueva.
5. **Given** dos agentes trabajando a la vez en el mismo proyecto con actividad reciente, **When** uno de ellos inicia conversación, **Then** no cierra la sesión del otro.
6. **Given** varios agentes que actualizan a la vez el contador de huella, **When** se lee el contador, **Then** siempre es un número válido; si alguna vez se encuentra corrupto, se reinicia en lugar de quedar inservible.

---

### User Story 4 - La compresión ahorra de verdad en listados y la métrica dice la verdad (Priority: P3)

Las salidas largas y repetitivas (búsquedas por texto con muchas coincidencias, listados de archivos, salida detallada de tests) se resumen conservando el principio, el final, los errores y un conteo de lo omitido, con una referencia para recuperar el original. El reporte de ahorro distingue «sin ganancia» de «degradado».

**Why this priority**: recupera el ahorro que la US2 sacrifica por seguridad y hace observable el rendimiento real. Aporta valor, pero no corrige un fallo de corrección.

**Independent Test**: comprimir tres fixtures reales (búsqueda con más de 100 coincidencias, listado largo, salida detallada de tests) y medir el ahorro y la conservación de errores; revisar `mem pack savings`.

**Acceptance Scenarios**:

1. **Given** una búsqueda por texto con más de 100 coincidencias, **When** se comprime, **Then** el resultado conserva literales las primeras y últimas líneas, cuenta lo omitido por archivo e incluye una referencia que devuelve el original íntegro.
2. **Given** un listado que contiene líneas de error, fallo o pánico, **When** se comprime, **Then** esas líneas se conservan siempre.
3. **Given** una salida en la que no se gana nada al comprimir, **When** se registra en las estadísticas, **Then** no cuenta como degradación.

---

### User Story 5 - Codex y OpenCode con expectativas claras (Priority: P3)

En Codex, la persona y el agente saben que las salidas de herramientas externas no se pueden comprimir y que, para explorar código, conviene usar las tools de gomemory, que comprimen en origen. En OpenCode se aplican las mismas reglas de seguridad de la US2.

**Why this priority**: es una limitación del host que no se puede eliminar; documentarla y orientar al agente evita expectativas falsas.

**Independent Test**: `mem doctor` en una máquina con Codex informa «compresión de salidas: solo en origen»; el protocolo entregado a Codex incluye la orientación; en OpenCode, las pruebas de la US2 dan el mismo resultado.

**Acceptance Scenarios**:

1. **Given** Codex instalado, **When** la persona ejecuta `mem doctor`, **Then** ve que la compresión de salidas en Codex ocurre solo en origen y por qué.
2. **Given** un agente en Codex, **When** recibe el protocolo, **Then** el protocolo le indica preferir las tools de código de gomemory.
3. **Given** OpenCode con la compresión de salidas activa, **When** una herramienta de lectura exacta o de comando de lectura devuelve su salida, **Then** se aplican las mismas exclusiones que en Claude.

---

### Edge Cases

- **Hooks ajenos en el mismo archivo** (otros proveedores, multiplexores): limpiar el ámbito contrario nunca los toca.
- **Salida de arranque mayor que el tope incluso solo con lo crítico**: lo crítico tiene un tamaño acotado y probado que siempre cabe; si no cupiera, se prioriza la carga de tools y el protocolo, y lo demás se recorta.
- **Compactación dentro de una conversación**: no es una conversación nueva. Conserva la identidad de la conversación y aplica los reinicios propios de la compactación ya existentes.
- **Conversación sin identificador del host**: se continúa la conversación registrada; si no hay ninguna, se abre una con un identificador local. *(Ajustado tras acr_715249c3, C-004: tratarla siempre como nueva borraba el estado de la conversación en curso.)*
- **Clasificación de listado sobre algo que no lo es**: se conservan errores y extremos, el original es recuperable por referencia y la guarda de literalidad sigue activa.
- **Archivo de estado ilegible o corrupto**: se reinicia sin romper el turno ni producir salida espuria.
- **Dos invocaciones casi simultáneas del mismo evento con contenido distinto**: no se tratan como duplicadas; solo se descarta la repetición del mismo evento con el mismo contenido dentro de una ventana corta.

## Requirements *(mandatory)*

### Functional Requirements

**Integridad de hooks (US1)**
- **FR-001**: `mem install` y `mem update` MUST dejar cada subcomando de hook de gomemory registrado una sola vez entre el ámbito de usuario y el de proyecto de Claude Code. Los hooks globales sirven a todos los proyectos y nunca se retiran desde un proyecto: el proyecto solo registra los subcomandos que el global no cubre (p. ej. `tool-output`) y retira de su propio `settings.json` los que ya cubre el global. Codex (solo ámbito de usuario) y OpenCode (plugin único en `~/.config/opencode/plugins`) no admiten duplicación. *(Corregido en la implementación el 2026-10-04: la redacción anterior, «conservar el ámbito configurado», habría borrado los hooks globales y dejado sin memoria a los demás proyectos.)*
- **FR-001a**: Mientras persista la duplicación, el arranque de sesión MUST mostrar a la persona (canal de aviso, no contexto del modelo) un aviso con el comando que la corrige, y MUST NOT modificar la configuración del host.
- **FR-002**: El sistema MUST ignorar la segunda invocación del mismo evento con el mismo contenido dentro de una ventana corta (red de seguridad ante registros duplicados), sin producir salida ni efectos.
- **FR-003**: `mem doctor` MUST advertir los hooks de gomemory duplicados entre ámbitos como un problema, con el comando que lo resuelve.
- **FR-004**: Toda salida que gomemory inyecta por hooks MUST caber en el tope del canal del host (10 000 caracteres en Claude Code), medida sobre el texto que llega al modelo.
- **FR-005**: Cuando el contenido excede el tope, el sistema MUST priorizar por secciones (carga de tools > protocolo > política de delegación > documento de plan > memoria del proyecto), recortar las de menor prioridad e indicar cómo obtener el resto.
- **FR-006**: El sistema MUST registrar como entregado solo lo que realmente se emitió.
- **FR-007**: El documento de plan del primer prompt MUST NOT repetir la memoria del proyecto si ya se entregó en el arranque de la misma conversación.
- **FR-008**: En los agentes con señal observable de entrada a plan (Claude Code), el recordatorio de modo plan MUST emitirse solo al inicio de la conversación, tras una compactación o al entrar en plan. En los agentes sin esa señal (Codex, OpenCode), MUST seguir emitiéndose en cada turno (FR-003/FR-006 de la feature 019, vigentes para ellos).
- **FR-008a**: En los canales que se acumulan en el historial (`user-prompt-submit` de Claude Code y Codex), la política de delegación (Octopus) MUST emitirse solo al inicio de la conversación, tras una compactación o cuando cambia su activación. En OpenCode se inyecta por `system.transform`, que reconstruye el system prompt en cada petición sin acumularse: ahí MUST seguir presente en cada turno, porque omitirla la haría desaparecer. *(Precisado en la implementación el 2026-10-04.)*

**Compresión segura de salidas (US2)**
- **FR-009**: La compresión de salidas MUST reconocer código Go aunque no empiece por la cabecera de paquete, y código indentado en general.
- **FR-010**: La compresión de salidas MUST NOT alterar los espacios iniciales de ninguna línea que conserve.
- **FR-011**: La compresión de salidas MUST NOT resumir prosa; solo puede resumir JSON masivo, logs, diffs y listados.
- **FR-012**: La compresión de salidas MUST conservar completos los arrays de resultados de tamaño moderado (hasta 50 elementos).
- **FR-013**: La compresión de salidas MUST excluir las herramientas de lectura exacta o diagnóstico: lectura de fragmentos de código, comandos de lectura (`sed`, `cat`, `head`, `tail`, `git diff`, `git show`) y el diagnóstico de gomemory (`mem doctor`). Las búsquedas por patrón (Grep) siguen siendo comprimibles, pero solo como listado o log, nunca como prosa, y conservando completos los resultados según FR-012.
- **FR-014**: El umbral mínimo de la compresión de salidas MUST subir a un nivel en el que el ahorro compense el riesgo (2 000 tokens aproximados).
- **FR-015**: La nota explicativa de las marcas de omisión MUST aparecer como máximo una vez por conversación.

**Estado por conversación (US3)**
- **FR-016**: El sistema MUST tener un único punto de inicio de conversación, común a todos los agentes, que reinicie todo el estado por turno y registre la identidad y la hora de inicio de la conversación.
- **FR-017**: El inicio de conversación MUST dispararse en el arranque de sesión del host (salvo en una compactación), en el inicio de sesión explícito que usa OpenCode y en el autoarranque del servidor de memoria.
- **FR-018**: Un aviso pendiente MUST quedar asociado a la conversación que lo generó, entregarse solo a esa conversación y descartarse en cualquier otra.
- **FR-019**: El aviso pendiente MUST poder entregarse también en el primer prompt tras el arranque.
- **FR-020**: El recordatorio de guardado MUST medir el tiempo desde el inicio de la conversación (o desde el último guardado, si es posterior), no desde el inicio de la sesión.
- **FR-021**: Al iniciar una conversación, una sesión sin actividad durante más de 4 horas MUST cerrarse con un resumen automático y su respaldo, y abrirse una nueva; una sesión con actividad reciente MUST NOT rotarse.
- **FR-022**: Los archivos de estado compartidos MUST escribirse de forma atómica, y un valor corrupto MUST reiniciarse en vez de leerse como cero.

**Ahorro y métrica (US4)**
- **FR-023**: El sistema MUST reconocer los listados (búsquedas con ruta:línea, listados de rutas) y resumirlos conservando literales las primeras 15 y las últimas 5 líneas y todas las líneas de error, fallo o pánico, con un conteo de lo omitido por archivo o directorio y una referencia recuperable.
- **FR-024**: Las estadísticas MUST distinguir «sin ganancia» de «degradado», y el reporte de ahorro MUST mostrarlos por separado.

**Observabilidad de las protecciones**
- **FR-028**: Cada invocación duplicada descartada (FR-002), cada recorte por presupuesto (FR-005, con el tamaño original y el emitido) y cada salida excluida de la compresión (FR-013) MUST registrarse en el registro de canales existente, sin contenido, solo el evento y las cifras.
- **FR-029**: `mem doctor` MUST mostrar los conteos de esos eventos de los últimos 7 días por agente y advertir cuando haya recortes o duplicados.

**Codex y OpenCode (US5)**
- **FR-025**: `mem doctor` MUST informar que en Codex la compresión de salidas ocurre solo en origen y por qué.
- **FR-026**: El protocolo entregado a Codex MUST orientar a usar las tools de código de gomemory para explorar código.
- **FR-027**: La integración de OpenCode MUST pasar a la compresión de salidas la información necesaria para aplicar las exclusiones de FR-013.

### Key Entities

- **Conversación**: una interacción continua de una persona con un agente, desde que abre hasta que cierra, que sobrevive a las compactaciones. Atributos: identidad (del host o generada localmente) y hora de inicio. Es la dueña del estado por turno.
- **Estado por turno**: huella de contexto acumulada, marcas de último recordatorio, aviso pendiente al agente y marcas de «ya entregado». Pertenece a una conversación.
- **Sesión de memoria**: agrupación persistente de memorias y checkpoints. Puede abarcar varias conversaciones y se rota por inactividad.
- **Sección de salida de hook**: fragmento con prioridad y condición de recortable que compone una salida inyectada.
- **Listado**: salida de herramienta orientada a líneas, con rutas o ruta:línea, que admite un resumen de cabeza, cola y conteo.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En una conversación nueva en modo plan sobre este repositorio, el 0 % de las salidas de gomemory llega truncada o como vista previa (hoy fallan 2 de 2).
- **SC-002**: Cada evento de hook produce exactamente una salida; el número de recordatorios duplicados por turno es 0.
- **SC-003**: El agente invoca las tools de memoria en su primer turno en 10 de 10 conversaciones nuevas.
- **SC-004**: En los fixtures reales de código, búsqueda de resultados y diagnóstico, el 100 % de las líneas conservadas coincide byte a byte con el original y no se pierde ningún resultado.
- **SC-005**: En 10 ciclos de cerrar y abrir conversación en Codex, Claude y OpenCode, 0 avisos llegan a una conversación distinta de la que los generó.
- **SC-006**: Tras 1 000 actualizaciones concurrentes, el contador de huella es siempre un número válido.
- **SC-007**: Los listados largos se reducen al menos un 40 %, conservando el 100 % de sus líneas de error, y el resumen tarda menos que el presupuesto de tiempo del hook.
- **SC-008**: El reporte de ahorro muestra 0 «degradaciones» atribuidas a casos sin ganancia.
- **SC-009**: En Claude Code, los recordatorios por turno ocupan al menos un 80 % menos de caracteres en una conversación de 20 turnos sin cambios de estado; en Codex y OpenCode, la regla de delegación deja de repetirse en cada turno.
- **SC-010**: Tras provocar un duplicado, un recorte y una exclusión, `mem doctor` muestra los tres eventos con sus conteos.

## Assumptions

- El tope del canal de hooks de Claude Code es de 10 000 caracteres (ya registrado en el proyecto, specs/019). Otros hosts usan el mismo tope por prudencia.
- Claude Code y Codex aportan un identificador de conversación en el evento de arranque, y OpenCode en `session.created`; si falta, se genera uno local.
- Se acepta que, con agentes simultáneos en el mismo proyecto, el reloj de recordatorios lo marque la última conversación iniciada: el peor caso es un recordatorio adelantado o retrasado.
- El umbral de inactividad para rotar sesiones es de 4 horas.
- Se acepta reducir el alcance de la compresión de salidas en Claude a cambio de corrección; el ahorro se recupera con el resumen de listados.
- La limitación de Codex (no reescribe salidas de herramientas) es del host y queda fuera del alcance resolverla.
- Se reutilizan los mecanismos existentes de referencia recuperable, guarda de literalidad, respaldo de sesión y registro de entregas.
- El proceso de entrega sigue la regla del proyecto: cada historia se verifica contra el binario instalado, no solo con tests.
