# Reducir y verificar el contexto entregado a Codex

ID: HCD-001
Tipo: technical
Carril: full
Motivo: Compatibilidad del host, selección de evidencia y medición del ahorro contienen decisiones abiertas que se afectan entre sí.

## Intención
Reducir el contexto innecesario que gomemory entrega a Codex antes de que el modelo lo reciba, conservando la evidencia necesaria para resolver la tarea y la recuperación exacta de los originales omitidos. Demostrar el resultado mediante comparaciones reproducibles que distingan selección, compresión, deduplicación y consumo observado.

La unidad debe producir una mejora operativa comprobada frente al comportamiento actual, no solo un informe ni un porcentaje calculado sobre candidatos que nunca se habrían entregado. Entroly es un candidato de evaluación, no una dependencia elegida.


## Alcance
Incluido: mejora de la entrega propia, diagnóstico del host y evaluación comparativa reproducible, con el detalle siguiente.
- Inventariar las rutas actuales de entrega de contexto, sus presupuestos, su compresión, su delta de sesión y sus límites de recuperación.
- Registrar la versión y superficie de Codex realmente ensayadas y comprobar en un laboratorio aislado qué transformación llega al modelo. Diferenciar CLI interactiva, ejecución no interactiva y app cuando se ensayen; no extrapolar resultados entre ellas.
- Mejorar `pack_build` y `mem pack build` con presupuesto explícito de respuesta completa, contenido obligatorio protegido y selección condicionada por la consulta, reutilizando los componentes existentes. Según Q3, las demás respuestas propias conservan sus contratos actuales; no se añade un presupuesto implícito a `get_context`, `search_memories`, `get_memory` ni `search_code`.
- Según Q4, la recuperación de originales se aísla por proyecto: `pack_retrieve` acepta `project` opcional y `mem pack retrieve` acepta `--project`, ambos por defecto el proyecto actual, igual que `pack_build`. Una ref de otro proyecto, inexistente o caducada responde «no encontrada» sin revelar su existencia ni su contenido.
- Separar las métricas de selección por presupuesto, compresión de contenido equivalente y deduplicación por sesión; incluir el coste de avisos, referencias y recuperaciones.
- Evaluar el motor actual y las capacidades pertinentes de Entroly con versiones o commits fijados, fuentes equivalentes y criterios de calidad comunes.
- Conservar compatibilidad de las interfaces existentes y documentar qué ahorro se ha medido y qué límites permanecen.
- Limitar la primera entrega a mejorar las respuestas propias de gomemory y diagnosticar las externas, conforme a Q1. Acordar la condición cuantitativa de éxito antes de aprobar la definición.

Excluido: sustitución de la memoria persistente, adopción automática de dependencias e interceptación implícita de inferencia, con el detalle siguiente.
- Implementar una ruta de reducción para salidas externas de terminal u otros MCP en esta primera entrega, conforme a Q1.
- Reemplazar la memoria persistente de gomemory o mantener una segunda fuente de verdad de decisiones y preferencias.
- Adoptar Entroly, incorporar Python/Rust/Node al runtime de producción o cambiar el proveedor de inferencia sin decisión explícita posterior a la evaluación.
- Activar un proxy de inferencia, modificar credenciales o interceptar tráfico de una suscripción como consecuencia implícita de esta unidad.
- Atribuir ahorro facturado a bytes reducidos, estadísticas locales o recibos de ejecución de hooks.
- Sustituir la compactación de conversación que gestiona Codex; comprimir después de recibir el original y contabilizarlo como ahorro retroactivo.
- Implementar cambios de producción durante la preparación y aprobación de esta definición.

## ¿Qué pasa si no lo hacemos?
Las respuestas propias seguirán comprimiéndose donde ya está habilitado, pero las rutas no cubiertas seguirán entregando contenido completo y las cifras agregadas podrán confundirse con ahorro de compresión equivalente. No se conoce todavía cuánto mejoraría una sesión real ni si Entroly aporta ventaja frente al motor actual. Mantener el comportamiento actual sigue siendo una opción válida si la evaluación no demuestra beneficio neto.

## Alternativa más barata
Ajustar presupuestos, formato de salida, instrucciones de recuperación y métricas de los componentes Go existentes, sin añadir un motor externo. Esta alternativa es la referencia preferida para comparar coste y complejidad; si cumple los criterios acordados, no se justifica adoptar Entroly. Una evaluación sin mejora demostrable debe informar ese resultado y no habilitar por defecto una integración nueva.

## Base comprobada y límites de la investigación
- `adapters/primary/cli/context_output.go`: ya combina delta de sesión y compresión propia, con referencias recuperables en nivel max. `full=true` evita el delta y no debe registrar entregas que el agente principal no recibió.
- `application/usecases/build_context_pack.go`: ya selecciona candidatos por consulta, deduplica, aplica prioridades y presupuesto, y comprime elementos no críticos. Sus estadísticas agregadas comparan candidatos con elementos entregados; mezclan descarte y compresión y no prueban ahorro facturado.
- `adapters/primary/cli/hook_tool_output.go`: la sustitución de salidas para Codex está deshabilitada; `adapters/primary/setup/codex_setup.go` conserva el hook antiguo para retirarlo.
- `adapters/primary/cli/cmd_wrap.go`: conecta stdout/stderr directamente; envolver Codex con este comando no comprime sus resultados.
- Versión local observada en la investigación previa: `codex-cli 0.165.0`. El rechazo upstream del campo de sustitución está documentado para versiones anteriores en specs 033 y 038; no se ha reproducido todavía en 0.165.0. El estado de soporte actual permanece sin verificar.
- Entroly distingue selección local por hooks/MCP de transformación de peticiones por proxy/SDK. Un recibo de selección no demuestra reducción en la petición al proveedor ni preservación de calidad.
- Fuentes externas consultadas el 2026-10-29: https://github.com/juyterman1000/entroly y https://github.com/juyterman1000/entroly/blob/main/docs/integration-hub.md . La evaluación deberá fijar un commit; `main` no es una referencia reproducible de aceptación.

## Normas aplicables
Constitución 2.2.1: §3 (capas y ports), §9 y §20 (configuración y secretos), §10 (pruebas de comportamiento), §18 (documento HCD único) y §25.2 (composition root existente). Playbook: HCD-FLOW-02 (aclarar alcance), HCD-FLOW-03 (aprobación citada), HCD-DEF-01 (definición), HCD-TRACE-01 (criterios observables), HCD-EVIDENCE-01 (evidencia vigente) y HCD-CLEAN-01–04 (límites de arquitectura).

## Historias
### US1. Diagnosticar el soporte real de Codex
Como persona que usa gomemory con Codex, quiero conocer qué rutas reducen contexto realmente, para distinguir una limitación del host de una configuración o integración incompleta.
- C-01: Dado un ensayo de Codex, cuando se registra su resultado, entonces constan versión, superficie, configuración pertinente sin secretos, revisión de gomemory, procedimiento y evidencia de la frontera observada.
- C-02: Dado un ensayo de sustitución externa, cuando no se puede observar fielmente la entrada del modelo o el host rechaza la sustitución, entonces se informa no verificable o no soportado con su evidencia y no se presenta como compresión activa.
- C-03: Dado el diagnóstico, cuando se consulta una ruta, entonces se distingue entre compresión propia, selección de contexto, delta de sesión, transformación externa y compactación de conversación.

### US2. Recibir contexto útil dentro de un presupuesto
Como agente que resuelve una tarea, quiero recibir evidencia pertinente bajo un presupuesto explícito, para evitar contexto innecesario sin perder instrucciones obligatorias ni originales recuperables.
- C-04: Dada una consulta y un presupuesto, cuando se compone una entrega propia, entonces el tamaño de la respuesta serializada completa respeta el presupuesto declarado, incluyendo formato, avisos y referencias; la unidad y el contador usados son explícitos.
- C-05: Dado contenido obligatorio que por sí solo excede el presupuesto, cuando se solicita la entrega, entonces se informa desbordamiento explícito sin omitir silenciosamente ese contenido ni afirmar que se cumplió el límite.
- C-06: Dadas consultas distintas sobre el mismo corpus con evidencia relevante diferente y un control sin coincidencia, cuando se selecciona contexto, entonces se conserva la evidencia esperada de cada consulta y el control informa ausencia de coincidencia sin rellenar con material irrelevante.
- C-07: Dada una omisión recuperable, cuando se recupera su referencia, entonces se obtienen los bytes originales y se comprueba su integridad; una referencia inexistente, caducada o de otro proyecto produce un resultado explícito y no devuelve contenido ajeno.
- C-08: Dada una entrega posterior a compactación o una petición `full=true`, cuando se recupera contexto, entonces no se suprime evidencia suponiendo que el destinatario ya la leyó; se preserva el aislamiento entre sesiones y proyectos.
- C-09: Dado contenido privado o secretos detectados, cuando se prepara contexto, entonces no se persisten originales sensibles para permitir recuperación ni aparecen secretos en recibos, trazas o informes.

### US3. Medir reducción sin mezclar fenómenos
Como persona que evalúa la mejora, quiero métricas atribuibles y reproducibles, para saber qué se redujo y cuánto costó recuperar la evidencia necesaria.
- C-10: Dado un par de entregas comparables, cuando se informa la reducción, entonces se separan candidatos no seleccionados, texto comprimido, delta de sesión y sobrecoste de metadatos, evitando sumar como independientes reducciones superpuestas.
- C-11: Dada una tarea que requiere recuperaciones adicionales, cuando se calcula el resultado neto, entonces se suma toda entrega y recuperación necesaria de ambos brazos y se informa también la latencia y el resultado de calidad.
- C-12: Dada una medición basada en bytes UTF-8 o en un contador local de tokens, cuando se publica, entonces se identifica esa unidad y método; solo se informa consumo o coste del proveedor si existe evidencia atribuible a la petición correspondiente.
- C-13: Dado un recibo de reducción, cuando se inspecciona, entonces permite identificar fuentes y revisiones, parámetros, material conservado y omitido, razones y referencias de recuperación, sin volcar originales sensibles ni añadir un coste oculto a la respuesta medida.

