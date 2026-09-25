# Feature Specification: Motor nativo de compresión de contexto (prácticas de Headroom)

**Feature Branch**: `033-native-context-compression`

**Created**: 2026-09-25

**Status**: Draft

**Input**: User description: "como podemos incluir un headroom a gomemory para ganar optimización extra: https://github.com/headroomlabs-ai/headroom". Aclaración de la persona: «quiero algo más amplio aplicando la mejor práctica de headroom y así optimizar al máximo que se pueda y que debe ser agnóstico, no actor. Que el propio motor de gomemory realice compresión». Adjuntó el diagrama de Headroom: CacheAligner → ContentRouter (SmartCrusher JSON, CodeCompressor AST, Kompress texto) → CCR, más memoria entre agentes, `learn` y MCP.

## Contexto del problema *(no normativo — fundamenta el porqué)*

gomemory ya reduce lo que entrega al agente. Arma paquetes de contexto con
presupuesto de tokens y los pasa por una compresión **estructural**
determinista, que solo colapsa espacios y párrafos repetidos. En contenido
denso (JSON, trazas, logs, listados de código, resultados de búsqueda) esa
compresión apenas ahorra.

Headroom (Apache 2.0) demuestra que una capa de compresión local, consciente
del tipo de contenido y reversible, ahorra del 20 al 60 % en tareas de agente
y del 60 al 95 % en JSON repetitivo, sin degradación medible. Pero Headroom es
un **actor externo**: un proceso con otro runtime que se interpone entre el
agente y el proveedor como proxy o envoltorio.

La persona no quiere ese actor. Quiere que **el propio motor de gomemory**
aplique las mismas buenas prácticas, sin procesos intermedios y sin depender
del agente. El diagrama de Headroom se traduce así a gomemory:

| Práctica de Headroom | Equivalente nativo en gomemory |
|---|---|
| CacheAligner: no romper el prefijo cacheado del proveedor | Salida estable byte a byte y ordenada de lo fijo a lo volátil |
| Live-zone: comprimir solo lo nuevo | No reenviar en la sesión lo que ya se entregó; enviar solo el delta |
| ContentRouter | Detección del tipo de contenido y elección del compresor |
| SmartCrusher (JSON) | Compresor de datos estructurados por estadística de campos |
| CodeCompressor (AST) | Compresor de código que conserva firmas y declaraciones |
| Kompress (modelo de texto) | Compresor de prosa y logs determinista, sin modelo de IA |
| CCR: compresión reversible | Marcadores de omisión con recuperación del original desde gomemory |
| min_input_words | Umbral mínimo: lo pequeño pasa intacto |
| Cross-agent memory | Ya existe: gomemory es la memoria única entre agentes |
| `headroom learn` | Ajuste adaptativo: lo que el agente recupera a menudo se comprime menos |
| Output shaper | Directiva de concisión opcional en el protocolo inyectado |

Tres restricciones propias de gomemory marcan el diseño:

1. **Binario único y autónomo.** No se añade ningún runtime ni modelo de IA
   externo. Todo es determinista y local.
2. **Agnóstico.** El motor no sabe qué agente lo usa. El ahorro llega igual a
   cualquier cliente, y cada runtime solo aporta la forma de enchufarse.
3. **Local y privado.** Nada sale de la máquina y el contenido privado nunca se
   comprime ni se guarda.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Contexto de gomemory comprimido al máximo y reversible (Priority: P1)

Cada vez que gomemory entrega contenido al agente (contexto del proyecto,
paquetes de contexto, resultados de búsqueda de memorias o de código, paquetes
delegados a subagentes), el motor detecta el tipo de cada bloque y le aplica el
compresor adecuado. Donde omite algo, deja una marca breve con una referencia.
Si el agente necesita el original, lo pide a gomemory con esa referencia y lo
recibe íntegro.

**Why this priority**: es el núcleo de la optimización y concentra la mayor
parte del ahorro. Todo lo demás se apoya en este motor.

