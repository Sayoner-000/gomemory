# Feature Specification: Compactación de contexto sin pérdida de memoria

**Feature Branch**: `030-auto-compact-context`

**Created**: 2026-09-10

**Status**: Draft

**Input**: User description: "Compactación automática de contexto al superar CompactThreshold, respetando la spec 008 (FR-008: el servidor no ejecuta compactación; FR-011: agnóstica al agente). Aclaración del usuario: «esto debe ser agnóstico 100 %». El usuario pidió después reorientar esta spec hacia tres mecanismos: contexto de compactación acotado a la sesión, persistencia del resumen compactado y captura pasiva de aprendizajes, con la directiva al agente como parte secundaria y sin turno forzado."

## Contexto del problema *(no normativo — fundamenta el porqué)*

La primera versión de esta spec buscaba provocar la compactación desde gomemory,
forzando un turno extra para que el agente la ejecutara. Dos hallazgos
cambiaron el enfoque:

1. **En varios clientes el agente no puede compactar su propia conversación.**
   Un turno forzado solo podía terminar pidiéndole a la persona que compactara,
   y ese turno se paga con el contexto ya grande.
2. **La compactación automática ya la trae el cliente.** Un sistema de memoria
   no necesita compactar en cada turno: la compactación la dispara siempre el
   cliente, con su compactación automática nativa o a mano. Lo que sí puede
   hacer la memoria es engancharse antes y después para que no se pierda nada:
   - **Antes de compactar**, entregar al compresor solo la memoria de **esta
     sesión**, recortada y con tope de tamaño, más la orden de guardar el resumen
     compactado como memoria.
   - **Después de compactar**, reinyectar el protocolo y un contexto compacto,
     con los pasos de recuperación.
   - **Al terminar un subagente**, extraer de su mensaje final las secciones de
     aprendizajes y guardarlas sin duplicados, sin gastar tokens del agente.

gomemory ya tiene la recuperación posterior a la compactación (mismos tres
pasos). Hoy le faltan dos cosas:

- Lo que entrega alrededor de la compactación es el contexto **del proyecto
  entero**, no el de la sesión. Pesa más y no le dice al compresor qué registró
  la memoria en esta conversación.
- Al terminar un subagente solo registra actividad (archivos y comandos); los
  aprendizajes que el subagente escribió se pierden si nadie los guarda a mano.

**Principio rector: la compactación es del cliente, no perder memoria es de
gomemory.** Esta feature no dispara, no fuerza ni ejecuta compactaciones. Hace
que cualquier compactación, automática o manual, en cualquier cliente, conserve
lo que la memoria registró.

**Agnóstico al 100 % (exigido por el usuario).** Ningún requisito, criterio de
éxito ni texto emitido nombra un agente, un cliente o un comando de cliente. El
contrato se describe con capacidades que un cliente puede o no ofrecer. Qué
capacidades ofrece cada cliente concreto se resuelve y verifica en el plan.

## Capacidades del cliente *(normativo — vocabulario del contrato)*

| Capacidad | Qué permite | Si el cliente no la ofrece |
|-----------|-------------|----------------------------|
| **C1. Aporte previo a la compactación** | Añadir texto que el compresor tiene en cuenta al generar el resumen compactado. | La memoria de la sesión se entrega solo después de compactar (C2). |
| **C2. Canal al agente tras la compactación** | Entregar texto que el agente lee al reanudar después de compactar. | No hay recuperación automática; el diagnóstico lo informa. |
| **C3. Señal de fin de subagente con su texto final** | Recibir, al terminar un subagente, el mensaje final que produjo. | No hay captura pasiva para ese cliente; el diagnóstico lo informa. |
| **C4. Canal al agente en el fin de turno o en el turno siguiente** | Entregar al agente un texto breve sin interrumpir el turno. | La directiva de US4 no llega al agente; solo queda el aviso a la persona. |
| **C5. Canal a la persona** | Mostrar un aviso visible a la persona. | Sin aviso visible; no afecta a las demás historias. |
| **C6. Resumen compactado entregado a la integración** | Que la integración reciba el texto del resumen que produjo la compactación. | El resumen lo persiste el agente por orden de texto (FR-007, FR-008). |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Lo que la memoria registró en la sesión sobrevive a la compactación (Priority: P1)

