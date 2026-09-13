# Feature Specification: Gate de pre-escritura, evidencia de anclas y masa de memorias

**Feature Branch**: `031-memory-gate-evidence-mass`

**Created**: 2026-09-13

**Status**: Draft

**Input**: User description: "Plan: ideas de mcview aplicadas a gomemory (gate · evidencia · masa). Tres entregas independientes, en orden de riesgo creciente: Gate → Evidencia → Masa. Cada una es un commit propio. Este plan sirve de entrada para speckit-specify como feature 031." (El árbol de tareas completo, las reglas de aristas y la verificación end-to-end del plan original son la entrada de `/speckit-plan`.)

## Contexto del problema *(no normativo: explica el porqué)*

gomemory guarda memorias (decisiones, bugfixes, patrones…) y las enlaza entre sí.
Tres huecos, verificados sobre el almacén real del proyecto, hacen que lo
guardado pierda valor con el tiempo:

1. **Se guardan duplicados sin que nadie lo note.** Al guardar solo se detecta
   el duplicado exacto (misma clave de tema o mismo título). La detección por
   similitud existe, pero corre después y solo en la revisión manual. Hoy hay
   dos memorias duplicadas (207 y 209: mismo tipo, título y sesión).
2. **Los enlaces entre memorias no pesan en nada.** Los enlaces se crean, pero
   solo se listan los 12 más recientes, y casi todos son ruido de registros
   automáticos (checkpoints). Ningún ranking los usa.
3. **Las memorias ancladas a un archivo nunca se contrastan con el código.** Si
   el archivo se borró o se movió, la memoria se sigue presentando como vigente.

Principios que se adoptan del analizador mcview, y que valen para las tres
piezas:

- **Cada número dice qué NO afirma.** Una similitud no es una identidad, una
  masa no es importancia ni corrección, y un ancla sin evidencia es una
  hipótesis, nunca una orden de borrado.
- **Los umbrales se miden, no se eligen.** El umbral actual de similitud (0.09)
  está calibrado para dar recall en la revisión manual. Si se usara al guardar,
  saltaría el aviso casi siempre y el agente aprendería a ignorarlo.
- **El silencio no debe parecerse al fallo.** Que una comprobación no se haya
  podido hacer tiene que distinguirse de que se hiciera y no encontrara nada.
- **Determinismo.** Con los mismos datos, la misma salida.

## Clarifications

### Session 2026-09-13

- Q: ¿La anomalía 207/209 (una memoria actualizada horas después de crearse, con título igual a otra) forma parte de esta feature? → A: No. Es un posible defecto del sistema que ya corre: se reproduce y se cierra directamente, fuera del SDD (regla de trabajo 1 y memoria 93). Esta feature solo depende de que se haya cerrado antes de calibrar el gate (ver Dependencias).
- Q: Si la comprobación de duplicados falla, ¿el aviso desaparece sin más (fail-open silencioso)? → A: No. El guardado nunca se bloquea, pero la respuesta dice que la comprobación no se hizo. Así se aplica el principio "el silencio no debe parecerse al fallo" que el propio plan cita. Esto cambia la hoja 1.2 del plan original, que devolvía un resultado vacío sin más.
- Q: La verificación del plan espera que la memoria 204 suba en el ranking de masa con la tarea "sinapsis cache". → A: La 204 es un checkpoint (verificado con get_memory 204), y los checkpoints quedan fuera de la masa. El criterio se reduce a 197, 200 y 202.
- Q: (validación del 2026-09-13) 197, 200 y 202 no entran en el top 10 porque las tres tienen 0 relaciones en el almacén real, y la masa no sube nodos aislados. ¿Qué pasa con SC-006? → A: La persona decidió enmendarlo a lo que la masa sí mide: las memorias enlazadas a los resultados ganan masa, y las aisladas conservan como mucho su cuota de semilla.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Aviso de posible duplicado al guardar (Priority: P1)

Cuando el agente (o la persona) guarda una memoria que se parece mucho a otra
ya guardada del mismo tipo, la respuesta del guardado le avisa, nombra la
memoria parecida y su similitud, y le indica cómo actualizarla en vez de
duplicarla. El guardado se hace igual: el aviso orienta, no bloquea.

**Why this priority**: Es el hueco con daño comprobado (207/209 duplicadas) y
evita que el problema crezca. Además es la pieza de menor riesgo: solo añade
texto a una respuesta y no altera lo que se guarda.

**Independent Test**: Se prueba sola guardando dos veces un contenido casi
igual: el segundo guardado avisa y nombra al primero. Un tema nuevo no avisa.