**Independent Test**: con un corpus de memorias y resultados reales (JSON,
trazas, código y prosa), pedir el mismo contexto con el motor nativo y con la
compresión estructural, comparar tokens y recuperar cada omisión por su
referencia hasta reconstruir el original.

**Acceptance Scenarios**:

1. **Given** un bloque JSON con una lista de 200 registros casi iguales y 3 con error, **When** el motor lo comprime, **Then** conserva los 3 registros con error, los valores atípicos, el primero y el último, resume el resto con su recuento y deja una referencia al original.
2. **Given** un bloque de código fuente extenso, **When** el motor lo comprime, **Then** conserva importaciones relevantes, firmas, declaraciones de tipos y comentarios de documentación, y sustituye los cuerpos omitidos por marcas recuperables.
3. **Given** un log o traza con miles de líneas repetitivas, **When** el motor lo comprime, **Then** colapsa las repeticiones con su recuento y conserva íntegras las líneas de error, las advertencias y la cabecera de cada traza de pila.
4. **Given** una marca de omisión en el contexto entregado, **When** el agente pide el original con su referencia, **Then** recibe el contenido exacto, byte a byte.
5. **Given** un bloque por debajo del umbral mínimo, **When** pasa por el motor, **Then** sale sin ningún cambio.
6. **Given** cualquier bloque con código, rutas, URLs, identificadores, números de versión o mensajes de error, **When** el motor lo comprime, **Then** lo que queda visible de esos elementos aparece literal, nunca parafraseado.

---

### User Story 2 - No pagar dos veces lo mismo en una sesión (Priority: P1)

Durante una sesión larga, gomemory recuerda qué contenido ya entregó. Cuando
vuelve a inyectar contexto, el prefijo ya enviado sale idéntico byte a byte,
lo que permite que el proveedor lo reutilice de su caché. Lo que el agente ya
tiene no se repite: solo viaja lo nuevo o lo que cambió.

**Why this priority**: en sesiones largas, el contenido repetido y la pérdida
de la caché del proveedor cuestan tanto como el contenido sin comprimir, y
esta parte no pierde información.

**Independent Test**: simular varios turnos de una sesión, medir cuánto
contenido se repite entre inyecciones y comprobar que el prefijo estable es
idéntico entre turnos.

**Acceptance Scenarios**:

1. **Given** dos inyecciones de contexto seguidas en la misma sesión sin memorias nuevas, **When** se comparan, **Then** la parte estable es idéntica byte a byte y la segunda no repite contenido ya entregado.
2. **Given** una memoria nueva guardada entre dos turnos, **When** se inyecta contexto, **Then** solo viaja esa memoria como delta, y el prefijo estable no cambia.
3. **Given** que el agente compacta su conversación, **When** gomemory vuelve a inyectar contexto, **Then** trata la sesión como si no se hubiera entregado nada, porque el agente perdió lo anterior, y reenvía lo necesario.
4. **Given** contenido volátil (fechas, actividad reciente, estado de sesión), **When** se arma la salida, **Then** va al final y separado, para que no invalide el prefijo estable.

---

### User Story 3 - Comprimir cualquier salida de herramienta, desde cualquier agente (Priority: P2)

Cualquier agente, script o gancho puede pasar a gomemory un texto arbitrario
(la salida de una herramienta, un log, un resultado de búsqueda, un archivo) y
recibirlo comprimido con las mismas reglas y la misma reversibilidad. En los
runtimes que permiten reescribir la salida de una herramienta antes de que la
vea el modelo, gomemory se engancha ahí para que la compresión ocurra sola.

**Why this priority**: la salida de las herramientas es donde más tokens se
gastan en un agente. Llevar el motor ahí amplía el ahorro más allá del
contenido de gomemory, sin intermediarios.

**Independent Test**: pasar al punto de entrada genérico la salida real de un
comando voluminoso, comparar tokens y recuperar el original; en un runtime que
admita reescritura de salidas, comprobar que la salida llega comprimida al
modelo sin intervención de la persona.