### US4. Elegir una mejora con evidencia comparativa
Como mantenedor, quiero comparar el motor actual con la estrategia candidata y Entroly, para adoptar la alternativa más simple que demuestre beneficio.
- C-14: Dado el corpus de evaluación fijado antes de ejecutar los ensayos, cuando se comparan los brazos, entonces usan las mismas tareas, fuentes, revisión, presupuesto, modelo y política de recuperaciones, y se publican resultados por tarea y agregados.
- C-15: Dada la evaluación de Entroly, cuando se ejecuta, entonces su revisión y dependencias están fijadas y el ensayo está aislado de la configuración y memoria reales; no se añaden dependencias de producción por ejecutar la comparación.
- C-16: Dado el informe comparativo, cuando se decide una integración, entonces se justifican beneficio neto, calidad, latencia, licencia, dependencias y mantenimiento frente a la alternativa Go; si falta evidencia, se declara inconcluso sin prometer ahorro.
- C-17: Dada la mejora elegida, cuando se valida, entonces satisface los umbrales acordados antes de la comparación y conserva todos los criterios críticos de recuperación, privacidad e instrucciones obligatorias.

## Requisitos de evaluación y decisiones pendientes
La referencia es el comportamiento actual realmente entregado, no la concatenación artificial de todos los candidatos. Se conservarán los resultados adversos y los controles sin coincidencia. Las pruebas unitarias de selección no sustituyen la observación del artefacto servido y la calidad de las tareas.

Árbol de preparación y dependencia para la evaluación, sin sustituir los slices ni el checklist posteriores:
1. Fijar las fronteras y versiones observables → inventario verificable.
2. Fijar corpus, respuestas esperadas y parámetros comparables → conjunto reproducible (depende de 1).
3. Capturar el brazo actual → referencia entregada (depende de 2).
4. Capturar cada estrategia candidata → entregas y recuperaciones completas (depende de 2).
5. Verificar recuperación e instrucciones obligatorias → resultados de integridad (depende de 3 y 4).
6. Comparar calidad, reducción neta y latencia → informe por tarea y decisión justificada (depende de 5 y de los umbrales acordados).

Alcance acordado en Q1: la primera entrega mejora las respuestas propias de gomemory y diagnostica las externas; no implementa una ruta de reducción para salidas externas de terminal u otros MCP.

Condición de éxito acordada en Q2: evaluar 20 tareas reproducibles que cubran búsqueda, recuperación exacta, continuidad tras compactación y controles sin coincidencia. Exigir reducción neta agregada de al menos 20%, sin pérdida en los criterios de calidad definidos y aumento de latencia p95 de como máximo 10%. El coste medido incluye avisos, referencias y recuperaciones. Los casos concretos, sus resultados esperados, la unidad y el procedimiento de medición se fijan antes de ejecutar los ensayos conforme a C-12 y C-14; no se ajustan después para alcanzar los umbrales.

## Aclaraciones
- [x] Q1. ¿La primera entrega debe mejorar únicamente las salidas propias de gomemory y diagnosticar las externas, o también implementar una ruta de reducción para herramientas externas? → A: mejorar las respuestas propias de gomemory y diagnosticar las externas. La primera entrega no implementará una ruta de reducción de salidas externas de terminal u otros MCP.
- [x] Q2. ¿Qué umbrales mínimos de reducción neta, preservación de calidad y latencia se exigirán, y sobre qué corpus de tareas Codex representativo? → A: evaluar 20 tareas reproducibles que cubran búsqueda, recuperación exacta, continuidad tras compactación y controles sin coincidencia. Exigir reducción neta agregada de al menos 20%, sin pérdida en los criterios de calidad definidos y aumento de latencia p95 de como máximo 10%. Incluir avisos, referencias y recuperaciones en el coste medido.
- [x] Q3. ¿Cómo debe aplicarse el presupuesto completo a las respuestas propias sin parámetro de presupuesto: conservar sus contratos y exigir límite completo solo en pack_build/CLI pack build, o añadir un presupuesto explícito opt-in también a get_context, search_memories, get_memory y search_code, con error si el contenido obligatorio o exacto no cabe? → A: exigir el presupuesto de respuesta completa en pack_build y CLI pack build; conservar los contratos actuales de las demás respuestas propias.
- [x] Q4. ¿Cómo se recuperan las omisiones de un pack construido para otro proyecto si la recuperación se aísla por proyecto: añadir project opcional a pack_retrieve y --project a mem pack retrieve (por defecto el actual), o no emitir refs cuando el pack es de otro proyecto? → Project opcional (Recomendado): añadir project opcional a pack_retrieve (MCP) y --project a mem pack retrieve, por defecto el actual, igual que pack_build.

## Slices
Los números siguientes remiten al Checklist; ninguna tarea se considera ejecutada por estar redactada. Los slices amplían la entrega actual sin implementar reducción de herramientas externas. Se mantiene la definición aprobada `0570c726d460`.

### VS-01. Consultar un diagnóstico de reducción verificable para Codex
Tareas: 1, 2, 3, 4
Criterios: C-01, C-02, C-03
Resultado observable: el diagnóstico identifica la versión y superficie ensayadas, diferencia cada mecanismo y comunica soporte confirmado, rechazo o ausencia de observación suficiente con evidencia reproducible y sin secretos.
Comprobación de entrega: consultar el diagnóstico en un laboratorio aislado y contrastar su estado con las capturas del host; no basta detectar que un hook se ejecutó.
Límites: reutilizar el diagnóstico y los adaptadores existentes; ninguna ruta de sustitución externa se habilita como parte de este incremento.
Dependencias: ninguna.

### VS-02. Medir la entrega actual con un recibo y una referencia reproducible
Tareas: 5, 6, 7, 8, 9, 10
Criterios: C-10, C-11, C-12, C-13, C-14
Resultado observable: una entrega propia permite conocer por separado selección, compresión, delta y metadatos, así como recuperar su procedencia; el informe suma las recuperaciones y distingue bytes o tokens locales de consumo observado del proveedor.
Comprobación de entrega: reproducir una tarea con recuperación y comprobar que el recibo y el total incluyen todas las respuestas realmente entregadas, sin doble contabilización; fijar antes de los ensayos el corpus de 20 tareas, sus resultados esperados y los parámetros de comparación.
Límites: el recibo no vuelca originales sensibles. La referencia conserva el comportamiento actual y no usa como base una concatenación artificial de candidatos.
Dependencias: VS-01 para identificar la frontera de observación válida. La captura de referencia precede a la comparación con cambios de selección o compresión.

### VS-03. Recibir contexto pertinente dentro del presupuesto completo
Tareas: 11, 12, 13, 14, 15
Criterios: C-04, C-05, C-06
Resultado observable: las respuestas propias seleccionan evidencia según la consulta y respetan un presupuesto que cuenta formato, avisos y referencias; si el contenido obligatorio excede el límite, informan el desbordamiento explícitamente.
Comprobación de entrega: observar la respuesta serializada por CLI/MCP para dos consultas con evidencia distinta, un control sin coincidencia y un presupuesto inferior al contenido obligatorio.
Límites: extender la composición y el presupuesto existentes; no sustituir la memoria persistente ni omitir instrucciones obligatorias para cumplir el límite.
Dependencias: VS-02 para medir la reducción frente a la referencia preservada.

### VS-04. Recuperar evidencia íntegra sin mezclar sesiones ni persistir secretos
Tareas: 16, 17, 18, 19, 20
Criterios: C-07, C-08, C-09
Resultado observable: las omisiones de la entrega propia se recuperan con bytes originales verificables; las referencias inválidas o de otro proyecto fallan explícitamente; tras compactación y con `full=true` no se suprime evidencia que el destinatario necesita.
Comprobación de entrega: recorrer entrega y recuperación con originales conocidos, referencias inexistentes o caducadas, dos proyectos, dos sesiones, compactación y contenido privado de prueba; inspeccionar también recibos y persistencia para descartar filtraciones.
Límites: aprovechar el almacén de originales y el delta existentes. Los secretos de prueba son sintéticos y no se persisten como originales recuperables.
Dependencias: VS-03 para verificar las referencias de las respuestas presupuestadas; las garantías existentes de integridad y privacidad se conservan desde los incrementos anteriores, no se posponen hasta este slice.

### VS-05. Comparar alternativas y demostrar el beneficio neto acordado
Tareas: 21, 22, 23, 24
Criterios: C-14, C-15, C-16, C-17
Resultado observable: el informe compara el comportamiento actual, la mejora Go y Entroly sobre las mismas 20 tareas y publica resultados por tarea, calidad, reducción neta y latencia; justifica la alternativa más simple o declara el resultado inconcluso o insuficiente.
Comprobación de entrega: reproducir los brazos con fuentes, versiones, modelo, presupuesto y política de recuperación fijados; comprobar reducción neta agregada de al menos 20%, ninguna pérdida en los criterios de calidad y aumento de latencia p95 de como máximo 10%, incluyendo avisos, referencias y recuperaciones.
Límites: Entroly se evalúa en un entorno aislado con revisión y dependencias fijadas; el ensayo no incorpora dependencias de producción ni activa un proxy. Un resultado insuficiente no se presenta como entrega exitosa ni cambia los umbrales acordados.
Dependencias: VS-02 para el corpus y la referencia; VS-03 y VS-04 para la mejora completa y sus garantías.

