# Especificación de funcionalidad: Octopus AAR — Enrutador Adaptativo de Agentes

**Rama de funcionalidad**: `octopus-aar`

**Creado**: 2026-09-02

**Estado**: Borrador

**Entrada**: Descripción del usuario: "Octopus AAR — Adaptive Agent Router: capacidad de enrutamiento adaptativo, consciente del contexto y del presupuesto, que decide si una unidad de trabajo debe ejecutarse en el agente principal (inline) o delegarse a uno o varios subagentes. Octopus no ejecuta agentes: entrega política de enrutamiento a runtimes compatibles, minimizando transferencia de contexto, consumo de tokens, coordinación y razonamiento duplicado. Debe permitir configurar el apartado en la TUI como un módulo llamado Octopus AAR, encender o apagar según lo requiera el usuario, por ser una funcionalidad grande."

## Escenarios de usuario y pruebas *(obligatorio)*

### Historia de usuario 1 - Decidir si una tarea se delega o no (Prioridad: P1)

Como agente principal a punto de ejecutar una unidad de trabajo, quiero preguntar si conviene resolverla yo mismo o delegarla a un subagente, y recibir una respuesta con una razón concreta, para no crear subagentes que cuesten más de lo que ahorran.

**Por qué esta prioridad**: Es el núcleo del producto y el mínimo entregable con valor. Sin esta decisión no existe nada del resto: los planes, los presupuestos y la telemetría solo dan forma a esta respuesta.

**Prueba independiente**: Se puede enviar una unidad de trabajo con sus capacidades de runtime y su presupuesto restante, y comprobar que se devuelve exactamente una ruta (`INLINE`, `DELEGATE`, `WAIT` o `REJECT`) acompañada de una razón legible, sin consultar ningún modelo externo y sin telemetría previa.

**Escenarios de aceptación**:

1. **Dado** un cambio local trivial, **cuando** se evalúa el enrutamiento, **entonces** la ruta es `INLINE` y la razón indica que el sobrecosto de delegar supera el beneficio esperado.
2. **Dado** una investigación independiente del repositorio, de solo lectura y con contexto acotado, **cuando** el runtime declara soporte de subagentes y hay presupuesto disponible, **entonces** la ruta puede ser `DELEGATE` con presupuesto de contexto y de salida declarados.
3. **Dado** un runtime que declara `subagentes = false`, **cuando** se evalúa cualquier unidad de trabajo, **entonces** no se devuelve ninguna ruta que exija ejecución delegada.
4. **Dado** una unidad de trabajo con una dependencia directa sin resolver, **cuando** se evalúa el enrutamiento, **entonces** la ruta es `WAIT` y se enumeran las dependencias pendientes.
5. **Dado** dos evaluaciones con entradas idénticas, **cuando** se comparan sus resultados, **entonces** la ruta y la razón son las mismas.

---

### Historia de usuario 2 - Encender o apagar el módulo Octopus AAR (Prioridad: P2)

Como persona que usa gomemory, quiero ver "Octopus AAR" como un módulo en la pantalla de configuración de la TUI y encenderlo o apagarlo, para decidir si quiero esta capacidad grande activa en mi proyecto o prefiero que el sistema se comporte exactamente como antes.

**Por qué esta prioridad**: La capacidad es grande y no todo el mundo la quiere. Sin un interruptor visible y persistente, activar Octopus sería una imposición; con él, la funcionalidad es adoptable de forma gradual y reversible.

**Prueba independiente**: Se puede abrir la pantalla de configuración de la TUI, ver la fila "Octopus AAR" con su estado, alternarlo, cerrar y reabrir la TUI, y comprobar que el estado se conservó y que el comportamiento del sistema cambió en consecuencia.

**Escenarios de aceptación**:

1. **Dado** un proyecto sin configuración previa de Octopus, **cuando** se abre la pantalla de configuración, **entonces** el módulo "Octopus AAR" aparece listado y su estado es apagado.
2. **Dado** el módulo apagado, **cuando** se solicita una decisión de enrutamiento por cualquier vía, **entonces** el sistema responde que la capacidad está desactivada y no emite decisiones, presupuestos ni telemetría de enrutamiento.
3. **Dado** el módulo apagado, **cuando** se compara la salida del contexto del proyecto con la de antes de esta funcionalidad, **entonces** no aparece ninguna sección, campo ni instrucción nueva atribuible a Octopus.
4. **Dado** el módulo encendido, **cuando** se reinicia la herramienta, **entonces** sigue encendido sin volver a configurarlo.
5. **Dado** que se alterna el módulo, **cuando** se confirma el cambio, **entonces** se muestra un mensaje de estado que indica si quedó activado o desactivado.