**Acceptance Scenarios**:

1. **Given** una salida voluminosa, **When** se entrega al punto de entrada genérico de compresión, **Then** vuelve comprimida, con referencias recuperables y el recuento de tokens antes y después.
2. **Given** un runtime que permite reescribir salidas de herramientas y la compresión automática activada, **When** una herramienta devuelve una salida por encima del umbral, **Then** el modelo recibe la versión comprimida con sus referencias.
3. **Given** un runtime que no permite reescribir salidas, **When** se instala gomemory, **Then** el resto de la feature funciona igual y el diagnóstico informa de que ese enganche no aplica.
4. **Given** una salida que contiene contenido marcado como privado o credenciales detectadas, **When** pasa por el motor, **Then** esa parte no se guarda como original recuperable.

---

### User Story 4 - Medir el ahorro y ajustarlo solo (Priority: P2)

La persona ve cuánto ahorra cada compresor sobre su propio trabajo: tokens
antes y después, número de usos, veces que el agente tuvo que recuperar un
original y tiempo añadido. Si el agente recupera a menudo lo que un compresor
omite, gomemory reduce la agresividad de ese compresor para ese tipo de
contenido y ese proyecto.

**Why this priority**: sin medición propia, "optimizar al máximo" es una
suposición. La tasa de recuperación es la señal objetiva de que se comprimió
de más.

**Independent Test**: tras varias sesiones, consultar el informe y verificar
las cifras por compresor; forzar recuperaciones repetidas de un tipo de
contenido y comprobar que su compresión se suaviza.

**Acceptance Scenarios**:

1. **Given** un historial de uso, **When** la persona consulta las estadísticas, **Then** ve por compresor los tokens ahorrados, los usos, la tasa de recuperación y la latencia media.
2. **Given** un compresor cuya tasa de recuperación supera el umbral de ajuste en un proyecto, **When** vuelve a actuar, **Then** conserva más contenido que antes y el informe muestra el cambio.
3. **Given** una persona que quiere comparar, **When** pide una comparación puntual sobre un texto, **Then** ve lado a lado la salida sin comprimir, la estructural y la del motor nativo.

---

### User Story 5 - Respuestas más breves del agente (Priority: P3)

Con la opción activada, el protocolo que gomemory inyecta pide al agente
respuestas concisas, colocadas donde no rompen el prefijo estable.

**Why this priority**: Headroom reduce también los tokens de salida, pero el
efecto depende del agente y es el menos medible. Por eso es opcional y va al
final.

**Independent Test**: activar la opción y comprobar que la directiva aparece
en la zona volátil del contexto inyectado sin alterar el prefijo estable.

**Acceptance Scenarios**:

1. **Given** la opción activada, **When** se inyecta el protocolo, **Then** incluye la directiva de concisión en la zona volátil.
2. **Given** la opción desactivada (por defecto), **When** se inyecta el protocolo, **Then** no la incluye.

---

### Edge Cases

- **Error interno de un compresor**: ese bloque se entrega con la compresión estructural y el error se registra; nunca se corta la entrega.
- **El resultado comprimido es más largo que el de la compresión estructural**: se descarta y se usa el estructural.
- **Contenido mixto** (texto con JSON y código incrustados): cada fragmento se trata con su compresor, y la prosa que los rodea sale entera o con el compresor de prosa.
- **JSON inválido o truncado**: se trata como texto; nunca se entrega JSON inventado.
- **Código en un lenguaje no reconocido**: se usa la compresión genérica de código (sangría y bloques), y si no hay confianza, la estructural.
- **Referencia de recuperación caducada o desconocida**: la respuesta lo dice claramente y sugiere volver a pedir el contenido original a su fuente.
- **Original muy grande**: se guarda con deduplicación por huella; dos contenidos idénticos comparten un único original.
- **Varias sesiones o agentes en paralelo**: el registro de lo entregado es por sesión; las referencias de recuperación son válidas desde cualquier sesión del mismo proyecto.
- **Compactación del agente**: invalida el registro de lo entregado en esa sesión (User Story 2, escenario 3).
- **Almacenamiento de originales lleno o sin espacio**: se deja de comprimir con pérdida (se entrega solo la compresión estructural) hasta que haya espacio, y el diagnóstico lo avisa.

