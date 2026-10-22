# Feature Specification: mem expand — supertools inspiradas en ECC, activas desde la instalación en todos los harnesses

**Feature Branch**: `main` (rama existente; sin creación de rama en esta invocación)

**Created**: 2026-10-21

**Status**: Draft

**Input**: User description: "Plan: mem expand, supertools inspiradas en ECC (affaan-m/ECC v2.2.3, MIT) para todos los harnesses. Se traen las cuatro piezas que faltan en gomemory —instincts (continuous-learning-v2), rules (hookify + GateGuard), shield (AgentShield) y flow-skills (verification-loop, search-first, iterative-retrieval, strategic-compact)—, reimplementadas sin Node ni Python. `mem expand` queda activo desde la instalación en Claude Code, Codex y OpenCode, sin opt-in; cada supertool se dispara sola desde los hooks según el contexto del evento a través de un router puro y determinista con un único punto de entrada de hooks. La línea de comandos solo informa (status) y ofrece un interruptor de emergencia (off/on). Las flow-skills se reinterpretan sobre las primitivas de gomemory y son neutrales de harness. No se trae lo que ya existe (memoria persistente, resúmenes de sesión, pre-compactación, revisión) ni el catálogo masivo de agentes y skills de dominio."

## Clarifications

### Session 2026-10-21

Decisiones tomadas por defecto a partir del plan de entrada (sin preguntas abiertas):

- El interruptor de emergencia admite apagado global y por supertool (`mem expand off|on [<supertool>]`); ambos se guardan en los ajustes del proyecto y son la única forma de desactivar el módulo.
- Las reglas también se evalúan al terminar el turno del agente (evento de parada), así que el punto de entrada único se cablea en inicio de sesión, envío de prompt, antes y después de cada herramienta, y parada.
- Tras /speckit-analyze (F1/F2): en Codex se amplía el matcher a shell + parche de archivos (+ delegación si existe) y se verifica en sesión interactiva real; SC-001 se mantiene en 3 de 3.
- La evolución de un instinct a skill mediante un modelo de lenguaje (`mem expand instincts evolve`) es la historia de menor prioridad; nunca escribe una skill sin aprobación explícita de la persona, y su encaje con el generador ya existente se decide en el plan.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Bloqueo automático de comandos destructivos sin configurar nada (Priority: P1)

Como persona que instala gomemory en Claude Code, Codex u OpenCode, quiero que, sin ningún paso de habilitación, un comando destructivo propuesto por el agente (`rm -rf`, `git reset --hard`, `git push --force`, `git checkout -- .`) se bloquee con un motivo visible antes de ejecutarse, y poder añadir reglas propias en `.gomemory/rules/*.md` que avisen o bloqueen según el evento y un patrón.

**Why this priority**: Es el mayor valor inmediato y el único que protege contra pérdida de datos irreversible. Además exige el router y el punto de entrada único activos desde la instalación, que son la base del resto de supertools.

**Independent Test**: Hacer una instalación limpia en cada harness y, sin configurar nada, pedir al agente `rm -rf /tmp/x`: la herramienta no se ejecuta y la persona ve el motivo. Después, crear una regla propia de tipo aviso sobre un patrón de prompt y comprobar que aparece el aviso sin bloquear.

**Acceptance Scenarios**:

1. **Given** una instalación limpia sin reglas propias, **When** el agente intenta ejecutar `rm -rf /tmp/x`, **Then** la ejecución se deniega con un motivo legible y la decisión es la misma en Claude Code, Codex y OpenCode.
2. **Given** una regla propia con acción `warn` sobre un patrón de comando, **When** el agente propone un comando que coincide, **Then** el comando se permite y el agente recibe el aviso como contexto adicional.
3. **Given** una regla propia con acción `block` para el evento de prompt, **When** la persona envía un prompt que coincide, **Then** el prompt se detiene con el motivo de la regla.
4. **Given** un comando inocuo (`ls`, `git status`), **When** el agente lo ejecuta, **Then** se permite sin emitir ningún texto.
5. **Given** una regla malformada, con patrón inválido o que supera el tamaño máximo, **When** se evalúa cualquier evento, **Then** esa regla se ignora, el resto se aplica y la herramienta nunca queda bloqueada por el fallo de una regla.
6. **Given** que la evaluación supera el presupuesto de tiempo, **When** llega el evento, **Then** se permite la acción y no se emite nada.