---

### Historia de usuario 3 - Enrutar un plan completo respetando dependencias y límites (Prioridad: P3)

Como agente principal con un plan de tareas ya descompuesto, quiero un plan de enrutamiento completo que diga qué queda inline, qué se delega, qué espera y qué puede correr en paralelo, para ejecutar el plan sin abrir un subagente por tarea.

**Por qué esta prioridad**: Extiende la decisión unitaria al caso real de uso (un plan de tareas) y es donde aparecen las protecciones que evitan la explosión de agentes.

**Prueba independiente**: Se puede entregar un grafo de tareas con dependencias y comprobar que el plan de enrutamiento resultante respeta esas dependencias, agrupa solo tareas independientes en paralelo y no supera los topes configurados de agentes ni de concurrencia.

**Escenarios de aceptación**:

1. **Dado** un plan donde T002 depende de T001, **cuando** se genera el plan de enrutamiento, **entonces** T002 nunca queda en el mismo grupo paralelo que T001.
2. **Dado** dos tareas mutuamente independientes y un runtime con `paralelo = true` y `max_parallel >= 2`, **cuando** ambas son aptas para delegar, **entonces** pueden asignarse al mismo grupo paralelo.
3. **Dado** un runtime con `max_parallel = 2`, **cuando** se forman los grupos paralelos, **entonces** ningún grupo contiene más de dos ejecuciones delegadas concurrentes.
4. **Dado** un plan con 20 tareas independientes y un tope de 4 subagentes por plan, **cuando** se genera el plan de enrutamiento, **entonces** se recomiendan como máximo 4 agentes delegados y el resto queda inline o en espera.
5. **Dado** una profundidad máxima de delegación de 1, **cuando** se construye el contrato de una tarea delegada, **entonces** ese contrato no autoriza al hijo a crear otro subagente.
6. **Dado** un plan con dependencias, **cuando** cambia el estado de una tarea completada, **entonces** una nueva evaluación conserva el trabajo ya realizado y no lo vuelve a enrutar como pendiente.

---

### Historia de usuario 4 - Entregar contexto mínimo y contrato acotado a lo delegado (Prioridad: P4)

Como agente principal que va a delegar, quiero que la tarea delegada reciba solo el contexto que necesita y un contrato con objetivo, alcance, permisos y forma de resultado, para que el subagente pueda terminar por su cuenta sin heredar toda mi conversación.

**Por qué esta prioridad**: El aislamiento de contexto es el objetivo de optimización principal. Sin él, delegar duplica contexto y la delegación deja de rendir.

**Prueba independiente**: Se puede pedir el contrato de una tarea delegada que solo requiere dos archivos y una memoria relevante, y comprobar que el paquete de contexto contiene esos elementos y no la conversación completa, ni memorias ajenas, ni el resto de los planes o especificaciones.

**Escenarios de aceptación**:

1. **Dado** una tarea delegada que requiere dos artefactos y una memoria relevante, **cuando** se construye su paquete de contexto, **entonces** no se incluyen historial de conversación ni memorias no relacionadas.
2. **Dado** cualquier tarea delegada, **cuando** se emite su contrato de ejecución, **entonces** contiene objetivo, alcance, restricciones, presupuesto de contexto y forma esperada del resultado.
3. **Dado** una investigación declarada de solo lectura, **cuando** se emite su contrato, **entonces** los permisos declarados no incluyen escritura ni ninguna capacidad ausente en el flujo principal.
4. **Dado** un paquete de contexto candidato que contiene credenciales, tokens o claves, **cuando** se construye el contexto delegado, **entonces** ese material se excluye.
5. **Dado** un subagente que devuelve una transcripción extensa, **cuando** el resultado vuelve al agente principal, **entonces** lo que se integra es un resultado estructurado acotado (conclusión, evidencia, artefactos, pendientes) y no la transcripción completa.

---

### Historia de usuario 5 - Respetar el presupuesto de la sesión (Prioridad: P5)