**Acceptance Scenarios**:

1. **Given** una memoria guardada de tipo "decisión", **When** se guarda otra del mismo tipo con título y contenido casi iguales, **Then** la memoria se guarda y la respuesta incluye "Posible duplicado de #N «título» (similitud x.xx)" con la indicación de repetir con clave de tema o revisar la memoria N.
2. **Given** memorias existentes, **When** se guarda una memoria sobre un tema claramente nuevo, **Then** la respuesta no incluye aviso de duplicado.
3. **Given** una memoria guardada con clave de tema, **When** se guarda otra con esa misma clave (actualización), **Then** no hay aviso, porque la clave de tema ya resuelve la identidad.
4. **Given** una memoria casi igual a otra pero de tipo distinto, **When** se guarda, **Then** no hay aviso.
5. **Given** la comprobación de duplicados no se puede realizar (p. ej., falla la lectura de memorias), **When** se guarda una memoria, **Then** la memoria se guarda y la respuesta indica que la comprobación de duplicados no se hizo, algo distinto de no mostrar aviso.
6. **Given** el guardado desde la línea de comandos, **When** se guarda un casi-duplicado, **Then** el aviso sale por el canal de diagnóstico y no mezcla la salida normal.

---

### User Story 2 - Anclas sin evidencia en el contexto del proyecto (Priority: P2)

Cuando el agente carga el contexto del proyecto, ve una sección que lista las
memorias ancladas a archivos que ya no están donde decían estar: indica si el
archivo parece haberse movido (y a dónde, si hay un único candidato) o si no
aparece en ningún lado. La sección advierte que es una hipótesis y que antes
de juzgar u olvidar la memoria hay que verificar.

**Why this priority**: Evita que el agente actúe con conocimiento anclado a
código que ya no existe. Solo lee y avisa; no modifica memorias.

**Independent Test**: Se prueba sola con un almacén que tiene una memoria
anclada a un archivo borrado y otra a un archivo existente. Solo la primera
aparece en la sección.

**Acceptance Scenarios**:

1. **Given** una memoria anclada a un archivo del proyecto que ya no existe y cuyo nombre no aparece en ningún otro sitio, **When** se carga el contexto, **Then** la memoria aparece en "Anclas sin evidencia" como huérfana candidata.
2. **Given** una memoria anclada a un archivo que ya no existe en su ruta, pero cuyo nombre de archivo aparece una sola vez en otra ruta del índice de código, **When** se carga el contexto, **Then** aparece como "movida" con la ruta candidata.
3. **Given** el mismo caso, pero con el nombre de archivo presente en varias rutas, **When** se carga el contexto, **Then** aparece como "movida (ambigua)" sin afirmar una ruta concreta.
4. **Given** una memoria anclada a un archivo existente, **When** se carga el contexto, **Then** no aparece en la sección.
5. **Given** una memoria anclada a una ruta absoluta fuera del proyecto (p. ej., la 148), **When** se carga el contexto, **Then** no aparece en la sección, porque no es verificable.
6. **Given** que no hay índice de código disponible, **When** se carga el contexto, **Then** la comprobación usa solo el disco: nunca afirma "movida" y no genera avisos falsos para anclas existentes.

---

### User Story 3 - Enlaces relevantes y ranking de masa (Priority: P3)

La sección de memorias enlazadas del contexto deja de mostrar los enlaces más
recientes y muestra los de mayor "masa": centralidad en el grafo de memorias,
sembrada en lo que importa ahora (la sesión activa y las memorias ancladas a
código activo). Los registros automáticos quedan fuera. Además, la persona
puede pedir el ranking de masa del proyecto, en general o sembrado en una
tarea, y cada salida dice qué NO afirma ese número.

**Why this priority**: Convierte los enlaces, que hoy no se usan, en una señal
útil. Es la pieza de mayor riesgo porque cambia el orden del contexto actual.

**Independent Test**: Se prueba sola con un almacén con enlaces entre memorias
normales y checkpoints: la sección no muestra checkpoints, ordena por masa, y
dos ejecuciones dan la misma salida.

**Acceptance Scenarios**:

1. **Given** enlaces entre memorias y checkpoints, **When** se carga el contexto, **Then** la sección de enlaces no incluye ningún enlace con un checkpoint en un extremo.
2. **Given** una sesión activa con memorias propias enlazadas, **When** se carga el contexto, **Then** los enlaces que tocan esas memorias o sus vecinas aparecen antes que otros enlaces más recientes pero ajenos.
3. **Given** que no hay sesión activa ni memorias ancladas a código activo, **When** se carga el contexto, **Then** la masa se siembra uniformemente y la sección sigue ordenada de forma determinista.
4. **Given** la petición del ranking de masa del proyecto, **When** se ejecuta, **Then** se muestra la lista de memorias por masa descendente (sin checkpoints) y la línea "masa = centralidad en el grafo de memorias sembrado en {semillas}; no mide importancia ni corrección".
5. **Given** la petición del ranking sembrado en una tarea (p. ej., "sinapsis cache"), **When** se ejecuta, **Then** las memorias relacionadas con esa tarea y sus vecinas suben respecto del ranking general.
6. **Given** la misma petición ejecutada dos veces sobre los mismos datos, **When** se comparan las salidas, **Then** son idénticas.

---

### User Story 4 - El paquete de contexto por tarea incluye vecinos con masa (Priority: P4)

Al pedir un paquete de contexto para una tarea, además de las memorias que la
búsqueda encuentra, el paquete incluye hasta 5 memorias vecinas de mayor masa,
marcadas como opcionales, para que el agente vea lo que está conectado aunque
no comparta palabras con la tarea.

**Why this priority**: Aprovecha la masa de US3 en el flujo por tarea. Depende
del cálculo de masa y es opcional: sin datos de enlaces, el paquete no cambia.

**Independent Test**: Se prueba construyendo un paquete para una tarea cuya
memoria encontrada tiene una vecina enlazada que la búsqueda no encuentra: la
vecina aparece como opcional. Sin enlaces, el paquete es idéntico al actual.

**Acceptance Scenarios**:

1. **Given** una tarea cuya búsqueda devuelve memorias con vecinas enlazadas, **When** se construye el paquete, **Then** incluye hasta 5 vecinas de mayor masa, marcadas como opcionales, que no estaban ya en el paquete.
2. **Given** que no hay datos de enlaces disponibles, **When** se construye el paquete, **Then** el resultado es idéntico al comportamiento actual.
3. **Given** un paquete con presupuesto de tamaño, **When** se añaden vecinas, **Then** el presupuesto se respeta: las opcionales son lo primero que se descarta.

---

### Edge Cases

- **Guardado que actualiza en vez de crear** (misma clave de tema o título exacto): la memoria resultante no puede aparecer como "posible duplicado" de sí misma.
- **Almacén vacío o sin memorias del mismo tipo**: no hay aviso ni error.
- **Textos de tamaño muy distinto** (una línea frente a un documento largo): no se comparan, porque su similitud no dice nada. Es el filtro de tamaño.
- **Checkpoints**: nunca disparan el aviso de duplicado, no entran en la masa y no aparecen en la sección de enlaces.
- **Ancla con ruta vacía, absoluta fuera del proyecto o que apunta a un directorio**: no es verificable y no se lista.
- **Índice de código desactualizado**: una ruta candidata a "movida" es una hipótesis; la leyenda lo dice.
- **Muchas anclas sin evidencia**: se listan como máximo 8, y la sección cede ante el presupuesto de tamaño del contexto.
- **Enlaces de veredicto** ("en conflicto", "no hay conflicto"): no transportan masa.
- **Enlace de sustitución** (A sustituye a B): la masa fluye de B hacia A, nunca de A hacia B.
- **Memoria sin enlaces salientes**: su masa vuelve a las semillas y no se pierde.
- **Enlaces a memorias olvidadas**: se ignoran sin error.
- **Orden de entrada distinto** (mismos datos en otro orden): la salida no cambia.

## Requirements *(mandatory)*

### Functional Requirements

**Gate de pre-escritura (US1)**

- **FR-001**: Antes de guardar una memoria, el sistema MUST compararla con las memorias existentes del mismo tipo y detectar las casi-duplicadas por similitud de título y de título+contenido.
- **FR-002**: El sistema MUST excluir de la comparación los checkpoints y las memorias de otro tipo, y MUST no avisar cuando el guardado actualiza una memoria existente (por clave de tema ya usada o por título exacto). Un guardado con una clave de tema NUEVA sí se compara, y si el candidato tiene clave de tema, el aviso sugiere reutilizarla.
- **FR-003**: El sistema MUST descartar los pares cuyo tamaño de texto difiera tanto que la similitud no pueda superar el umbral (filtro de tamaño).
- **FR-004**: Los umbrales de similitud del gate MUST ser propios, distintos del umbral de la revisión manual, y MUST fijarse con una medición sobre el almacén real del proyecto. La medición se documenta como tabla umbral × número de avisos × recall de pares duplicados conocidos.
- **FR-005**: El sistema MUST guardar siempre la memoria, haya aviso o no. El gate es consultivo.
- **FR-006**: Si hay casi-duplicados, la respuesta MUST mostrar hasta 3, ordenados por similitud descendente, cada uno con id, título y similitud, más la indicación de repetir con clave de tema o revisar esa memoria.
- **FR-007**: El sistema MUST no mostrar como casi-duplicado la memoria que resultó del propio guardado (caso de actualización).
- **FR-008**: Si la comprobación no puede realizarse, la respuesta MUST indicarlo de forma distinguible de "sin casi-duplicados", y el guardado MUST completarse igualmente.
- **FR-009**: El gate MUST aplicarse igual al guardado desde el protocolo de herramientas del agente y desde la línea de comandos. En la línea de comandos, el aviso va al canal de diagnóstico.

