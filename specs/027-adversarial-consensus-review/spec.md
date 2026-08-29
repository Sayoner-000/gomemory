# Especificación de funcionalidad: Revisión Adversarial por Consenso

**Rama de funcionalidad**: `main`

**Creado**: 2026-08-29

**Estado**: Borrador

**Entrada**: Incorporar a GoMemory una capacidad de revisión adversarial basada en dos revisores independientes de solo lectura que analizan exactamente el mismo target congelado, producen hallazgos estructurados con evidencia verificable, y permiten que un motor de consenso determine qué defectos quedan confirmados. Los defectos confirmados de severidad alta o crítica pueden corregirse mediante un agente de corrección separado y alcance mínimo, y se revalidan con una nueva ronda de revisores independientes. GoMemory conserva únicamente el conocimiento reutilizable derivado del proceso (problema, causa raíz, resolución) — nunca el razonamiento completo ni los transcripts de los agentes — y lo hace recuperable en sesiones futuras a través de su mecanismo normal de contexto. El protocolo debe funcionar con cualquier proveedor/modelo y con distintos niveles de aislamiento del runtime, y sus invariantes (congelamiento del target, corroboración independiente, límite de rondas, estados terminales) deben quedar garantizadas por GoMemory, no solo por instrucciones de prompt.

## Escenarios de usuario y pruebas *(obligatorio)*

### Historia de usuario 1 - Validar un cambio antes de darlo por terminado (Prioridad: P1)

Como agente o persona desarrolladora que acaba de implementar o modificar un artefacto técnico (código, especificación, plan, configuración), quiero someterlo a una revisión adversarial de dos revisores independientes antes de considerarlo aprobado, para que ningún defecto significativo dependa únicamente del criterio de quien lo implementó.

**Por qué esta prioridad**: Es el flujo central de la funcionalidad — sin esto no existe revisión adversarial. Todo lo demás (corrección, re-revisión, memoria) depende de que este flujo produzca un veredicto confiable.

**Prueba independiente**: Se puede iniciar una revisión sobre un target conocido (por ejemplo un diff con un defecto deliberado) y comprobar que se congela una única identidad de target, que ambos revisores la reciben sin verse entre sí, y que se obtiene un veredicto final (aprobado, escalado o incompleto) respaldado por un ledger de consenso.

**Escenarios de aceptación**:

1. **Given** un target con un defecto reproducible, **When** ambos revisores lo identifican de forma independiente como el mismo comportamiento y mecanismo causal, **Then** el sistema genera un único hallazgo confirmado que referencia a ambas fuentes.
2. **Given** un target sin defectos significativos, **When** ambos revisores concluyen que no hay hallazgos confirmados de severidad crítica o alta, **Then** el veredicto final es aprobado.
3. **Given** una revisión en curso, **When** el target cambia antes de que ambos revisores entreguen su resultado, **Then** la ronda se invalida y no se calcula un consenso sobre resultados inconsistentes.

---

### Historia de usuario 2 - Corregir solo lo confirmado y volver a verificar (Prioridad: P2)

Como agente responsable de cerrar el ciclo de revisión, quiero que los defectos confirmados de severidad crítica o alta se corrijan con el cambio mínimo suficiente y se vuelvan a verificar con revisores independientes, para evitar tanto la corrección de hallazgos no confirmados como una aprobación falsa después de un intento de arreglo.

**Por qué esta prioridad**: Sin esta historia, la revisión detecta defectos pero no cierra el ciclo — el valor de negocio completo (issue detectado → corregido → verificado) requiere esta segunda mitad del flujo.

**Prueba independiente**: Se puede tomar un hallazgo confirmado de severidad alta, ejecutar la corrección autorizada, y comprobar que el resultado incluye únicamente los archivos relacionados con ese hallazgo, que se generó un registro exacto de los cambios (fix delta), y que una nueva ronda de revisores concluye si el defecto quedó resuelto, sin resolver o si se introdujo una regresión.

**Escenarios de aceptación**:

1. **Given** un hallazgo confirmado de severidad alta o crítica, **When** se autoriza la corrección, **Then** el agente de corrección recibe únicamente el target, el hallazgo confirmado, su evidencia y las reglas del proyecto — nunca hallazgos sospechosos sin autorización explícita.
2. **Given** una corrección aplicada, **When** se ejecuta la re-revisión, **Then** los revisores evalúan exclusivamente si el defecto original fue resuelto, si los invariantes relacionados se mantienen y si hay regresiones causadas por el cambio — no una revisión completa sin límites.
3. **Given** dos rondas de corrección consecutivas sin resolver el defecto, **When** se agota el presupuesto de rondas configurado, **Then** el veredicto final es escalado y no se agregan rondas adicionales de forma silenciosa.
4. **Given** un revisor que falla o no responde durante cualquier ronda, **When** se evalúa el veredicto, **Then** el resultado es incompleto y nunca aprobado.

---

### Historia de usuario 3 - Reutilizar el conocimiento de revisiones pasadas (Prioridad: P3)

Como agente que inicia una nueva implementación, quiero que el conocimiento reutilizable de defectos confirmados y resueltos en revisiones anteriores esté disponible a través del mecanismo normal de contexto de GoMemory, para evitar repetir el mismo patrón de fallo sin tener que consultar el historial completo de revisiones pasadas.

**Por qué esta prioridad**: Es el diferencial de valor a largo plazo de la funcionalidad — convierte revisiones puntuales en conocimiento preventivo — pero depende de que las historias 1 y 2 ya produzcan veredictos y resoluciones confiables.

**Prueba independiente**: Se puede aprobar una revisión con un defecto confirmado y resuelto, comprobar que se genera una memoria estructurada (problema, causa raíz, resolución, verificación) sin transcripts completos, y verificar que una sesión de trabajo posterior la recupera a través del contexto normal sin pedir explícitamente el historial de revisiones.

**Escenarios de aceptación**:

1. **Given** una revisión aprobada con un defecto confirmado y resuelto de forma reutilizable, **When** se ejecuta la promoción de memoria, **Then** se almacena una memoria estructurada con problema, causa raíz, resolución y verificación, sin cadena de razonamiento ni transcripts completos.
2. **Given** conocimiento equivalente ya almacenado sobre el mismo patrón de fallo, **When** se ejecuta la promoción de memoria, **Then** el sistema refuerza o actualiza la memoria existente en lugar de crear una duplicada.
3. **Given** una memoria de revisión ya promovida, **When** una sesión futura consulta el contexto normal del proyecto, **Then** el conocimiento relevante aparece sin necesidad de una consulta especial al historial de revisiones.

---

### Historia de usuario 4 - Consultar el estado y el historial de una revisión (Prioridad: P4)

Como persona desarrolladora que necesita auditar qué se revisó, quiero consultar el estado de una revisión en curso, su historial y el detalle de una revisión específica, para entender qué defecto se detectó, qué corrección se intentó y cuál fue el resultado sin tener que reconstruirlo manualmente.

**Por qué esta prioridad**: Es una capacidad de soporte/auditoría; añade valor una vez que ya existen revisiones registradas por las historias anteriores, pero no es indispensable para el primer ciclo completo de revisión-corrección.

**Prueba independiente**: Se puede consultar el estado de una revisión en curso, listar el historial de revisiones pasadas y ver el detalle de una revisión concreta (target, revisores, hallazgos, consenso, correcciones, veredicto) sin acceder directamente al almacenamiento interno.

**Escenarios de aceptación**:

1. **Given** una revisión en curso, **When** se consulta su estado, **Then** se muestra en qué etapa del protocolo se encuentra (revisando, en consenso, corrigiendo, re-revisando, finalizada).
2. **Given** varias revisiones pasadas, **When** se consulta el historial, **Then** se listan con su identificador, target y veredicto final.
3. **Given** el identificador de una revisión específica, **When** se solicita su detalle, **Then** se muestra la cadena completa de trazabilidad: target, hallazgos, consenso, correcciones y veredicto.

### Casos límite

