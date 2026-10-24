# Feature Specification: Base de memoria optimizada, idempotente y con índices

**Feature Branch**: `041-migracion-transaccional`

**Created**: 2026-10-23

**Status**: Draft

**Input**: User description: "genera la especificacion 1. ¿Hago el cambio de la migración en una sola transacción? Toca el código que crea el esquema de la base, así que prefiero tu visto bueno." Ampliaciones de la persona (2026-10-23): "optimizar al maximo la base con las mejores practicas", "me refiero a nuestra base original", "el SQL Lite .. que sea idempontente .. con index".

## Contexto y evidencia

La base original de gomemory es la memoria de cada proyecto, un archivo SQLite en el almacén global. Guarda memorias, sesiones, revisiones, instincts, compresión y también el grafo de código propio de gomemory. Esta feature la optimiza en tres frentes: cómo se prepara, cómo se abre y qué índices tiene.

**1. Preparación lenta y no atómica.** La primera apertura confirma unos 66 pasos sueltos (53 creaciones y 13 ampliaciones) y asegura en disco después de cada uno. La medición del 2026-10-23 en Windows CI (workflow `diag-windows-timing`, run 39941668694, 5 repeticiones por prueba) dio:

| Momento | Mediana | Peor caso | Plazo disponible |
|---|---|---|---|
| Preparar la memoria de un proyecto nuevo | 554 ms | 1.760 ms | — |
| Abrir una memoria ya preparada | 6 ms | 8 ms | — |
| Consolidar lo aprendido al cerrar el turno (incluye preparar) | 581 ms | 1.809 ms | 2.000 ms |
| Registrar una sugerencia o un bloqueo (incluye preparar) | 42 ms | 1.592 ms | 2.000 ms |

Los hooks globales actúan en cualquier proyecto. El primer hook en un proyecto nuevo paga la preparación dentro de un plazo pensado para no frenar al agente, y en el peor caso agota el 90 % de ese plazo. En Windows CI eso ya produce pérdidas intermitentes: lo aprendido no se consolida y la sugerencia no queda registrada. Además, si la preparación falla a mitad, la base queda con una parte de las estructuras creada.

**2. Configuración mínima.** Cada conexión solo fija la espera ante bloqueo y el modo de registro de escritura anticipada. No aplica el resto de prácticas recomendadas para SQLite en este modo: asegurar en disco solo en los puntos de integridad, caché de páginas, tablas temporales en memoria, lectura mapeada, límite al tamaño del registro y estadísticas de consulta al día.

**3. Índices incompletos.** Las consultas de sesión activa y de sesiones recientes, que se ejecutan en cada arranque de sesión y en cada turno, recorren la tabla de sesiones entera y además la ordenan en una estructura temporal. El plan de consulta del 2026-10-23 sobre una base nueva lo muestra: `SCAN sessions` y `USE TEMP B-TREE FOR ORDER BY`. El resto de las consultas revisadas ya usan un índice.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - El primer turno en un proyecto nuevo no pierde trabajo (Priority: P1)

Una persona con gomemory instalado de forma global abre su agente en un repositorio donde gomemory nunca trabajó. Durante ese primer turno, gomemory sugiere una skill, bloquea un comando destructivo u observa comandos repetidos. Al cerrar el turno, todo queda registrado y consolidado, igual que en un proyecto con memoria previa.

**Why this priority**: es la pérdida que ya se ve. El primer contacto con un proyecto es cuando la persona evalúa si gomemory funciona.

**Independent Test**: en un proyecto sin memoria previa, ejecutar el hook que dispara una sugerencia y luego el de cierre de turno. Comprobar que la sugerencia queda registrada y que lo observado queda consolidado, sin reintentos.

**Acceptance Scenarios**:

1. **Given** un proyecto sin memoria previa en Windows, **When** el primer hook registra una sugerencia, **Then** la sugerencia aparece en el registro de eventos del proyecto.
2. **Given** un proyecto sin memoria previa con 16 observaciones de un mismo comando, **When** se cierra el primer turno, **Then** el comando aparece consolidado con sus 16 observaciones.
3. **Given** las pruebas del ciclo de expand repetidas 20 veces seguidas en Windows CI, **When** terminan, **Then** ninguna repetición pierde registros ni consolidaciones.

---

### User Story 2 - La preparación es idempotente y de todo o nada (Priority: P1)

Quien mantiene gomemory necesita que preparar la base se pueda repetir cualquier número de veces con el mismo resultado. También necesita que, si se interrumpe o falla, no deje la base a medias.