Árbol de incrementos y dependencias:
```text
HCD-001: reducir contexto con beneficio verificable
├─ VS-01: entregar diagnóstico con evidencia
└─ VS-02: entregar medición atribuible y referencia (dep: VS-01)
   └─ VS-03: entregar contexto pertinente presupuestado (dep: VS-02)
      └─ VS-04: entregar recuperación íntegra y aislada (dep: VS-03)
         └─ VS-05: entregar comparación y veredicto (dep: VS-02, VS-03, VS-04)
```
Las hojas atómicas de este árbol son las tareas del Checklist, agrupadas por los rangos de cada slice. Sus dependencias se indican en cada línea; las tareas sin dependencia mutua pueden prepararse en paralelo sin requerir delegación.

## Checklist
<!-- Los nombres TestHCD001* son pruebas a crear durante implementación, no pruebas ya existentes ni evidencia. La verificación automatizada deberá comprobar que el test nombrado existe y se ejecutó: un filtro sin coincidencias no cuenta como PASS. -->
- [x] 1. Registrar el inventario de rutas de entrega propias y externas con versiones, configuración no sensible y revisión del binario en este documento → inspección manual contrastada con `codex --version`, `mem --version` y la configuración efectiva, con clasificación de cada mecanismo (C-01, C-03).
- [x] 2. Capturar el resultado de una sonda inocua de sustitución externa en Codex CLI interactivo dentro de un laboratorio aislado (dep: 1) → observación manual del texto visible al modelo o del rechazo del host; si no hay captura fiable, registrar no verificable y su causa, sin habilitar el hook en la instalación real (C-01, C-02).
- [x] 3. Extender la salida de diagnóstico existente con estados por mecanismo respaldados por la evidencia de la sonda (dep: 2) → prueba de contrato `TestHCD001DiagnosticStates` con soporte observado, rechazo y estado desconocido, sin convertir versión o ejecución de hook en prueba de soporte (C-02, C-03).
- [x] 4. Registrar la comprobación del diagnóstico servido por el binario de trabajo (dep: 3) → observación manual que contraste versión, revisión, superficie y estados con la sonda de 2 y confirme ausencia de secretos (C-01, C-02, C-03).
- [x] 5. Fijar en este documento el manifiesto de 20 tareas con fuentes de huella conocida, entradas y resultados esperados antes de ensayar candidatos (dep: 4) → inspección de 20 identificadores únicos, cinco casos por categoría: búsqueda, recuperación exacta, continuidad tras compactación y ausencia de coincidencia; cada caso declara sus criterios de calidad y las respuestas propias que ejercita (C-14).
- [x] 6. Fijar en este documento el protocolo pareado de medición para los tres brazos (dep: 5) → inspección de unidad primaria bytes UTF-8 del texto completo visible, fórmula de reducción neta, inclusión de recuperaciones, modelo/presupuesto comunes, política de recuperación, cinco repeticiones por caso en orden rotado, latencia total por tarea con reloj monotónico y p95 por rango más cercano sobre las mismas 100 observaciones por brazo; fallos y tiempos agotados no se descartan (C-11, C-12, C-14, C-17).
- [x] 7. Separar el cálculo de selección, compresión equivalente, delta y metadatos en la contabilidad de entrega reutilizando los tipos y contadores existentes (dep: 6) → prueba `TestHCD001ReductionAccounting` con sumas conservadas, denominador cero, recuperaciones y ausencia de doble contabilización; el contador local se etiqueta como estimación cuando corresponda (C-10, C-11, C-12).
- [x] 8. Incorporar un recibo de procedencia a la entrega propia mediante metadatos compatibles con las interfaces existentes (dep: 7) → prueba de contrato `TestHCD001DeliveryReceipt` que valide fuentes/revisiones, selección/omisión/razones, refs resolubles, redacción sensible y contabilización del recibo visible sin duplicar originales (C-13).
- [x] 9. Crear un ejecutor de evaluación aislado que capture entregas y recuperaciones completas por caso conforme al protocolo fijado (dep: 6, 8) → prueba de contrato `TestHCD001EvaluationCapture` con una tarea conocida cuya suma, duración y resultado esperado sean verificables y cuya frontera no observable se marque no verificable, sin atribuir ahorro al proveedor (C-11, C-12, C-14).
- [x] 10. Capturar la referencia del comportamiento anterior a las mejoras de selección y presupuesto sobre el corpus fijado (dep: 9) → observación del brazo actual con revisión y ejecutable de huella fijada, entradas y entregas completas para los 20 casos y 100 observaciones, configuración y almacenamiento aislados; conservar resultados en el informe de este documento y capturas locales referenciadas por huella, antes de ejecutar 11–15 (C-14).
- [x] 11. Reservar en el presupuesto de entrega propia el tamaño del formato, avisos y referencias antes de seleccionar contenido (dep: 10) → prueba `TestHCD001SerializedBudget` con respuestas CLI/MCP, caracteres multibyte y límites exactos, que mida la respuesta final completa con la unidad declarada (C-04).
- [x] 12. Aplicar un resultado explícito de desbordamiento cuando el contenido obligatorio exceda el presupuesto disponible (dep: 11) → prueba `TestHCD001MandatoryOverflow` que compruebe ausencia de truncamiento silencioso de instrucciones obligatorias y de afirmaciones falsas de cumplimiento del límite (C-05).
- [x] 13. Condicionar la selección de evidencia propia por la consulta reutilizando la recuperación y prioridades actuales (dep: 11) → prueba `TestHCD001QuerySelection` sobre dos consultas del mismo corpus con evidencia esperada diferente y presupuesto idéntico (C-06).
- [x] 14. Emitir ausencia de coincidencia sin rellenar el presupuesto con candidatos irrelevantes (dep: 13) → prueba `TestHCD001NoMatch` con consulta de control, preservando solo el contexto obligatorio aplicable y un estado explícito de ausencia de evidencia pertinente (C-06).
- [x] 15. Conectar la composición presupuestada a `pack_build` y `mem pack build` según Q3, conservando los contratos de las demás respuestas propias (dep: 12, 13, 14) → prueba de contrato `TestHCD001OwnDelivery` que ejercite ambos formatos CLI y el texto MCP, y mida su serialización final; inspección del mismo comportamiento en el binario servido para una consulta, un no-match y un desbordamiento (C-04, C-05, C-06).
- [x] 16. Asegurar que cada referencia de omisión presupuestada resuelve exactamente al original verificado por huella (dep: 15) → prueba `TestHCD001ExactRecovery` con originales multibyte, varias omisiones y escritura fallida del almacén; nunca se entrega una ref cuyo original no quedó disponible (C-07).
- [x] 17. Rechazar explícitamente referencias inexistentes, caducadas o pertenecientes a otro proyecto mediante el límite de recuperación existente (dep: 16) → prueba de integración `TestHCD001ReferenceIsolation` con almacén real aislado y dos proyectos, sin devolver contenido ajeno ni errores con datos sensibles (C-07, C-09).
- [x] 18. Preservar el reinicio de entregas tras compactación en la composición presupuestada (dep: 15) → prueba de contrato `TestHCD001CompactionReset` que compruebe la nueva entrega de evidencia necesaria tras reinicio y la independencia entre dos sesiones (C-08).
- [x] 19. Preservar la recuperación `full=true` sin aplicar delta ni marcar como leído por otro destinatario (dep: 15) → prueba `TestHCD001FullDelivery` con destinatario secundario y principal en la misma sesión, comprobando que el principal recibe la evidencia que todavía no leyó (C-08).
- [x] 20. Excluir originales sensibles de la persistencia recuperable y de los recibos en el recorrido de entrega y recuperación (dep: 8, 16, 17, 18, 19) → prueba de integración `TestHCD001PrivateDelivery` con secretos sintéticos y marcas privadas, inspección de base real aislada, recibos y salidas; sin persistencia de los originales sensibles ni refs que los expongan (C-09).
- [x] 21. Fijar el candidato Entroly en un entorno de evaluación aislado con commit, dependencias, licencia e invocación reproducibles (dep: 6) → inspección manual del manifiesto de evaluación y huellas, sin cambios en dependencias de producción, configuración real, memoria ni rutas de inferencia; si exige un cambio fuera de alcance, registrar bloqueo sin asumir autorización (C-15, C-16).
- [x] 22. Capturar el brazo de mejora Go sobre el corpus y protocolo fijados (dep: 10, 16, 17, 18, 19, 20) → observación del ejecutor de 9 con los 20 casos y 100 observaciones, huella de binario, configuración, entregas, recuperaciones, calidad y latencia por tarea; fallos conservados en los resultados (C-11, C-14, C-17).
- [x] 23. Capturar el brazo Entroly sobre el mismo corpus y protocolo sin incorporar una segunda memoria persistente (dep: 9, 10, 21) → observación del ejecutor de 9 con la misma evidencia de entrada y 100 observaciones, capturas completas y recuperaciones; declarar limitación o no comparabilidad cuando no pueda cumplir la frontera o las condiciones fijadas (C-14, C-15, C-16).
- [ ] 24. Publicar en este documento el veredicto comparativo por tarea y agregado de los tres brazos (dep: 22, 23) → recalcular desde capturas la reducción neta, calidad y p95 conforme a 6; comprobar >=20% de reducción, ausencia de pérdida de calidad y <=10% de aumento p95 para la mejora elegida; justificar complejidad/licencia/mantenimiento, declarar FAIL o inconcluso si corresponde y no presentar una adopción externa como aprobada implícitamente (C-16, C-17).