Como desarrollador en una sesión larga, quiero que, cuando mi cliente compacte
la conversación, lo que gomemory registró en esta sesión llegue al compresor y
al agente que reanuda, en forma breve y acotada, para no perder decisiones ni
hallazgos al compactar y no cargar el contexto del proyecto entero otra vez.

**Why this priority**: Es la brecha principal y la que decide si
compactar es seguro. Aporta valor por sí sola en cualquier cliente con C1 o C2,
y además reduce lo que se reinyecta tras compactar.

**Independent Test**: En una sesión con varias memorias guardadas, compactar la
conversación. Verificar que el texto entregado alrededor de la compactación
contiene esas memorias (tipo, título, extracto breve e identificador) y que su
tamaño respeta el tope.

**Acceptance Scenarios**:

1. **Given** una sesión con memorias guardadas y un cliente con C1, **When** el
   cliente va a compactar, **Then** el compresor recibe la memoria de esta
   sesión en forma breve, con un identificador por entrada para recuperar el
   detalle bajo demanda.
2. **Given** la compactación ya hecha y un cliente con C2, **When** el agente
   reanuda, **Then** recibe primero los pasos de recuperación y la memoria de la
   sesión, y después el contexto de proyecto en forma compacta, todo dentro del
   tope de tamaño.
3. **Given** una sesión que aún no guardó ninguna memoria, **When** se compacta,
   **Then** el agente recibe los pasos de recuperación y una nota de que la
   sesión no tiene memoria propia todavía, no un bloque vacío.
4. **Given** una segunda compactación en la misma sesión, **When** se genera el
   texto de compactación, **Then** incluye el resumen persistido de la
   compactación anterior (US2) y no repite entradas.

---

### User Story 2 - El resumen compactado se guarda como memoria de la sesión (Priority: P2)

Como desarrollador, quiero que el resumen que genera mi cliente al compactar
quede guardado en la memoria persistente, para que lo trabajado antes de la
compactación sea recuperable en sesiones futuras y no dependa solo de la ventana
actual.

**Why this priority**: Convierte cada compactación en memoria duradera. Depende
de US1 solo en parte: la orden de persistir ya existe en la recuperación actual,
aquí se refuerza y se vuelve verificable.

**Independent Test**: Compactar una sesión y verificar que, antes de que el
agente haga otro trabajo, el resumen compactado queda guardado como resumen de
la sesión, una sola vez.

**Acceptance Scenarios**:

1. **Given** un cliente con C1, **When** el cliente compacta, **Then** el
   resumen compactado lleva al inicio la orden neutral de persistirlo como
   resumen de la sesión antes de cualquier otro trabajo.
2. **Given** un cliente sin C1 pero con C2, **When** el agente reanuda, **Then**
   la misma orden le llega como primer paso de la recuperación.
3. **Given** que el agente persiste el resumen dos veces para la misma
   compactación, **When** se guarda la segunda, **Then** se consolida sobre la
   primera y no se crea un duplicado.
4. **Given** un cliente con C6, **When** termina la compactación, **Then** la
   integración persiste el resumen por sí misma, sin acción del agente.
5. **Given** que se persiste un resumen compactado, **When** el agente guarda
   una memoria después, **Then** esa memoria sigue asociada a la misma sesión.

---

### User Story 3 - Captura pasiva de aprendizajes de subagentes (Priority: P3)

Como desarrollador que delega trabajo en subagentes, quiero que los aprendizajes
que un subagente escribe en su respuesta final se guarden solos en la memoria,
sin que el agente principal gaste tokens en guardarlos, para no perder hallazgos
que hoy mueren con el subagente.

