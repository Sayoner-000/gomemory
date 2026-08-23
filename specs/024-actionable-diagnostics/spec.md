# Feature Specification: Diagnóstico accionable y vitalidad de los canales

**Feature Branch**: `024-actionable-diagnostics`

**Created**: 2026-08-23

**Status**: Draft

**Input**: Petición de que el informe de estado explique en lenguaje claro qué significa cada
resultado y cómo corregirlo, más el riesgo verificado de que un canal deje de funcionar sin que
nada lo reporte. Se separó de la especificación 022, que atiende la estructura de los canales.

## Contexto del problema

El informe de estado enumera hoy diecisiete canales con un símbolo y una frase técnica. La frase
describe **el mecanismo**, no lo que le pasa a quien lee ni lo que debe hacer.

Un ejemplo real de su salida:

```
❌ gomemory   opencode   user     plan_entry     no encontrado: <ruta del complemento>
➖ gomemory   opencode   project  plan_guard     el ciclo del agente no ofrece un punto
                                                de decisión antes de presentar el plan
```

La primera línea nombra un archivo ausente, pero no dice qué se pierde por ello ni con qué
comando se restablece. La segunda parece una falla por su redacción, aunque es una limitación
esperada que no requiere acción alguna. Quien lee tiene que saber de antemano cómo funciona el
sistema para distinguir una de otra.

Hay tres problemas encadenados:

1. **El resultado no dice qué se pierde.** Un canal caído tiene una consecuencia concreta para
   quien trabaja, y esa consecuencia no aparece.
2. **El resultado no dice cómo se corrige.** El comando que restablece cada canal existe, pero
   quien lee el informe tiene que buscarlo en la documentación.
3. **Una limitación esperada se lee igual que un defecto.** El informe ya distingue ambos
   estados con símbolos, pero el texto que los acompaña no cambia de registro.

### El fallo que ningún informe reporta

Los hooks del complemento de un agente dependen de dos operaciones que ese agente marca como
experimentales. Si las renombra, la inyección del protocolo y del contexto se pierde **sin
ninguna señal**: las rutas de error del complemento absorben el fallo, ninguna invocación deja
rastro, y el informe de estado sigue reportando el canal como correcto porque comprueba que el
archivo existe, no que funcione.

Es decir: el informe verifica **presencia**, no **actividad**. Un canal presente y muerto es
indistinguible de uno sano.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Entender qué pasa y cómo corregirlo, sin consultar documentación (Priority: P1)

Quien ejecuta el informe de estado necesita, para cada resultado que no esté correcto, saber qué
se pierde y qué hacer. Hoy obtiene una descripción del mecanismo y tiene que traducirla.

**Why this priority**: es la petición directa de quien usa el producto y entrega valor sin
depender de nada más. Un informe que nombra el problema pero no la salida deja el trabajo a
medias.

**Independent Test**: provocar un canal caído y comprobar que el informe indica el efecto y el
comando de corrección, sin que quien lee tenga que abrir la documentación.

**Acceptance Scenarios**:

1. **Given** un canal en estado de falla, **When** se consulta el informe, **Then** se indica
   qué capacidad se pierde por ello y con qué comando se restablece.
2. **Given** un canal con una limitación esperada, **When** se consulta el informe, **Then** el
   texto declara que no requiere acción, y no se ofrece comando de corrección.
3. **Given** varios canales en falla que se corrigen con el mismo comando, **When** se consulta
   el informe, **Then** el comando se propone una sola vez, agrupando los canales que resuelve.
4. **Given** un informe sin fallas, **When** se consulta, **Then** lo declara explícitamente y
   no propone acciones.
5. **Given** una falla cuya corrección modifica artefactos fuera del proyecto, **When** se
   propone el comando, **Then** se advierte el alcance del cambio antes de que se ejecute.
6. **Given** el informe en su forma legible por máquina, **When** se consulta, **Then** el
   efecto y la corrección están disponibles como datos, no solo en el texto para personas.

---

### User Story 2 - Que un canal presente pero muerto se distinga de uno sano (Priority: P1)

Quien depende de un complemento para recibir el protocolo necesita enterarse si dejó de
funcionar porque el agente cambió su interfaz. Hoy el fallo es indistinguible del funcionamiento
normal.