Como persona que paga los tokens de la sesión, quiero que Octopus reparta un presupuesto entre el agente principal, la delegación y una reserva de validación, y que nunca lo desborde en silencio, para que la delegación no consuma lo que hacía falta para verificar el trabajo.

**Por qué esta prioridad**: Convierte el enrutamiento en una decisión económica verificable y evita el fallo más caro: quedarse sin presupuesto justo antes de validar.

**Prueba independiente**: Se puede fijar un presupuesto con reserva de validación, agotar el fondo de delegación y comprobar que las siguientes delegaciones opcionales pasan a `INLINE` o `REJECT` en vez de tomar tokens de la reserva.

**Escenarios de aceptación**:

1. **Dado** un presupuesto restante de delegación menor al costo estimado de delegar, **cuando** se evalúa una delegación opcional, **entonces** la ruta es `INLINE` o `REJECT` y la razón menciona la presión de presupuesto.
2. **Dado** que los tokens restantes solo existen dentro de la reserva de validación, **cuando** se evalúa una tarea de implementación opcional, **entonces** la reserva se mantiene intacta.
3. **Dado** un presupuesto total configurado por porcentajes, **cuando** se cambian esos porcentajes, **entonces** los fondos derivados cambian en consecuencia sin valores fijos impuestos por el sistema.
4. **Dado** un entorno sin conteo exacto de tokens del proveedor, **cuando** se evalúa el enrutamiento, **entonces** se produce igualmente una decisión válida basada en estimaciones, y esas cifras se declaran como estimadas.

---

### Historia de usuario 6 - Simular y medir el enrutamiento (Prioridad: P6)

Como persona que evalúa si Octopus le conviene, quiero ver qué haría con mi plan sin que ejecute nada, y después contrastar lo estimado contra lo realmente consumido, para saber si el enrutamiento ahorra o no.

**Por qué esta prioridad**: Da confianza antes de adoptar y evidencia después. No es imprescindible para enrutar, pero sí para justificar la adopción.

**Prueba independiente**: Se puede pedir la simulación de un plan y comprobar que se describe cada ruta con su razón y sus presupuestos estimados, sin que se cree ningún subagente.

**Escenarios de aceptación**:

1. **Dado** el modo de simulación, **cuando** se genera el plan de enrutamiento, **entonces** no se inicia ningún subagente y la salida explica qué quedaría inline, qué se delegaría, qué podría correr en paralelo y por qué.
2. **Dado** una ejecución terminada, **cuando** el runtime informa el consumo real y el resultado, **entonces** ese reporte queda registrado y asociado a la decisión que lo originó.
3. **Dado** un conjunto de decisiones y reportes, **cuando** se consultan las métricas, **entonces** se pueden distinguir conteos por ruta, consumo estimado, consumo real, reducción de contexto y éxitos, fallos, reintentos y repliegues.
4. **Dado** que solo hay estimaciones y no medición exacta, **cuando** se reporta el ahorro, **entonces** se declara como estimado y no como cifra exacta.
5. **Dado** cualquier métrica emitida, **cuando** se inspecciona su contenido, **entonces** no incluye contenido de contexto, transcripciones ni razonamiento privado.

---

### Historia de usuario 7 - Sobrevivir a delegaciones que fallan (Prioridad: P7)

Como agente principal que delegó una tarea y recibió un fallo o un "me falta contexto", quiero una política acotada de reintento, expansión y repliegue, para no quedar atrapado en un ciclo de reintentos ni perder el trabajo parcial.

**Por qué esta prioridad**: Es la red de seguridad de la delegación. Sin ella, un fallo puede costar más que no haber delegado.

**Prueba independiente**: Se puede simular un fallo de la tarea delegada y comprobar que se recomienda como máximo un reintento automático y luego un repliegue a ejecución inline, conservando el resultado parcial útil.

**Escenarios de aceptación**:

1. **Dado** un tope de un reintento, **cuando** una tarea delegada falla dos veces, **entonces** no se recomienda un segundo reintento automático.
2. **Dado** un hijo que responde "contexto insuficiente" enumerando lo que le falta, **cuando** hay presupuesto disponible, **entonces** se puede construir un paquete de contexto ampliado y reintentar una sola vez.
3. **Dado** un hijo que vuelve a responder "contexto insuficiente" tras la ampliación, **cuando** se evalúa la política, **entonces** no se produce otra ampliación automática.
4. **Dado** una delegación fallida cuyo trabajo el agente principal puede asumir, **cuando** se evalúa el repliegue, **entonces** se recomienda ejecución inline y se entrega el resultado parcial útil producido.