## Requirements *(mandatory)*

### Functional Requirements

**Motor y enrutado**

- **FR-001**: gomemory DEBE incluir un motor de compresión propio que se ejecute dentro de gomemory, sin procesos, servicios, runtimes ni modelos de IA externos.
- **FR-002**: El motor DEBE clasificar cada bloque de contenido al menos como: datos estructurados (JSON), código fuente, log o traza, diff, tabla o listado, prosa, o mixto; y DEBE aplicar a cada bloque el compresor de su tipo.
- **FR-003**: El motor DEBE ser determinista: la misma entrada con la misma configuración produce siempre la misma salida, byte a byte.
- **FR-004**: Los bloques por debajo de un umbral mínimo configurable DEBEN salir sin cambios.
- **FR-005**: El motor DEBE usar el resultado de un compresor solo si tiene menos tokens que la compresión estructural del mismo bloque; si no, DEBE usar la estructural.
- **FR-006**: Ante un error de cualquier compresor, el motor DEBE entregar ese bloque con la compresión estructural, sin propagar el error al agente.

**Compresores por tipo**

- **FR-007**: El compresor de datos estructurados DEBE conservar siempre los elementos con error o estado anómalo, los valores estadísticamente atípicos, el primer y el último elemento de cada lista, y la estructura (claves y tipos); y DEBE resumir los elementos redundantes con su recuento. La selección DEBE basarse en la variación estadística de los campos, no en listas fijas de palabras clave.
- **FR-008**: El compresor de código DEBE conservar importaciones usadas, firmas de funciones y métodos, declaraciones de tipos, constantes y comentarios de documentación, y PUEDE omitir cuerpos con marcas recuperables. DEBE reconocer al menos los lenguajes de los stacks de la constitución (Go, Python, Java, TypeScript/JavaScript, SQL) y aplicar una compresión genérica por bloques al resto.
- **FR-009**: El compresor de logs y trazas DEBE colapsar líneas repetidas o casi repetidas con su recuento y conservar siempre las líneas de error, las advertencias y la cabecera de cada traza de pila.
- **FR-010**: El compresor de prosa DEBE ser extractivo: puede eliminar frases duplicadas o redundantes con contenido ya entregado, pero nunca reescribe ni parafrasea.
- **FR-011**: Ningún compresor DEBE alterar el texto que conserva. Código, rutas, URLs, identificadores, números, versiones y mensajes de error visibles DEBEN aparecer literales. Una violación detectada DEBE descartar el resultado del compresor.

**Reversibilidad**

- **FR-012**: Toda omisión con pérdida DEBE dejar en su lugar una marca breve con una referencia estable y el recuento de lo omitido.
- **FR-013**: gomemory DEBE guardar el original de cada contenido comprimido con pérdida, deduplicado por huella, y DEBE devolverlo íntegro cuando se pida por su referencia, tanto a los agentes como desde la línea de comandos.
- **FR-014**: Los originales guardados DEBEN caducar tras un plazo configurable y DEBEN respetar un tope de espacio configurable. Al alcanzar el tope, se liberan primero los más antiguos.
- **FR-015**: Las memorias guardadas NUNCA se alteran por la compresión. La compresión afecta solo a lo que se entrega.

**Estabilidad de caché y deltas de sesión**