**Why this priority**: el daño es alto y silencioso. Comparte P1 con la historia anterior porque
un informe accionable que verifica lo incorrecto sigue dando una falsa tranquilidad.

**Independent Test**: simular que una de las operaciones del complemento desaparece y comprobar
que el informe lo reporta en lugar de seguir en verde.

**Acceptance Scenarios**:

1. **Given** un complemento instalado cuyo canal de inyección no se ejerce desde hace más de un
   umbral configurable, **When** se consulta el informe, **Then** se reporta como inactivo, con
   la fecha del último uso y la corrección sugerida.
2. **Given** un canal que falla al ejecutarse, **When** se consulta el informe, **Then** se
   reporta la falla con su causa registrada.
3. **Given** un fallo en una de esas rutas, **When** ocurre, **Then** no interrumpe el turno de
   quien trabaja.
4. **Given** un proyecto donde ese agente nunca se usó, **When** se consulta el informe,
   **Then** la ausencia de actividad no se reporta como falla.
5. **Given** un complemento instalado con una versión anterior, **When** se consulta el informe,
   **Then** la detección de inactividad funciona igual sobre él.

---

### User Story 3 - Detectar el cambio de interfaz antes de publicar (Priority: P2)

Quien mantiene el proyecto necesita enterarse de que un agente renombró una operación antes de
que el cambio llegue a las máquinas de otras personas.

**Why this priority**: cubre el caso más probable —el agente cambia y se actualiza— con el
mecanismo más barato. Va en P2 porque la Historia 2 ya cubre el caso en el que el cambio llegó
sin avisar.

**Independent Test**: declarar en el complemento una operación que la interfaz del agente no
ofrece y comprobar que la verificación previa a publicar falla nombrándola.

**Acceptance Scenarios**:

1. **Given** un complemento que declara una operación ausente de la interfaz publicada por el
   agente, **When** se ejecuta la verificación, **Then** falla nombrando la operación.
2. **Given** un entorno donde la interfaz del agente no está disponible, **When** se ejecuta la
   verificación, **Then** se omite sin error, en lugar de fallar por una causa ajena al código.
3. **Given** un complemento cuyas operaciones existen todas, **When** se ejecuta la
   verificación, **Then** pasa.

---

### Edge Cases

- Una falla se corrige con un comando que a su vez requiere que otra cosa esté presente: la
  corrección propuesta debe ser ejecutable tal como se muestra, o declarar su requisito.
- El umbral de inactividad se cumple porque la persona no trabajó en varios días, no porque el
  canal esté roto: el informe debe distinguir sesiones sin actividad de canales sin respuesta.
- El registro de actividad por canal crece sin límite: necesita política de retención.
- La corrección propuesta afecta a otros proyectos por ser de ámbito de usuario: debe advertirse
  antes de proponerla como acción simple.
- Un canal se reporta inactivo y su corrección es reinstalar, pero reinstalar sobrescribiría
  algo que el equipo personalizó: la propuesta debe respetar lo que está bajo control del equipo.
- El agente publica su interfaz en un formato que cambia entre versiones: la verificación debe
  degradarse a omitida antes que producir un falso negativo.

## Requirements *(mandatory)*

### Functional Requirements

**Informe accionable (Historia 1)**

- **FR-001**: Cada resultado del informe que no esté correcto DEBE indicar qué capacidad se
  pierde, en términos de lo que deja de ocurrir para quien trabaja.
- **FR-002**: Cada resultado corregible DEBE indicar el comando que lo corrige.
- **FR-003**: Un resultado que corresponda a una limitación esperada DEBE declarar que no
  requiere acción, y NO DEBE ofrecer comando de corrección.
- **FR-004**: Cuando varios resultados se corrijan con el mismo comando, el informe DEBE
  proponerlo una sola vez agrupando los resultados que resuelve.
- **FR-005**: Un informe sin fallas DEBE declararlo explícitamente.
- **FR-006**: Cuando la corrección modifique artefactos fuera del proyecto, el informe DEBE
  advertir el alcance antes de proponerla.
- **FR-007**: El efecto y la corrección DEBEN estar disponibles también en la forma del informe
  legible por máquina, como datos independientes del texto para personas.