---

### Historia de usuario 8 - Aprender de lo ya ejecutado (Prioridad: P8)

Como persona que usa Octopus de forma sostenida, quiero que las decisiones mejoren con la evidencia acumulada de ejecuciones anteriores, sin que ese aprendizaje pueda saltarse los límites duros.

**Por qué esta prioridad**: Es una optimización, no un requisito. El sistema debe funcionar bien desde el primer uso y mejorar después.

**Prueba independiente**: Se puede alimentar evidencia histórica agregada para un patrón de tarea y comprobar que la preferencia de delegación cambia, y que aun así una restricción de presupuesto o de capacidad la sigue anulando.

**Escenarios de aceptación**:

1. **Dado** evidencia histórica de que un patrón de tarea consume menos delegado que inline, **cuando** se evalúa una tarea de ese patrón, **entonces** la preferencia por delegar puede aumentar.
2. **Dado** esa misma evidencia, **cuando** el presupuesto, las dependencias, la seguridad o las capacidades del runtime lo impiden, **entonces** la restricción dura prevalece sobre la evidencia.
3. **Dado** el conocimiento de enrutamiento persistido, **cuando** se inspecciona su contenido, **entonces** solo contiene métricas agregadas y patrones reutilizables, nunca razonamiento privado.
4. **Dado** un proyecto sin historial alguno, **cuando** se evalúa el enrutamiento, **entonces** se produce igualmente una decisión válida con política determinista.

---

### Casos límite

- ¿Qué ocurre cuando el módulo está apagado y un runtime pide una decisión de enrutamiento de todos modos? Se responde que la capacidad está desactivada, sin emitir decisión ni telemetría.
- ¿Qué ocurre con un grafo de tareas con dependencias circulares? Se detecta y se informa como entrada inválida en vez de producir un plan con espera perpetua.
- ¿Qué ocurre cuando todas las tareas del plan quedan en `WAIT`? Se declara el bloqueo y las dependencias que lo causan, en lugar de devolver un plan vacío sin explicación.
- ¿Qué ocurre cuando el runtime declara `contexto_aislado = false`? El sobrecosto estimado de delegar aumenta y la preferencia se inclina hacia `INLINE`.
- ¿Qué ocurre cuando el runtime no declara capacidades? Se asume el conjunto más conservador: sin subagentes, y por tanto `INLINE`.
- ¿Qué ocurre cuando el usuario fuerza la delegación pero el runtime no la soporta o la seguridad no la permite? La restricción dura prevalece y la decisión lo explica.
- ¿Qué ocurre cuando dos tareas independientes escriben sobre el mismo artefacto? No se agrupan en paralelo, aunque no exista dependencia declarada entre ellas.
- ¿Qué ocurre cuando el trabajo propuesto ya está hecho, en curso o cubierto por el contexto del agente principal? Se reutiliza ese trabajo en vez de recomendar una delegación equivalente.
- ¿Qué ocurre cuando el presupuesto total configurado es menor que el costo de una sola delegación? Todo queda `INLINE` o `REJECT`, sin delegaciones parciales.
- ¿Qué ocurre cuando el resultado de un hijo excede el presupuesto de integración? Se reduce a un resumen estructurado que preserva conclusiones, evidencia, artefactos y pendientes.
- ¿Qué ocurre cuando el módulo se apaga en mitad de un plan ya enrutado? Las decisiones pendientes dejan de emitirse; el trabajo completado se conserva.

## Requisitos *(obligatorio)*

### Requisitos funcionales

**Módulo y activación**

- **FR-001**: El sistema DEBE exponer Octopus AAR como un módulo con estado propio, apagado por defecto, configurable de forma persistente por proyecto.
- **FR-002**: La pantalla de configuración de la TUI DEBE listar el módulo con el nombre "Octopus AAR", mostrar su estado (activado/desactivado) y permitir alternarlo, confirmando el cambio con un mensaje de estado.
- **FR-003**: Con el módulo apagado, el sistema NO DEBE emitir decisiones de enrutamiento, contratos, presupuestos, métricas ni instrucciones de protocolo atribuibles a Octopus por ninguna vía (TUI, línea de comandos o interfaz de agente).
- **FR-004**: Con el módulo apagado, el comportamiento observable del resto de gomemory DEBE ser idéntico al que tenía antes de esta funcionalidad.
- **FR-005**: El estado del módulo DEBE persistir entre ejecuciones sin volver a configurarlo.