---

### User Story 2 - Módulo siempre activo, visible y con interruptor de emergencia (Priority: P1)

Como persona que usa gomemory, quiero que `mem install`, `mem setup-mcp` y `mem update` dejen el módulo cableado en los tres harnesses sin preguntarme, poder consultar con `mem expand status` qué está cableado, qué se activó recientemente y qué encontró el escáner, y poder apagarlo todo o una supertool concreta con `mem expand off [<supertool>]` en una emergencia.

**Why this priority**: Sin cableado automático, sin desinstalación limpia y sin interruptor, el módulo sería un riesgo operativo (hooks huérfanos o duplicados, imposibilidad de apagar un falso positivo que bloquea el trabajo).

**Independent Test**: Instalar desde cero, ejecutar `mem expand status` y comprobar que los tres harnesses aparecen cableados. Ejecutar `mem expand off`, repetir el comando destructivo y comprobar que no se bloquea; ejecutar `mem expand on` y comprobar que vuelve a bloquearse. Desinstalar y comprobar que no quedan entradas del módulo en ninguna configuración del harness.

**Acceptance Scenarios**:

1. **Given** una instalación previa sin el módulo, **When** la persona ejecuta `mem update`, **Then** el punto de entrada del módulo queda cableado en todos los harnesses instalados sin intervención.
2. **Given** el módulo cableado, **When** se ejecuta la instalación dos veces seguidas, **Then** no aparecen entradas duplicadas.
3. **Given** el módulo cableado, **When** la persona desinstala gomemory, **Then** no queda ninguna entrada del módulo en la configuración de Claude Code, Codex ni OpenCode, y las entradas ajenas a gomemory se conservan.
4. **Given** `mem expand off`, **When** ocurre cualquier evento, **Then** ninguna supertool se activa ni emite nada; `mem expand on` lo restablece todo, incluidas las supertools apagadas una a una.
5. **Given** `mem expand off rules`, **When** ocurre un evento, **Then** solo rules queda apagada y el resto de supertools sigue activo.
6. **Given** cualquier estado, **When** la persona ejecuta `mem expand status`, **Then** ve, por harness, si está cableado, el estado de cada supertool, las últimas activaciones y el último resultado del escáner.
7. **Given** la ayuda de `mem`, **When** la persona la consulta, **Then** `expand` aparece en el índice y tiene ayuda detallada.

---

### User Story 3 - Sugerencias de flujo en el momento justo (Priority: P2)

Como persona que trabaja con un agente, quiero que, cuando el contexto lo indique, el agente reciba una sola vez por episodio una sugerencia breve de usar la skill de flujo adecuada: verificar antes de confirmar cambios (`verify-loop`), buscar antes de construir (`search-first`), recuperar contexto acotado antes de delegar (`iterative-retrieval`) o compactar en un punto estratégico tras muchas herramientas (`strategic-compact`). Las skills se instalan siempre en los tres harnesses y usan las capacidades de gomemory.

**Why this priority**: Aporta disciplina de flujo sin coste de contexto permanente, pero depende del router (P1) y no protege contra daño directo.

**Independent Test**: Con una instalación limpia, pedir al agente un `git commit` y comprobar que recibe una vez la sugerencia de `verify-loop` y no la recibe de nuevo en el mismo episodio. Comprobar que las cinco skills existen en los directorios de skills de los tres harnesses.

**Acceptance Scenarios**:

1. **Given** el agente a punto de ejecutar `git commit` o abrir una solicitud de cambios, **When** llega el evento, **Then** se aplica rules y el agente recibe la sugerencia de `verify-loop` como contexto adicional, una sola vez por episodio.
2. **Given** un prompt que pide añadir una dependencia o una integración, **When** se envía, **Then** el agente recibe la sugerencia de `search-first`.
3. **Given** que se superó el umbral de herramientas desde la última compactación, **When** llega el siguiente evento de herramienta, **Then** el agente recibe la sugerencia de `strategic-compact`.
4. **Given** una delegación a un subagente, **When** llega el evento, **Then** el agente recibe la sugerencia de `iterative-retrieval`.
5. **Given** un evento ambiguo que no encaja claramente en ningún disparador, **When** se evalúa, **Then** no se emite nada.
6. **Given** cada skill instalada, **When** se lee, **Then** prefiere las capacidades MCP de gomemory, ofrece la alternativa por línea de comandos `mem …`, sigue siendo completa sin ninguna de las dos, no contiene comandos exclusivos de un harness fuera de su tabla por harness y atribuye la idea original de ECC bajo licencia MIT.