- **FR-008**: El efecto y la corrección de cada canal DEBEN declararse junto al canal, de modo
  que un canal nuevo los traiga consigo en lugar de requerir edición del informe.

**Vitalidad (Historia 2)**

- **FR-009**: El sistema DEBE registrar cuándo se ejerció por última vez cada canal de inyección.
- **FR-010**: El informe DEBE reportar como inactivo el canal cuya última actividad supere un
  umbral configurable, indicando la fecha del último uso.
- **FR-011**: El informe DEBE distinguir un canal sin actividad porque no hubo sesiones de uno
  que no responde habiéndolas.
- **FR-012**: Las rutas de error del complemento NO DEBEN absorber el fallo sin dejar rastro:
  DEBEN registrarlo donde el informe pueda leerlo.
- **FR-013**: Ningún fallo de esas rutas DEBE interrumpir el turno de quien trabaja.
- **FR-014**: Un canal que nunca se ejerció en un proyecto donde ese agente no se usa NO DEBE
  reportarse como falla.
- **FR-015**: La detección de inactividad DEBE funcionar sobre complementos instalados por
  versiones anteriores.
- **FR-016**: El registro de actividad por canal DEBE tener una política de retención declarada.

**Verificación previa a publicar (Historia 3)**

- **FR-017**: La verificación previa a publicar DEBE comprobar que cada operación que el
  complemento declara existe en la interfaz que el agente publica, y fallar nombrando las
  ausentes.
- **FR-018**: Esa verificación DEBE omitirse sin error cuando la interfaz del agente no esté
  disponible en el entorno.

### Key Entities

- **Resultado del informe**: el estado de un canal en un ámbito para un agente, con su símbolo,
  la descripción del mecanismo, el efecto sobre quien trabaja y la corrección cuando aplica.
- **Efecto**: qué deja de ocurrir cuando ese canal no funciona, expresado en términos de la
  persona y no del mecanismo.
- **Corrección**: el comando que restablece el canal, con su alcance declarado.
- **Registro de actividad**: cuándo se ejerció por última vez cada canal, y el rastro de los
  fallos que se produjeron al intentarlo.
- **Umbral de inactividad**: el tiempo tras el cual un canal sin uso pasa a reportarse como
  inactivo. Configurable, con valor por defecto.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Quien ejecuta el informe con un canal caído sabe qué se pierde y qué comando lo
  corrige, sin abrir documentación ni consultar a otra persona.
- **SC-002**: Ninguna limitación esperada del informe se lee como algo que requiere acción,
  verificado pidiendo a alguien ajeno al proyecto que clasifique cada línea.
- **SC-003**: Un canal de inyección que dejó de ejercerse aparece señalado la siguiente vez que
  se consulta el informe, con la fecha de su último uso.
- **SC-004**: Un cambio de nombre en la interfaz del agente se detecta antes de publicar, y no
  en producción.
- **SC-005**: Un fallo en las rutas de inyección queda registrado y visible en el informe, sin
  haber interrumpido el turno en el que ocurrió.
- **SC-006**: Cada canal declarado trae consigo su efecto y su corrección; añadir un canal no
  requiere editar el informe.
- **SC-007**: La forma del informe legible por máquina expone efecto y corrección como datos.

## Assumptions

- El informe de estado existente es el punto de partida. Esta feature enriquece sus resultados;
  no lo reemplaza.
- El umbral de inactividad tiene un valor por defecto razonable y es configurable.
- El registro de actividad vive junto al resto del estado del proyecto y respeta la política de
  privacidad vigente.
- La interfaz publicada por el agente es consultable en el entorno de desarrollo, aunque no
  necesariamente en todos los entornos donde corre la verificación.
- Las correcciones propuestas son comandos que ya existen. Esta feature no crea comandos nuevos
  de reparación.

## Out of Scope

- Corregir automáticamente los canales en falla. El informe propone; quien trabaja decide.
- La estructura de la matriz de canales y su ciclo de vida. Corresponde a la especificación 022,
  de la que esta depende: el informe se deriva de esa declaración.
- Medir el costo en contexto de cada canal. Corresponde a la especificación 023.
- Añadir agentes o canales nuevos.
