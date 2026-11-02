# Compresión automática de contexto para Codex, OpenCode y Claude Code

ID: HCD-014
Tipo: evolution
Carril: full
Motivo: Hay decisiones interdependientes sobre cobertura de sesión, mecanismos por agente y conservación recuperable del contexto.

## Intención
Extender gomemory para reducir automáticamente el contexto enviado al modelo durante sesiones de Codex, OpenCode y Claude Code, con recuperación verificable, diagnóstico de cobertura y medición del ahorro neto.


## Alcance
Incluido: automatización, diagnóstico y validación de compresión durante sesiones de los tres agentes, con los límites siguientes.
- Compresión automática durante sesiones de Codex, OpenCode y Claude Code, sin depender de que el modelo invoque manualmente `pack_compress` en cada turno.
- Inventario verificable por versión y modalidad de ejecución: respuestas propias, herramientas externas, historial y petición saliente al modelo. El objetivo es cubrir la sesión; una integración parcial debe declararse parcial y no basta para afirmar cobertura integral.
- Reutilización del motor Go, selección por presupuesto, almacén de originales, recibos y contabilidad existentes. Los mecanismos específicos de cada agente pertenecen a sus adaptadores.
- Recuperación exacta del contenido elegible omitido, continuidad tras la compactación del host y conservación de instrucciones, contratos de herramientas y material que requiera lectura exacta.
- La recuperación automática se limita al proyecto y a una identidad estable de sesión, conservada al compactar o reanudar esa misma sesión (Q3, opción A). La recuperación manual existente por CLI/MCP conserva su alcance por proyecto.
- Diagnóstico que distinga configuración, ejecución del mecanismo, sustitución consumida por el host y cobertura real del contenido enviado.
- Comprobaciones reproducibles de ahorro neto, calidad de la tarea, recuperación y latencia en los tres agentes.
- Evaluar mecanismos nativos y una ruta opcional de envío mediante proxy local cuando aquellos no permitan cubrir toda la sesión (Q1, opción A). La evaluación no activa tráfico ni cambia credenciales; la viabilidad debe comprobarse por versión, modalidad y autenticación del agente antes de proponer su activación.

Excluido: sustitución de sistemas existentes y cambios ajenos al control de contexto descrito en esta unidad.
- Sustituir la memoria persistente, HCD o el grafo de código; duplicar capacidades ya existentes sin una carencia reproducida.
- Dar por controlado el historial o el tráfico que no atraviese una integración comprobada.
- Alterar archivos fuente originales, cambiar modelos o credenciales, o activar nuevas rutas de tráfico sin una decisión explícita.
- Presentar estimaciones locales como ahorro facturado por el proveedor, o garantizar ahorro en contenido que no admita reducción segura.
- Adoptar Entroly como dependencia por el solo hecho de servir de referencia.

## ¿Qué pasa si no lo hacemos?
Gomemory seguirá reduciendo sus propias respuestas y algunas salidas de herramientas, pero no ofrecerá una política automática con cobertura comprobada de la sesión en los tres agentes. El volumen de herramientas externas y del historial continuará dependiendo del host; activar `max` no cerrará esa diferencia. La disponibilidad de un hook o de una herramienta MCP podrá confundirse con ahorro efectivo si no se observa qué consume el modelo.

## Alternativa más barata
Conservar las integraciones actuales, mejorar el diagnóstico y usar recuperación acotada en origen, presupuestos y herramientas de compresión explícitas. Esta alternativa cuesta menos y puede reducir mucho el contexto de gomemory, pero no satisface por sí sola el objetivo solicitado de compresión automática de toda la sesión. Antes de ampliar la arquitectura se medirá esa línea base; no se presupone que hace falta reemplazar el motor.

## Base comprobada y normas

- Revisión del 2026-11-02: gomemory 2.43.1, nivel `max`, almacén de originales escribible; Codex 0.166.0 y OpenCode 2.2.5. El soporte actual de sustitución externa de Codex no se verificó; gomemory lo mantiene deshabilitado por rechazos históricos.
- `application/usecases/build_context_pack.go`: selección por consulta y presupuesto, deduplicación, protección de elementos críticos y recibos. `domain/context_pack.go`: huellas del original y lo entregado, referencias y contabilidad diferenciada. Son capacidades existentes que esta evolución debe aprovechar.
- `adapters/primary/cli/context_output.go`: compresión en origen y delta. `adapters/primary/cli/hook_tool_output.go`: rutas de sustitución por agente. `infrastructure/plugin/opencode/gomemory.ts`: adaptación v1/v2 de resultados y continuidad de compactación.
- El diagnóstico identifica Claude activo y OpenCode como mejor esfuerzo. Eso verifica configuración, no prueba consumo final ni ahorro de sesión.
- Entroly, commit `8a7bb45b71ddf2c0b2b9ecab58914ddc550798cb`, sirve de referencia: su hook Codex añade selección en `UserPromptSubmit`; su rescate de sesión necesita que el tráfico atraviese el proxy. Ninguna de esas rutas demuestra cobertura universal por MCP.
- Constitución vigente: §§3–4 (capas y composición), §§8–9 (errores y configuración), §10 (pruebas), §12 (observabilidad), §§15 y 20 (privacidad y seguridad), §18 (documento HCD único), §21 (simplicidad, contratos y evidencia).
- Playbook: HCD-FLOW-01/02/03 (carril, aclaraciones y aprobación), HCD-DEF-01 (definición), HCD-TRACE-01 (criterios observables), HCD-EVIDENCE-01 (evidencia vigente) y HCD-CLEAN-01–04 (límites del núcleo).
- Las pruebas focalizadas de la revisión anterior son antecedentes; no son evidencia de implementación ni de cierre de esta unidad.

## Historias
### US1. Reducir contexto automáticamente durante la sesión
Como persona que usa Codex, OpenCode o Claude Code, quiero que gomemory reduzca automáticamente el contexto elegible antes de que lo consuma el modelo, para evitar volumen redundante sin invocar herramientas de compresión manualmente en cada turno.
- C-01: Dado cada agente en una versión y modalidad registradas, cuando una sesión produzca contenido elegible voluminoso, entonces la ruta acordada entregará contenido reducido sin una invocación manual del modelo y permitirá comprobar qué representación consumió el host.
- C-02: Dado un contenido pequeño, no reducible o una lectura que requiera exactitud, cuando se evalúe para compresión, entonces se conservará íntegro y el diagnóstico explicará la exclusión o ausencia de ganancia.

### US2. Conocer la cobertura real por agente
Como persona que configura gomemory, quiero distinguir qué partes de la sesión están cubiertas y qué mecanismos funcionan realmente, para no interpretar una instalación correcta como compresión integral.
- C-03: Dado un agente y su modalidad, cuando se consulte el diagnóstico, entonces se distinguirán respuestas propias, herramientas externas, historial y petición saliente, con estados comprobado, parcial, no disponible o no verificado y su motivo.
- C-04: Dado un hook instalado que el host ignore o cuyo resultado no consuma, cuando se compruebe la integración, entonces no se informará compresión efectiva ni cobertura integral basándose solo en su registro o ejecución.

### US3. Recuperar originales y conservar el contrato de la sesión
Como persona que delega trabajo al agente, quiero recuperar exactamente el material omitido y conservar las instrucciones y los contratos, para que reducir contexto no destruya la evidencia necesaria para trabajar.
- C-05: Dado un bloque elegible omitido con referencia, cuando se recupere automáticamente desde su proyecto y la identidad estable de su sesión, entonces se obtendrá la representación original protegida cuya huella coincida con el recibo; otra sesión no podrá recuperarlo por esa vía y no se emitirán referencias cuyo original no se haya guardado. Compactar o reanudar la misma sesión conserva su identidad y acceso. La recuperación manual existente por CLI/MCP continúa limitada por proyecto (Q3, opción A).
- C-06: Dado un fallo de compresión o de almacenamiento, cuando no se pueda asegurar recuperación, entonces el contenido elegible original continuará disponible, sin sustitución destructiva ni ocultamiento del fallo; se mantendrán las reglas de protección de secretos.
- C-07: Dado un flujo con herramientas y compactación del host, cuando se active la reducción, entonces se conservarán la correspondencia de llamadas y resultados, las instrucciones obligatorias y la recuperación del contexto necesario, sin suponer que gomemory sustituye la compactación del host.

### US4. Medir el beneficio y la calidad
Como responsable de gomemory, quiero comparar sesiones equivalentes con y sin compresión automática, para aceptar la evolución mediante evidencia y no solo porcentajes de reducción del motor.
- C-08: Dado un corpus versionado y condiciones registradas por agente, cuando se comparen ambas modalidades, entonces el informe separará selección, compresión, delta, metadatos y recuperaciones posteriores, declarará el contador utilizado y distinguirá estimaciones locales de uso observado del proveedor.
- C-09: Dadas tareas con resultados verificables y un corpus reducible versionado, cuando se comparen ambas modalidades, entonces se exigirá ahorro neto positivo por agente, sin regresiones en los resultados verificables de las tareas ni fallos de recuperación, y se medirá y reportará la latencia adicional (Q2, opción A). No se declarará éxito solo por reducir texto ni se extrapolará este resultado a cualquier sesión.