**Evidencia de anclas (US2)**

- **FR-010**: El sistema MUST clasificar cada ancla de memoria en uno de estos grados: **vigente** (el archivo existe), **movida** (falta, y su nombre de archivo aparece en una única ruta del índice de código; se da esa ruta), **movida ambigua** (falta, y el nombre aparece en varias rutas; no se afirma ninguna), **huérfana candidata** (falta y no aparece) y **no verificable** (ruta vacía o absoluta fuera del proyecto).
- **FR-011**: El contexto del proyecto MUST incluir una sección "Anclas sin evidencia" solo con las anclas movidas, movidas ambiguas y huérfanas candidatas, como máximo 8, con la leyenda "hipótesis, no orden de borrado: verifica y usa judge_memories/forget_memory".
- **FR-012**: La sección MUST respetar el presupuesto de tamaño del contexto y omitirse si no cabe o si no hay anclas que listar.
- **FR-013**: Sin índice de código disponible, la clasificación MUST basarse solo en el disco: nunca produce "movida" ni avisos para anclas existentes.
- **FR-014**: La clasificación MUST ser de solo lectura: no modifica, juzga ni olvida memorias.

**Masa de memorias (US3, US4)**

- **FR-015**: El sistema MUST calcular la masa de cada memoria como centralidad por recorrido aleatorio con reinicio a un conjunto de semillas ponderadas (PageRank personalizado), con resultado determinista para los mismos datos y total de masa igual a 1.
- **FR-016**: El grafo de masa MUST seguir estas reglas de aristas:

  | Enlace | En el grafo | Motivo |
  |--------|-------------|--------|
  | relacionada / compatible / de alcance | en ambos sentidos, con peso igual a la confianza del enlace | un enlace automático (confianza 0.5) es una cuota ambigua; uno juzgado (1.0) es inequívoco |
  | A sustituye a B | solo de B hacia A | la masa del sustituido fluye al vigente, nunca al revés |
  | en conflicto / no hay conflicto | excluido | es un veredicto, no una transición |
  | cualquier enlace con un checkpoint | excluido | registro automático que hoy acapara la sección |

- **FR-017**: La masa de las memorias sin enlaces salientes MUST devolverse a las semillas, sin perderse ni repartirse uniformemente.
- **FR-018**: La sección de memorias enlazadas del contexto MUST ordenarse por la suma de la masa de sus dos extremos, en lugar de por recencia, y MUST excluir los enlaces con checkpoints. Semillas: memorias de la sesión activa y memorias ancladas a código activo; si no hay ninguna, semillas uniformes.
- **FR-019**: La persona MUST poder consultar el ranking de masa del proyecto con un número máximo de resultados y, opcionalmente, sembrado en una tarea descrita en texto. La salida MUST terminar con la línea fija "masa = centralidad en el grafo de memorias sembrado en {semillas}; no mide importancia ni corrección".
- **FR-020**: Al construir un paquete de contexto por tarea con datos de enlaces disponibles, el sistema MUST sembrar la masa en las memorias encontradas por la búsqueda (peso decreciente según su posición) y añadir hasta 5 vecinas de mayor masa que no estén ya en el paquete, como contenido opcional.
- **FR-021**: Sin datos de enlaces, el paquete de contexto por tarea MUST ser idéntico al actual.

**Transversales**

- **FR-022**: Cada valor numérico o grado nuevo (similitud, grado de evidencia, masa) MUST mostrarse junto a lo que NO afirma, en la salida y en la documentación.
- **FR-023**: Las tres capacidades MUST poder entregarse y revertirse por separado.
- **FR-024**: La documentación de usuario MUST describir las tres capacidades, cada una con su bloque "qué NO afirma".

### Key Entities *(include if feature involves data)*