---

### User Story 4 - Escaneo de seguridad de la configuración del agente (Priority: P2)

Como persona que configura varios harnesses, quiero que gomemory revise, en solo lectura, la configuración de mis agentes (hooks, servidores MCP, permisos, archivos de instrucciones) y me avise de secretos en claro, hooks que descargan y ejecutan código remoto o apuntan fuera de mi directorio personal, servidores MCP con credenciales embebidas y permisos demasiado amplios. Quiero el informe con `mem doctor --shield` (y en JSON), y un aviso al iniciar sesión solo si la configuración cambió y hay hallazgos.

**Why this priority**: Reduce un riesgo real de fuga de credenciales y de ejecución de código no confiable, es barato y no depende de rules ni de instincts.

**Independent Test**: Con fixtures que contienen un token en claro, un hook `curl … | sh`, un servidor MCP con credencial inline y un permiso comodín, ejecutar `mem doctor --shield --json` y comprobar que aparecen los cuatro hallazgos con el secreto redactado. Iniciar dos sesiones sin cambiar la configuración y comprobar que el escaneo automático solo se ejecuta una vez.

**Acceptance Scenarios**:

1. **Given** una configuración con un secreto en claro, **When** se ejecuta `mem doctor --shield`, **Then** aparece un hallazgo con archivo y severidad, y el valor del secreto nunca se muestra.
2. **Given** un hook que descarga y ejecuta un script remoto, **When** se escanea, **Then** se informa como hallazgo de severidad alta.
3. **Given** una configuración sin cambios desde el último escaneo, **When** se inicia una sesión, **Then** no se vuelve a escanear ni se emite nada.
4. **Given** una configuración modificada sin hallazgos, **When** se inicia una sesión, **Then** se escanea y no se emite nada.
5. **Given** un archivo de configuración enorme o un número excesivo de archivos, **When** se escanea, **Then** se respetan los topes por archivo y por recuento, y el informe indica lo que no se analizó.
6. **Given** hallazgos críticos, **When** se ejecuta `mem doctor --shield`, **Then** el código de salida lo refleja; el escaneo nunca modifica ningún archivo.

---

### User Story 5 - Aprendizaje continuo por instincts (Priority: P3)

Como persona que repite patrones de trabajo y corrige al agente de forma recurrente, quiero que gomemory observe, sin coste visible, los resultados de las herramientas y mis prompts, proponga comportamientos atómicos con una confianza entre 0,3 y 0,9 por proyecto, los promueva a globales cuando aparezcan en dos o más proyectos e inyecte al inicio de sesión los de confianza alta, y poder revisarlos con `mem expand instincts list` y confirmarlos o rechazarlos con la skill `instincts`.

**Why this priority**: Es la supertool más incierta (calidad de la heurística) y no es necesaria para el valor de las anteriores.

**Independent Test**: Repetir varias veces en dos proyectos distintos la misma corrección o secuencia de comandos, comprobar con `mem expand instincts list` que aparece un instinct con confianza creciente y alcance global, y que al iniciar una sesión nueva aparece en el contexto cuando su confianza es igual o mayor que 0,7.

**Acceptance Scenarios**:

1. **Given** cualquier resultado de herramienta, **When** se observa, **Then** la observación nunca bloquea, nunca escribe en la salida visible del agente y respeta el presupuesto de tiempo.
2. **Given** el mismo patrón observado repetidamente en un proyecto, **When** se recalcula, **Then** su confianza sube sin superar 0,9 y no se crean duplicados.
3. **Given** el mismo instinct en dos o más proyectos, **When** se recalcula, **Then** pasa a alcance global.
4. **Given** instincts con confianza ≥ 0,7, **When** se inicia una sesión, **Then** se inyectan en el contexto junto a las preferencias; los de menor confianza no.
5. **Given** un instinct que la persona rechaza, **When** se vuelve a observar el patrón, **Then** no se reinyecta.
6. **Given** un instinct confirmado, **When** la persona lo confirma mediante la skill, **Then** queda guardado como preferencia.