Árbol de tareas atómicas y orden de ejecución:
```text
HCD-001
├─ VS-01: 1 → 2 → 3 → 4
├─ VS-02: 5 → 6 → 7 → 8 → 9 → 10 (dep: 4)
├─ VS-03: 11 → {12 ∥ 13 → 14} → 15 (dep: 10)
├─ VS-04: {16 → 17 ∥ 18 ∥ 19} → 20 (dep: 15)
└─ VS-05: {21 → 23 ∥ 22} → 24
   21 depende de 6; 23 de 9, 10, 21; 22 de 10, 16–20.
```
Los comandos de pruebas se concretan como `go test <paquete propietario> -run '^TestHCD001Nombre$' -count=1 -v`; cada comprobación exige ver el nombre de la prueba ejecutada. Las observaciones manuales se registrarán como tales mediante `mem hcd verify --manual`, con huellas y límites, nunca como evidencia automática. Los archivos de pruebas, el ejecutor y las capturas son artefactos ejecutables/de evidencia; la definición, decisiones y el informe permanecen en este documento HCD único.

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-30 00:03 · tarea 1 · manual · caducada
      Inventario contrastado con codex-cli 0.165.0 y gomemory 2.41.0; SHA256 de ambos binarios y HEAD registrados en el documento. Configuración global inspeccionada selectivamente: MCP gomemory y features.hooks=true, hooks de ciclo de vida/expand presentes, sin tool-output codex. Rutas propias contrastadas con context_output.go, cmd_mcp.go, cmd_mcp_code_tools.go y build_context_pack.go; externas diferenciadas sin inferir soporte del host. No se imprimieron credenciales.
- 2026-10-30 00:04 · tarea 2 · manual · caducada
      Sonda TUI en PTY: codex 0.165.0 --no-daemon --no-alt-screen con cwd temporal y overrides hooks/mcp_servers vacíos; no se alteró configuración persistente. Sandbox causó apertura SQLite fallida; reintento con permisos mostró TUI. Sin petición de inferencia ni oráculo independiente del texto de entrada: sustitución externa NO VERIFICABLE. No confirma rechazo histórico ni soporte actual. Limitación y frontera registradas en el documento según C-02.
- 2026-10-30 00:04 · tarea 3 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD001DiagnosticStates$' -count=1 -v` (salida 1)
      === RUN   TestHCD001DiagnosticStates
          hcd001_diagnostic_test.go:14: enabled=false: debe separar política local de evidencia actual: "inactivo"
          hcd001_diagnostic_test.go:14: enabled=true: debe separar política local de evidencia actual: "solo en origen (Codex no permite reescribir salidas)"
          hcd001_diagnostic_test.go:17: no puede extrapolar el rechazo histórico al host actual: "solo en origen (Codex no permite reescribir salidas)"
      --- FAIL: TestHCD001DiagnosticStates (0.00s)
      FAIL
      FAIL	mem/adapters/primary/cli	0.414s
      FAIL
- 2026-10-30 00:04 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run '^(TestHCD001DiagnosticStates|TestToolOutputHookStates_.*|TestDoctorCompression)$' -count=1 -v` (salida 0)
      === RUN   TestToolOutputHookStates_CodexSoloEnOrigen
      --- PASS: TestToolOutputHookStates_CodexSoloEnOrigen (0.00s)
      === RUN   TestToolOutputHookStates_OpencodeBestEffort
      --- PASS: TestToolOutputHookStates_OpencodeBestEffort (0.00s)
      === RUN   TestDoctorCompression
      --- PASS: TestDoctorCompression (0.00s)
      === RUN   TestHCD001DiagnosticStates
      --- PASS: TestHCD001DiagnosticStates (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.342s
- 2026-10-30 00:05 · tarea 4 · manual · caducada
      Binario de trabajo construido desde ./infrastructure, ejecutado doctor --json. Comprobación pendiente de contrastar con resultado visible de la sección compression.
- 2026-10-30 00:05 · tarea 4 · manual · caducada
      Inspección completada del binario /private/tmp/gomemory-hcd001-work SHA256 59eb9cb719a59326204fb0728c548af6803117680640ed42975768fb8d4a052e: doctor --json exit0, compression.level=max, originales escribibles, codex informa sustitución local deshabilitada y soporte actual sin verificar con rechazo histórico 0.159.0. Cinco mecanismos diferenciados, coherentes con inventario y sonda; sección impresa sin credenciales. Sustituye la nota anterior que decía pendiente.
- 2026-10-30 00:08 · tarea 5 · manual · caducada
      Manifiesto fijado antes de ensayar candidatos: tests/fixtures/hcd001/corpus.json SHA256 f766c0d541960dcfbf88fbd147dbfe67f84245476486ab543ac8ac5e1906740d, 20 IDs únicos (cinco por categoría), cinco fuentes con huellas y literales esperados, entradas y banderas de recuperación/reinicio. Tabla y límites del corpus sintético documentados; cada condición de calidad es obligatoria por caso.
- 2026-10-30 00:08 · tarea 6 · manual · caducada
      Protocolo fijado antes de candidatos en el documento: tres brazos con mismos bytes/consulta/presupuesto4000 y max; bytesUTF8 completos primarios, tokens locales secundarios; frontera salidaCLI/MCP separada de entradaCodex, inconcluso sin captura; cinco repeticiones por20casos,100observaciones, orden rotado, estado aislado, reloj monotónico, política de recuperación común; fórmula reducción neta y p95 nearest-rank definidos, fallos no descartados; umbrales20%/cero pérdidas/10% conservados.
- 2026-10-30 00:08 · tarea 7 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD001ReductionAccounting$' -count=1 -v` (salida 1)
      === RUN   TestHCD001ReductionAccounting
          hcd001_accounting_test.go:29: métrica atribuible ausente: SelectionSavedTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: CompressionSavedTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: DeltaSavedTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: MetadataTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: RecoveryTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: CounterKind
          hcd001_accounting_test.go:29: métrica atribuible ausente: SelectionSavedTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: CompressionSavedTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: DeltaSavedTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: MetadataTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: RecoveryTokens
          hcd001_accounting_test.go:29: métrica atribuible ausente: CounterKind
      --- FAIL: TestHCD001ReductionAccounting (0.02s)
      FAIL
      FAIL	mem/application/usecases	0.378s
      FAIL
- 2026-10-30 00:09 · tarea 7 · verde · caducada · `go test ./application/usecases -run '^(TestHCD001ReductionAccounting|TestBuildContextPack_StatsInvariants)$' -count=1 -v` (salida 0)
      === RUN   TestBuildContextPack_StatsInvariants
      --- PASS: TestBuildContextPack_StatsInvariants (0.02s)
      === RUN   TestHCD001ReductionAccounting
      --- PASS: TestHCD001ReductionAccounting (0.01s)
      PASS
      ok  	mem/application/usecases	0.242s
- 2026-10-30 00:09 · tarea 8 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD001DeliveryReceipt$' -count=1 -v` (salida 1)
      === RUN   TestHCD001DeliveryReceipt
          hcd001_receipt_test.go:23: recibo de omisión ausente: {"Items":null,"Budget":1,"RawTokenCount":427,"TokenCount":0,"CompressionRate":1,"Stats":{"RawTokens":427,"FinalTokens":0,"SavedTokens":427,"CompressionRatio":1,"ItemsRetrieved":1,"ItemsDuplicate":0,"ItemsCritical":0,"ItemsRelevant":0,"ItemsOptional":0,"ItemsDiscarded":1,"SelectedRawTokens":0,"SelectionSavedTokens":427,"CompressionSavedTokens":0,"DeltaSavedTokens":0,"MetadataTokens":0,"RecoveryTokens":0,"Scope":"item_content_only; delta, serialized metadata and subsequent recoveries not measured here","CounterKind":"injected local counter; not provider usage"}}
      --- FAIL: TestHCD001DeliveryReceipt (0.02s)
      FAIL
      FAIL	mem/application/usecases	0.226s
      FAIL
- 2026-10-30 00:10 · tarea 8 · verde · caducada · `go test ./application/usecases -run '^(TestHCD001DeliveryReceipt|TestHCD001ReductionAccounting)$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ReductionAccounting
      --- PASS: TestHCD001ReductionAccounting (0.02s)
      === RUN   TestHCD001DeliveryReceipt
      --- PASS: TestHCD001DeliveryReceipt (0.01s)
      PASS
      ok  	mem/application/usecases	0.270s
- 2026-10-30 00:10 · tarea 9 · rojo observado · caducada · `go test ./tests/contract -run '^TestHCD001EvaluationCapture$' -count=1 -v` (salida 1)
      # mem/tests/contract
      found packages main (agent_integration_doc_test.go) and contract (hcd001_evaluation_test.go) in /Users/josegomezj/home/rcw/gomemory/tests/contract
      FAIL	mem/tests/contract [setup failed]
      FAIL
- 2026-10-30 00:10 · tarea 9 · rojo observado · caducada · `go test ./tests/contract -run '^TestHCD001EvaluationCapture$' -count=1 -v` (salida 1)
      === RUN   TestHCD001EvaluationCapture
          hcd001_evaluation_test.go:11: captura no conforme: exit status 2
              /Library/Developer/CommandLineTools/usr/bin/python3: can't open file '/Users/josegomezj/home/rcw/gomemory/tests/contract/../../scripts/hcd001_evaluate.py': [Errno 2] No such file or directory
      --- FAIL: TestHCD001EvaluationCapture (0.05s)
      FAIL
      FAIL	mem/tests/contract	0.350s
      FAIL
- 2026-10-30 00:11 · tarea 9 · verde · caducada · `go test ./tests/contract -run '^TestHCD001EvaluationCapture$' -count=1 -v` (salida 0)
      === RUN   TestHCD001EvaluationCapture
      --- PASS: TestHCD001EvaluationCapture (0.14s)
      PASS
      ok  	mem/tests/contract	0.408s
- 2026-10-30 00:12 · tarea 10 · manual · caducada
      Referencia capturada con binario construido desde git archive de6c747abd945dc6a7afb45e79be17baf29fe49e27, antes de cambios11-15. scripts/hcd001_evaluate.py completó100observaciones,20casos×5, en almacenes GOMEMORY_DATA_HOME temporales. /private/tmp/gomemory-hcd001-baseline.json conserva SHA256 binario/corpus, stdout completo, recuperaciones, latencia monotónica y calidad.25fallos de calidad conservados, sin errores de proceso. Frontera stdoutCLI; entradaCodex no observable, aceptación global inconclusa; orden entre brazos no garantizado.
- 2026-10-30 00:12 · tarea 11 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD001SerializedBudget$' -count=1 -v` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd001_budget_test.go:25:194: unknown field Renderer in struct literal of type usecases.ContextRequest
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-10-30 00:13 · tarea 11 · verde · caducada · `go test ./application/usecases -run '^TestHCD001SerializedBudget$' -count=1 -v` (salida 0)
      === RUN   TestHCD001SerializedBudget
      --- PASS: TestHCD001SerializedBudget (0.02s)
      PASS
      ok  	mem/application/usecases	0.399s