**Decisión de enrutamiento**

- **FR-006**: Dada una unidad de trabajo, su presupuesto restante y las capacidades del runtime, el sistema DEBE producir exactamente una decisión de enrutamiento entre `INLINE`, `DELEGATE`, `WAIT` y `REJECT`, y DEBE poder agrupar unidades delegadas en ejecución paralela.
- **FR-007**: Toda decisión DEBE incluir una razón concisa que describa la política aplicada, sin exponer razonamiento privado del modelo.
- **FR-008**: El sistema DEBE analizar la unidad de trabajo antes de decidir, considerando al menos objetivo, complejidad, dependencias, contexto requerido, alcance afectado, tamaño esperado del resultado, potencial de paralelización, riesgo y capacidades del runtime.
- **FR-009**: La decisión DEBE producirse con reglas deterministas y sin requerir la ejecución de ningún modelo de lenguaje.
- **FR-010**: Para entradas idénticas, la decisión y su razón DEBEN ser reproducibles.
- **FR-011**: El sistema DEBE recomendar `DELEGATE` únicamente cuando el beneficio esperado supere el sobrecosto estimado de delegar; la mera disponibilidad de subagentes NO DEBE bastar.
- **FR-012**: El sistema DEBE permitir clasificar unidades de trabajo por categoría de forma extensible, sin que la decisión dependa exclusivamente de esa clasificación.
- **FR-013**: Antes de recomendar una delegación, el sistema DEBE comprobar si trabajo equivalente ya está completado, en curso, registrado en memoria o cubierto por el contexto del agente principal, y reutilizarlo cuando sea seguro.

**Planes, dependencias y concurrencia**

- **FR-014**: Dado un conjunto de unidades de trabajo con dependencias, el sistema DEBE producir un plan de enrutamiento completo con la decisión de cada unidad.
- **FR-015**: El sistema NO DEBE agrupar en ejecución concurrente unidades con dependencias directas sin resolver ni unidades que muten estado en conflicto.
- **FR-016**: El sistema DEBE respetar el límite de concurrencia declarado por el runtime y el límite configurado en Octopus, tomando el más restrictivo.
- **FR-017**: El sistema DEBE aplicar un tope configurable de agentes delegados por plan.
- **FR-018**: El sistema DEBE aplicar una profundidad máxima de delegación configurable, con valor por defecto 1, y NO DEBE admitir delegación recursiva ilimitada.
- **FR-019**: El sistema DEBE distinguir tareas de ruta crítica de tareas de soporte, validación u opcionales al decidir.
- **FR-020**: El sistema DEBE poder reevaluar un plan cuando cambien dependencias, presupuesto, capacidades del runtime o resultados de ejecución, preservando el trabajo ya completado.
- **FR-021**: El sistema DEBE rechazar como entrada inválida un grafo de tareas con dependencias circulares, señalando el ciclo.

**Contexto y contrato**

- **FR-022**: Toda unidad delegada DEBE recibir un contrato de ejecución acotado con objetivo, alcance, restricciones, permisos requeridos, presupuesto de contexto y forma esperada del resultado, suficiente para terminar de forma independiente.
- **FR-023**: El contexto entregado a una unidad delegada DEBE limitarse a lo relevante para su objetivo y NO DEBE incluir por defecto la conversación completa, las memorias no relacionadas ni el conjunto de planes y especificaciones del proyecto.
- **FR-024**: Cuando exista la capacidad de construcción de paquetes de contexto de gomemory, el sistema DEBE usarla para armar el contexto delegado, priorizando requisitos explícitos de la tarea, artefactos directamente afectados, restricciones del proyecto aplicables, memorias directamente relevantes y salidas de dependencias.
- **FR-025**: Toda unidad delegada DEBE recibir un presupuesto de contexto explícito, y el sistema DEBE preferir el menor presupuesto con el que la tarea pueda completarse; NO DEBE ampliarlo solo porque quede presupuesto global sin usar.
- **FR-026**: Toda unidad delegada DEBE recibir un presupuesto de salida cuando el runtime lo admita, orientado a un resultado estructurado antes que a narrativa extensa.
- **FR-027**: La construcción del contexto delegado DEBE excluir credenciales, secretos, claves y tokens.
- **FR-028**: Un contrato delegado NO DEBE declarar permisos superiores a los disponibles en el flujo principal; delegar NO DEBE elevar privilegios.