- ¿Qué ocurre cuando ambos revisores inspeccionan digests de target distintos (por ejemplo, por una condición de carrera al congelar)? El sistema debe rechazar el cálculo de consenso o marcar la revisión como incompleta, nunca inferir una equivalencia.
- ¿Qué ocurre cuando un revisor reporta un hallazgo sin evidencia suficiente? El hallazgo no debe poder promoverse automáticamente a confirmado, independientemente de su severidad declarada.
- ¿Qué ocurre cuando los dos revisores llegan a conclusiones incompatibles sobre el mismo comportamiento? Se clasifica como contradicción y no puede disparar corrección automática por sí sola.
- ¿Qué ocurre cuando el runtime no permite aislamiento real entre revisores (por ejemplo, ejecución secuencial en el mismo contexto)? El resultado debe declarar explícitamente un nivel de independencia degradado y nunca presentarse como una revisión completamente independiente.
- ¿Qué ocurre cuando se reenvía el mismo hallazgo de un mismo revisor para la misma revisión (por reintento de red o de agente)? No debe crear un hallazgo duplicado.
- ¿Qué ocurre cuando un hallazgo confirmado es de severidad media, baja o informativa? No debe dispararse corrección automática salvo autorización explícita configurada.
- ¿Qué ocurre si la corrección introduce una regresión detectada en la re-revisión? El hallazgo relacionado se marca como regresado, no como resuelto, y cuenta para el presupuesto de rondas.
- ¿Qué ocurre cuando una revisión queda aprobada? Eso por sí solo no autoriza ni dispara operaciones de entrega (commit, push, merge, PR, deploy o release); esas acciones requieren autorización separada.