---

### User Story 6 - Evolucionar un instinct a skill (Priority: P4)

Como persona con un instinct consolidado, quiero pedir con `mem expand instincts evolve <instinct>` que un modelo de lenguaje, elegido con la misma selección de harness que Socratica, proponga una skill derivada, revisarla y aprobarla antes de que se escriba.

**Why this priority**: Es el único paso que requiere un modelo de lenguaje y no aporta valor sin instincts consolidados.

**Independent Test**: Con un instinct de confianza alta, ejecutar `evolve`, comprobar que se muestra la propuesta y que solo se escribe la skill tras aprobación explícita; sin aprobación no se escribe nada.

**Acceptance Scenarios**:

1. **Given** un instinct existente y un harness disponible, **When** se ejecuta `evolve`, **Then** se muestra una propuesta de skill y no se escribe nada hasta aprobarla.
2. **Given** que no hay ningún harness disponible o la generación falla, **When** se ejecuta `evolve`, **Then** se informa del motivo sin dejar archivos a medias.

---

### Edge Cases

- El proyecto no tiene `.gomemory/rules/`: solo aplica la regla de serie de comandos destructivos.
- Una regla propia contiene caracteres de control de terminal: se sanean antes de mostrarse.
- Una regla propia define un patrón regular de coste patológico: el presupuesto de tiempo impide que bloquee la herramienta; en caso de duda se permite.
- Dos reglas coinciden con acciones distintas: prevalece la más restrictiva (`block` > `warn` > `allow`) y se informa el motivo de la que decide.
- La regla de serie bloquea un caso legítimo: la persona puede apagar rules con el interruptor o anular la regla de serie desde sus reglas propias.
- La base de datos de memoria no está disponible: el módulo permite y no emite nada; las observaciones de instincts esperan en su cola acotada hasta el siguiente plegado (si la cola llega a su tope se descartan las más antiguas).
- El payload del evento es inválido o no se reconoce el dialecto del harness: se permite sin emitir nada.
- Instalaciones antiguas con hooks de gomemory cuyo subcomando no se reconocía: la desinstalación y la deduplicación reconocen también el punto de entrada del módulo.
- Varias sesiones simultáneas en el mismo proyecto: la deduplicación de sugerencias es por episodio y no se mezcla entre sesiones.
- El escáner encuentra un secreto dentro de un archivo de instrucciones del agente: lo informa redactado y nunca lo persiste en memoria.
- Una skill de flujo menciona una capacidad MCP que deja de existir: la prueba de contrato falla antes de publicar.

## Requirements *(mandatory)*

### Functional Requirements

**Router y punto de entrada**

- **FR-001**: El sistema MUST decidir, una sola vez por evento y de forma pura y determinista, qué acciones de supertool corresponden a partir de: tipo de evento, herramienta, comando, ruta de archivo, número de herramientas interceptadas (las de shell, escritura de archivos y delegación que llegan al módulo) desde la última compactación, huella de la configuración del agente y presencia de reglas.
- **FR-002**: El sistema MUST exponer un único punto de entrada de hooks para el módulo, cableado en los eventos de inicio de sesión, envío de prompt, antes de herramienta, después de herramienta y parada.
- **FR-003**: Cada evento MUST resolverse dentro de un presupuesto de 150 ms; si se agota, el sistema MUST permitir la acción y no emitir nada.
- **FR-004**: Ante cualquier error, payload inválido o duda, el sistema MUST permitir y no emitir nada (fallo abierto), salvo una decisión `block` ya determinada por una regla válida.
- **FR-005**: La decisión de bloqueo, aviso o permiso MUST expresarse en el dialecto propio de cada harness (Claude Code, Codex, OpenCode, texto y neutral) con el mismo resultado lógico en todos, salvo las excepciones de paridad declaradas con evidencia (ver Assumptions).

**Activación, migración y desinstalación**