- 2026-10-30 00:13 · tarea 12 · verde · caducada · `go test ./application/usecases -run '^TestHCD001MandatoryOverflow$' -count=1 -v` (salida 0)
      === RUN   TestHCD001MandatoryOverflow
      --- PASS: TestHCD001MandatoryOverflow (0.02s)
      PASS
      ok  	mem/application/usecases	0.249s
- 2026-10-30 00:13 · tarea 13 · verde · caducada · `go test ./application/usecases -run '^TestHCD001QuerySelection$' -count=1 -v` (salida 0)
      === RUN   TestHCD001QuerySelection
      --- PASS: TestHCD001QuerySelection (0.02s)
      PASS
      ok  	mem/application/usecases	0.216s
- 2026-10-30 00:13 · tarea 14 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD001NoMatch$' -count=1 -v` (salida 1)
      === RUN   TestHCD001NoMatch
          hcd001_nomatch_test.go:16: ausencia no explícita: {"Items":null,"Receipts":null,"Budget":4000,"RawTokenCount":0,"TokenCount":0,"CompressionRate":0,"Stats":{"RawTokens":0,"FinalTokens":0,"SavedTokens":0,"CompressionRatio":0,"ItemsRetrieved":0,"ItemsDuplicate":0,"ItemsCritical":0,"ItemsRelevant":0,"ItemsOptional":0,"ItemsDiscarded":0,"SelectedRawTokens":0,"SelectionSavedTokens":0,"CompressionSavedTokens":0,"DeltaSavedTokens":0,"MetadataTokens":174,"RecoveryTokens":0,"Scope":"complete serialized pack; subsequent recoveries not measured here","CounterKind":"injected local counter; not provider usage"}}
      --- FAIL: TestHCD001NoMatch (0.02s)
      FAIL
      FAIL	mem/application/usecases	0.236s
      FAIL
- 2026-10-30 00:13 · tarea 14 · verde · caducada · `go test ./application/usecases -run '^TestHCD001NoMatch$' -count=1 -v` (salida 0)
      === RUN   TestHCD001NoMatch
      --- PASS: TestHCD001NoMatch (0.01s)
      PASS
      ok  	mem/application/usecases	0.211s
- 2026-10-30 00:19 · tarea 15 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD001OwnDelivery$' -count=1 -v` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/hcd001_delivery_test.go:46:15: undefined: buildPackOutput
      adapters/primary/cli/hcd001_delivery_test.go:55:14: undefined: buildPackOutput
      adapters/primary/cli/hcd001_delivery_test.go:61:16: undefined: buildPackOutput
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-10-30 00:20 · tarea 15 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD001OwnDelivery$' -count=1 -v` (salida 1)
      === RUN   TestHCD001OwnDelivery
          hcd001_delivery_test.go:51: json=true: respuesta completa=1295 > presupuesto=1100 o sin evidencia
      --- FAIL: TestHCD001OwnDelivery (0.02s)
      FAIL
      FAIL	mem/adapters/primary/cli	0.331s
      FAIL
- 2026-10-30 00:22 · tarea 15 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD001OwnDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001OwnDelivery
      --- PASS: TestHCD001OwnDelivery (0.03s)
      PASS
      ok  	mem/adapters/primary/cli	0.312s
- 2026-10-30 00:22 · tarea 15 · manual · caducada
      Binario servido construido desde ./infrastructure (SHA256 6ff9dc7004ce3c09c010eeaedca1070d1010c77de9a7f6b179ed389da53fedc3) en laboratorio aislado (HOME y repo temporales, memorias sintéticas, --no-speckit --no-code-graph). mem pack build --task limite --max-tokens 1100: exit0, 597 tokens aprox. de respuesta completa con recibo de procedencia; --json: exit0, 735 tokens, MatchStatus matched. --task zzsincoincidencia1zz: exit0, aviso explícito 'Sin coincidencias', cero items, 126 tokens de metadatos. --task regla --max-tokens 60 (decisión crítica): exit1, error explícito 'excede el presupuesto de 60 tokens', sin truncado. MCP pack_build cubierto por TestHCD001OwnDelivery (texto medido <= presupuesto y error explícito). get_context/search_memories/get_memory/search_code sin cambios (Q3).
- 2026-10-30 00:23 · tarea 16 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD001ExactRecovery$' -count=1 -v` (salida 1)
      === RUN   TestHCD001ExactRecovery
          hcd001_recovery_test.go:78: failFrom=0: se esperaban varias omisiones, hubo 0
      --- FAIL: TestHCD001ExactRecovery (0.02s)
      FAIL
      FAIL	mem/application/usecases	0.204s
      FAIL
- 2026-10-30 00:24 · tarea 16 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD001ExactRecovery$' -count=1 -v` (salida 1)
      === RUN   TestHCD001ExactRecovery
          hcd001_recovery_test.go:89: failFrom=0: refs entregadas=0, originales disponibles=2
      --- FAIL: TestHCD001ExactRecovery (0.02s)
      FAIL
      FAIL	mem/application/usecases	0.205s
      FAIL
- 2026-10-30 00:25 · tarea 16 · verde · caducada · `go test ./application/usecases -run '^TestHCD001ExactRecovery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ExactRecovery
      --- PASS: TestHCD001ExactRecovery (0.02s)
      PASS
      ok  	mem/application/usecases	0.201s
- 2026-10-30 00:25 · tarea 16 · manual · caducada
      Binario servido en laboratorio aislado: mem pack build --json --max-tokens 900 omitió memory:3 por presupuesto con ref f24b054ee000; mem pack retrieve devolvió 2254 bytes cuyo SHA-256 coincide exactamente con OriginalSHA256 del recibo. Refs solo se emiten tras Put exitoso (keepOmittedOriginal); fallo de escritura => recibo sin ref.
- 2026-10-30 00:30 · tarea 17 · rojo observado · caducada · `go test ./tests/integration -run '^TestHCD001ReferenceIsolation$' -count=1 -v` (salida 1)
      === RUN   TestHCD001ReferenceIsolation
          hcd001_reference_isolation_test.go:54: ref ajena debe rechazarse sin contenido ni datos: "dato exclusivo de alfa ñandú" <nil>
      --- FAIL: TestHCD001ReferenceIsolation (0.01s)
      FAIL
      FAIL	mem/tests/integration	0.280s
      FAIL
- 2026-10-30 00:32 · tarea 17 · verde · caducada · `go test ./tests/integration -run '^TestHCD001ReferenceIsolation$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ReferenceIsolation
      --- PASS: TestHCD001ReferenceIsolation (0.01s)
      PASS
      ok  	mem/tests/integration	0.255s
- 2026-10-30 00:32 · tarea 17 · manual · caducada
      Binario servido (SHA256 a6f37979e3cb0fb3c11a79bb3090a3582e57c3a1758095bef5042e0352411b4c) en laboratorio aislado: ref eccbd3a4672c de una omisión del pack se recupera en su proyecto (2222 bytes); mem pack retrieve --project beta sale con código 2 y mensaje genérico 'no encontrada o caducada' sin contenido; ref inexistente ffffffffffff también código 2. Dedup del almacén pasa a sha256(proyecto+NUL+contenido) para que dos proyectos con el mismo contenido no compartan fila (cubierto en la prueba).
- 2026-10-30 00:33 · tarea 18 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD001CompactionReset$' -count=1 -v` (salida 0)
      === RUN   TestHCD001CompactionReset
      --- PASS: TestHCD001CompactionReset (0.07s)
      PASS
      ok  	mem/adapters/primary/cli	0.358s
- 2026-10-30 00:33 · tarea 19 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD001FullDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001FullDelivery
      --- PASS: TestHCD001FullDelivery (0.05s)
      PASS
      ok  	mem/adapters/primary/cli	0.333s