**Presupuesto**

- **FR-029**: El sistema DEBE admitir un presupuesto jerárquico de sesión repartido al menos entre agente principal, fondo de delegación y reserva de validación, con proporciones configurables y sin porcentajes impuestos.
- **FR-030**: Toda ruta delegada DEBE evaluarse contra el presupuesto de delegación restante.
- **FR-031**: El sistema NO DEBE consumir la reserva de validación para delegación opcional salvo autorización explícita, ni exceder el presupuesto global en silencio.
- **FR-032**: Cuando no haya conteo exacto de tokens disponible, el sistema DEBE operar con estimaciones que consideren contexto de entrada, contrato, salida esperada, coordinación e integración.
- **FR-033**: Toda cifra de costo o ahorro derivada de estimaciones DEBE presentarse como estimada, nunca como medición exacta.

**Capacidades del runtime y límites**

- **FR-034**: El sistema DEBE adaptar su estrategia a las capacidades declaradas por el runtime y DEBE seguir siendo utilizable cuando falten capacidades opcionales.
- **FR-035**: Cuando el runtime no declare capacidades o declare que no admite subagentes, el sistema DEBE preferir `INLINE`.
- **FR-036**: Cuando el runtime declare que no aísla contexto, el sistema DEBE aumentar el sobrecosto estimado de delegar.
- **FR-037**: El sistema NO DEBE requerir un proveedor de modelo, un modelo concreto ni un framework de agentes específico para producir una decisión.
- **FR-038**: El sistema NO DEBE crear, ejecutar, coordinar ni cancelar subagentes; esa responsabilidad es del runtime.

**Resultados y fallos**

- **FR-039**: El resultado de una unidad delegada DEBE volver al agente principal como salida estructurada y acotada; la transcripción completa del hijo NO DEBE inyectarse por defecto en el contexto del padre.
- **FR-040**: Cuando el resultado exceda el presupuesto de integración, DEBE reducirse a un resumen estructurado que preserve conclusiones, evidencia, artefactos modificados, decisiones, fallos, pendientes y referencias necesarias aguas abajo.
- **FR-041**: El sistema DEBE aplicar un tope configurable de reintentos automáticos de una delegación fallida, con valor por defecto 1, y NO DEBE reintentar de forma indefinida.
- **FR-042**: Cuando un hijo informe contexto insuficiente enumerando lo que le falta, el sistema DEBE poder autorizar una única ampliación acotada del paquete de contexto cuando el presupuesto lo permita.
- **FR-043**: Cuando una delegación falle y el agente principal pueda asumir el trabajo, el sistema DEBE poder recomendar repliegue a ejecución inline y entregar el resultado parcial útil cuando sea seguro.

**Simulación, telemetría y aprendizaje**

- **FR-044**: El sistema DEBE poder producir un plan de enrutamiento en modo simulación, explicando cada ruta, sus presupuestos estimados y su razón, sin iniciar ningún subagente.
- **FR-045**: El sistema DEBE aceptar reportes de ejecución del runtime con consumo real, duración y calidad del resultado, asociados a la decisión que los originó.
- **FR-046**: El sistema DEBE exponer métricas de enrutamiento que permitan distinguir conteos por ruta, consumo estimado y real, reducción de contexto, éxitos, fallos, reintentos, repliegues y ancho de paralelismo.
- **FR-047**: Ninguna métrica ni conocimiento persistido DEBE contener contenido de contexto, transcripciones ni razonamiento privado.
- **FR-048**: El sistema DEBE producir decisiones válidas sin ningún historial previo; el aprendizaje es una optimización, no un requisito.
- **FR-049**: El sistema PUEDE usar evidencia histórica agregada para ajustar sus estimaciones, pero esa evidencia NO DEBE anular restricciones de presupuesto, dependencias, seguridad, capacidades del runtime, fan-out ni recursión.

**Control del usuario e interfaces**