- **FR-006**: `mem install` y `mem setup-mcp` MUST cablear siempre el módulo en Claude Code, Codex y OpenCode, sin opción de activación previa.
- **FR-007**: `mem update` MUST cablear el módulo en las instalaciones existentes mediante la migración de versión de protocolo.
- **FR-008**: La instalación repetida MUST ser idempotente (sin entradas duplicadas) y la desinstalación MUST eliminar todas las entradas del módulo conservando las ajenas.
- **FR-009**: El reconocimiento de entradas de hooks de gomemory MUST derivarse de un registro único de subcomandos, verificado por prueba, para que ningún subcomando nuevo quede sin reconocer.

**CLI**

- **FR-010**: `mem expand status` MUST mostrar, por harness, el estado de cableado, el estado de cada supertool, las activaciones recientes y el último resultado del escáner.
- **FR-011**: `mem expand off|on [<supertool>]` MUST apagar o encender el módulo completo o una supertool concreta (nombres válidos: `rules`, `shield`, `flow` —las sugerencias de flow-skills— e `instincts`), persistiendo el estado en los ajustes del proyecto; es la única forma de desactivarlo.
- **FR-012**: Con el módulo o una supertool apagados, MUST no activarse ni emitirse nada de lo apagado.
- **FR-013**: `expand` MUST figurar en el índice de ayuda y tener ayuda detallada, con verificación en ambos sentidos.

**Rules**

- **FR-014**: El sistema MUST cargar reglas propias desde `.gomemory/rules/*.md`, cada una con evento (`bash`, `file`, `prompt`, `stop` o `any`), patrón, tipo de coincidencia (`contains` por defecto, o expresión regular), acción (`warn` o `block`), activación (`enabled`, verdadera por defecto) y, opcionalmente, el nombre de una regla de serie que sustituye (`overrides`).
- **FR-015**: Las reglas MUST sanearse (caracteres de control de terminal) al leerlas, tener un tamaño máximo por regla y un número máximo de reglas, y las inválidas MUST ignorarse sin afectar a las válidas.
- **FR-016**: El sistema MUST incluir una regla de serie que bloquee comandos destructivos: `rm -rf` (y variantes de opciones), `git reset --hard`, `git push --force`/`-f` y `git checkout -- .`.
- **FR-017**: Rules MUST evaluarse antes de cada herramienta, al enviar un prompt y en la parada; con varias coincidencias prevalece la acción más restrictiva.
- **FR-018**: Las reglas compiladas MUST reutilizarse entre eventos mientras sus archivos no cambien.

**Flow-skills**

- **FR-019**: El sistema MUST instalar siempre cinco skills (`verify-loop`, `search-first`, `iterative-retrieval`, `strategic-compact`, `instincts`) en cada harness presente en la máquina (Claude Code, Codex, OpenCode), sin crear el directorio de configuración de un harness ausente, desde una fuente canónica única y con verificación de sincronía con las copias publicadas.
- **FR-020**: Cada skill MUST nombrar las capacidades MCP de gomemory como vía preferente cuando existan (si una operación no tiene capacidad MCP, la CLI `mem …` es la vía principal), ofrecer la alternativa `mem …`, seguir siendo completa sin ninguna de las dos, no contener comandos de gestor de paquetes de Node ni rutas o comandos exclusivos de un harness fuera de una tabla por harness, y tener un nombre igual al de su directorio.
- **FR-021**: Una prueba de contrato MUST fallar si una skill menciona una capacidad MCP inexistente en gomemory o un comando exclusivo de un harness fuera de su tabla.
- **FR-022**: Las skills MUST acompañarse de un aviso de atribución MIT a ECC.
- **FR-023**: El router MUST sugerir, como contexto adicional y como máximo una vez por episodio: `verify-loop` ante confirmación de cambios o apertura de solicitud de cambios; `search-first` ante prompts de añadir dependencia o integración; `strategic-compact` al superar el umbral de herramientas interceptadas desde la última compactación; `iterative-retrieval` ante delegación a subagente.
- **FR-024**: Contenido de cada skill:
  - `verify-loop`: detectar el stack (Go, TypeScript, PowerShell) y verificar compilación, análisis estático, pruebas con detección de carreras, lint y diff; consultar bugs previos sobre los archivos tocados; aplicar rules al diff; guardar el veredicto; proponer revisión adversarial si el riesgo es alto.
  - `search-first`: buscar en memoria, luego en el grafo de código, luego en skills instaladas y por último fuera; terminar con una decisión Adoptar/Extender/Construir guardada.
  - `iterative-retrieval`: bucle de recuperación, evaluación y refinamiento de como máximo tres ciclos sobre los paquetes de contexto y el grafo, cuyo resultado es el contexto acotado que se entrega al subagente.
  - `strategic-compact`: cuándo compactar (tras investigar, tras un hito, tras un enfoque fallido; nunca a mitad de implementación), guardar el resumen de sesión antes de compactar y tabla del medidor de contexto nativo de cada harness.
  - `instincts`: revisar, confirmar (como preferencia) o rechazar instincts propuestos.