## Requisitos *(obligatorio)*

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir iniciar una revisión adversarial sobre un target concreto, identificable por SHA de commit, hash de árbol, hash de diff, digest de artefacto, conjunto de archivos o revisión de documento.
- **FR-002**: El sistema DEBE generar una representación estable del target cuando no exista un identificador inmutable natural.
- **FR-003**: El sistema DEBE congelar la identidad del target (tipo, revisión, digest, alcance, marca de tiempo) antes de iniciar la revisión, y ambos revisores DEBEN recibir exactamente la misma revisión congelada.
- **FR-004**: El sistema DEBE invalidar la ronda de revisión si detecta que el target cambió después de congelarlo y antes de calcular el consenso.
- **FR-005**: El sistema DEBE requerir exactamente dos resultados de revisor independientes (revisor A y revisor B) antes de avanzar al consenso.
- **FR-006**: El sistema DEBE impedir que un revisor reciba los hallazgos, prompts o resultados parciales del otro revisor durante la evaluación independiente.
- **FR-007**: El sistema DEBE ejecutar a los revisores en modo exclusivamente de lectura: sin edición de archivos, aplicación de parches, commits, push, creación de PR, modificación de memoria ni delegación de correcciones.
- **FR-008**: El sistema DEBE permitir configurar proveedor y modelo por revisor cuando el runtime lo soporte, priorizando diversidad en este orden: familias de modelo distintas, modelos distintos, configuraciones distintas, ejecuciones aisladas del mismo modelo.
- **FR-009**: El sistema DEBE registrar y exponer el nivel de independencia efectivamente alcanzado en cada revisión (pleno o degradado, con motivo), y NUNCA DEBE presentar una revisión degradada como una revisión multiagente completamente independiente.
- **FR-010**: El sistema DEBE requerir que cada hallazgo enviado por un revisor incluya ubicación, severidad, categoría, afirmación, clase de evidencia, evidencia concreta y nivel de confianza.
- **FR-011**: El sistema DEBE restringir la severidad de un hallazgo a los niveles CRITICAL, HIGH, MEDIUM, LOW e INFO, cada uno con un significado documentado.
- **FR-012**: El sistema DEBE restringir la clase de evidencia a un conjunto fijo (determinística, reproducida, contractual, análisis estático, fallo de prueba, observación en ejecución, probabilística).
- **FR-013**: El sistema NO DEBE promover automáticamente a confirmado un hallazgo sin evidencia suficiente, sin importar la severidad declarada.
- **FR-014**: El sistema DEBE comparar los hallazgos independientes de ambos revisores y clasificar cada uno como confirmado, sospechoso, contradicción o informativo, sin que el motor de consenso introduzca defectos nuevos.
- **FR-015**: El sistema DEBE considerar dos hallazgos equivalentes cuando describen el mismo comportamiento, el mismo mecanismo causal y un alcance afectado compatible — nunca basándose únicamente en similitud textual.
- **FR-016**: El sistema DEBE clasificar como sospechoso un defecto identificado por un solo revisor, y este estado NO DEBE poder disparar una corrección automática.
- **FR-017**: El sistema DEBE clasificar como contradicción las conclusiones incompatibles de ambos revisores sobre el mismo comportamiento.
- **FR-018**: El sistema DEBE producir, por cada revisión, un ledger de consenso estructurado con la identidad del target, la identidad de los revisores y la clasificación confirmado/sospechoso/contradicción/informativo con referencia a sus hallazgos de origen.
- **FR-019**: El sistema DEBE aplicar una política de auto-corrección configurable, limitada por defecto a hallazgos confirmados de severidad CRITICAL o HIGH; los hallazgos MEDIUM, LOW, INFO y los sospechosos NO DEBEN corregirse automáticamente.
- **FR-020**: El sistema DEBE restringir al agente de corrección a recibir únicamente el target, el o los hallazgos confirmados autorizados, su evidencia y las reglas del proyecto aplicables — nunca hallazgos sospechosos sin autorización explícita.
- **FR-021**: El sistema DEBE exigir que toda corrección siga el principio de cambio mínimo suficiente: sin limpieza oportunista, sin refactors de componentes no relacionados, sin cambios de API innecesarios, sin abordar hallazgos no autorizados y sin ampliar el alcance original.
- **FR-022**: El sistema DEBE registrar, tras cada corrección, un registro de cambios (fix delta) con el digest del target base, el digest del target corregido, los hallazgos abordados, las rutas modificadas, los pasos de verificación y el digest del diff.
- **FR-023**: El sistema DEBE ejecutar, tras cada corrección, dos nuevas evaluaciones de revisor independientes limitadas a verificar que el defecto original fue resuelto, comprobar invariantes relacionados y detectar regresiones causadas directamente por la corrección — no una revisión completa sin límites.
- **FR-024**: El sistema DEBE clasificar el estado posterior a la corrección de cada hallazgo confirmado como resuelto, sin resolver o regresado.
- **FR-025**: El sistema DEBE aplicar un presupuesto máximo configurable de rondas de corrección (2 por defecto) y NO DEBE excederlo de forma automática ni silenciosa.
- **FR-026**: El sistema DEBE finalizar toda revisión en exactamente uno de tres estados terminales: aprobada, escalada o incompleta.
- **FR-027**: El sistema DEBE reservar el estado aprobada para revisiones sin hallazgos confirmados de severidad CRITICAL o HIGH sin resolver.
- **FR-028**: El sistema DEBE producir el estado escalada cuando exista un defecto severo sin resolver, una contradicción severa, agotamiento del presupuesto de rondas, o una decisión que requiera intervención humana.
- **FR-029**: El sistema DEBE producir el estado incompleta cuando el protocolo no pudo ejecutarse correctamente (revisor no disponible, target inaccesible, target modificado durante la revisión, respuesta de revisor inválida o evidencia insuficiente).
- **FR-030**: El sistema DEBE tratar el fallo de cualquiera de los dos revisores como resultado incompleto, nunca como una aprobación implícita.
- **FR-031**: El sistema NO DEBE almacenar automáticamente cadena de razonamiento, prompts en bruto, transcripts completos de los revisores, especulación temporal ni hallazgos duplicados como parte de la memoria de revisión.
- **FR-032**: El sistema DEBE permitir que una revisión aprobada (u opcionalmente una escalada de alto valor) produzca una memoria estructurada y reutilizable con problema, causa raíz, resolución, verificación y nivel de confianza, vinculada a la revisión y a la revisión del target de origen.
- **FR-033**: El sistema DEBE promover a memoria permanente únicamente hallazgos confirmados, resueltos y reutilizables, u opcionalmente confirmados, escalados y de alto valor para conservar una decisión pendiente importante.
- **FR-034**: El sistema DEBE comprobar si existe conocimiento equivalente antes de almacenar una nueva memoria de revisión, y DEBE preferir actualizar o reforzar el conocimiento existente sobre crear una memoria duplicada.
- **FR-035**: El sistema DEBE hacer recuperable el conocimiento derivado de revisiones a través del mecanismo normal de contexto del proyecto, sin requerir una consulta especial al historial de revisiones.
- **FR-036**: El sistema DEBE ofrecer una forma de iniciar una revisión, enviar resultados de revisor, consultar el estado, obtener el consenso, registrar una corrección y finalizar la revisión.
- **FR-037**: El sistema DEBE ofrecer una vía de línea de comandos para iniciar una revisión sobre un diff, un commit o un archivo/ruta, y para consultar el estado, el historial y el detalle de una revisión específica, sin que la propia interfaz de línea de comandos ejecute modelos.
- **FR-038**: El sistema DEBE conservar, con trazabilidad completa entre sí, como mínimo: la revisión, el target, las ejecuciones de revisor, los hallazgos, los mapeos de consenso, las rondas de corrección, el veredicto y las promociones de memoria.
- **FR-039**: El sistema DEBE tratar como idempotente el reenvío del mismo hallazgo del mismo revisor para la misma revisión, sin crear duplicados.
- **FR-040**: El sistema DEBE permitir que las ejecuciones del revisor A y del revisor B ocurran de forma concurrente, pero NO DEBE producir un veredicto final hasta disponer de ambos resultados válidos.
- **FR-041**: El sistema NO DEBE otorgar a los revisores permisos de escritura a través de ninguna capacidad que provea, y DEBE excluir de todo lo que persiste secretos, tokens, credenciales, cadena de razonamiento e información sensible innecesaria.
- **FR-042**: El sistema DEBE registrar métricas suficientes para analizar el protocolo, como mínimo: duración, hallazgos totales/confirmados/sospechosos, contradicciones, rondas de corrección, veredicto final, y memorias promovidas/deduplicadas.
- **FR-043**: Una revisión aprobada, por sí sola, NO DEBE autorizar ni disparar operaciones de commit, push, merge, PR, deploy o release.
- **FR-044**: El sistema DEBE distribuir una guía de participación en el protocolo (skill) que enseñe al agente orquestador la secuencia completa —resolver el target, congelar su identidad, crear las ejecuciones independientes, obtener resultados estructurados, registrar hallazgos, solicitar consenso, ejecutar correcciones autorizadas, realizar la re-revisión, finalizar y promover conocimiento—, instalándola por los mismos medios que el resto de guías que ya distribuye, sin requerir edición manual de archivos de instrucciones del agente.
- **FR-045**: La guía de participación DEBE permanecer independiente del proveedor (sin instrucciones específicas de un agente o modelo concreto, salvo adaptadores opcionales) y NO DEBE ser la única sede de ninguna invariante del protocolo: toda regla que el sistema pueda verificar por sí mismo se verifica en el sistema, no en el texto de la guía.