- **FR-016**: Todo contexto que gomemory inyecta DEBE ordenarse de lo más estable a lo más volátil, con el contenido volátil (fechas, actividad reciente, estado de sesión, directivas opcionales) agrupado al final.
- **FR-017**: Entre dos inyecciones de una misma sesión sin cambios en la parte estable, esa parte DEBE ser idéntica byte a byte.
- **FR-018**: gomemory DEBE registrar por sesión qué contenido entregó y NO DEBE reenviar en esa sesión contenido ya entregado e inalterado; en su lugar PUEDE citar su referencia.
- **FR-019**: Cuando el agente compacte su conversación, gomemory DEBE olvidar lo entregado en esa sesión y volver a entregar el contexto necesario.

**Puntos de entrada agnósticos**

- **FR-020**: El motor DEBE aplicarse a todo lo que gomemory entrega: contexto del proyecto, paquetes de contexto, resultados de búsqueda de memorias y de código, contexto posterior a la compactación y paquetes delegados a subagentes.
- **FR-021**: gomemory DEBE ofrecer un punto de entrada genérico, disponible desde la línea de comandos y desde las herramientas del agente, que reciba cualquier texto y lo devuelva comprimido con referencias, más un punto equivalente para recuperar originales.
- **FR-022**: En los runtimes soportados cuyo sistema de ganchos permita reescribir la salida de una herramienta antes de que la vea el modelo, gomemory DEBE ofrecer ese enganche, activable y desactivable y **apagado por defecto**, porque reescribe lo que el modelo ve de herramientas ajenas a gomemory. En los demás, la feature funciona igual sin él. *(Aclarado en el plan, research R11.)*
- **FR-023**: El motor NO DEBE depender de qué agente lo invoca. Toda la lógica de compresión es común, y cada runtime solo aporta el enganche.

**Privacidad y seguridad**

- **FR-024**: El contenido marcado como privado y las credenciales detectadas NO DEBEN guardarse como originales recuperables ni aparecer en estadísticas o registros.
- **FR-025**: Todo el proceso DEBE ocurrir en la máquina local; la feature no DEBE enviar datos a ningún servicio.

**Medición y ajuste adaptativo**

- **FR-026**: gomemory DEBE registrar por proyecto y por compresor: usos, tokens antes y después, recuperaciones de originales y latencia. Las estadísticas DEBEN poder consultarse desde la línea de comandos y desde las herramientas del agente.
- **FR-027**: Si la tasa de recuperación de un compresor supera un umbral configurable en un proyecto, gomemory DEBE reducir su agresividad para ese tipo de contenido en ese proyecto y registrar el ajuste.
- **FR-028**: gomemory DEBE ofrecer una comparación puntual entre la salida sin comprimir, la estructural y la del motor nativo sobre un texto dado.
- **FR-029**: El diagnóstico de gomemory DEBE informar del estado del motor: nivel activo, espacio usado por los originales, enganches disponibles por runtime y ajustes adaptativos vigentes.

**Configuración**

- **FR-030**: La compresión DEBE tener niveles configurables en la configuración de gomemory: *ninguna*, *estructural* (comportamiento actual) y *máxima* (motor nativo completo). En la configuración se guardan como `none`, `structural` y `max`.
- **FR-031**: La estabilidad de caché, los deltas de sesión y el umbral mínimo no pierden información, así que DEBEN estar siempre activos en el nivel *máxima*, que es el de las instalaciones nuevas. En *estructural* NO DEBEN aplicarse, para que la salida siga idéntica a la versión anterior (SC-008). *(Aclarado en el plan, research R13.)*
- **FR-032**: La directiva de concisión del agente DEBE estar desactivada por defecto.

### Key Entities