**Why this priority**: Es independiente de la compactación y tiene valor propio,
pero solo cubre el trabajo delegado. Reduce la dependencia de que el agente
recuerde guardar.

**Independent Test**: Terminar un subagente cuya respuesta final tiene una
sección de aprendizajes con tres ítems válidos. Verificar que se guardan tres
memorias, y que repetir la misma respuesta no crea ninguna nueva.

**Acceptance Scenarios**:

1. **Given** un cliente con C3 y un subagente cuya respuesta final contiene una
   sección de aprendizajes, **When** el subagente termina, **Then** cada ítem
   válido se guarda como memoria con la procedencia «subagente».
2. **Given** una respuesta sin sección de aprendizajes, **When** el subagente
   termina, **Then** no se guarda ninguna memoria extraída; solo el registro de
   actividad vigente.
3. **Given** un ítem ya guardado antes, **When** vuelve a aparecer, **Then** no
   se crea un duplicado.
4. **Given** un ítem que contiene una parte marcada como privada, **When** se
   guarda, **Then** esa parte no se persiste.

---

### User Story 4 - Aviso de preparación al agente al superar el umbral (Priority: P4)

Como desarrollador que activó la opción, quiero que, cuando la huella de
gomemory supere el umbral de compactación, el agente reciba un aviso breve para
guardar lo pendiente, además del aviso que ya veo yo, para que una compactación
posterior encuentre la memoria al día.

**Why this priority**: Es secundaria. La compactación automática ya la trae el
cliente y US1–US2 protegen la memoria cuando ocurre; este aviso solo adelanta la
preparación. No fuerza turnos ni compactaciones.

**Independent Test**: Con la opción activada y la huella por encima del umbral,
cerrar un turno y verificar que el agente recibe el aviso por C4 una sola vez
por pausa. Con la opción apagada, verificar que la salida es idéntica a la
actual.

**Acceptance Scenarios**:

1. **Given** la opción activada, un umbral mayor que 0 y la huella por encima,
   **When** termina un turno en un cliente con C4, **Then** el agente recibe el
   aviso sin que el turno se interrumpa ni se prolongue.
2. **Given** la opción desactivada (valor por defecto), **When** la huella supera
   el umbral, **Then** el comportamiento es exactamente el de la spec 008.
3. **Given** que no hay nada pendiente de guardar, **When** el agente recibe el
   aviso, **Then** no crea memorias de relleno.

---

### Edge Cases

- **Sesión muy activa**: si la memoria de la sesión excede el tope, se muestran
  primero las entradas accionables (conflictos y decisiones) y las más
  recientes, y se indica cuántas quedaron fuera con la forma de recuperarlas.
- **Compactación sin sesión de memoria activa**: se entregan los pasos de
  recuperación y el contexto de proyecto compacto; nunca un error visible.
- **Cliente sin C1 ni C2**: no hay protección automática alrededor de la
  compactación; el diagnóstico lo muestra para ese cliente.
- **Respuesta de subagente enorme**: la extracción solo lee la última sección de
  aprendizajes y descarta ítems demasiado cortos o sin contenido real.
- **Sección de aprendizajes en otro idioma**: se reconocen los encabezados en
  español y en inglés.
- **Subagente anidado**: la captura aplica a cualquier subagente con C3; la
  memoria se asocia a la sesión principal.
- **Aviso de US4 y refuerzo de preferencias en el mismo turno**: rige la regla
  vigente de un solo mensaje por turno; el aviso de preparación tiene prioridad.
- **Umbral ≤ 0**: US4 queda inactiva aunque la opción esté activada. US1–US3 no
  dependen del umbral.

## Requirements *(mandatory)*

### Functional Requirements

**Memoria de la sesión alrededor de la compactación (US1)**

- **FR-001**: El sistema DEBE poder generar un **contexto de compactación**
  acotado a la sesión activa: memorias guardadas en ella, resumen de
  compactaciones previas de la misma sesión y último prompt registrado, cada
  entrada en forma breve (tipo, título, extracto e identificador).