- **FR-050**: El sistema DEBE respetar anulaciones explícitas de política por parte del usuario o del sistema llamante, al menos: delegación desactivada, delegación forzada, tope de subagentes, tope de concurrencia, profundidad máxima, preferencia por inline y presupuesto de tokens.
- **FR-051**: Una delegación forzada DEBE seguir respetando las restricciones duras de seguridad y de capacidades del runtime.
- **FR-052**: El sistema DEBE poder exponerse a agentes compatibles mediante una superficie mínima y estable para solicitar enrutamiento de una tarea, enrutar un plan, informar resultados de ejecución y consultar el estado del enrutamiento; esa superficie NO DEBE ser condición para que la política funcione internamente.
- **FR-053**: El sistema PUEDE ofrecer acceso por línea de comandos a la simulación, el enrutamiento, el estado, el consumo y el historial; la línea de comandos NO DEBE ser condición para que la política funcione.
- **FR-054**: El sistema DEBE poder enrutar cualquier grafo de tareas estructurado; la integración con Spec Kit u otro planificador NO DEBE ser una dependencia dura.

### Invariantes críticas

Estas invariantes son verificables y ninguna implementación puede violarlas:

- **INV-AAR-001**: Una tarea no se delega solo porque exista capacidad de subagentes.
- **INV-AAR-002**: Toda delegación tiene un beneficio esperado que justifica su sobrecosto.
- **INV-AAR-003**: Toda tarea delegada tiene un objetivo acotado.
- **INV-AAR-004**: El contexto delegado está acotado a la unidad de trabajo por defecto.
- **INV-AAR-005**: El presupuesto global de delegación no se excede en silencio.
- **INV-AAR-006**: La reserva de validación no se consume por delegación opcional por defecto.
- **INV-AAR-007**: Las tareas paralelas no tienen dependencias de ejecución sin resolver entre sí.
- **INV-AAR-008**: Los límites de concurrencia del runtime se respetan.
- **INV-AAR-009**: La delegación recursiva está acotada.
- **INV-AAR-010**: El fan-out de agentes está acotado.
- **INV-AAR-011**: Una delegación fallida no reintenta de forma indefinida.
- **INV-AAR-012**: Las transcripciones completas del hijo no se inyectan en el contexto del padre por defecto.
- **INV-AAR-013**: El razonamiento privado no se persiste.
- **INV-AAR-014**: Las decisiones de enrutamiento no eluden las restricciones de seguridad del runtime.
- **INV-AAR-015**: Octopus es utilizable sin aprendizaje persistente.
- **INV-AAR-016**: Octopus es utilizable sin conteo exacto de tokens del proveedor.
- **INV-AAR-017**: Octopus no requiere un proveedor de modelo específico.
- **INV-AAR-018**: Octopus no ejecuta subagentes ni asume responsabilidad de ejecución específica del proveedor.
- **INV-AAR-019**: Con el módulo apagado, Octopus no produce ninguna salida ni efecto observable.

### Entidades clave

- **Unidad de trabajo**: tarea acotada candidata a ejecutarse inline o delegarse. Atributos: identificador, objetivo, tipo, dependencias, alcance, complejidad, riesgo, requisitos de contexto y resultado esperado.
- **Decisión de enrutamiento**: resultado de evaluar una unidad de trabajo. Atributos: unidad, ruta, razón, presupuesto de contexto, presupuesto de salida, grupo paralelo, costo estimado y confianza.
- **Plan de enrutamiento**: conjunto de decisiones para un grafo de tareas, con el presupuesto aplicado y los grupos paralelos formados. Es asesor y revisable, no inmutable.
- **Capacidades del runtime**: lo que el entorno de ejecución declara: soporte de subagentes, paralelismo, aislamiento de contexto, selección de modelo, continuidad de agentes y concurrencia máxima.
- **Paquete de contexto**: contexto mínimo relevante preparado para una unidad de trabajo, con estrategia, presupuesto y elementos requeridos y opcionales.
- **Contrato de ejecución**: descripción estructurada que recibe un subagente: objetivo, alcance, restricciones, permisos, presupuesto de contexto y forma del resultado.
- **Presupuesto**: reparto jerárquico de recursos de la sesión entre agente principal, fondo de delegación y reserva de validación, con consumo estimado y restante.
- **Resultado delegado**: salida estructurada y acotada de una unidad delegada: estado, resumen, evidencia, símbolos afectados, artefactos y pendientes.
- **Reporte de ejecución**: consumo real, duración y calidad informados por el runtime para una decisión concreta.
- **Conocimiento de enrutamiento**: evidencia agregada por patrón de tarea (ejecuciones, consumo medio inline y delegado, tasa de éxito, recomendación), sin razonamiento privado.
- **Política del usuario**: anulaciones explícitas de comportamiento (delegación desactivada o forzada, topes, preferencia por inline, presupuesto).
- **Estado del módulo**: ajuste persistente por proyecto que activa o desactiva toda la capacidad.