## Aclaraciones
- [x] Q1. ¿Puede esta unidad evaluar una ruta opcional de envío mediante proxy local si un agente no permite cubrir toda la sesión con sus mecanismos nativos? Opciones: A) Evaluar integración nativa y proxy opcional cuando sea necesario (recomendada para perseguir cobertura integral, sin activar tráfico todavía). B) Solo mecanismos nativos, aceptando que puede quedar cobertura parcial. C) Exigir cobertura integral nativa y detener la propuesta para cualquier agente que no la admita. → A
- [x] Q2. ¿Qué criterio de aceptación usamos para el beneficio de la compresión? A) Ahorro neto positivo por agente sobre un corpus reducible versionado, sin regresiones en los resultados verificables de las tareas ni fallos de recuperación, y con latencia adicional medida y reportada (recomendada: exige beneficio real sin inventar porcentajes antes de medir). B) Fijar ahora mínimos de ahorro y máximos de latencia; indicar los valores y la tolerancia de calidad antes de aprobar. → A
- [x] Q3. Q3: La tarea 17 exige aislamiento entre sesiones, pero RetrieveOriginal y el almacén actuales solo separan por proyecto y permiten recuperar referencias en sesiones posteriores. ¿Qué alcance debe tener la nueva restricción? A) Aislar la recuperación automática por una identidad estable de sesión que sobreviva a compactación/reanudación, manteniendo la recuperación manual existente por proyecto (recomendada: incorpora aislamiento automático sin romper CLI/MCP actuales). B) Aislar también toda recuperación manual por sesión, modificando los contratos CLI/MCP existentes. C) Mantener referencias compartidas entre sesiones del mismo proyecto y revisar el criterio de aislamiento aprobado. → A
- [x] Q4. Q4: Para las comparaciones reales de las tareas 21–23, ¿qué modalidad debemos evaluar? A) Conservar las rutas habituales: Codex con suscripción ChatGPT, Claude con su autenticación habitual y OpenCode con Ollama; evaluar primero la captura nativa disponible, sin copiar tokens ni redirigir tráfico real (recomendada para mantener las condiciones actuales; la captura completa y el modelo de OpenCode aún deben concretarse). B) Usar cuentas API dedicadas y modelos explícitos elegidos por la persona; preparar después la propuesta exacta de captura y reversión antes de activar cualquier ruta real. Indica A o B; no compartas claves ni tokens. → y probar con las sesiones de opencode y claude
para probar lo que falta ...
- [x] Q5. Q5: ¿se autoriza un grabador local temporal en 127.0.0.1 que reenvíe sin cambios al proveedor habitual y guarde solo cuerpos para capturar el corpus, y ajustar el comparador para aceptar la retirada de saltos finales declarada por el host? → La persona respondió literalmente: «autorizo.. y ajusta lo necesario». Se autoriza el grabador local temporal propuesto y el ajuste del comparador para la retirada de saltos finales declarada por el host.

## Slices
Las tareas de cada incremento se detallan en el Checklist; su estado y evidencia vigente los determina el motor. Sus dependencias son obligatorias. Los incrementos cubren todos los criterios y preservan la definición aprobada. La evidencia de capacidades precede al diseño del adaptador que dependa de ella.

### VS-01. Consultar cobertura real de compresión por agente
Tareas: 1, 2, 3, 4, 5, 6
Criterios: C-03, C-04
Dependencias: ninguna.
Entrega: diagnóstico observable para Codex, OpenCode y Claude Code que diferencia configuración, ejecución y consumo comprobado, e identifica cobertura de respuestas propias, herramientas externas, historial y petición saliente por versión y modalidad. Una prueba de mecanismo ignorado por el host nunca produce un estado de compresión efectiva.
Base de código: diagnóstico de compresión en CLI y declaraciones de integración por agente; reutilizar esos bordes antes de crear otro sistema de activación.
Comprobación del incremento: informe reproducible por agente y prueba negativa de hook registrado pero no consumido. La captura de evidencia registra versiones, modalidad y límite observado, sin datos sensibles.

### VS-02. Obtener salidas reducidas automáticamente mediante mecanismos nativos compatibles
Tareas: 7, 8, 9, 10, 25
Criterios: C-01, C-02, C-04
Dependencias: VS-01.
Entrega: la ruta nativa cuya capacidad se haya comprobado reduce contenido elegible antes de su consumo, sin llamadas manuales a `pack_compress`; conserva lecturas exactas y contenidos sin ganancia. Los agentes o modalidades sin sustitución admitida mantienen una cobertura explícitamente parcial, pendiente de VS-03, sin simular compatibilidad.
Base de código: `hook_tool_output.go`, entrega comprimida en origen y plugin OpenCode v1/v2. La desactivación de sustitución externa en Codex solo se reconsidera con evidencia de su contrato vigente.
Comprobación del incremento: sesión controlada por cada ruta nativa disponible que contraste texto producido y texto consumido, junto con casos de exclusión y ausencia de ganancia.

### VS-03. Comprobar una ruta opcional para cubrir herramientas e historial fuera de los hooks nativos
Tareas: 11, 12, 13, 14
Criterios: C-01, C-03, C-04, C-07
Dependencias: VS-01; utiliza los límites observados en VS-02.
Entrega: prueba local de viabilidad de una ruta de envío controlada, con proveedor de prueba, que permita observar reducción automática de herramientas e historial elegible y conservación de la estructura de mensajes. Se entrega una decisión documentada por agente, modalidad y autenticación: ruta viable, restricción comprobada o ausencia de cobertura.
Este incremento habilita la cobertura que los mecanismos nativos no proporcionen. No fija todavía un proxy propio ni una dependencia externa: el diseño concreto depende de la evidencia anterior y debe descomponerse antes de ejecutar sus tareas. Evaluar no implica activar tráfico real ni cambiar credenciales. Si la ruta resulta viable, su propuesta de activación identificará los cambios concretos; una decisión de la persona precederá cualquier redirección real. Una limitación sin resolver impide declarar satisfecha la cobertura integral de la unidad y exige tratar su cierre de alcance con la persona.
Comprobación del incremento: peticiones sintéticas capturadas antes y después de la transformación, sin alterar llamadas y resultados de herramientas, instrucciones obligatorias ni el historial original del host; registrar explícitamente qué tráfico queda fuera.

### VS-04. Recuperar originales y continuar la sesión ante fallos y compactación
Tareas: 15, 16, 17, 18
Criterios: C-02, C-05, C-06, C-07
Dependencias: VS-02 y la ruta local evaluada en VS-03.
Entrega: recuperación verificable y continuidad de los caminos anteriores, reutilizando el almacén y los recibos existentes. Una reducción solo publica referencias a originales protegidos persistidos; si falla la compresión o el almacenamiento, se conserva el contenido sin sustitución destructiva. Tras compactar, el contexto necesario y las referencias siguen siendo utilizables dentro de su alcance.
Base de código: motor nativo, almacén de originales, recibos y mecanismos actuales de recuperación de contexto y reinicio del delta.
Comprobación del incremento: recuperación por huella, fallos inyectados de compresión y almacenamiento, aislamiento de la recuperación automática entre proyectos y sesiones, compatibilidad de la recuperación manual por proyecto y recorrido de compactación con preservación de contratos y material de lectura exacta.

### VS-05. Comparar el beneficio completo en los tres agentes
Tareas: 19, 20, 21, 22, 23, 24
Criterios: C-01, C-03, C-08, C-09
Dependencias: VS-01, VS-02, VS-03 y VS-04; las mediciones en rutas reales requieren que su activación haya sido acordada cuando corresponda.
Entrega: comparación reproducible de sesiones equivalentes con y sin automatización sobre un corpus reducible versionado, con resultados por agente y modalidad. Se distinguen selección, compresión, delta, metadatos y recuperaciones, así como estimaciones locales y uso observado del proveedor. El resultado incluye corrección de las tareas y latencia adicional.
Comprobación del incremento: ahorro neto positivo por agente, sin regresiones verificables ni fallos de recuperación. Una ruta no ejercida o no viable se informa como cobertura pendiente; ni una prueba sintética ni un porcentaje de reducción del motor bastan para declarar completa la unidad.

## Checklist

Reglas para ejecutar las comprobaciones:
- Los comandos y nombres de pruebas HCD014 son contratos de los artefactos por construir, no comprobaciones ejecutadas en esta fase. Cada prueba nueva deberá existir y haberse ejecutado: una salida sin pruebas seleccionadas no acredita nada. Los ejecutores nuevos terminarán con error si no ejecutan casos, faltan capturas requeridas o se usa una modalidad distinta de la declarada.
- Las pruebas del host utilizarán un proyecto desechable, contenido sintético y configuración aislada; no editarán la configuración habitual ni incorporarán credenciales, prompts privados o datos personales a la evidencia. Se reutilizarán los patrones de aislamiento de `scripts/audit_037_smoke.py` y el etiquetado de visibilidad de `scripts/hcd001_evaluate.py`.
- Los registros de capacidades incluirán agente, versión, modalidad, mecanismo, clase de contenido, configuración observada, ejecución observada, consumo comprobado, huellas y motivo. Una captura de stdout de gomemory no demuestra entrada al modelo. Si no se puede observar ese borde, se informa no verificado y no se afirma cobertura integral.
- Las comprobaciones manuales se registrarán mediante `mem hcd verify --manual`, con la observación concreta y el artefacto protegido que la sustenta. No se marcarán casillas a mano.
- Las tareas 8–10 dependen de los informes 2–4: se trabaja únicamente sobre contratos nativos comprobados. Si esos informes exigen un diseño aún no descompuesto, se vuelve a `/hcd-tasks` antes de editar ese adaptador; no se interpreta la tarea como permiso para improvisar un mecanismo incompatible.
- Autoverificación de atomicidad: las tareas 12 y 13 quedaron concretadas tras las sondas 2–4 y el informe 11: banco loopback con `httptest`, protocolos Responses/Chat Completions/Messages y transformación limitada a resultados textuales de shell asociados a llamadas conocidas. Se reutiliza el puerto `Compressor`; no se instala un proxy ni se redirige tráfico real. El historial elegible se limita a esos resultados de herramientas, conservando prosa, llamadas, lecturas exactas y formas desconocidas. La tarea 10 queda concretada por la sonda 2; la instalación y restricción de versión se separan en 25. Las tareas 8 y 9 quedaron concretadas por las sondas de las tareas 3 y 4: los adaptadores actuales ya producen las formas admitidas, por lo que se validan con los contratos existentes sin refactorizar solo para aparentar un cambio. Las restantes tareas tienen un artefacto o una propiedad observable delimitados.
- Las tareas 21–23 requieren disponer de una ruta que permita observar el consumo real. Una activación opcional requiere primero la decisión sobre la propuesta de la tarea 14. Si no hay una ruta viable, se conserva la tarea pendiente y se trata el límite con la persona; no se transforma una restricción en un resultado aprobado.