- 2026-10-30 00:36 · tarea 20 · verde · caducada · `go test ./tests/integration -run '^TestHCD001PrivateDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001PrivateDelivery
      --- PASS: TestHCD001PrivateDelivery (0.03s)
      PASS
      ok  	mem/tests/integration	0.289s
- 2026-10-30 00:36 · tarea 20 · manual · caducada
      Sin rojo natural: la garantía ya existía (ProtectText en BuildContextPack + Put rechaza IsPrivateContent). Validez comprobada por mutación temporal (quitar ProtectText del bucle): la prueba falla con 'la salida expone contenido sensible' y por el control; código restaurado y diff idéntico. La prueba inspecciona la base SQLite real aislada (compression_originals descomprimidos), recibos JSON, salida renderizada y cada recuperación por ref, con refs de compresión del motor nativo y de omisión por presupuesto.
- 2026-10-30 00:39 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD001DiagnosticStates$' -count=1 -v` (salida 0)
      === RUN   TestHCD001DiagnosticStates
      --- PASS: TestHCD001DiagnosticStates (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.417s
- 2026-10-30 00:39 · tarea 7 · verde · caducada · `go test ./application/usecases -run '^TestHCD001ReductionAccounting$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ReductionAccounting
      --- PASS: TestHCD001ReductionAccounting (0.01s)
      PASS
      ok  	mem/application/usecases	0.246s
- 2026-10-30 00:39 · tarea 8 · verde · caducada · `go test ./application/usecases -run '^TestHCD001DeliveryReceipt$' -count=1 -v` (salida 0)
      === RUN   TestHCD001DeliveryReceipt
      --- PASS: TestHCD001DeliveryReceipt (0.01s)
      PASS
      ok  	mem/application/usecases	0.177s
- 2026-10-30 00:39 · tarea 9 · verde · caducada · `go test ./tests/contract -run '^TestHCD001EvaluationCapture$' -count=1 -v` (salida 0)
      === RUN   TestHCD001EvaluationCapture
      --- PASS: TestHCD001EvaluationCapture (0.13s)
      PASS
      ok  	mem/tests/contract	0.408s
- 2026-10-30 00:39 · tarea 11 · verde · caducada · `go test ./application/usecases -run '^TestHCD001SerializedBudget$' -count=1 -v` (salida 0)
      === RUN   TestHCD001SerializedBudget
      --- PASS: TestHCD001SerializedBudget (0.01s)
      PASS
      ok  	mem/application/usecases	0.172s
- 2026-10-30 00:39 · tarea 12 · verde · caducada · `go test ./application/usecases -run '^TestHCD001MandatoryOverflow$' -count=1 -v` (salida 0)
      === RUN   TestHCD001MandatoryOverflow
      --- PASS: TestHCD001MandatoryOverflow (0.01s)
      PASS
      ok  	mem/application/usecases	0.156s
- 2026-10-30 00:39 · tarea 13 · verde · caducada · `go test ./application/usecases -run '^TestHCD001QuerySelection$' -count=1 -v` (salida 0)
      === RUN   TestHCD001QuerySelection
      --- PASS: TestHCD001QuerySelection (0.01s)
      PASS
      ok  	mem/application/usecases	0.166s
- 2026-10-30 00:39 · tarea 14 · verde · caducada · `go test ./application/usecases -run '^TestHCD001NoMatch$' -count=1 -v` (salida 0)
      === RUN   TestHCD001NoMatch
      --- PASS: TestHCD001NoMatch (0.01s)
      PASS
      ok  	mem/application/usecases	0.192s
- 2026-10-30 00:39 · tarea 15 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD001OwnDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001OwnDelivery
      --- PASS: TestHCD001OwnDelivery (0.04s)
      PASS
      ok  	mem/adapters/primary/cli	0.326s
- 2026-10-30 00:39 · tarea 16 · verde · caducada · `go test ./application/usecases -run '^TestHCD001ExactRecovery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ExactRecovery
      --- PASS: TestHCD001ExactRecovery (0.01s)
      PASS
      ok  	mem/application/usecases	0.169s
- 2026-10-30 00:39 · tarea 17 · verde · caducada · `go test ./tests/integration -run '^TestHCD001ReferenceIsolation$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ReferenceIsolation
      --- PASS: TestHCD001ReferenceIsolation (0.01s)
      PASS
      ok  	mem/tests/integration	0.272s
- 2026-10-30 00:39 · tarea 18 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD001CompactionReset$' -count=1 -v` (salida 0)
      === RUN   TestHCD001CompactionReset
      --- PASS: TestHCD001CompactionReset (0.07s)
      PASS
      ok  	mem/adapters/primary/cli	0.358s
- 2026-10-30 00:39 · tarea 19 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD001FullDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001FullDelivery
      --- PASS: TestHCD001FullDelivery (0.08s)
      PASS
      ok  	mem/adapters/primary/cli	0.356s
- 2026-10-30 00:39 · tarea 1 · manual · caducada
      Re-registro bajo la huella final del árbol: el inventario del documento (codex-cli 0.165.0, gomemory 2.41.0, registros MCP/hooks, rutas propias y externas) no fue alterado por los cambios 15-20. Delta relevante: pack_build/mem pack build ahora presupuestan la serialización completa, y pack_retrieve/mem pack retrieve aceptan project (Q4). Las rutas externas no cambiaron. No se repitió la inspección de la configuración global.
- 2026-10-30 00:39 · tarea 2 · manual · caducada
      Re-registro bajo la huella final: la sonda Codex 0.165.0 NO se repitió. Su resultado (sustitución externa no verificable; sin oráculo de la entrada al modelo) no depende del código de gomemory cambiado en 15-20, que no toca hooks ni salidas externas. La limitación C-02 se mantiene.
- 2026-10-30 00:39 · tarea 4 · manual · caducada
      Binario final a6f37979e3cb0fb3c11a79bb3090a3582e57c3a1758095bef5042e0352411b4c: doctor --json exit 0; compression.level=max, store_writable=true; codex 'solo en origen; sustitución externa deshabilitada; soporte actual sin verificar (rechazo histórico 0.159.0)'; cinco mecanismos diferenciados y coherentes con el inventario y la sonda. Solo se imprimió la sección de compresión, sin credenciales.
- 2026-10-30 00:39 · tarea 5 · manual · caducada
      Re-registro bajo la huella final: el manifiesto tests/fixtures/hcd001/corpus.json no cambió (SHA256 f766c0d541960dcfbf88fbd147dbfe67f84245476486ab543ac8ac5e1906740d, confirmado por los informes de captura de 22). 20 IDs, cinco por categoría. No se ajustó tras observar resultados.
- 2026-10-30 00:39 · tarea 6 · manual · caducada
      Re-registro bajo la huella final: el protocolo del documento no se modificó; la tarea 22/24 lo aplicó tal cual (bytes UTF-8 con recuperaciones, 100 observaciones por brazo, p95 por rango más cercano, fallos conservados, umbrales sin cambios).
- 2026-10-30 00:39 · tarea 10 · manual · caducada
      Referencia recapturada el 2026-10-29 con un binario reconstruido desde git archive de 6c747abd (SHA256 89b872d1, que difiere del e53f03a2 de la tarea 10 por la ruta de construcción): 100 observaciones, 370960 bytes (idéntico a la captura original 1714e25d), 25 fallos recovery, 0 errores. Captura hcd001-baseline-rerun.json, SHA256 652f4e53.
- 2026-10-30 00:40 · tarea 21 · manual · caducada
      BLOQUEO registrado, no se fijó el candidato. La persona eligió literalmente «Registrar bloqueo (Recomendado)»: evaluar Entroly exige descargar código externo e instalar dependencias, que las normas del entorno restringen a un contenedor no autorizado. No se descargó ni ejecutó nada externo, sin cambios en dependencias de producción, configuración real, memoria ni rutas de inferencia. Sección 'Entroly bloqueado' del documento.
- 2026-10-30 00:40 · tarea 22 · manual · caducada
      Ejecutor de 9 sobre el corpus f766c0d5 con el binario Go final a6f37979: 100 observaciones (20 casos x 5), 0 errores de proceso, 426460 bytes con recuperaciones, 25 fallos de calidad (todos recovery, iguales a la referencia), p95 92,92 ms. Referencia recapturada a continuación (370960 bytes, p95 87,78 ms). Captura hcd001-go.json SHA256 f51f605b; entregas completas y latencia monotónica por observación. Frontera: stdout de la CLI, no la entrada del modelo.
- 2026-10-30 00:40 · tarea 23 · manual · caducada
      NO COMPARABLE, declarado según el check: sin captura del brazo Entroly por el bloqueo de la tarea 21 (decisión literal de la persona «Registrar bloqueo (Recomendado)»). No se le atribuye ningún resultado ni se incorporó una segunda memoria persistente.
- 2026-10-30 00:42 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD001DiagnosticStates$' -count=1 -v` (salida 0)
      === RUN   TestHCD001DiagnosticStates
      --- PASS: TestHCD001DiagnosticStates (0.00s)
      PASS
      ok  	mem/adapters/primary/cli	0.298s
- 2026-10-30 00:42 · tarea 7 · verde · vigente · `go test ./application/usecases -run '^TestHCD001ReductionAccounting$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ReductionAccounting
      --- PASS: TestHCD001ReductionAccounting (0.02s)
      PASS
      ok  	mem/application/usecases	0.189s
- 2026-10-30 00:42 · tarea 8 · verde · vigente · `go test ./application/usecases -run '^TestHCD001DeliveryReceipt$' -count=1 -v` (salida 0)
      === RUN   TestHCD001DeliveryReceipt
      --- PASS: TestHCD001DeliveryReceipt (0.01s)
      PASS
      ok  	mem/application/usecases	0.178s
- 2026-10-30 00:42 · tarea 9 · verde · vigente · `go test ./tests/contract -run '^TestHCD001EvaluationCapture$' -count=1 -v` (salida 0)
      === RUN   TestHCD001EvaluationCapture
      --- PASS: TestHCD001EvaluationCapture (0.12s)
      PASS
      ok  	mem/tests/contract	0.388s
- 2026-10-30 00:42 · tarea 11 · verde · vigente · `go test ./application/usecases -run '^TestHCD001SerializedBudget$' -count=1 -v` (salida 0)
      === RUN   TestHCD001SerializedBudget
      --- PASS: TestHCD001SerializedBudget (0.01s)
      PASS
      ok  	mem/application/usecases	0.181s
- 2026-10-30 00:42 · tarea 12 · verde · vigente · `go test ./application/usecases -run '^TestHCD001MandatoryOverflow$' -count=1 -v` (salida 0)
      === RUN   TestHCD001MandatoryOverflow
      --- PASS: TestHCD001MandatoryOverflow (0.01s)
      PASS
      ok  	mem/application/usecases	0.177s
- 2026-10-30 00:42 · tarea 13 · verde · vigente · `go test ./application/usecases -run '^TestHCD001QuerySelection$' -count=1 -v` (salida 0)
      === RUN   TestHCD001QuerySelection
      --- PASS: TestHCD001QuerySelection (0.01s)
      PASS
      ok  	mem/application/usecases	0.165s
- 2026-10-30 00:42 · tarea 14 · verde · vigente · `go test ./application/usecases -run '^TestHCD001NoMatch$' -count=1 -v` (salida 0)
      === RUN   TestHCD001NoMatch
      --- PASS: TestHCD001NoMatch (0.01s)
      PASS
      ok  	mem/application/usecases	0.167s
- 2026-10-30 00:42 · tarea 15 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD001OwnDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001OwnDelivery
      --- PASS: TestHCD001OwnDelivery (0.04s)
      PASS
      ok  	mem/adapters/primary/cli	0.336s
- 2026-10-30 00:42 · tarea 16 · verde · vigente · `go test ./application/usecases -run '^TestHCD001ExactRecovery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ExactRecovery
      --- PASS: TestHCD001ExactRecovery (0.01s)
      PASS
      ok  	mem/application/usecases	0.175s
- 2026-10-30 00:42 · tarea 17 · verde · vigente · `go test ./tests/integration -run '^TestHCD001ReferenceIsolation$' -count=1 -v` (salida 0)
      === RUN   TestHCD001ReferenceIsolation
      --- PASS: TestHCD001ReferenceIsolation (0.01s)
      PASS
      ok  	mem/tests/integration	0.302s
- 2026-10-30 00:42 · tarea 18 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD001CompactionReset$' -count=1 -v` (salida 0)
      === RUN   TestHCD001CompactionReset
      --- PASS: TestHCD001CompactionReset (0.08s)
      PASS
      ok  	mem/adapters/primary/cli	0.404s
- 2026-10-30 00:42 · tarea 19 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD001FullDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001FullDelivery
      --- PASS: TestHCD001FullDelivery (0.04s)
      PASS
      ok  	mem/adapters/primary/cli	0.333s
- 2026-10-30 00:42 · tarea 20 · verde · vigente · `go test ./tests/integration -run '^TestHCD001PrivateDelivery$' -count=1 -v` (salida 0)
      === RUN   TestHCD001PrivateDelivery
      --- PASS: TestHCD001PrivateDelivery (0.03s)
      PASS
      ok  	mem/tests/integration	0.276s
- 2026-10-30 00:42 · tarea 1 · manual · vigente
      Inventario del documento vigente (codex-cli 0.165.0, gomemory 2.41.0, registros MCP/hooks, rutas propias/externas). Cambios 15-20: pack_build/mem pack build presupuestan la serialización completa; pack_retrieve/mem pack retrieve aceptan project (Q4); rutas externas sin cambios. No se repitió la inspección de configuración global. (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)
- 2026-10-30 00:42 · tarea 2 · manual · vigente
      La sonda Codex 0.165.0 no se repitió; su resultado (sustitución externa no verificable, sin oráculo de entrada al modelo) no depende del código cambiado en 15-20, que no toca hooks ni salidas externas. Limitación C-02 vigente. (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)
- 2026-10-30 00:42 · tarea 4 · manual · vigente
      Binario final a6f37979 (el gofmt solo toca un test): doctor --json exit 0; level=max; store_writable=true; codex 'solo en origen; sustitución externa deshabilitada; soporte actual sin verificar (rechazo histórico 0.159.0)'; cinco mecanismos coherentes con inventario y sonda; sin credenciales. (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)
- 2026-10-30 00:42 · tarea 5 · manual · vigente
      Manifiesto tests/fixtures/hcd001/corpus.json sin cambios (SHA256 f766c0d5…, confirmado en los informes de captura); 20 IDs, cinco por categoría; no se ajustó tras observar resultados. (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)
- 2026-10-30 00:42 · tarea 6 · manual · vigente
      Protocolo del documento sin modificar; aplicado tal cual en 22/24 (bytes UTF-8 con recuperaciones, 100 observaciones por brazo, p95 por rango más cercano, fallos conservados, umbrales sin cambios). (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)
- 2026-10-30 00:42 · tarea 10 · manual · vigente
      Referencia recapturada con un binario reconstruido desde git archive de 6c747abd (SHA256 89b872d1; difiere de e53f03a2 por la ruta de construcción): 100 observaciones, 370960 bytes (idéntico a la captura original 1714e25d), 25 fallos recovery, 0 errores; hcd001-baseline-rerun.json SHA256 652f4e53. (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)
- 2026-10-30 00:42 · tarea 21 · manual · vigente
      BLOQUEO registrado, no se fijó el candidato: la persona eligió literalmente «Registrar bloqueo (Recomendado)». Entroly exige descargar código externo e instalar dependencias en un contenedor no autorizado. No se descargó ni ejecutó nada externo; sin cambios de producción, configuración, memoria ni inferencia. (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)
- 2026-10-30 00:42 · tarea 22 · manual · vigente
      Ejecutor de 9, corpus f766c0d5, binario Go a6f37979: 100 observaciones, 0 errores, 426460 bytes, 25 fallos recovery (iguales a la referencia), p95 92,92 ms; referencia recapturada a continuación: 370960 bytes, p95 87,78 ms. hcd001-go.json SHA256 f51f605b. Frontera: stdout de la CLI. (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)
- 2026-10-30 00:42 · tarea 23 · manual · vigente
      NO COMPARABLE, declarado según el check: sin brazo Entroly por el bloqueo de 21 (decisión literal «Registrar bloqueo (Recomendado)»); sin resultado atribuido y sin segunda memoria persistente. (re-registro tras gofmt de tests/contract/hcd001_evaluation_test.go, sin cambio de comportamiento)

## Siguiente
Cancelada: Cancelo HCD-001: FAIL de reducción neta (−14,96%), se continúa en una nueva unidad

## Registro de implementación

### Inventario observado (tarea 1)
Fecha: 2026-10-29. Checkout: `6c747abd945dc6a7afb45e79be17baf29fe49e27`; sin modificaciones de producción al inventariar. Superficie ensayada: CLI local; no extrapolar a la app ni a exec.

| Elemento | Observación |
|---|---|
| Codex | `codex-cli 0.165.0`, binario SHA-256 `a45099154d7fb4a5ed8a56f40966a95deb938e508bab23e6985035c709e9a85a` |
| gomemory instalado | `gomemory 2.41.0`, binario SHA-256 `c03c2ab35d613d8af715168bc6c30352288ce7917aab7df775d4bdb25b5dc6b3` |
| Registro | MCP global `gomemory` presente; `features.hooks=true`; hooks SessionStart, UserPromptSubmit, PreToolUse, PostToolUse y Stop de memoria/expand presentes; no hay registro `tool-output codex` |
| Contexto y búsquedas | `get_context`/`get_plan_context`, `search_memories`, `get_memory` y `search_code` tienen rutas propias de compresión; delta en contexto y búsquedas; compresión condicionada al nivel y refs recuperables |
| Recuperación | `pack_retrieve` devuelve originales; `get_symbol` ofrece metadatos/relaciones, no equivale a entregar cuerpos comprimidos |
| Packs | `pack_build` selecciona memoria/artefactos con presupuesto y compresión; sus cifras actuales mezclan selección y compresión |
| Externas | Terminal y otros MCP no atraviesan la entrega propia; `rewriteToolOutput` no emite reemplazo para Codex; `mem wrap` conecta stdout/stderr directamente |
| Compactación | Pertenece al host; el hook de gomemory recupera memoria y reinicia entregas, no reemplaza el mecanismo del modelo |

El inventario verifica registros y rutas, no demuestra que una versión actual del host acepte reescritura externa. La marca visible de omisiones en respuestas propias de esta sesión sí demuestra que existe entrega reducida en origen, sin cuantificar ahorro de la sesión ni del proveedor.

### Sonda de la frontera Codex (tarea 2)
Se intentó iniciar la TUI 0.165.0 en una PTY, sin prompt ni tarea de agente, con `--no-daemon --no-alt-screen`, directorio temporal y overrides de invocación `hooks={}`/`mcp_servers={}`. No se modificó la configuración persistente ni se activó sustitución externa. El primer inicio falló por acceso SQLite restringido por sandbox; no se interpreta como corrupción. El inicio con permisos de ejecución mostró la TUI, pero esta observación no ofrece un oráculo independiente del texto enviado al modelo y no se emitió una petición de inferencia ni una salida MCP sustituida.

Resultado: **no verificable en esta sonda** para sustitución externa en 0.165.0. No se ha reproducido el rechazo histórico ni confirmado soporte actual. El diagnóstico debe conservar la política local de sustitución deshabilitada y expresar que falta evidencia end-to-end para la versión/superficie actual. La limitación se registra conforme a C-02; abrir una TUI o ejecutar un hook no prueba que el modelo haya recibido un reemplazo. App y exec permanecen sin ensayar.

### Diagnóstico servido (tarea 4)
Construcción: `go build -o /private/tmp/gomemory-hcd001-work ./infrastructure`. SHA-256 observado: `59eb9cb719a59326204fb0728c548af6803117680640ed42975768fb8d4a052e`. `doctor --json` terminó con código 0 e informó nivel `max`, almacén escribible, política Codex de sustitución deshabilitada y soporte actual sin verificar con rechazo histórico acotado a 0.159.0. La nueva sección `mechanisms` diferencia los cinco mecanismos. Se imprimió únicamente la sección de compresión; no se recogieron credenciales ni cuerpos privados.

### Manifiesto fijado antes de candidatos (tarea 5)
Fixture ejecutable: `tests/fixtures/hcd001/corpus.json`, SHA-256 `f766c0d541960dcfbf88fbd147dbfe67f84245476486ab543ac8ac5e1906740d`. Incluye cinco fuentes sintéticas, cada una con huella de contenido UTF-8. Los contratos ejercitan autenticación, privacidad, presupuesto, compactación y recuperación; no se extrapola el resultado a todos los trabajos de programación ni se confunde el corpus sintético con una sesión de usuario real.

| Casos (uno por fuente, en el orden citado) | Ruta | Resultado esperado |
|---|---|---|
| search-01, search-02, search-03, search-04, search-05 | búsqueda propia | fuente correspondiente y literal de contrato esperado presentes, sin evidencia de otra fuente como sustituto |
| recovery-01, recovery-02, recovery-03, recovery-04, recovery-05 | entrega reducida y recuperación | original recuperado idéntico byte por byte y con su SHA-256; se cuenta entrega más recuperación |
| compaction-01, compaction-02, compaction-03, compaction-04, compaction-05 | delta, reinicio y entrega propia | evidencia necesaria vuelve a entregarse tras reinicio; casos pares ejercitan además `full=true` sin marcar lectura del principal |
| no_match-01, no_match-02, no_match-03, no_match-04, no_match-05 | búsqueda propia con zzsincoincidenciaNzz | cero evidencia seleccionada por coincidencia; solo contexto obligatorio si aplica y estado explícito sin coincidencia |

Orden de fuentes: autenticacion, privacidad, presupuesto, compactacion, recuperacion. Cada caso declara su entrada, fuente esperada, literal obligatorio y banderas de recuperación/reinicio en el fixture. Calidad por caso: todas las condiciones aplicables son obligatorias; no se compensa un fallo con el éxito de otro caso. Los originales de recuperación son sintéticos, no secretos. La prueba con secretos sintéticos de tarea 20 es un control separado, no un original que el corpus obliga a persistir.

### Protocolo fijado (tarea 6)
- Brazos: actual (checkout de referencia `6c747abd945dc6a7afb45e79be17baf29fe49e27`), mejora Go y selección Entroly fijada por commit. Cada uno recibe los mismos bytes de las fuentes, consulta, presupuesto y secuencia de entregas/recuperaciones. Configuración de compresión `max`; presupuesto de pack de 4000 tokens locales aproximados; no se cambia ese contador entre brazos. Las diferencias de serialización forman parte del coste y se describen.
- Unidad primaria: bytes UTF-8 de todo texto visible al modelo en las respuestas propias, incluidos sobres visibles, instrucciones, recibos, avisos, referencias y todas las recuperaciones requeridas por el caso. No se cuenta una entrada común como ahorro; no se compara contra la concatenación artificial de candidatos. El contador local de tokens es secundario y aproximado; no son tokens facturados.
- Frontera: registrar por separado salida real de CLI/MCP y captura verificada de entrada al modelo. Si esta última no puede observarse, se puede medir la primera, pero el resultado end-to-end de Codex permanece no verificable y no habilita una afirmación de aceptación global ni de ahorro del proveedor. Un adaptador de evaluación que no reproduce una ruta real no sustituye su evidencia.
- Ejecución: cinco repeticiones de cada uno de los 20 casos, con estado aislado y reinicializado por caso/repetición. Rotar orden actual/Go/Entroly entre repeticiones. Cronómetro monotónico desde la solicitud de entrega hasta completar las recuperaciones necesarias y comprobaciones de calidad; inicialización del laboratorio se registra aparte y no se mezcla con latencia de solicitud. Sin calentamiento pagado ni repetición implícita de tareas reales.
- Política de recuperación: los casos recovery recuperan íntegro el original correspondiente aunque el literal ya esté visible; los casos search recuperan solo si falta el literal esperado, contabilizando la recuperación; compaction ejecuta entrega, reinicio y nueva entrega, con `full=true` en casos pares; no-match no solicita originales inexistentes. Esta política es idéntica en los tres brazos.
- Reducción neta agregada: `1 - suma(bytes_mejora_con_recuperaciones) / suma(bytes_actual_con_recuperaciones)` sobre los pares válidos previstos. Denominador cero o cualquier caso ausente/no comparable produce inconcluso; no se eliminan fallos para mejorar el porcentaje. Publicar también los 20 resultados por tarea y las 100 observaciones, no solo la cifra agregada.
- Calidad: cero pérdidas en las condiciones esperadas del fixture y en los controles críticos de presupuesto, instrucciones obligatorias, integridad y privacidad. Un caso que falla marca el brazo como no conforme, aunque la tasa agregada mejore.
- Latencia: ordenar las 100 duraciones completas por brazo; p95 = elemento 95 (rango más cercano, índices desde 1). Aumento relativo = `p95_mejora / p95_actual - 1`. p95 actual cero, muestras ausentes o timeout no resuelto implica resultado inconcluso; los timeouts se conservan como fallos, no como duraciones exitosas.
- Umbrales de la mejora elegida: reducción neta >=20%, cero pérdidas de calidad, aumento p95 <=10%. Solo se declara PASS con todos los casos y fronteras requeridos verificados. Entroly puede quedar no comparable o bloqueado por límites de integración, sin atribuirle un resultado favorable ni activar dependencias de producción. Ningún umbral se ajusta después de observar resultados.

### Referencia capturada (tarea 10)
El binario de referencia se construyó desde un `git archive` de `6c747abd945dc6a7afb45e79be17baf29fe49e27` en un directorio temporal; no se alteró el checkout de trabajo. El ejecutor `scripts/hcd001_evaluate.py` completó 100 observaciones (20 casos por cinco repeticiones) con ese binario, antes de cambios de selección/presupuesto. Captura local: `/private/tmp/gomemory-hcd001-baseline.json`; incluye binario/corpus por SHA-256, stdout completo de cada entrega y recuperación, latencia monotónica y resultados por caso.

Se observaron 25 condiciones de calidad fallidas en la referencia, sin errores de proceso; se conservan para investigar y comparar, no se borran ni reinterpretan como éxitos. La frontera observada es stdout CLI, no entrada del modelo Codex: la aceptación global permanece inconclusa. El ejecutor ejecuta un brazo explícito por invocación y no garantiza aún la rotación entre brazos; esa limitación debe conservarse al interpretar latencia comparativa.

### Entroly bloqueado (tareas 21 y 23)
Decisión de la persona el 2026-10-29: «Registrar bloqueo (Recomendado)». Evaluar Entroly exige descargar `github.com/juyterman1000/entroly` en un commit fijado e instalar sus dependencias. Según las normas del entorno, eso solo puede hacerse dentro de un contenedor, y no se autorizó. No se descargó ni se ejecutó código externo, ni se tocaron dependencias de producción, configuración real o memoria. El brazo Entroly queda **no comparable**: no se le atribuye ningún resultado, ni favorable ni adverso.

### Brazo mejora Go (tarea 22)
Binario del árbol de trabajo (`a6f37979e3cb0fb3c11a79bb3090a3582e57c3a1758095bef5042e0352411b4c`), corpus `f766c0d5…`, `scripts/hcd001_evaluate.py --arm go`: 100 observaciones, cero errores de proceso. Para que la latencia fuera comparable, la referencia se recapturó justo después con un binario reconstruido desde `git archive` de `6c747abd` (`89b872d1…`). La huella difiere de la de la tarea 10 (`e53f03a2…`) porque la ruta de construcción entra en el binario, pero los bytes coinciden con la captura de la tarea 10 (370 960 en ambas). Capturas locales: `hcd001-go.json` (`f51f605b…`) y `hcd001-baseline-rerun.json` (`652f4e53…`) en el directorio temporal de la sesión; la referencia de la tarea 10 es `1714e25d…`. Límite que se conserva: los brazos se ejecutan uno tras otro, sin rotación intercalada, y la frontera observada es stdout de la CLI, no la entrada del modelo Codex.

### Veredicto comparativo (tarea 24): **FAIL**
| Categoría (5 casos × 5 repeticiones) | Bytes actual | Bytes Go | Reducción | Calidad actual | Calidad Go |
|---|---|---|---|---|---|
| search-01…05 (por caso) | 33 290 | 41 270 | −24,0% | 5/5 | 5/5 |
| recovery-01…05 (por caso) | 7 130–7 350 | igual | 0,0% | 0/5 | 0/5 |
| compaction-01…05 (por caso) | 30 575 | 30 575 | 0,0% | 5/5 | 5/5 |
| no_match-01…05 (por caso) | 3 050 | 6 170 | −102,3% | 5/5 | 5/5 |
| **Agregado (100 observaciones)** | **370 960** | **426 460** | **−14,96%** | 75/100 | 75/100 |

- Reducción neta: −14,96% (el coste aumenta). Umbral ≥20%: **FAIL**.
- Calidad: no hay pérdidas respecto de la referencia, pero ambos brazos fallan los 25 casos recovery, así que ninguno es conforme según el protocolo. Causa observada: `pack compress` no omite nada en esas fuentes (`fallback_reason=no_gain`, 317 tokens) y la limpieza estructural quita el salto final, de 1267 a 1266 bytes. El caso no llega a ejercitar la recuperación por ref. El corpus fijado no se ajusta después de observar resultados.
- Latencia p95: 87,78 ms actual frente a 92,92 ms Go, +5,86% (umbral ≤10%: cumple).
- Causa del aumento: el recibo de procedencia visible (≈1410 bytes por pack con 5 fuentes) y las líneas de contabilidad nuevas; en no-match, además, el aviso explícito. El corpus no tensa el presupuesto de 4000 tokens (las 5 fuentes ocupan ≈1584), y cada consulta search selecciona las 5 fuentes. Por eso la selección no aporta ahorro que compense los metadatos.
- Comparación con Entroly: no comparable (bloqueo registrado). No hay alternativa elegida y no se adopta nada externo.

Conclusión: las mejoras de VS-03/VS-04 aportan garantías verificadas (presupuesto de la respuesta completa, desbordamiento explícito, recuperación exacta, aislamiento por proyecto, privacidad), pero **no** el beneficio neto acordado en Q2. La tarea 24 queda sin verificar, como exige la definición. Opciones de cierre que requieren decisión de la persona y no se aplican aquí: (a) un recibo compacto u opt-in (`--receipt`) que no viaje por defecto; (b) una nueva unidad HCD con un corpus que tense el presupuesto y unos casos recovery que realmente produzcan omisiones, fijados antes de medir; (c) cerrar la unidad registrando el resultado insuficiente.