- **Bloque de contenido**: fragmento a comprimir, con su tipo detectado, tamaño y procedencia (contexto, paquete, búsqueda o salida de herramienta).
- **Resultado de compresión**: contenido entregado, compresor aplicado, tokens antes y después, referencias generadas y motivo de degradación si la hubo.
- **Original recuperable**: contenido íntegro identificado por su huella, con referencia, fecha de caducidad, proyecto y tamaño.
- **Registro de entrega de sesión**: qué contenido (por huella) recibió ya el agente en una sesión; se reinicia al compactar.
- **Estadísticas de compresión**: acumulado por proyecto y compresor de usos, tokens ahorrados, recuperaciones y latencia.
- **Ajuste adaptativo**: agresividad efectiva de un compresor para un tipo de contenido en un proyecto, con la señal que lo motivó.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En un corpus de referencia con contenido técnico real (JSON, logs, código y prosa), el nivel *máximo* entrega al menos un 40 % menos de tokens que el nivel *estructural*.
- **SC-002**: En contenido JSON repetitivo del corpus, el ahorro frente al contenido sin comprimir es de al menos un 70 %.
- **SC-003**: En una sesión simulada de 20 turnos, el contenido que gomemory reenvía y que el agente ya tenía baja al menos un 80 % frente a la versión actual.
- **SC-004**: En esa misma sesión, la parte estable del contexto inyectado es idéntica byte a byte en el 100 % de los turnos sin cambios en la memoria.
- **SC-005**: El 100 % de las omisiones del corpus se recupera byte a byte por su referencia mientras no caduque.
- **SC-006**: En un conjunto de prueba con código, rutas, URLs, versiones y mensajes de error, el 100 % de esos elementos visibles aparece literal tras la compresión.
- **SC-007**: El motor añade menos de 50 ms por cada 10 000 tokens de entrada en el 95 % de los casos.
- **SC-008**: Con el nivel *estructural*, la salida es idéntica a la de la versión anterior; con el nivel *ninguna*, idéntica a la entrada.
- **SC-009**: Ningún contenido marcado como privado aparece en los originales guardados, en las estadísticas ni en los registros del conjunto de prueba.
- **SC-010**: La tasa de recuperación de originales se mantiene por debajo del 10 % de las omisiones en uso real. Si la supera, el ajuste adaptativo la reduce en las sesiones siguientes.

## Assumptions

- **Headroom como referencia, no como dependencia.** Se adoptan sus prácticas y su diseño, no su código ni su proceso. Si en la implementación se reutiliza algún algoritmo concreto de su código (Apache 2.0), se respetan la atribución y el aviso de licencia.
- **Sin modelo de IA para la prosa.** Kompress es un modelo entrenado. Su equivalente aquí es extractivo y determinista, así que ahorra menos en prosa pero conserva el binario único, el determinismo y la estabilidad de la caché.
- **Nivel por defecto.** Instalación nueva: *máxima*, porque toda omisión es recuperable. Actualización de una instalación existente: se mantiene *estructural*, y la persona pasa a *máxima* desde la configuración, para no cambiar el comportamiento sin avisar. El plan lo confirma según la migración de ajustes existente.
- **Memoria entre agentes.** gomemory ya es la memoria compartida y deduplicada entre agentes, así que esa parte del diagrama de Headroom no requiere trabajo nuevo.
- **"Learn" de Headroom.** Se cubre con el ajuste adaptativo de FR-027 y con la captura de aprendizajes que gomemory ya tiene. No se escriben archivos de instrucciones del agente.
- **Enganches de reescritura de salidas.** Solo algunos runtimes permiten reescribir la salida de una herramienta. El plan verifica, contra el binario real de cada runtime soportado, cuáles lo admiten hoy, según la regla de trabajo "primero reproduce la realidad".
- **Valores por defecto.** El umbral mínimo, el plazo de caducidad de los originales, el tope de espacio y el umbral de ajuste adaptativo se fijan en el plan con el corpus de referencia. Punto de partida razonable: 50 palabras, 7 días, 256 MB y 20 % de recuperación.
- **Corpus de referencia.** Se construye a partir de contenido real de gomemory y de salidas típicas de herramientas de agente. Las cifras de SC-001 a SC-003 se validan contra él.
- **Imágenes fuera de alcance.** gomemory no entrega imágenes al agente, así que la compresión de imágenes de Headroom no aplica.