**Why this priority**: la preparación corre en cada apertura de cualquier versión. Si no es idempotente y atómica, una interrupción o un error deja bases en un estado que ninguna versión espera.

**Independent Test**: preparar la misma base varias veces seguidas y comparar su estructura, que debe ser idéntica. Provocar un fallo a mitad de la preparación y comprobar que la base no gana ninguna estructura.

**Acceptance Scenarios**:

1. **Given** una base ya preparada, **When** se vuelve a preparar 3 veces, **Then** su estructura y sus datos son idénticos a los de después de la primera vez.
2. **Given** una base donde un paso de la preparación falla a mitad, **When** termina la preparación, **Then** la base no contiene ninguna de las estructuras de esa preparación y la apertura informa del error.
3. **Given** dos procesos que preparan a la vez la misma base nueva, **When** ambos terminan, **Then** los dos la ven completa y ninguno falla.

---

### User Story 3 - Las memorias existentes se abren igual que antes (Priority: P1)

Una persona con proyectos que llevan meses de memoria actualiza gomemory. Sus memorias se abren sin pasos manuales, conservan su contenido y ganan las estructuras e índices nuevos.

**Why this priority**: el esquema solo se amplía de forma aditiva (spec 009, HU4). Optimizar la base no puede romper memorias creadas por versiones anteriores.

**Independent Test**: abrir con la versión nueva una memoria creada por una versión anterior. Comprobar que gana las columnas e índices nuevos y que sus datos siguen intactos.

**Acceptance Scenarios**:

1. **Given** una memoria creada por una versión anterior, **When** la versión nueva la abre, **Then** gana las columnas e índices nuevos y ningún dato previo cambia.
2. **Given** una memoria nueva preparada con la versión nueva, **When** se compara con una preparada por la versión anterior, **Then** ambas tienen las mismas tablas, columnas y búsquedas de texto, y la nueva tiene además los índices de FR-015.

---

### User Story 4 - Cada turno consulta y escribe con el menor coste posible (Priority: P2)

Quien mantiene gomemory quiere que la base se abra con las mejores prácticas de SQLite y tenga los índices que piden sus consultas frecuentes. Así ningún turno recorre tablas enteras ni paga sincronizaciones a disco evitables, aunque la memoria crezca.

**Why this priority**: es la optimización de fondo que pide la persona. Reduce el coste de cada turno, no solo el del primero.

**Independent Test**: consultar la configuración efectiva de cualquier conexión y el plan de las consultas frecuentes. Comprobar que la configuración coincide con FR-011 y que ninguna consulta frecuente recorre una tabla entera.

**Acceptance Scenarios**:

1. **Given** cualquier conexión a la base, **When** se consulta su configuración, **Then** coincide con FR-011.
2. **Given** las consultas frecuentes de sesión, memoria, grafo, revisión y eventos, **When** se obtiene su plan de consulta, **Then** ninguna recorre una tabla entera.
3. **Given** el servidor MCP o un comando de la CLI que escribió en la base, **When** cierra de forma ordenada, **Then** las estadísticas de consulta quedan al día con un coste acotado (FR-013).
4. **Given** un proceso que termina de forma abrupta tras confirmar una escritura, **When** se vuelve a abrir la base, **Then** la escritura está y la base es íntegra.
5. **Given** una base ya preparada y otra conexión con una escritura abierta, **When** un hook abre la base, **Then** la apertura termina sin esperar a que esa escritura acabe (FR-018).

---

### Edge Cases

- **Dos procesos preparan a la vez la misma base nueva** (hooks duplicados del primer arranque): uno la prepara y el otro espera y la encuentra lista. Ninguno falla (feature 035).
- **La preparación se interrumpe a mitad** (proceso terminado por su plazo, corte de energía): la base queda completa o como si no se hubiera empezado, y la siguiente apertura la prepara entera.
- **Una columna o un índice ya existen** porque los creó una versión anterior: el paso no hace nada y no aborta el resto.
- **Un índice nuevo no se puede crear sobre una base antigua** (por ejemplo, un índice único sobre datos duplicados): no aplica a esta feature, porque los índices nuevos no son únicos. Si en el futuro lo fueran, el plan define cómo se trata.
- **Corte de energía justo después de confirmar una escritura**: con el aseguramiento reservado a los puntos de integridad, puede perderse la última escritura confirmada, pero la base nunca queda corrupta.
- **El sistema de archivos no admite lectura mapeada**: la base funciona igual sin ella. El ajuste es una mejora, no un requisito.
- **El almacén no admite escritura** (disco lleno o de solo lectura): la apertura falla con un error claro y no deja una base parcial. Lo garantiza la preparación de todo o nada (FR-001). No tiene prueba automática propia, porque simular un disco lleno de forma portable no es fiable.
- **Una base antigua con datos que impiden crear un índice único existente** (relaciones duplicadas frente a `idx_relations_pair`): el índice no se crea y el resto de la preparación se confirma (FR-006). Cambio aceptado respecto de hoy: como la base queda marcada como al día, ese paso opcional ya no se reintenta en cada apertura, sino la próxima vez que suba la versión de esquema. Antes se reintentaba siempre y fallaba siempre, sin ganar nada.
- **La plataforma no admite alguna estructura de búsqueda de texto**: el resto de la preparación se completa, como hoy.