- **FR-002**: El contexto de compactación NO DEBE incluir memorias de otras
  sesiones ni de otros proyectos.
- **FR-003**: Cuando el cliente ofrezca C1, el sistema DEBE entregar el contexto
  de compactación al compresor antes de compactar.
- **FR-004**: Cuando el cliente ofrezca C2, el texto posterior a la compactación
  DEBE contener, en este orden: los pasos de recuperación, el contexto de
  compactación y el contexto de proyecto en forma compacta.
- **FR-005**: El texto posterior a la compactación DEBE respetar un tope de
  tamaño no mayor que el presupuesto del contexto de arranque (spec 008,
  FR-001). Los pasos de recuperación y los conflictos sin resolver no se
  recortan nunca.
- **FR-006**: Si el contexto de compactación supera su parte del tope, DEBE
  priorizar las entradas accionables y las más recientes, e indicar cuántas
  quedaron fuera y cómo recuperarlas.

**Persistencia del resumen compactado (US2)**

- **FR-007**: Cuando el cliente ofrezca C1, el sistema DEBE pedir al compresor
  que ponga al inicio del resumen compactado una orden neutral de persistirlo
  como resumen de la sesión antes de cualquier otro trabajo.
- **FR-008**: Los pasos de recuperación posteriores a la compactación DEBEN
  conservar esa misma orden como primer paso, para los clientes sin C1.
- **FR-009**: Persistir el resumen de la misma compactación más de una vez NO
  DEBE crear duplicados: se consolida sobre la entrada existente (spec 008,
  FR-013).

**Captura pasiva de aprendizajes (US3)**

- **FR-010**: Cuando el cliente ofrezca C3, el sistema DEBE extraer los ítems de
  la **última** sección de aprendizajes del mensaje final del subagente,
  reconociendo encabezados en español y en inglés, y guardar cada ítem válido
  como memoria con procedencia «subagente».
- **FR-011**: Un ítem es válido solo si tiene contenido suficiente (umbral mínimo
  de longitud y de palabras). Sin sección de aprendizajes no se guarda nada
  extraído.
- **FR-012**: La captura pasiva NO DEBE crear duplicados de memorias existentes
  y DEBE respetar las marcas de contenido privado.
- **FR-013**: La captura pasiva NO DEBE requerir acciones del agente ni añadir
  texto a su contexto; es trabajo del hook.

**Aviso de preparación al agente (US4)**

- **FR-014**: El sistema DEBE ofrecer una opción «aviso de compactación al
  agente», desactivada por defecto y editable desde la configuración
  interactiva junto al umbral de compactación.
- **FR-015**: Con la opción desactivada, la salida del fin de turno DEBE ser
  idéntica a la actual (spec 008) en cualquier cliente.
- **FR-016**: Con la opción activada, el umbral mayor que 0 y la huella en el
  umbral o por encima, el sistema DEBE entregar al agente, por C4, un aviso
  breve de guardar lo pendiente, con la misma decisión de umbral y la misma
  pausa entre avisos que el recordatorio vigente. El aviso a la persona (C5) se
  mantiene.
- **FR-017**: El aviso NO DEBE interrumpir, prolongar ni forzar turnos, y NO DEBE
  entregarse a subagentes.

**Transversales**

- **FR-018**: Ningún texto emitido por esta feature DEBE nombrar agentes,
  clientes ni comandos de cliente. El texto es idéntico para todos los clientes.
- **FR-019**: El sistema NO DEBE disparar, forzar ni ejecutar compactaciones. La
  compactación es siempre del cliente (se preserva FR-008 de la spec 008).
- **FR-020**: El diagnóstico DEBE indicar, por cliente instalado, qué
  capacidades (C1–C5) aprovecha esta feature.
- **FR-021**: Incorporar un cliente nuevo NO DEBE requerir cambios en los textos
  emitidos ni en las reglas de decisión; solo declarar sus capacidades.