- **Aviso de posible duplicado**: resultado consultivo del guardado. Contiene la memoria parecida (id, título), la similitud y la sugerencia de acción. No es una fusión ni un bloqueo.
- **Grado de evidencia de ancla**: clasificación de la ruta asociada a una memoria frente al disco y al índice de código actuales (vigente, movida, movida ambigua, huérfana candidata, no verificable). Es derivado: no se persiste.
- **Grafo de masa**: nodos (memorias no checkpoint) y aristas dirigidas y ponderadas, derivadas de los enlaces según las reglas de FR-016.
- **Semillas**: conjunto de memorias con peso que define "lo que importa ahora" (sesión activa, anclas a código activo o resultados de una tarea). Se nombran en la salida.
- **Masa**: valor por memoria, entre 0 y 1, que suma 1 en el grafo. Mide centralidad relativa a las semillas, no importancia.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Al volver a guardar el título y el contenido de un duplicado conocido (par 207/209), la respuesta nombra la memoria duplicada en el 100 % de los intentos y la memoria queda guardada.
- **SC-002**: Aplicado a todo el histórico real de memorias no checkpoint, el gate habría avisado en como mucho el 10 % de los guardados, y detecta el 100 % de los pares duplicados confirmados. Si ambos objetivos no pueden cumplirse a la vez, la tabla de calibración muestra el compromiso y la persona elige el umbral.
- **SC-003**: Guardar una memoria de un tema nuevo no produce aviso en ninguno de los casos de prueba.
- **SC-004**: Toda ancla listada en "Anclas sin evidencia" sobre el almacén real corresponde a un archivo que de verdad no está en su ruta, comprobado a mano: 0 falsos positivos. Ninguna ruta absoluta fuera del proyecto aparece.
- **SC-005**: La sección de memorias enlazadas del contexto real contiene 0 enlaces con checkpoints (hoy la mayoría lo son).
- **SC-006** (enmendado el 2026-09-13 por decisión de la persona): En el ranking de masa sembrado en una tarea, toda memoria enlazada a los resultados de la búsqueda tiene más masa que en el ranking sin tarea, y una memoria sin enlaces nunca supera su cuota de semilla. En el almacén real, con la tarea "sinapsis cache", la 155 (enlazada a la 154, que es un resultado) entra en el top 10 aunque la búsqueda no la devuelve, y 197/200 (sin relaciones) quedan con su cuota.
- **SC-007**: Dos ejecuciones consecutivas del ranking de masa y del contexto sobre los mismos datos producen salidas idénticas (100 %).
- **SC-008**: El contexto del proyecto nunca supera su presupuesto de tamaño configurado tras añadir las secciones nuevas.
- **SC-009**: Sin datos de enlaces, el paquete de contexto por tarea es idéntico, byte a byte, al que se produce hoy.
- **SC-010**: Guardar una memoria no tarda perceptiblemente más que hoy en un almacén del tamaño actual del proyecto (cientos de memorias).

## Assumptions

- **Prerrequisito fuera del alcance**: la anomalía 207/209 se investiga y se cierra directamente, sin SDD (regla de trabajo 1 y memoria 93), antes de calibrar el gate. Fusionar u olvidar 207/209 es irreversible y requiere aprobación explícita de la persona.
- La calibración de umbrales lee el almacén real en solo lectura. El artefacto de medición no se versiona; el resultado se guarda como memoria de decisión.
- El almacén tiene del orden de cientos de memorias, así que una comparación exacta contra todas las del mismo tipo es asumible y no hace falta una aproximación probabilística.
- "Código activo" reutiliza la noción de hotspot que el contexto ya usa. No se define una nueva.
- El índice de código puede estar vacío o desactualizado; la evidencia de anclas funciona con lo que haya y lo declara.
- Los enlaces automáticos existentes y sus confianzas (0.5 automático, 1.0 juzgado) se usan tal cual; esta feature no cambia cómo se crean.
- El MCP en ejecución sigue sirviendo el binario anterior hasta reiniciarse. La validación contra el sistema real exige comprobar antes que el artefacto servido es el nuevo.
- Las memorias de prueba creadas durante la validación real se borran después, con aprobación de la persona.
- **Fuera de alcance** (decisión del plan): extracción AST, detección de módulos por clustering (ya la da el grafo de código externo), atlas o visualización del grafo, bloqueos o contratos, y aproximaciones probabilísticas de similitud. Del principio de similitud solo se toma el filtro de tamaño.
- Prioridad si hay que recortar: US1 → US2 → US3 → US4. Cada una se entrega sola.