## Requirements *(mandatory)*

### Functional Requirements

**Preparación idempotente y atómica**

- **FR-001**: La preparación de la base MUST confirmarse como una única unidad: o queda completa o no queda ningún cambio.
- **FR-002**: Preparar una base nueva MUST requerir un único aseguramiento en disco, en lugar de uno por estructura. Se acepta verificarlo de forma indirecta, por el tiempo medido en Windows (SC-002): contar sincronizaciones exigiría instrumentar el sistema de archivos.
- **FR-003**: Preparar una base MUST ser idempotente: repetirla sobre una base ya preparada no cambia su estructura ni sus datos.
- **FR-004**: Abrir una base creada por cualquier versión anterior MUST añadir las estructuras e índices que falten sin modificar ni borrar datos. El esquema sigue siendo solo aditivo.
- **FR-005**: Que falle añadir una columna que ya existe MUST NOT impedir el resto de la preparación.
- **FR-006**: Que falle crear una estructura opcional (las búsquedas de texto y los índices auxiliares que hoy se toleran) MUST NOT impedir el resto de la preparación ni dejarla a medias.
- **FR-007**: Dos procesos que preparan a la vez la misma base nueva MUST terminar ambos con la base completa, con la espera y los reintentos que ya existen.
- **FR-018**: Abrir una base que ya está al día MUST NOT pedir el bloqueo de escritura ni esperar a otros escritores. La base guarda la versión de esquema que la preparó. Si es igual o mayor que la de la versión en curso (una base ya abierta por un binario más nuevo), la preparación se omite y la versión guardada nunca baja. Si es menor, se prepara y la versión se actualiza dentro de la misma unidad.
- **FR-019**: Cualquier cambio en la estructura que produce la preparación MUST ir acompañado de un cambio de la versión de esquema. Una prueba MUST impedir que la estructura cambie sin que cambie esa versión.
- **FR-008**: Salvo los índices de FR-015, la estructura resultante MUST ser idéntica a la que produce la versión anterior. Se verifica comparando el estado resultante, no solo que la preparación no dé error.

**Configuración de conexión (mejores prácticas de SQLite)**

- **FR-011**: Toda conexión a la base MUST abrirse con:
  - la espera ante bloqueo antes que cualquier otro ajuste;
  - registro de escritura anticipada;
  - aseguramiento en disco solo en los puntos de integridad;
  - caché de páginas acotada;
  - tablas temporales en memoria;
  - lectura mapeada acotada;
  - límite al tamaño del registro tras cada punto de control.
- **FR-012**: La misma configuración MUST aplicarse a todas las aperturas de bases propias de gomemory: la base de memoria (también cuando se migra desde su ubicación legada) y los instincts globales. Cada una conserva su tiempo de espera ante bloqueo, que es deliberado.
- **FR-013**: Al cerrar de forma ordenada la base desde el servidor MCP o un comando de la CLI, gomemory MUST actualizar de forma incremental las estadísticas de consulta, con un tope de trabajo acotado: el cierre no puede crecer más de 100 ms en el peor caso en Windows CI. Los hooks de vida corta terminan sin ese paso y quedan fuera de este requisito.
- **FR-014**: La comprobación de integridad referencial entre tablas MUST NOT activarse en esta feature. Cambia el comportamiento ante datos huérfanos existentes y se trata aparte.

**Índices**

- **FR-015**: La base MUST tener índices que sostengan las consultas de sesión activa y de sesiones recientes por proyecto, sin recorrer la tabla ni ordenar en una estructura temporal.
- **FR-016**: Una prueba MUST verificar el plan de una lista vigilada de consultas frecuentes de la base (sesión, memorias, grafo, revisiones, eventos e instincts) y fallar si alguna recorre una tabla entera. Añadir a esa lista cada consulta nueva que se ejecute en cada turno, sesión o hook es una regla de revisión documentada en `docs/DATABASE.md`; no la hace cumplir ninguna prueba.
- **FR-017**: Todo índice nuevo MUST crearse de forma idempotente, igual que el resto del esquema, y MUST NOT ser único, para que nunca falle sobre los datos de una base existente.