- **FR-022**: Cuando el cliente ofrezca C6, la integración DEBE persistir el
  resumen compactado directamente, sin depender del agente. FR-007 y FR-008
  quedan como respaldo para los clientes sin C6.
- **FR-023**: Persistir un resumen compactado NO DEBE cerrar la sesión de memoria
  activa, y después de una compactación DEBE haber una sesión activa, para que
  lo guardado a continuación siga asociado a la sesión.

### Key Entities *(include if feature involves data)*

- **Contexto de compactación**: vista breve y acotada de la memoria de una sola
  sesión, con identificadores para ampliar bajo demanda.
- **Resumen compactado persistido**: resumen de sesión guardado a partir del
  resumen que produce el cliente al compactar; uno por compactación.
- **Aprendizaje capturado**: memoria creada a partir de un ítem de la sección de
  aprendizajes de un subagente, con procedencia «subagente».
- **Capacidades del cliente**: conjunto C1–C5 que declara cada integración;
  determina qué partes de la feature operan en ese cliente.
- **Opción de aviso de compactación al agente**: preferencia del proyecto,
  booleana, apagada por defecto.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tras una compactación en un cliente con C1 o C2, el 100 % de las
  memorias guardadas en la sesión, dentro del tope, aparece con su identificador
  en el texto que recibe el agente.
- **SC-002**: En un proyecto con 100 memorias o más, el texto posterior a la
  compactación ocupa como mucho la mitad que en la versión anterior, sin perder
  los pasos de recuperación ni los conflictos.
- **SC-003**: En clientes con C6, el 100 % de las compactaciones guionizadas
  deja el resumen persistido exactamente una vez. En clientes sin C6 pero con C1
  o C2, al menos 9 de cada 10, antes de que el agente haga otro trabajo.
- **SC-004**: Una respuesta de subagente con N ítems de aprendizaje válidos
  produce N memorias; repetir la misma respuesta produce 0 nuevas.
- **SC-005**: La captura pasiva consume 0 acciones del agente y añade 0
  caracteres a su contexto.
- **SC-006**: Ninguna parte de la feature genera turnos adicionales: 0 turnos
  forzados en una sesión de 50 turnos por encima del umbral.
- **SC-007**: Los textos emitidos contienen 0 nombres de agentes, clientes o
  comandos de cliente, verificado contra una lista de términos prohibidos.
- **SC-008**: Con la opción de US4 desactivada, la salida del fin de turno no
  cambia en ningún cliente (0 diferencias en una batería de turnos).

## Assumptions

- La compactación automática por tamaño la ofrece el propio cliente. Esta
  feature no la reimplementa ni la sustituye; la compactación programática
  desde la integración queda fuera de alcance.
- «Sesión» es la sesión de memoria activa de gomemory: las memorias guardadas
  mientras está abierta se asocian a ella (conducta vigente).
- De cada sesión se registra hoy solo el último prompt, no el historial de
  prompts. El contexto de compactación usa lo que existe; ampliar el registro de
  prompts queda fuera de alcance.
- La huella del umbral es la que ya mide la spec 008 (texto aportado por
  gomemory, no la ventana completa), se reinicia al compactar y al iniciar
  sesión, y la pausa entre avisos sigue en 30 minutos.
- Los umbrales de validez de un aprendizaje se fijan en el plan (referencia
  inicial: 20 caracteres y 4 palabras como mínimo).
- No hay garantía de que el agente o el compresor obedezcan una orden de texto.
  La mitigación es la redundancia: la orden va al compresor (C1) y a la
  recuperación (C2), y el cumplimiento es verificable (SC-003).
- Qué capacidades ofrece cada cliente concreto, y por qué canal técnico, se
  establece y verifica contra el cliente en ejecución durante el plan.
- La spec 008 no se modifica: esta feature no hace el fin de turno bloqueante y
  respeta su FR-008. Solo recibirá una referencia cruzada al implementar.