### Entidades clave

- **Target**: artefacto congelado sometido a revisión (tipo, revisión, digest, alcance, marca de tiempo); puede ser un commit, un diff, un archivo o conjunto de archivos, una especificación, un plan, una configuración, una arquitectura, una migración, un contrato de API o una implementación completa.
- **Revisor (A/B)**: ejecución independiente de solo lectura responsable exclusivamente de analizar el target y producir hallazgos, sin ver el resultado del otro revisor.
- **Hallazgo**: observación estructurada de un revisor con ubicación, severidad, categoría, afirmación, clase de evidencia, evidencia y confianza.
- **Evidencia**: soporte concreto de un hallazgo, clasificado como determinística, reproducida, contractual, de análisis estático, de fallo de prueba, de observación en ejecución o probabilística.
- **Motor de consenso**: proceso que normaliza y compara los hallazgos independientes de ambos revisores para clasificarlos, sin generar hallazgos nuevos por sí mismo.
- **Hallazgo confirmado**: defecto identificado independientemente por ambos revisores y considerado equivalente por el motor de consenso.
- **Hallazgo sospechoso**: defecto identificado por un único revisor; no puede disparar corrección automática.
- **Contradicción**: par de conclusiones incompatibles de ambos revisores sobre el mismo comportamiento.
- **Agente de corrección (Fix Actor)**: entidad independiente de los revisores autorizada a modificar el target exclusivamente para resolver hallazgos confirmados y autorizados, siguiendo el cambio mínimo suficiente.
- **Registro de cambios (Fix Delta)**: conjunto exacto de modificaciones de una corrección, con target base, target corregido, hallazgos abordados, rutas modificadas, verificación y digest del diff.
- **Ledger de consenso / linaje de revisión**: registro que conecta target, revisión, hallazgos, consenso, corrección, re-revisión y veredicto, permitiendo reconstruir qué se revisó y qué resultó.
- **Veredicto**: estado terminal de una revisión (aprobada, escalada o incompleta).
- **Memoria de revisión**: conocimiento reutilizable extraído de una revisión aprobada o de alto valor (problema, causa raíz, resolución, verificación, confianza), sin razonamiento completo ni transcripts.