**Shield**

- **FR-025**: El sistema MUST escanear en solo lectura los archivos de configuración conocidos de cada harness (ajustes, configuración MCP, archivos de instrucciones del agente) con topes por archivo y por recuento.
- **FR-026**: El escáner MUST detectar: secretos en claro, hooks que descargan y ejecutan código remoto o ejecutan rutas fuera del directorio personal, servidores MCP con credenciales embebidas y permisos de autorización amplios; cada hallazgo MUST indicar archivo, categoría y severidad, con los secretos siempre redactados.
- **FR-027**: `mem doctor --shield` MUST mostrar el informe como sección propia, también en JSON, con un código de salida que refleje hallazgos críticos.
- **FR-028**: Al iniciar sesión, el escaneo automático MUST ejecutarse solo si cambió la huella de la configuración del agente, y MUST emitir un aviso solo si hay hallazgos.

**Instincts**

- **FR-029**: El sistema MUST observar resultados de herramientas y prompts sin bloquear ni escribir en la salida visible, dentro del presupuesto de tiempo.
- **FR-030**: El sistema MUST proponer instincts mediante una heurística determinista (secuencias de comandos repetidas y patrones de corrección de la persona), sin agente en segundo plano.
- **FR-031**: Cada instinct MUST tener una confianza entre 0,3 y 0,9, un alcance (proyecto o global), un número de observaciones y los proyectos en que se vio; MUST ser único por identificador estable, sin duplicados.
- **FR-032**: Un instinct visto en dos o más proyectos, con confianza ≥ 0,5 en cada uno, MUST promoverse a global; por debajo de ese umbral no se registra fuera de su proyecto (evita difundir ruido).
- **FR-033**: Los instincts con confianza ≥ 0,7 y no rechazados MUST inyectarse en el contexto del inicio de sesión junto a las preferencias.
- **FR-034**: `mem expand instincts list` MUST listar los instincts en solo lectura con confianza, alcance y observaciones.
- **FR-035**: `mem expand instincts evolve <instinct>` MUST proponer una skill derivada mediante un modelo de lenguaje con la selección de harness existente, y MUST no escribir nada sin aprobación explícita.

**Alcance excluido**

- **FR-036**: El módulo MUST NOT duplicar capacidades existentes de gomemory (memoria persistente, resúmenes de sesión, pre-compactación, revisión adversarial) ni incorporar catálogos de agentes o skills de dominio, ni depender de Node o Python en tiempo de ejecución.

### Key Entities