- [x] 1. Definir el registro de evidencia de cobertura por agente en `application/ports` con los campos descritos y estados comprobado, parcial, no disponible y no verificado → `go test ./application/usecases -run '^TestHCD014CoverageEvidenceContract$' -count=1`, con casos que rechacen promover configuración o ejecución a consumo comprobado.
- [x] 2. Capturar el contrato nativo de Codex instalado en `docs/evidence/hcd014/codex-native.md` (dep: 1) → observación manual de una sesión interactiva aislada con herramienta externa sintética: versión, modalidad, entrada/salida del mecanismo, representación que consume el host o límite exacto que impide comprobarlo; distinguir expresamente `codex exec` de la modalidad interactiva.
- [x] 3. Capturar el contrato nativo de OpenCode instalado en `docs/evidence/hcd014/opencode-native.md` (dep: 1) → observación manual de una sesión aislada con salida sintética y captura del resultado consumido; declarar versión y forma v1/v2, sin inferir consumo por la sola mutación del objeto del plugin.
- [x] 4. Capturar el contrato nativo de Claude Code instalado en `docs/evidence/hcd014/claude-native.md` (dep: 1) → observación manual de una sesión aislada con herramienta externa sintética y contraste entre resultado original y resultado consumido; registrar versión, modalidad y alcance real de la sustitución admitida.
- [x] 5. Publicar la matriz de cobertura en el diagnóstico CLI reutilizando `cmd_doctor_compression.go` (dep: 1, 2, 3, 4) → `go test ./adapters/primary/cli -run '^TestHCD014DoctorCoverageMatrix$' -count=1`, con los tres agentes y las cuatro clases de contenido; la lectura del informe no ejecuta sondas ni altera tráfico.
- [x] 6. Impedir estados de compresión efectiva sin evidencia de consumo en el diagnóstico (dep: 5) → `go test ./adapters/primary/cli -run '^TestHCD014DoctorRejectsUnconsumedHook$' -count=1`, con hook ausente, instalado pero ignorado y evidencia de otra versión o modalidad.
- [x] 7. Validar la decisión de exclusión compartida que ya usan los bordes automáticos existentes (dep: 6) → `go test ./adapters/primary/cli ./adapters/secondary/compression/native -run '^(TestToolOutputExclusion_.*|TestToolOutputMinTokens_Umbral|TestEngineSmallBlockIntact|TestEngineToolOutput_.*)$' -count=1`, con lectura exacta, contenido pequeño, formato desconocido y ausencia de ganancia; cada caso conserva el original y expone su motivo.
- [x] 8. Validar la entrega estructurada existente de Claude contra el contrato comprobado en la tarea 4 (dep: 4, 7) → `go test ./tests/contract -run '^TestHookToolOutput$' -count=1`, con el payload representativo del host, reducción elegible y estructura intacta; contrastar después la salida con el procedimiento manual de la tarea 4 antes de declarar consumo efectivo.
- [x] 9. Validar la publicación existente de resultados de OpenCode contra el contrato comprobado en la tarea 3 (dep: 3, 7) → `node --test infrastructure/plugin/opencode/gomemory.test.mjs`, con los casos existentes de reducción v2 identificables para la forma instalada, conservación de partes no textuales y consumo contrastado con la tarea 3.
- [x] 10. Implementar el dialecto optativo `codex-shell` para sustituir exclusivamente resultados textuales de `Bash` mediante `continue:false` y `stopReason`, conservando silencio para el dialecto legado y para MCP/formas no probadas (dep: 2, 7) → `go test ./adapters/primary/cli -run '^TestHCD014CodexCapabilityDelivery$' -count=1`: con el payload real de la tarea 2, referencia recuperable y lectura exacta excluida; el registro y la comprobación de versión se verifican por separado en 25.
- [x] 11. Documentar la viabilidad de endpoint controlable por agente y autenticación en `docs/evidence/hcd014/controlled-route.md` (dep: 2, 3, 4) → revisión manual con referencias a documentación oficial vigente y pruebas locales de configuración, diferenciando API configurable y sesión de suscripción; no reutilizar credenciales de suscripción como claves API ni declarar viabilidad por el nombre del agente.
- [x] 12. Construir el banco `tests/integration/hcd014_route_helpers_test.go`: POST loopback con `httptest` y captura literal de cuerpos Responses (`input`), Chat Completions (`messages`) y Messages (`messages`) observados en 11 (dep: 11) → `go test ./tests/integration -run '^TestHCD014ControlledRouteCapture$' -count=1`, con captura completa de mensajes y llamadas/resultados de herramientas, exclusivamente en loopback, sin proveedor real ni credenciales y con identidad intacta en modo sin transformación.
- [x] 13. Implementar `adapters/secondary/requestcompression` sobre `ports.Compressor` para reemplazar solo resultados textuales de shell con llamada identificable en Responses/Chat Completions/Messages; usar el motor nativo en el banco 12 y conservar prosa, lecturas exactas, argumentos desconocidos y tipos no reconocidos (dep: 7, 12) → `go test ./tests/integration -run '^TestHCD014ControlledRouteTransform$' -count=1`, con herramientas e historial sintéticos, instrucciones y orden preservados, campos desconocidos intactos, prefijo estable sin cambios y originales del host inmutables; un tipo de petición no reconocido se conserva íntegro.
- [x] 14. Redactar la propuesta concreta de activación y reversión de cada ruta viable en `docs/evidence/hcd014/activation-proposal.md` (dep: 11, 13) → revisión manual de destino local, proceso, configuración exacta por agente, alcance, autenticación admitida, cobertura excluida y reversión; la propuesta permanece sin aplicar y cualquier ruta no viable queda identificada como tal.
- [x] 15. Asegurar persistencia de originales protegidos antes de publicar referencias en los caminos automáticos evaluados (dep: 8, 9, 10, 13) → `go test ./tests/integration -run '^TestHCD014PersistBeforeDelivery$' -count=1`, con recuperación posterior al reinicio y comparación SHA-256 contra la representación protegida, sin promesas de recuperar material privado eliminado por política.
- [x] 16. Conservar contenido ante fallo de compresión o almacenamiento en esos caminos (dep: 15) → `go test ./tests/integration -run '^TestHCD014AutomaticFailurePassthrough$' -count=1`, con fallo de escritura, timeout y fallo del compresor; no se publica ref inexistente y se conserva la protección de secretos.
- [x] 17. Restringir la recuperación automática al proyecto y a una identidad estable de sesión conservada al compactar o reanudar; mantener la recuperación manual existente por proyecto (Q3 A; dep: 15) → `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1`, con dos proyectos y dos sesiones, referencias automáticas cruzadas rechazadas, recuperación permitida al reanudar la sesión propietaria, ausencia de identidad sin acceso automático y compatibilidad de recuperación manual por proyecto.
- [x] 18. Preservar la continuidad de referencias y contexto necesario tras compactar (dep: 15, 16, 17) → `go test ./tests/integration -run '^TestHCD014CompactionContinuity$' -count=1`, con reinicio del delta, conservación de instrucciones y llamada/resultado, recuperación vigente y observación manual posterior del ciclo de compactación de cada host disponible.
- [x] 19. Congelar un corpus sintético de sesiones reducibles con oráculos de resultado en `tests/fixtures/hcd014/corpus.json` (dep: 2, 3, 4, 11) → `go test ./tests/contract -run '^TestHCD014CorpusContract$' -count=1`, que exige huellas y casos de salida voluminosa, lectura exacta, error, recuperación, compactación y contenido sin ganancia, sin secretos ni respuestas aceptadas por coincidencia vaga.
- [x] 20. Construir el comparador de sesiones `scripts/hcd014_evaluate.py` con entrada de capturas y resultado JSON (dep: 19) → `python3 scripts/hcd014_evaluate.py --self-test`, con contabilidad separada y ahorro neto que incluya instrucciones añadidas, metadatos y recuperaciones; rechazar cero casos, capturas incompletas o stdout local presentado como consumo del modelo y no inferir tokens del proveedor a partir de bytes.
- [ ] 21. Capturar la comparación de Codex con y sin automatización en `docs/evidence/hcd014/codex-comparison.json` (dep: 10, 14, 18, 20) → observación manual de los mismos casos y condiciones con versiones, modo, ruta, huellas de entradas consumidas, resultados verificables y latencias; una ruta opcional solo se ejerce después de decidir su activación concreta.
- [ ] 22. Capturar la comparación de OpenCode con y sin automatización en `docs/evidence/hcd014/opencode-comparison.json` (dep: 9, 14, 18, 20) → observación manual de los mismos casos y condiciones con versiones, modo, ruta, huellas de entradas consumidas, resultados verificables y latencias; una ruta opcional solo se ejerce después de decidir su activación concreta.
- [x] 23. Capturar la comparación de Claude Code con y sin automatización en `docs/evidence/hcd014/claude-comparison.json` (dep: 8, 14, 18, 20) → observación manual de los mismos casos y condiciones con versiones, modo, ruta, huellas de entradas consumidas, resultados verificables y latencias; una ruta opcional solo se ejerce después de decidir su activación concreta.
- [ ] 24. Emitir el veredicto agregado del corpus en `docs/evidence/hcd014/comparison-report.json` (dep: 21, 22, 23) → `python3 scripts/hcd014_evaluate.py --input-dir docs/evidence/hcd014 --output docs/evidence/hcd014/comparison-report.json`, con error si falta un agente, no hay ahorro neto positivo por agente, existe regresión verificable o falla recuperación; reportar latencia y cobertura, sin equiparar contador local a facturación ni extrapolar al tráfico no observado.