## Criterios de éxito *(obligatorio)*

### Resultados medibles

- **SC-001**: En el 100% de los casos de prueba de aceptación en los que ambos revisores identifican de forma independiente el mismo defecto, el sistema produce exactamente un hallazgo confirmado referenciando a ambas fuentes.
- **SC-002**: En el 100% de los casos en los que uno de los dos revisores falla o no responde, el veredicto final nunca es "aprobado".
- **SC-003**: El número de rondas de corrección ejecutadas nunca supera el límite configurado sin que la revisión termine explícitamente en estado "escalada".
- **SC-004**: El conocimiento derivado de una revisión aprobada aparece disponible en el contexto normal de una sesión de trabajo posterior sin que la persona o el agente tengan que consultar el historial completo de revisiones.
- **SC-005**: Ninguna memoria persistida por el proceso de revisión contiene razonamiento completo (cadena de pensamiento) ni transcripts íntegros de los agentes revisores, verificado por muestreo en el 100% de las revisiones aprobadas evaluadas.
- **SC-006**: Cuando ya existe conocimiento equivalente sobre un mismo patrón de fallo, la promoción de memoria refuerza o actualiza el registro existente en más del 90% de los casos evaluados, en lugar de crear uno duplicado.
- **SC-007**: Toda revisión finaliza en exactamente uno de los tres estados terminales definidos (aprobada, escalada, incompleta), sin estados indefinidos u omitidos, en el 100% de las ejecuciones evaluadas.
- **SC-008**: Un agente compatible que recibe la guía de participación completa el protocolo de punta a punta sin que se le expliquen los pasos en la conversación, y ninguna invariante del protocolo depende de que el texto de esa guía se respete: al eliminar la guía, los intentos de saltarse una ronda, corregir un hallazgo no confirmado o declarar un veredicto por cuenta propia siguen siendo rechazados.

## Suposiciones

- El runtime que ejecuta el protocolo soporta, como mínimo, ejecuciones aisladas de subagentes en modo secuencial; cuando no soporta paralelismo real o selección de modelo, el sistema degrada el nivel de independencia y lo declara explícitamente en vez de simularlo.
- No se exige que los dos revisores usen proveedores o modelos distintos: dos ejecuciones aisladas del mismo modelo son un nivel de independencia válido, aunque degradado, siempre que se declare como tal.
- El presupuesto por defecto de rondas de corrección es de 2 rondas, configurable por proyecto; al agotarse sin resolver el defecto, la revisión escala en vez de continuar indefinidamente.
- Por defecto, solo los hallazgos confirmados de severidad CRITICAL o HIGH son candidatos a corrección automática; el resto requiere autorización explícita o intervención humana, conforme a la política configurable.
- Cuando el target no tiene un identificador inmutable natural (por ejemplo, una especificación aún no versionada), el sistema genera una representación estable basada en el contenido evaluado.
- GoMemory ya expone un mecanismo de sesión y contexto (equivalente a `get_context`/`save_memory`) que esta funcionalidad reutiliza para la extracción y recuperación de conocimiento, sin introducir un sistema de memoria paralelo ni duplicar instrucciones estáticas en archivos de agente.
- Esta funcionalidad no decide ni ejecuta por sí sola operaciones de entrega (commit, push, merge, PR, deploy o release); cualquier acción de ese tipo requiere autorización separada, conforme a las reglas de trabajo ya vigentes en el proyecto.
- El detalle de arquitectura interna (organización de paquetes, herramientas MCP concretas, esquemas de persistencia) se define en la fase de planificación (`plan.md`) y no forma parte de esta especificación.