## Criterios de éxito *(obligatorio)*

### Resultados medibles

- **SC-001**: Con el módulo apagado, el comportamiento observable del sistema es idéntico al previo a esta funcionalidad: cero salidas, campos o instrucciones nuevas atribuibles a Octopus.
- **SC-002**: El 100 % de las decisiones de enrutamiento se entrega con una razón legible que explica la política aplicada.
- **SC-003**: Un plan de 20 tareas independientes con tope de 4 agentes delegados produce como máximo 4 delegaciones en el 100 % de las ejecuciones.
- **SC-004**: Un plan de hasta 50 tareas se enruta por completo en menos de 1 segundo, sin consultar ningún modelo externo.
- **SC-005**: Sin telemetría histórica previa, el 100 % de las unidades de trabajo recibe una decisión válida.
- **SC-006**: Para entradas idénticas, la ruta y la razón coinciden en el 100 % de las repeticiones.
- **SC-007**: Cero violaciones del presupuesto de contexto declarado en las unidades delegadas.
- **SC-008**: Cero consumos de la reserva de validación por delegación opcional sin autorización explícita.
- **SC-009**: Una persona puede encender o apagar el módulo desde la pantalla de configuración de la TUI en tres pulsaciones o menos, y el estado sobrevive al reinicio.
- **SC-010**: En modo simulación se crean cero subagentes.
- **SC-011**: Cero hallazgos de contenido de contexto, transcripciones, razonamiento privado o credenciales en métricas y conocimiento persistido, verificado por auditoría del material emitido.
- **SC-012**: Para las tareas delegadas, el contexto entregado es al menos un 50 % menor que el contexto completo del agente principal en la mediana de los casos, reportado como estimación cuando no hay medición exacta.
- **SC-013**: Cero grupos paralelos con dependencias sin resolver entre sus miembros y cero grupos que excedan el límite de concurrencia efectivo.
- **SC-014**: Cero reintentos automáticos por encima del tope configurado y cero ampliaciones de contexto por encima de la única ampliación autorizada.

## Supuestos

- El módulo nace apagado (opt-in). Es una capacidad grande y opcional: quien no la active no debe notar diferencia alguna. Se aparta del patrón opt-out de la mayoría de los ajustes del proyecto (sinapsis, brazo extensor de spec-kit) porque aquellos refinan un flujo que ya existe y este es un flujo nuevo completo; el precedente exacto dentro del proyecto es la sincronización de ADR, también en positivo y también apagada por defecto.
- El estado del módulo se guarda en la configuración persistente del proyecto, junto al resto de ajustes de gomemory, y por tanto es por proyecto y no global de la máquina.
- Octopus produce política, no ejecución. Ningún requisito de esta especificación implica crear, lanzar, coordinar ni cancelar procesos de agentes.
- Las cifras de tokens son estimaciones salvo que el runtime informe consumo real. La especificación no supone acceso a ninguna API de conteo del proveedor.
- Los topes por defecto se toman de la descripción de entrada: profundidad máxima de delegación 1, reintentos 1, y topes de agentes por plan y de concurrencia configurables. Se ajustarán con evidencia de uso.
- La política determinista es la primera versión. El enrutamiento asistido por modelo queda fuera del alcance inicial; si se incorpora, la propuesta del modelo sigue sujeta a la validación de presupuesto, dependencias, capacidades, seguridad, fan-out y recursión.
- La revisión adversarial por consenso se contempla como una estrategia de ejecución que Octopus puede seleccionar para trabajo de validación de alto riesgo, pero su implementación no forma parte del alcance de esta funcionalidad.
- La integración con Spec Kit se apoya en el grafo de tareas que ya producen sus comandos; no se modifica ese flujo ni se convierte a Spec Kit en dependencia.
- La superficie de agente (nombres de operaciones) y la de línea de comandos pueden ajustarse durante la implementación sin alterar la política descrita aquí.
- Las capacidades del runtime llegan declaradas por quien llama. Octopus no las detecta ni las infiere del entorno.