- **Supertool**: Unidad del módulo (rules, shield, flow-skills, instincts) con nombre, estado (activa/apagada) y eventos que la disparan.
- **Señales de evento**: Datos de entrada del router: tipo de evento, herramienta, comando, ruta, contador de herramientas desde la última compactación, huella de configuración y presencia de reglas.
- **Acción de supertool**: Resultado del router para un evento: evaluar rules, observar instinct, escanear, o sugerir una skill concreta.
- **Episodio de sugerencia**: Registro de qué sugerencias ya se emitieron en el episodio actual de una sesión, para emitir cada una como máximo una vez.
- **Regla**: Nombre, evento, patrón, tipo de coincidencia, acción, activación, regla de serie que sustituye (opcional), mensaje y origen (de serie o propia).
- **Veredicto de regla**: Permitir, avisar o bloquear, con el motivo y la regla que decide.
- **Hallazgo de shield**: Archivo, categoría, severidad y descripción redactada.
- **Huella de configuración**: Resumen de los archivos de configuración del agente que determina si hay que volver a escanear.
- **Instinct**: Comportamiento atómico propuesto, con identificador estable, descripción, confianza, alcance, número de observaciones, proyectos en que se vio y estado (propuesto, confirmado, rechazado).
- **Ajustes del módulo**: Interruptor global y por supertool.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tras una instalación limpia, sin ningún paso de habilitación, un comando destructivo de la lista de serie se bloquea con motivo visible en los 3 harnesses (3 de 3).
- **SC-002**: El 95 % de los eventos del módulo se resuelve en menos de 150 ms, medido sobre 100 invocaciones consecutivas.
- **SC-003**: Tras instalar, actualizar y desinstalar, quedan 0 entradas del módulo en las configuraciones de los 3 harnesses, y 0 entradas duplicadas tras instalar dos veces.
- **SC-004**: Con el interruptor apagado se producen 0 activaciones de supertools; al encenderlo se recupera el comportamiento en el siguiente evento.
- **SC-005**: Cada sugerencia de skill se emite como máximo 1 vez por episodio, y 0 veces en eventos que no encajan con un disparador.
- **SC-006**: El escáner detecta las 4 categorías de riesgo en fixtures que las contienen, sin mostrar nunca el valor de un secreto; sobre un conjunto versionado de configuraciones negativas (variables referenciadas, valores privados, permisos que añade gomemory) produce 0 hallazgos, y sobre la configuración real de la máquina de referencia no produce falsos positivos críticos.
- **SC-007**: Con la configuración sin cambios, el escaneo automático se ejecuta como máximo 1 vez por huella.
- **SC-008**: Las 5 skills están presentes en cada harness instalado (3 de 3 en la máquina de referencia) tras instalar y pasan la prueba de contrato (capacidades existentes, sin comandos exclusivos fuera de tabla, nombre igual al directorio).
- **SC-009**: La observación de instincts produce 0 bloqueos y 0 escrituras en la salida visible del agente; un patrón repetido en 2 proyectos alcanza alcance global.
- **SC-010**: El conjunto completo de pruebas del proyecto pasa con detección de carreras, y la integración continua (Go, TypeScript, PowerShell) queda en verde.

## Assumptions

- La lista de serie de comandos destructivos es la del plan; puede ampliarse en el plan sin cambiar la spec si no introduce falsos positivos en comandos comunes de lectura.
- El umbral de herramientas para sugerir compactación tiene un valor por defecto razonable (del orden de 50) que se fija en el plan.
- Un "episodio" se define igual que en la guardia de plan existente (se reinicia al compactar o iniciar sesión).
- Los instincts propuestos viven en una tabla aditiva de estadísticas por proyecto y en un registro global compartido entre proyectos; solo los que la persona confirma pasan al almacén de memorias, como preferencias. Las migraciones son aditivas y no alteran datos existentes.
- En Codex, la cobertura de herramientas (shell, edición de archivos por parche y delegación) se verifica en una sesión interactiva real antes de declararla; lo que no se pueda interceptar se documenta con evidencia.
- **Excepciones de paridad**: cuando un harness no ofrece el mecanismo nativo para una decisión (por ejemplo, OpenCode no puede impedir el fin de turno y entrega el aviso previo a una herramienta en el mensaje siguiente), la excepción MUST quedar declarada con evidencia real en la matriz de compatibilidad de la documentación; nunca se declara sin evidencia ni se resuelve aprobando en silencio. El bloqueo de comandos destructivos antes de ejecutarse (SC-001) no admite excepción.
- El escáner reutiliza el saneamiento de secretos existente; nunca persiste valores de secretos.
- La neutralidad de harness es obligatoria: ningún comportamiento depende de capacidades exclusivas de un harness salvo el dialecto de salida.
- La verificación final se hace contra los harnesses reales con un binario recompilado, no solo con pruebas unitarias.
- Dependencias: mecanismo de hooks existente de gomemory por harness, ajustes por proyecto, migración de versión de protocolo, instalación de skills de agente, `mem doctor`, selección de harness de Socratica.