- [x] 25. Registrar el hook shell de Codex únicamente para la versión comprobada `0.166.0`, con matcher `Bash`, dialecto `codex-shell`, preservación de hooks ajenos y guard de versión en ejecución (dep: 10) → `go test ./adapters/primary/cli ./adapters/primary/setup -run '^TestHCD014CodexShellInstallation' -count=1`, con versión comprobada, desconocida y ausente, reinstalación idempotente y ninguna sustitución tras cambiar a una versión no comprobada.

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-02 12:26 · tarea 1 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD014CoverageEvidenceContract$' -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd014_coverage_test.go:13:17: undefined: ports.CompressionCoverageScope
      application/usecases/hcd014_coverage_test.go:15:16: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:22:22: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:25:28: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:26:37: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:27:43: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:28:48: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:29:39: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:30:36: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:31:37: undefined: ports.CompressionCoverageEvidence
      application/usecases/hcd014_coverage_test.go:31:37: too many errors
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-02 12:27 · tarea 1 · verde · caducada · `go test ./application/usecases -run '^TestHCD014CoverageEvidenceContract$' -count=1 -v` (salida 0)
          --- PASS: TestHCD014CoverageEvidenceContract/complete (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/configured_only (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/executed_not_consumed (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/consumed_without_execution (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/different_version (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/different_mode (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/different_agent (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/different_class (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/missing_mechanism (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/missing_provenance (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/missing_hash (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/invalid_hash (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/unchanged (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/partial (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/unavailable (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/contradictory_unavailable (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/unavailable_without_reason (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/empty_scope (0.00s)
      PASS
      ok  	mem/application/usecases	0.460s
- 2026-11-02 12:27 · tarea 1 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD014CoverageEvidenceContract$' -count=1` (salida 1)
      --- FAIL: TestHCD014CoverageEvidenceContract (0.00s)
          --- FAIL: TestHCD014CoverageEvidenceContract/longer_delivery (0.00s)
              hcd014_coverage_test.go:65: status="verified" reason="reduced delivery observed within the declared scope" scope={Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output}, want "unverified" for {Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output}
          --- FAIL: TestHCD014CoverageEvidenceContract/same_size (0.00s)
              hcd014_coverage_test.go:65: status="verified" reason="reduced delivery observed within the declared scope" scope={Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output}, want "unverified" for {Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output}
          --- FAIL: TestHCD014CoverageEvidenceContract/missing_size (0.00s)
              hcd014_coverage_test.go:65: status="verified" reason="reduced delivery observed within the declared scope" scope={Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output}, want "unverified" for {Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output}
          --- FAIL: TestHCD014CoverageEvidenceContract/negative_size (0.00s)
              hcd014_coverage_test.go:65: status="verified" reason="reduced delivery observed within the declared scope" scope={Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output}, want "unverified" for {Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output}
      FAIL
      FAIL	mem/application/usecases	0.227s
      FAIL
- 2026-11-02 12:27 · tarea 1 · verde · caducada · `go test ./application/usecases -run '^TestHCD014CoverageEvidenceContract$' -count=1 -v` (salida 0)
          --- PASS: TestHCD014CoverageEvidenceContract/negative_size (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/configured_only (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/executed_not_consumed (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/consumed_without_execution (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/different_version (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/different_mode (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/different_agent (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/different_class (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/missing_mechanism (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/missing_provenance (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/missing_hash (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/invalid_hash (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/unchanged (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/partial (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/unavailable (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/contradictory_unavailable (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/unavailable_without_reason (0.00s)
          --- PASS: TestHCD014CoverageEvidenceContract/empty_scope (0.00s)
      PASS
      ok  	mem/application/usecases	0.211s
- 2026-11-02 12:32 · tarea 2 · manual · caducada
      Observado Codex CLI 0.166.0 interactivo en PTY y configuración/proyecto temporales, sandbox read-only, proveedor Responses SSE sintético loopback: PostToolUse matcher Bash revisado y confiado mediante UI normal recibió tool_response de exec_command; continue:false y stopReason sustituyeron el siguiente function_call_output por HCD014_REDUCED_SYNTHETIC, sin marcador original. Captura protegida y huellas en docs/evidence/hcd014/codex-native.json; procedimiento y límites en codex-native.md. No se probó exec, MCP, code mode, historial completo, calidad ni ahorro proveedor. Sin cambios en configuración habitual.
- 2026-11-02 12:34 · tarea 3 · manual · caducada
      Observado OpenCode v2.2.5 run --standalone en proyecto temporal y XDG aislados sin credenciales: callback v2 ctx.tool.hook execute.after recibió shell completed con HCD014_OPENCODE_ORIGINAL, mutó event.result.content, y la petición posterior capturada en proveedor OpenAI-compatible sintético local contenía mensaje tool HCD014_OPENCODE_REDUCED. Salida0; servidor de prueba cerrado. Evidencia limitada a shell/v2; docs/evidence/hcd014/opencode-native.md y JSON con huellas; no inferencia real ni ahorro facturado.
- 2026-11-02 12:35 · tarea 4 · manual · caducada
      Observado Claude Code2.1.316 CLI print en proyecto/configuración temporales, entorno mínimo y proveedor Messages SSE sintético loopback. Bash printf permitido produjo stdout HCD014_CLAUDE_ORIGINAL; PostToolUse conservó objeto de salida y reemplazó stdout mediante updatedToolOutput. Captura de siguiente POST: tool_result contenía HCD014_CLAUDE_REDUCED sin original. Exit0 sin denegaciones; servidor cerrado. Contrato Bash/print únicamente; docs/evidence/hcd014/claude-native.md y JSON con huellas. No inferencia real ni facturación, no MCP/historial/compactación probados.
- 2026-11-02 12:37 · tarea 5 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD014DoctorCoverageMatrix$' -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/hcd014_doctor_test.go:12:11: c.Coverage undefined (type doctorCompressionJSON has no field or method Coverage)
      adapters/primary/cli/hcd014_doctor_test.go:13:52: c.Coverage undefined (type doctorCompressionJSON has no field or method Coverage)
      adapters/primary/cli/hcd014_doctor_test.go:16:24: c.Coverage undefined (type doctorCompressionJSON has no field or method Coverage)
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-02 12:37 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD014DoctorCoverageMatrix$' -count=1 -v` (salida 0)
      === RUN   TestHCD014DoctorCoverageMatrix
      --- PASS: TestHCD014DoctorCoverageMatrix (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.457s
- 2026-11-02 12:38 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD014DoctorRejectsUnconsumedHook$' -count=1` (salida 1)
      --- FAIL: TestHCD014DoctorRejectsUnconsumedHook (0.00s)
          --- FAIL: TestHCD014DoctorRejectsUnconsumedHook/stdout_only (0.00s)
              hcd014_doctor_test.go:55: got {Scope:{Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output} Mechanism:hook Status:verified Reason:reduced delivery observed within the declared scope} want unverified
          --- FAIL: TestHCD014DoctorRejectsUnconsumedHook/callback_only (0.00s)
              hcd014_doctor_test.go:55: got {Scope:{Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output} Mechanism:hook Status:verified Reason:reduced delivery observed within the declared scope} want unverified
          --- FAIL: TestHCD014DoctorRejectsUnconsumedHook/no_visibility (0.00s)
              hcd014_doctor_test.go:55: got {Scope:{Agent:codex Version:0.166.0 Mode:interactive ContentClass:external_output} Mechanism:hook Status:verified Reason:reduced delivery observed within the declared scope} want unverified
      FAIL
      FAIL	mem/adapters/primary/cli	0.358s
      FAIL
- 2026-11-02 12:38 · tarea 6 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD014DoctorRejectsUnconsumedHook$' -count=1 -v` (salida 0)
      === RUN   TestHCD014DoctorRejectsUnconsumedHook
      === RUN   TestHCD014DoctorRejectsUnconsumedHook/captured
      === RUN   TestHCD014DoctorRejectsUnconsumedHook/absent
      === RUN   TestHCD014DoctorRejectsUnconsumedHook/ignored
      === RUN   TestHCD014DoctorRejectsUnconsumedHook/stdout_only
      === RUN   TestHCD014DoctorRejectsUnconsumedHook/callback_only
      === RUN   TestHCD014DoctorRejectsUnconsumedHook/no_visibility
      === RUN   TestHCD014DoctorRejectsUnconsumedHook/old_version
      === RUN   TestHCD014DoctorRejectsUnconsumedHook/other_mode
      --- PASS: TestHCD014DoctorRejectsUnconsumedHook (0.00s)
          --- PASS: TestHCD014DoctorRejectsUnconsumedHook/captured (0.00s)
          --- PASS: TestHCD014DoctorRejectsUnconsumedHook/absent (0.00s)
          --- PASS: TestHCD014DoctorRejectsUnconsumedHook/ignored (0.00s)
          --- PASS: TestHCD014DoctorRejectsUnconsumedHook/stdout_only (0.00s)
          --- PASS: TestHCD014DoctorRejectsUnconsumedHook/callback_only (0.00s)
          --- PASS: TestHCD014DoctorRejectsUnconsumedHook/no_visibility (0.00s)
          --- PASS: TestHCD014DoctorRejectsUnconsumedHook/old_version (0.00s)
          --- PASS: TestHCD014DoctorRejectsUnconsumedHook/other_mode (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.328s
- 2026-11-02 12:41 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli ./adapters/secondary/compression/native -run '^(TestToolOutputExclusion_.*|TestToolOutputMinTokens_Umbral|TestEngineSmallBlockIntact|TestEngineToolOutput_.*)$' -count=1 -v` (salida 0)
      === RUN   TestToolOutputExclusion_NoExcluyeOtrosComandos
      --- PASS: TestToolOutputExclusion_NoExcluyeOtrosComandos (0.00s)
      === RUN   TestToolOutputExclusion_HerramientasDeConsultaExacta
      --- PASS: TestToolOutputExclusion_HerramientasDeConsultaExacta (0.00s)
      === RUN   TestToolOutputExclusion_OpenCodeConComando
      --- PASS: TestToolOutputExclusion_OpenCodeConComando (0.00s)
      === RUN   TestToolOutputMinTokens_Umbral
      --- PASS: TestToolOutputMinTokens_Umbral (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.786s
      === RUN   TestEngineSmallBlockIntact
      --- PASS: TestEngineSmallBlockIntact (0.00s)
      === RUN   TestEngineToolOutput_ProsaIntacta
      --- PASS: TestEngineToolOutput_ProsaIntacta (0.00s)
      === RUN   TestEngineToolOutput_CodigoConservaIndentacion
      --- PASS: TestEngineToolOutput_CodigoConservaIndentacion (0.00s)
      === RUN   TestEngineToolOutput_ResultadosModeradosCompletos
      --- PASS: TestEngineToolOutput_ResultadosModeradosCompletos (0.00s)
      PASS
      ok  	mem/adapters/secondary/compression/native	0.642s
- 2026-11-02 12:41 · tarea 8 · verde · caducada · `go test ./tests/contract -run '^TestHookToolOutput$' -count=1 -v` (salida 0)
      === RUN   TestHookToolOutput
          hook_tool_output_test.go:90: claude-bash.json: 31874 → 3001 bytes en 384.393208ms
          hook_tool_output_test.go:90: claude-grep.json: 31990 → 313 bytes en 33.503958ms
          hook_tool_output_test.go:90: claude-mcp.json: 95742 → 36288 bytes en 29.495792ms
      --- PASS: TestHookToolOutput (1.23s)
      PASS
      ok  	mem/tests/contract	1.529s
- 2026-11-02 12:41 · tarea 9 · verde · caducada · `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (1.154792ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.419833ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.304375ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.337666ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.213459ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.182666ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.305834ms)
      ✔ expand: stop sends the last assistant text (0.259375ms)
      ✔ expand: stop without messages sends an empty last message (0.145375ms)
      ✔ expand: post-tool observes bash only (17.403334ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.87025ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.514875ms)
      ℹ tests 66
      ℹ suites 0
      ℹ pass 66
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 2000.857
- 2026-11-02 12:43 · tarea 10 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD014CodexCapabilityDelivery$' -count=1` (salida 1)
      --- FAIL: TestHCD014CodexCapabilityDelivery (0.00s)
          hcd014_codex_test.go:19: not the observed feedback dialect:  (unexpected end of JSON input)
      FAIL
      FAIL	mem/adapters/primary/cli	0.402s
      FAIL
- 2026-11-02 12:43 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD014CodexCapabilityDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD014CodexCapabilityDelivery
      === RUN   TestHCD014CodexCapabilityDelivery/mcp
      === RUN   TestHCD014CodexCapabilityDelivery/unknown_shape
      === RUN   TestHCD014CodexCapabilityDelivery/exact_read
      === RUN   TestHCD014CodexCapabilityDelivery/small
      === RUN   TestHCD014CodexCapabilityDelivery/invalid
      --- PASS: TestHCD014CodexCapabilityDelivery (0.00s)
          --- PASS: TestHCD014CodexCapabilityDelivery/mcp (0.00s)
          --- PASS: TestHCD014CodexCapabilityDelivery/unknown_shape (0.00s)
          --- PASS: TestHCD014CodexCapabilityDelivery/exact_read (0.00s)
          --- PASS: TestHCD014CodexCapabilityDelivery/small (0.00s)
          --- PASS: TestHCD014CodexCapabilityDelivery/invalid (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.328s
- 2026-11-02 12:46 · tarea 25 · rojo observado · caducada · `go test ./adapters/primary/cli ./adapters/primary/setup -run '^TestHCD014CodexShellInstallation' -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/hcd014_codex_test.go:50:12: undefined: setup.CodexShellFeedbackVersionSupported
      adapters/primary/cli/hcd014_codex_test.go:54:12: undefined: setup.CodexShellFeedbackVersionSupported
      adapters/primary/cli/hcd014_codex_test.go:57:18: undefined: syncCodexShellFeedbackHook
      adapters/primary/cli/hcd014_codex_test.go:59:20: undefined: syncCodexShellFeedbackHook
      adapters/primary/cli/hcd014_codex_test.go:60:13: undefined: syncCodexShellFeedbackHook
      adapters/primary/cli/hcd014_codex_test.go:63:11: undefined: setup.CodexShellFeedbackSupported
      FAIL	mem/adapters/primary/cli [build failed]
      ok  	mem/adapters/primary/setup	0.340s [no tests to run]
      FAIL
- 2026-11-02 12:47 · tarea 25 · verde · caducada · `go test ./adapters/primary/cli ./adapters/primary/setup -run '^TestHCD014CodexShellInstallation' -count=1 -v` (salida 0)
      === RUN   TestHCD014CodexShellInstallation
      --- PASS: TestHCD014CodexShellInstallation (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.372s
      === RUN   TestHCD014CodexShellInstallationVersion
      --- PASS: TestHCD014CodexShellInstallationVersion (0.00s)
      PASS
      ok  	mem/adapters/primary/setup	0.207s
- 2026-11-02 13:00 · tarea 25 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD014CodexShellInstallation$' -count=1` (salida 1)
              [[hooks.PostToolUse.hooks]]
              command = 'foreign-hook'
              type = 'command'
              [[hooks.PostToolUse]]
              matcher = '^Bash$'
              [[hooks.PostToolUse.hooks]]
              command = 'mem hook tool-output codex-shell'
              type = 'command'
              [[hooks.PostToolUse]]
              [[hooks.PostToolUse.hooks]]
              type = "command"
              command = "mem hook tool-output codex"
               (<nil>)
      FAIL
      FAIL	mem/adapters/primary/cli	0.589s
      FAIL
- 2026-11-02 13:01 · tarea 25 · verde · caducada · `go test ./adapters/primary/cli ./adapters/primary/setup -run '^TestHCD014CodexShellInstallation' -count=1 -v` (salida 0)
      === RUN   TestHCD014CodexShellInstallation
      --- PASS: TestHCD014CodexShellInstallation (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.514s
      === RUN   TestHCD014CodexShellInstallationVersion
      --- PASS: TestHCD014CodexShellInstallationVersion (0.00s)
      PASS
      ok  	mem/adapters/primary/setup	0.365s
- 2026-11-02 13:04 · tarea 11 · manual · caducada
      Revisadas las tres capturas nativas y documentación oficial vigente de configuración Codex, proveedores OpenCode v2 y variables Claude. Los endpoints personalizados funcionan con proveedores sintéticos en los protocolos Responses, Chat Completions y Messages para las modalidades documentadas. docs/evidence/hcd014/controlled-route.md separa viabilidad local de autenticación real y suscripción no verificada; no se leen ni copian credenciales ni se activa tráfico real. El banco siguiente usa solo net/http/httptest y los formatos observados.
- 2026-11-02 13:08 · tarea 12 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD014ControlledRouteCapture$' -count=1 -v` (salida 1)
      # mem/tests/integration [mem/tests/integration.test]
      tests/integration/hcd014_route_test.go:15:10: undefined: hcd014Capture
      FAIL	mem/tests/integration [build failed]
      FAIL
- 2026-11-02 13:08 · tarea 12 · verde · caducada · `go test ./tests/integration -run '^TestHCD014ControlledRouteCapture$' -count=1 -v` (salida 0)
      === RUN   TestHCD014ControlledRouteCapture
      --- PASS: TestHCD014ControlledRouteCapture (0.00s)
      PASS
      ok  	mem/tests/integration	0.477s
- 2026-11-02 13:09 · tarea 13 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD014ControlledRouteTransform$' -count=1 -v` (salida 1)
      # mem/tests/integration
      tests/integration/hcd014_transform_test.go:12:2: package mem/adapters/secondary/requestcompression is not in std (/opt/homebrew/Cellar/go/1.27.3/libexec/src/mem/adapters/secondary/requestcompression)
      FAIL	mem/tests/integration [setup failed]
      FAIL
- 2026-11-02 13:10 · tarea 13 · falló · caducada · `go test ./tests/integration -run '^TestHCD014ControlledRouteTransform$' -count=1 -v` (salida 1)
      === RUN   TestHCD014ControlledRouteTransform
      === RUN   TestHCD014ControlledRouteTransform/responses
          hcd014_transform_test.go:37: no recoverable reduction: { "instructions" : "KEEP EXACT", "input" : [{"role":"user","content":"KEEP USER"}, {"type":"function_call","call_id":"c1","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}"}, {"type":"fun
      === RUN   TestHCD014ControlledRouteTransform/chat
          hcd014_transform_test.go:37: no recoverable reduction: { "messages" : [{"role":"system","content":"KEEP EXACT"},{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"shell","arguments":"{\"command\":\"go test ./...\"}"}}]},{"r
      === RUN   TestHCD014ControlledRouteTransform/messages
          hcd014_transform_test.go:37: no recoverable reduction: { "system" : "KEEP EXACT", "messages" : [{"role":"assistant","content":[{"type":"tool_use","id":"c1","name":"Bash","input":{"command":"go test ./..."}}]},{"role":"user","content":[{"type":"tool_result
      --- FAIL: TestHCD014ControlledRouteTransform (0.03s)
          --- FAIL: TestHCD014ControlledRouteTransform/responses (0.01s)
          --- FAIL: TestHCD014ControlledRouteTransform/chat (0.01s)
          --- FAIL: TestHCD014ControlledRouteTransform/messages (0.01s)
      FAIL
      FAIL	mem/tests/integration	0.620s
      FAIL
- 2026-11-02 13:11 · tarea 13 · verde · caducada · `go test ./tests/integration -run '^TestHCD014ControlledRouteTransform$' -count=1 -v` (salida 0)
      === RUN   TestHCD014ControlledRouteTransform
      === RUN   TestHCD014ControlledRouteTransform/responses
      === RUN   TestHCD014ControlledRouteTransform/chat
      === RUN   TestHCD014ControlledRouteTransform/messages
      --- PASS: TestHCD014ControlledRouteTransform (0.05s)
          --- PASS: TestHCD014ControlledRouteTransform/responses (0.01s)
          --- PASS: TestHCD014ControlledRouteTransform/chat (0.01s)
          --- PASS: TestHCD014ControlledRouteTransform/messages (0.01s)
      PASS
      ok  	mem/tests/integration	0.312s
- 2026-11-02 13:12 · tarea 15 · verde · caducada · `go test ./tests/integration -run '^TestHCD014PersistBeforeDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD014PersistBeforeDelivery
      --- PASS: TestHCD014PersistBeforeDelivery (0.02s)
      PASS
      ok  	mem/tests/integration	0.395s
- 2026-11-02 13:12 · tarea 16 · verde · caducada · `go test ./tests/integration -run '^TestHCD014AutomaticFailurePassthrough$' -count=1 -v` (salida 0)
      === RUN   TestHCD014AutomaticFailurePassthrough
      === RUN   TestHCD014AutomaticFailurePassthrough/compressor_failure
      === RUN   TestHCD014AutomaticFailurePassthrough/missing_reference
      === RUN   TestHCD014AutomaticFailurePassthrough/write_failure
      === RUN   TestHCD014AutomaticFailurePassthrough/storage_timeout
      --- PASS: TestHCD014AutomaticFailurePassthrough (0.03s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/compressor_failure (0.00s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/missing_reference (0.00s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/write_failure (0.01s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/storage_timeout (0.01s)
      PASS
      ok  	mem/tests/integration	0.282s
- 2026-11-02 13:13 · tarea 19 · rojo observado · caducada · `go test ./tests/contract -run '^TestHCD014CorpusContract$' -count=1 -v` (salida 1)
      # mem/tests/contract
      found packages main (agent_integration_doc_test.go) and contract (hcd014_corpus_test.go) in /Users/josegomezj/home/rcw/gomemory/tests/contract
      FAIL	mem/tests/contract [setup failed]
      FAIL
- 2026-11-02 13:13 · tarea 19 · rojo observado · caducada · `go test ./tests/contract -run '^TestHCD014CorpusContract$' -count=1 -v` (salida 1)
      === RUN   TestHCD014CorpusContract
          hcd014_corpus_test.go:12: open ../fixtures/hcd014/corpus.json: no such file or directory
      --- FAIL: TestHCD014CorpusContract (0.00s)
      FAIL
      FAIL	mem/tests/contract	0.330s
      FAIL
- 2026-11-02 13:13 · tarea 19 · verde · caducada · `go test ./tests/contract -run '^TestHCD014CorpusContract$' -count=1 -v` (salida 0)
      === RUN   TestHCD014CorpusContract
      --- PASS: TestHCD014CorpusContract (0.00s)
      PASS
      ok  	mem/tests/contract	0.342s
- 2026-11-02 13:14 · tarea 20 · rojo observado · caducada · `python3 scripts/hcd014_evaluate.py --self-test` (salida 2)
      /Library/Developer/CommandLineTools/usr/bin/python3: can't open file '/Users/josegomezj/home/rcw/gomemory/scripts/hcd014_evaluate.py': [Errno 2] No such file or directory
- 2026-11-02 13:15 · tarea 20 · verde · caducada · `python3 scripts/hcd014_evaluate.py --self-test` (salida 0)
      test_compaction (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_complete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_hash (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_incomplete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_missing_case (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_no_gain (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_nonfinite (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_overhead (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_recovery (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stdout (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stub (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_wrong_oracle (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_zero_cases (__main__.self_test.<locals>.EvidenceChecks) ... ok
      ----------------------------------------------------------------------
      Ran 13 tests in 0.006s
      OK
- 2026-11-02 13:15 · tarea 20 · rojo observado · caducada · `python3 scripts/hcd014_evaluate.py --self-test` (salida 1)
      test_recovery (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stdout (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stub (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_wrong_oracle (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_zero_cases (__main__.self_test.<locals>.EvidenceChecks) ... ok
      ======================================================================
      FAIL: test_forged_recovery_content (__main__.self_test.<locals>.EvidenceChecks)
      ----------------------------------------------------------------------
      Traceback (most recent call last):
        File "/Users/josegomezj/home/rcw/gomemory/scripts/hcd014_evaluate.py", line 230, in test_forged_recovery_content
          def test_forged_recovery_content(self): self.check_bad(lambda d: d["cases"][3]["recoveries"][0].update(recovered_content="forged"))
        File "/Users/josegomezj/home/rcw/gomemory/scripts/hcd014_evaluate.py", line 214, in check_bad
          evaluate_agent(data, corpus, corpus_hash, "codex")
      AssertionError: InvalidEvidence not raised
      ----------------------------------------------------------------------
      Ran 14 tests in 0.007s
      FAILED (failures=1)
- 2026-11-02 13:15 · tarea 20 · verde · caducada · `python3 scripts/hcd014_evaluate.py --self-test` (salida 0)
      test_compaction (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_complete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_forged_recovery_content (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_hash (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_incomplete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_missing_case (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_no_gain (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_nonfinite (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_overhead (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_recovery (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stdout (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stub (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_wrong_oracle (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_zero_cases (__main__.self_test.<locals>.EvidenceChecks) ... ok
      ----------------------------------------------------------------------
      Ran 14 tests in 0.007s
      OK
- 2026-11-02 13:18 · tarea 14 · manual · caducada
      Revisada docs/evidence/hcd014/activation-proposal.md contra instalador y CLI reales: comandos nativos setup-mcp por agente y settings por proyecto, versiones y modos probados, reversión por toggle/restauración de copia previa, sin cambios de credenciales ni proveedor. La ruta controlada solo existe como httptest loopback y transformador sin listener; se declara no desplegable y no se inventa activación real. Puntos de configuración observados documentados, autenticación real y suscripción pendientes. No se aplicó ningún comando de instalación ni redirección.
- 2026-11-02 13:25 · tarea 17 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1 -v` (salida 1)
      # mem/tests/integration [mem/tests/integration.test]
      tests/integration/hcd014_isolation_test.go:19:22: undefined: persistence.NewAutomaticOriginalGrantsRepository
      tests/integration/hcd014_isolation_test.go:20:15: undefined: ports.AutomaticOriginalScope
      tests/integration/hcd014_isolation_test.go:22:19: undefined: usecases.GrantAutomaticOriginals
      tests/integration/hcd014_isolation_test.go:26:19: undefined: usecases.GrantAutomaticOriginals
      tests/integration/hcd014_isolation_test.go:31:21: undefined: persistence.NewAutomaticOriginalGrantsRepository
      tests/integration/hcd014_isolation_test.go:32:22: undefined: usecases.RetrieveAutomaticOriginal
      tests/integration/hcd014_isolation_test.go:34:29: undefined: ports.AutomaticOriginalScope
      tests/integration/hcd014_isolation_test.go:40:23: undefined: usecases.RetrieveAutomaticOriginal
      tests/integration/hcd014_isolation_test.go:43:25: undefined: usecases.RetrieveAutomaticOriginal
      tests/integration/hcd014_isolation_test.go:46:19: undefined: usecases.GrantAutomaticOriginals
      tests/integration/hcd014_isolation_test.go:46:19: too many errors
      FAIL	mem/tests/integration [build failed]
      FAIL
- 2026-11-02 13:25 · tarea 17 · verde · caducada · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1 -v` (salida 0)
      === RUN   TestHCD014RecoveryIsolation
      --- PASS: TestHCD014RecoveryIsolation (0.02s)
      PASS
      ok  	mem/tests/integration	0.822s
- 2026-11-02 13:26 · tarea 17 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1 -v` (salida 1)
      === RUN   TestHCD014RecoveryIsolation
          hcd014_isolation_test.go:83: deleted original revived an old session grant
      --- FAIL: TestHCD014RecoveryIsolation (0.02s)
      FAIL
      FAIL	mem/tests/integration	0.314s
      FAIL
- 2026-11-02 13:28 · tarea 17 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1 -v` (salida 1)
      === RUN   TestHCD014RecoveryIsolation
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/owner
          hcd014_isolation_test.go:114: automatic retrieval exit=2 want=0 stdout= stderr=flag provided but not defined: -automatic
              Uso: mem pack retrieve [opciones]
              OPCIONES
                -project string
                  	Proyecto que guardó la ref (default: proyecto actual) (default "001-790e4ff277f6269e")
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/foreign
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/missing
      --- FAIL: TestHCD014RecoveryIsolation (1.12s)
          --- FAIL: TestHCD014RecoveryIsolation/hook_and_cli (1.10s)
              --- FAIL: TestHCD014RecoveryIsolation/hook_and_cli/owner (0.03s)
              --- PASS: TestHCD014RecoveryIsolation/hook_and_cli/foreign (0.02s)
              --- PASS: TestHCD014RecoveryIsolation/hook_and_cli/missing (0.02s)
      FAIL
      FAIL	mem/tests/integration	1.517s
      FAIL
- 2026-11-02 13:29 · tarea 17 · verde · caducada · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1 -v` (salida 0)
      === RUN   TestHCD014RecoveryIsolation
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/owner
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/foreign
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/missing
      --- PASS: TestHCD014RecoveryIsolation (1.21s)
          --- PASS: TestHCD014RecoveryIsolation/hook_and_cli (1.20s)
              --- PASS: TestHCD014RecoveryIsolation/hook_and_cli/owner (0.03s)
              --- PASS: TestHCD014RecoveryIsolation/hook_and_cli/foreign (0.02s)
              --- PASS: TestHCD014RecoveryIsolation/hook_and_cli/missing (0.02s)
      PASS
      ok  	mem/tests/integration	1.543s
- 2026-11-02 13:32 · tarea 17 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1 -v` (salida 1)
      # mem/tests/integration [mem/tests/integration.test]
      tests/integration/hcd014_isolation_test.go:31:26: undefined: requestcompression.TransformForSession
      tests/integration/hcd014_isolation_test.go:31:132: undefined: requestcompression.SessionAccess
      FAIL	mem/tests/integration [build failed]
      FAIL
- 2026-11-02 21:50 · tarea 17 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1 -v` (salida 1)
      # mem/tests/integration [mem/tests/integration.test]
      tests/integration/hcd014_isolation_test.go:31:26: undefined: requestcompression.TransformForSession
      tests/integration/hcd014_isolation_test.go:31:132: undefined: requestcompression.SessionAccess
      FAIL	mem/tests/integration [build failed]
      FAIL
- 2026-11-02 21:55 · tarea 17 · verde · caducada · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1 -v` (salida 0)
      === RUN   TestHCD014RecoveryIsolation
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/owner
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/foreign
      === RUN   TestHCD014RecoveryIsolation/hook_and_cli/missing
      --- PASS: TestHCD014RecoveryIsolation (1.16s)
          --- PASS: TestHCD014RecoveryIsolation/hook_and_cli (1.11s)
              --- PASS: TestHCD014RecoveryIsolation/hook_and_cli/owner (0.03s)
              --- PASS: TestHCD014RecoveryIsolation/hook_and_cli/foreign (0.02s)
              --- PASS: TestHCD014RecoveryIsolation/hook_and_cli/missing (0.02s)
      PASS
      ok  	mem/tests/integration	1.596s
- 2026-11-02 21:56 · tarea 18 · verde · caducada · `go test ./tests/integration -run '^TestHCD014CompactionContinuity$' -count=1 -v` (salida 0)
      === RUN   TestHCD014CompactionContinuity
      === RUN   TestHCD014CompactionContinuity/codex
      === RUN   TestHCD014CompactionContinuity/opencode
      === RUN   TestHCD014CompactionContinuity/claude
      --- PASS: TestHCD014CompactionContinuity (1.16s)
          --- PASS: TestHCD014CompactionContinuity/codex (0.41s)
          --- PASS: TestHCD014CompactionContinuity/opencode (0.08s)
          --- PASS: TestHCD014CompactionContinuity/claude (0.07s)
      PASS
      ok  	mem/tests/integration	1.547s
- 2026-11-02 22:04 · tarea 18 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD014CompactionContinuity$' -count=1 -v` (salida 1)
      === RUN   TestHCD014CompactionContinuity
      === RUN   TestHCD014CompactionContinuity/recovery_hint
          hcd014_compaction_test.go:53: compaction lost the recovery hint for new reductions
      === RUN   TestHCD014CompactionContinuity/codex
      === RUN   TestHCD014CompactionContinuity/opencode
      === RUN   TestHCD014CompactionContinuity/claude
      --- FAIL: TestHCD014CompactionContinuity (1.42s)
          --- FAIL: TestHCD014CompactionContinuity/recovery_hint (0.53s)
          --- PASS: TestHCD014CompactionContinuity/codex (0.07s)
          --- PASS: TestHCD014CompactionContinuity/opencode (0.07s)
          --- PASS: TestHCD014CompactionContinuity/claude (0.07s)
      FAIL
      FAIL	mem/tests/integration	1.954s
      FAIL
- 2026-11-02 22:04 · tarea 18 · verde · caducada · `go test ./tests/integration -run '^TestHCD014CompactionContinuity$' -count=1 -v` (salida 0)
      === RUN   TestHCD014CompactionContinuity
      === RUN   TestHCD014CompactionContinuity/recovery_hint
      === RUN   TestHCD014CompactionContinuity/codex
      === RUN   TestHCD014CompactionContinuity/opencode
      === RUN   TestHCD014CompactionContinuity/claude
      --- PASS: TestHCD014CompactionContinuity (1.21s)
          --- PASS: TestHCD014CompactionContinuity/recovery_hint (0.45s)
          --- PASS: TestHCD014CompactionContinuity/codex (0.08s)
          --- PASS: TestHCD014CompactionContinuity/opencode (0.07s)
          --- PASS: TestHCD014CompactionContinuity/claude (0.07s)
      PASS
      ok  	mem/tests/integration	1.487s
- 2026-11-02 22:05 · tarea 1,5,6,7,8,10,12,13,15,16,17,18,19,25 · verde · caducada · `go test ./application/usecases ./adapters/primary/cli ./adapters/primary/setup ./adapters/secondary/compression/native ./tests/contract ./tests/integration -run '^(TestHCD014.*|TestToolOutputExclusion_.*|TestToolOutputMinTokens_Umbral|TestEngineSmallBlockIntact|TestEngineToolOutput_.*|TestHookToolOutput)$' -count=1 -v` (salida 0)
      === RUN   TestHCD014AutomaticFailurePassthrough/write_failure
      === RUN   TestHCD014AutomaticFailurePassthrough/storage_timeout
      === RUN   TestHCD014AutomaticFailurePassthrough/compressor_failure
      --- PASS: TestHCD014AutomaticFailurePassthrough (0.02s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/missing_reference (0.00s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/write_failure (0.01s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/storage_timeout (0.01s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/compressor_failure (0.00s)
      === RUN   TestHCD014ControlledRouteCapture
      --- PASS: TestHCD014ControlledRouteCapture (0.00s)
      === RUN   TestHCD014ControlledRouteTransform
      === RUN   TestHCD014ControlledRouteTransform/responses
      === RUN   TestHCD014ControlledRouteTransform/chat
      === RUN   TestHCD014ControlledRouteTransform/messages
      --- PASS: TestHCD014ControlledRouteTransform (0.04s)
          --- PASS: TestHCD014ControlledRouteTransform/responses (0.01s)
          --- PASS: TestHCD014ControlledRouteTransform/chat (0.01s)
          --- PASS: TestHCD014ControlledRouteTransform/messages (0.01s)
      PASS
      ok  	mem/tests/integration	1.838s
- 2026-11-02 22:05 · tarea 9 · verde · caducada · `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (0.467292ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.176208ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.610167ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.138416ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.142959ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.073958ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.147667ms)
      ✔ expand: stop sends the last assistant text (0.145584ms)
      ✔ expand: stop without messages sends an empty last message (0.072208ms)
      ✔ expand: post-tool observes bash only (15.116459ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.454916ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.191208ms)
      ℹ tests 66
      ℹ suites 0
      ℹ pass 66
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1973.031083
- 2026-11-02 22:05 · tarea 20 · verde · caducada · `python3 scripts/hcd014_evaluate.py --self-test` (salida 0)
      test_compaction (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_complete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_forged_recovery_content (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_hash (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_incomplete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_missing_case (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_no_gain (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_nonfinite (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_overhead (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_recovery (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stdout (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stub (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_wrong_oracle (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_zero_cases (__main__.self_test.<locals>.EvidenceChecks) ... ok
      ----------------------------------------------------------------------
      Ran 14 tests in 0.007s
      OK
- 2026-11-02 22:06 · tarea 24 · falló · caducada · `python3 scripts/hcd014_evaluate.py --input-dir docs/evidence/hcd014 --output docs/evidence/hcd014/comparison-report.json` (salida 1)
- 2026-11-02 22:07 · tarea 1,5,6,7,8,10,12,13,15,16,17,18,19,25 · verde · caducada · `go test ./application/usecases ./adapters/primary/cli ./adapters/primary/setup ./adapters/secondary/compression/native ./tests/contract ./tests/integration -run '^(TestHCD014.*|TestToolOutputExclusion_.*|TestToolOutputMinTokens_Umbral|TestEngineSmallBlockIntact|TestEngineToolOutput_.*|TestHookToolOutput|TestOpenCodePlugin_ConversacionYComando|TestOpenCodeV2_FabricaV1IdenticaALaReferencia)$' -count=1 -v` (salida 0)
      === RUN   TestHCD014AutomaticFailurePassthrough/storage_timeout
      === RUN   TestHCD014AutomaticFailurePassthrough/compressor_failure
      === RUN   TestHCD014AutomaticFailurePassthrough/missing_reference
      --- PASS: TestHCD014AutomaticFailurePassthrough (0.02s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/write_failure (0.01s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/storage_timeout (0.01s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/compressor_failure (0.00s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/missing_reference (0.00s)
      === RUN   TestHCD014ControlledRouteCapture
      --- PASS: TestHCD014ControlledRouteCapture (0.00s)
      === RUN   TestHCD014ControlledRouteTransform
      === RUN   TestHCD014ControlledRouteTransform/responses
      === RUN   TestHCD014ControlledRouteTransform/chat
      === RUN   TestHCD014ControlledRouteTransform/messages
      --- PASS: TestHCD014ControlledRouteTransform (0.05s)
          --- PASS: TestHCD014ControlledRouteTransform/responses (0.01s)
          --- PASS: TestHCD014ControlledRouteTransform/chat (0.01s)
          --- PASS: TestHCD014ControlledRouteTransform/messages (0.01s)
      PASS
      ok  	mem/tests/integration	1.935s
- 2026-11-02 22:07 · tarea 9 · verde · caducada · `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (0.890625ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.287125ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.120166ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.25125ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.168709ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.183125ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.217375ms)
      ✔ expand: stop sends the last assistant text (0.178625ms)
      ✔ expand: stop without messages sends an empty last message (0.098459ms)
      ✔ expand: post-tool observes bash only (15.907333ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.9955ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.631583ms)
      ℹ tests 66
      ℹ suites 0
      ℹ pass 66
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1975.991708
- 2026-11-02 22:07 · tarea 20 · verde · caducada · `python3 scripts/hcd014_evaluate.py --self-test` (salida 0)
      test_compaction (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_complete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_forged_recovery_content (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_hash (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_incomplete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_missing_case (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_no_gain (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_nonfinite (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_overhead (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_recovery (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stdout (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stub (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_wrong_oracle (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_zero_cases (__main__.self_test.<locals>.EvidenceChecks) ... ok
      ----------------------------------------------------------------------
      Ran 14 tests in 0.007s
      OK
- 2026-11-02 22:23 · tarea 1,5,6,7,8,10,12,13,15,16,17,19,25 · verde · caducada · `go test ./application/usecases ./adapters/primary/cli ./adapters/primary/setup ./adapters/secondary/compression/native ./tests/contract ./tests/integration -run '^(TestHCD014.*|TestToolOutputExclusion_.*|TestToolOutputMinTokens_Umbral|TestEngineSmallBlockIntact|TestEngineToolOutput_.*|TestHookToolOutput|TestOpenCodePlugin_ConversacionYComando|TestOpenCodeV2_FabricaV1IdenticaALaReferencia)$' -count=1 -v` (salida 0)
      === RUN   TestHCD014AutomaticFailurePassthrough/storage_timeout
      === RUN   TestHCD014AutomaticFailurePassthrough/compressor_failure
      === RUN   TestHCD014AutomaticFailurePassthrough/missing_reference
      --- PASS: TestHCD014AutomaticFailurePassthrough (0.02s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/write_failure (0.01s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/storage_timeout (0.01s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/compressor_failure (0.00s)
          --- PASS: TestHCD014AutomaticFailurePassthrough/missing_reference (0.00s)
      === RUN   TestHCD014ControlledRouteCapture
      --- PASS: TestHCD014ControlledRouteCapture (0.00s)
      === RUN   TestHCD014ControlledRouteTransform
      === RUN   TestHCD014ControlledRouteTransform/responses
      === RUN   TestHCD014ControlledRouteTransform/chat
      === RUN   TestHCD014ControlledRouteTransform/messages
      --- PASS: TestHCD014ControlledRouteTransform (0.04s)
          --- PASS: TestHCD014ControlledRouteTransform/responses (0.01s)
          --- PASS: TestHCD014ControlledRouteTransform/chat (0.01s)
          --- PASS: TestHCD014ControlledRouteTransform/messages (0.01s)
      PASS
      ok  	mem/tests/integration	1.922s
- 2026-11-02 22:23 · tarea 9 · verde · caducada · `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (1.296416ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.377125ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (1.429625ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.349875ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.229166ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.17025ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.25875ms)
      ✔ expand: stop sends the last assistant text (0.202ms)
      ✔ expand: stop without messages sends an empty last message (0.104042ms)
      ✔ expand: post-tool observes bash only (16.448958ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.795875ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.469041ms)
      ℹ tests 66
      ℹ suites 0
      ℹ pass 66
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 2026.295417
- 2026-11-02 22:23 · tarea 20 · verde · caducada · `python3 scripts/hcd014_evaluate.py --self-test` (salida 0)
      test_compaction (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_complete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_forged_recovery_content (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_hash (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_incomplete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_missing_case (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_no_gain (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_nonfinite (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_overhead (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_recovery (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stdout (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stub (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_wrong_oracle (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_zero_cases (__main__.self_test.<locals>.EvidenceChecks) ... ok
      ----------------------------------------------------------------------
      Ran 14 tests in 0.007s
      OK
- 2026-11-02 22:28 · tarea 4 · manual · caducada
      Claude Code 2.1.316 -p, claude-opus-5-5 con autenticación nativa, build actual 938104a0: fixture crudo 18394 B e73509c4 frente a stdout del evento 18393 B 6b5c66b1; updatedToolOutput consumido, ref c44dfc0adcad igual en hook, tool_result y respuesta del modelo. Alcance: Bash en -p; no provider_request ni interactiva. Detalle en docs/evidence/hcd014/claude-native.md.
- 2026-11-02 22:32 · tarea 7 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestToolOutputExclusion_' -count=1` (salida 1)
      --- FAIL: TestToolOutputExclusion_ComandosDeLecturaExacta (0.00s)
          hook_tool_output_safety_test.go:30: "mem pack retrieve abc123" debía excluirse (got "", false)
          hook_tool_output_safety_test.go:33: "mem pack retrieve abc123": una lectura exacta no se reescribe, salió {"hookSpecificOutput":{"hookEventName":"PostToolUse","updatedToolOutput":{"stderr":"","stdout":"COMPRIMIDO ⟦mem⟧ · ref=abcdefabcdef"}}}
          hook_tool_output_safety_test.go:30: "/opt/bin/mem pack retrieve --automatic --agent claude --session-id s abc123" debía excluirse (got "", false)
          hook_tool_output_safety_test.go:33: "/opt/bin/mem pack retrieve --automatic --agent claude --session-id s abc123": una lectura exacta no se reescribe, salió {"hookSpecificOutput":{"hookEventName":"PostToolUse","updatedToolOutput":{"stderr":"","stdout":"COMPRIMIDO ⟦mem⟧ · ref=abcdefabcdef"}}}
      FAIL
      FAIL	mem/adapters/primary/cli	0.507s
      FAIL
- 2026-11-02 22:32 · tarea 20 · rojo observado · caducada · `python3 scripts/hcd014_evaluate.py --self-test` (salida 1)
        File "/Users/josegomezj/home/rcw/gomemory/scripts/hcd014_evaluate.py", line 123, in evaluate_agent
          require(fixture["output"] in set(strings_in(baseline_bodies)), "baseline never consumed the frozen tool output")
        File "/Users/josegomezj/home/rcw/gomemory/scripts/hcd014_evaluate.py", line 31, in require
          raise InvalidEvidence(message)
      InvalidEvidence: baseline never consumed the frozen tool output
      ======================================================================
      FAIL: test_unknown_normalization (__main__.self_test.<locals>.EvidenceChecks)
      ----------------------------------------------------------------------
      Traceback (most recent call last):
        File "/Users/josegomezj/home/rcw/gomemory/scripts/hcd014_evaluate.py", line 266, in test_unknown_normalization
          def test_unknown_normalization(self): self.check_bad(lambda d: d.update(host_output_normalization="strip_whitespace"))
        File "/Users/josegomezj/home/rcw/gomemory/scripts/hcd014_evaluate.py", line 237, in check_bad
          evaluate_agent(data, corpus, corpus_hash, "codex")
      AssertionError: InvalidEvidence not raised
      ----------------------------------------------------------------------
      Ran 18 tests in 0.010s
      FAILED (failures=1, errors=1)
- 2026-11-02 22:42 · tarea 1 · verde · vigente · `go test ./application/usecases -run '^TestHCD014CoverageEvidenceContract$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.292s
- 2026-11-02 22:42 · tarea 5 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD014DoctorCoverageMatrix$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.281s
- 2026-11-02 22:42 · tarea 6 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD014DoctorRejectsUnconsumedHook$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.325s
- 2026-11-02 22:42 · tarea 7 · verde · vigente · `go test ./adapters/primary/cli ./adapters/secondary/compression/native -run '^(TestToolOutputExclusion_.*|TestToolOutputMinTokens_Umbral|TestEngineSmallBlockIntact|TestEngineToolOutput_.*)$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.335s
      ok  	mem/adapters/secondary/compression/native	0.142s
- 2026-11-02 22:42 · tarea 8 · verde · vigente · `go test ./tests/contract -run '^TestHookToolOutput$' -count=1` (salida 0)
      ok  	mem/tests/contract	1.837s
- 2026-11-02 22:42 · tarea 9 · verde · vigente · `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (1.319084ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.455625ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.205375ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.404042ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.216084ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.200334ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.2945ms)
      ✔ expand: stop sends the last assistant text (0.224291ms)
      ✔ expand: stop without messages sends an empty last message (0.136417ms)
      ✔ expand: post-tool observes bash only (17.52675ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (1.012541ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.62175ms)
      ℹ tests 66
      ℹ suites 0
      ℹ pass 66
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 2010.220417
- 2026-11-02 22:42 · tarea 10 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD014CodexCapabilityDelivery$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.341s
- 2026-11-02 22:42 · tarea 12 · verde · vigente · `go test ./tests/integration -run '^TestHCD014ControlledRouteCapture$' -count=1` (salida 0)
      ok  	mem/tests/integration	0.351s
- 2026-11-02 22:42 · tarea 13 · verde · vigente · `go test ./tests/integration -run '^TestHCD014ControlledRouteTransform$' -count=1` (salida 0)
      ok  	mem/tests/integration	0.364s
- 2026-11-02 22:42 · tarea 15 · verde · vigente · `go test ./tests/integration -run '^TestHCD014PersistBeforeDelivery$' -count=1` (salida 0)
      ok  	mem/tests/integration	0.380s
- 2026-11-02 22:42 · tarea 16 · verde · vigente · `go test ./tests/integration -run '^TestHCD014AutomaticFailurePassthrough$' -count=1` (salida 0)
      ok  	mem/tests/integration	0.323s
- 2026-11-02 22:42 · tarea 17 · verde · vigente · `go test ./tests/integration -run '^TestHCD014RecoveryIsolation$' -count=1` (salida 0)
      ok  	mem/tests/integration	1.595s
- 2026-11-02 22:42 · tarea 19 · verde · vigente · `go test ./tests/contract -run '^TestHCD014CorpusContract$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.247s
- 2026-11-02 22:42 · tarea 20 · verde · vigente · `python3 scripts/hcd014_evaluate.py --self-test` (salida 0)
      test_forged_recovery_content (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_hash (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_incomplete (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_missing_case (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_no_gain (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_nonfinite (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_normalization_allows_only_final_newlines (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_overhead (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_recovery (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stdout (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_stub (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_undeclared_host_normalization (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_unknown_normalization (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_wrong_oracle (__main__.self_test.<locals>.EvidenceChecks) ... ok
      test_zero_cases (__main__.self_test.<locals>.EvidenceChecks) ... ok
      ----------------------------------------------------------------------
      Ran 18 tests in 0.011s
      OK
- 2026-11-02 22:42 · tarea 25 · verde · vigente · `go test ./adapters/primary/cli ./adapters/primary/setup -run '^TestHCD014CodexShellInstallation' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.306s
      ok  	mem/adapters/primary/setup	0.172s
- 2026-11-02 22:43 · tarea 2 · manual · vigente
      Renovada por revisión de docs/evidence/hcd014/codex-native.md tras Q5: observación interactiva previa con proveedor sintético; codex-cli sigue en 0.166.0. No se repitió la sesión.
- 2026-11-02 22:43 · tarea 3 · manual · vigente
      Renovada por revisión de docs/evidence/hcd014/opencode-native.md tras Q5: observación previa con proveedor sintético y forma v2; opencode sigue en v2.2.5. No se repitió la sesión.
- 2026-11-02 22:43 · tarea 4 · manual · vigente
      Claude Code 2.1.316 -p, claude-opus-5-5, build actual: contraste crudo, stdout del evento, hook y tool_result en docs/evidence/hcd014/claude-native.md; el corpus real confirmó los cuerpos enviados. Alcance: Bash en -p.
- 2026-11-02 22:43 · tarea 11 · manual · vigente
      Revisado docs/evidence/hcd014/controlled-route.md con actualización tras Q5: grabador loopback comprobado con autenticación nativa en Claude y Codex; OpenCode sin comprobar por cupo agotado. Sin reutilizar credenciales.
- 2026-11-02 22:43 · tarea 14 · manual · vigente
      Revisada docs/evidence/hcd014/activation-proposal.md: ruta nativa propuesta sin aplicar, grabador de Q5 limitado a procesos de prueba, reversión y estado por agente; Codex y OpenCode identificados como pendientes.
- 2026-11-02 22:43 · tarea 23 · manual · vigente
      Seis casos reales en Claude Code 2.1.316, claude-opus-5-5, con cuerpos capturados en loopback: 275191 B frente a 241991 B, 33200 B netos, oráculos exactos tras prompts inequívocos; tres intentos fallidos declarados en docs/evidence/hcd014/claude-corpus.md, dos solo con automatización. El comparador acepta el agente. Bytes, no facturación.

## Siguiente
Consultar `mem hcd show --json --client codex` y seguir `next_action`. La implementación iniciada, los contratos nativos observados y los límites de la ruta local están documentados en `docs/evidence/hcd014/`. La configuración habitual sigue intacta. No activar redirecciones sin la decisión explícita correspondiente; no cerrar sin completar aislamiento, compactación y comparaciones reales exigidas por la unidad.