**Verificación y límites**

- **FR-009**: El resultado MUST medirse con la misma instrumentación por fases y el mismo workflow de diagnóstico en Windows que establecieron la línea base, y MUST anotarse junto a ella.
- **FR-010**: Los plazos de los hooks (150 ms para comprimir salidas, 2 s para consolidar y registrar) MUST NOT cambiar como parte de esta feature.

### Key Entities

- **Base de memoria del proyecto**: el archivo SQLite de cada proyecto en el almacén global. Contiene las memorias, las sesiones, las revisiones, los instincts, la compresión y el grafo de código propio de gomemory. Se prepara en la primera apertura y se amplía de forma aditiva en las siguientes.
- **Preparación** (en el código, la migración, `migrate`): la operación idempotente que deja la base con todas las estructuras e índices de la versión en curso. Con esta feature se confirma como una sola unidad.
- **Configuración de conexión**: los ajustes con los que se abre cada conexión a una base de gomemory.
- **Consulta frecuente**: una consulta que se ejecuta en cada turno, sesión o hook, y que debe estar sostenida por un índice.
- **Línea base**: la medición del 2026-10-23 por fases en Windows CI y los planes de consulta de una base nueva, contra los que se comparan los resultados.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En 20 ejecuciones seguidas en Windows CI de las pruebas que cubren el primer turno de un proyecto nuevo, el 100 % registran la sugerencia y consolidan lo observado.
- **SC-002**: El peor caso de preparar una base nueva en Windows CI baja de 1.760 ms a menos de 400 ms, es decir, a menos de una quinta parte del plazo de consolidación.
- **SC-003**: La consolidación al cerrar el primer turno de un proyecto nuevo usa, en el peor caso, menos del 25 % de su plazo. Hoy usa hasta el 90 %.
- **SC-004**: Abrir una base ya preparada sigue costando una mediana de 10 ms o menos en Windows CI (línea base: 6 ms).
- **SC-005**: El 100 % de las bases de versiones anteriores que usan las pruebas de compatibilidad se abren sin pérdida de datos y con las estructuras e índices nuevos.
- **SC-006**: La suite completa pasa en Linux, macOS y Windows sin cambiar ningún plazo de los hooks.
- **SC-007**: El 100 % de las conexiones a cada base propia de gomemory reportan la configuración de FR-011, con la excepción documentada del tiempo de espera de cada base.
- **SC-008**: Ninguna consulta frecuente de la base recorre una tabla entera. Hoy la de sesiones sí lo hace.
- **SC-009**: Preparar 3 veces seguidas la misma base deja una estructura idéntica byte a byte en su catálogo.
- **SC-010**: Con otra conexión manteniendo una escritura abierta, abrir una base ya preparada termina en menos de 50 ms, sin esperar a que esa escritura acabe.

## Assumptions

- La causa dominante del coste de la primera apertura es el número de confirmaciones en disco, no el volumen de estructuras. Lo respalda la diferencia entre 554 ms (base nueva) y 6 ms (base preparada) con el mismo proceso.
- La base admite crear y ampliar estructuras dentro de una única unidad confirmable, y el fallo de un paso tolerado no invalida la unidad. Se verificó con un prototipo contra el driver real, R2 y R3 de research.md.
- Se acepta la contrapartida de durabilidad estándar del modo de registro de escritura anticipada con aseguramiento reservado: ante un corte de energía puede perderse la última escritura confirmada, nunca la integridad. Si solo se cae el proceso, no se pierde nada.
- Los valores de la caché de páginas, la lectura mapeada y el límite del registro se fijan en el plan, acotados para procesos de vida corta como los hooks.
- La revisión de índices parte de los planes de consulta sobre una base nueva. Solo las consultas de sesiones recorrían la tabla entera; las demás revisadas ya usan un índice. La prueba de FR-016 extiende la vigilancia al resto.
- Queda fuera de alcance cambiar los plazos de los hooks, crear la base por adelantado en `mem install`, activar la integridad referencial y modificar las pruebas para que partan de una base preparada.
- Queda fuera de alcance el fallo intermitente de `TestHookToolOutput`: su trabajo dentro del plazo (como máximo 71 ms de 150) no incluye preparar la base, así que no lo explica esta causa.
- La instrumentación de la rama `diag/windows-timing` se reutiliza para medir y no se integra en `main`.
