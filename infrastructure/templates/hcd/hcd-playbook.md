# Playbook HCD

## 1. Propósito

Este Playbook fija el protocolo HCD de GoMemory para conducir una unidad desde la intención hasta la entrega verificada. Las skills acompañan al Implementador y delegan cada transición al motor `mem hcd`; el motor conserva el estado de control y aplica sus invariantes.

## 2. Principios HCD y terminología

1. Intención antes que solución: definir el resultado antes del diseño técnico.
2. Comportamiento verificable: cada historia tiene criterios observables.
3. Entrega incremental: preparar el siguiente slice sin diseñar toda la unidad.
4. Atomicidad antes de ejecutar: no iniciar tareas ambiguas ni agrupar resultados independientes.
5. Pruebas desde los criterios: definir la verificación antes de implementar.
6. Evidencia para verificar: un cambio de código por sí solo no demuestra cumplimiento.
7. Memoria con procedencia: cada decisión persistida identifica ámbito y origen.
8. Alcance explícito: no añadir capacidades por conveniencia técnica.
9. Bloqueo localizado: detener solo el trabajo afectado por incertidumbre.
10. Un solo enfoque activo: no ejecutar HCD y Spec Kit simultáneamente sobre una unidad.
11. Proporcionalidad: cada paso obligatorio aclara, decide, limita o verifica.
12. Continuidad: retomar no repite acciones ya ejecutadas.

Unidad de trabajo: feature, evolución, corrección o mejora técnica. Enfoque: protocolo activo (`hcd`, `speckit` o `unclassified`). Playbook: estas reglas fijadas y versionadas. Core HCD: modelo y reglas de definición, decisiones, trazabilidad, estados y verificación. Harness HCD: componente que conduce el protocolo y aplica controles. Adaptador: skill, TUI o CLI que usa el mismo core. Vertical slice: incremento comprobable que atraviesa las capas necesarias. Tarea atómica: acción acotada con resultado y verificación concretos. Evidencia: registro reproducible o inspeccionable. Aprobación: decisión explícita del Implementador sobre un objeto y revisión determinados.

## 3. Ceremonia de especificación y selección de enfoque

### HCD-FLOW-01. Selección única de enfoque
Descripción: Cada unidad comienza con una selección explícita de HCD o Spec Kit; una unidad no usa ambos enfoques simultáneamente.
Ámbito: creación y clasificación de unidades.
Comprobación: review
Evidencia: enfoque registrado en el estado de control y artefactos de la unidad.
Excepción: no admite excepción por unidad.
Origen: FR-001–FR-006

### HCD-FLOW-02. Aclarar antes de inferir
Descripción: Las ambigüedades que afectan alcance, comportamiento o aprobación se convierten en preguntas explícitas para el Implementador.
Ámbito: definición, tareas y decisiones.
Comprobación: review
Evidencia: aclaración registrada con pregunta, respuesta y decisión relacionada.
Excepción: no admite excepción si la ambigüedad bloquea el resultado.
Origen: FR-016a

### HCD-FLOW-03. Aprobación citada
Descripción: Una aprobación identifica el objeto y revisión mostrados y conserva la respuesta citada, el origen, el entorno y la fecha. La garantía es auditable y se basa en la confianza en el agente.
Ámbito: toda acción que requiere aprobación del Implementador.
Comprobación: structural
Evidencia: registro de aprobación asociado al objeto y a su revisión.
Excepción: no admite aprobación sin cita.
Origen: FR-017–FR-017b

## 4. Unidades de trabajo y sus tipos

### HCD-DEF-01. Tipo y campos particulares
Descripción: Cada unidad declara uno de los tipos `feature`, `evolution`, `fix` o `technical` y completa los campos particulares definidos para ese tipo.
Ámbito: `definition.md`.
Comprobación: structural
Evidencia: resultado de `validate` con tipo y campos comprobados.
Excepción: causa raíz pendiente solo se admite temporalmente en una unidad `fix`.
Origen: FR-014

## 5. Historias y criterios

### HCD-TRACE-01. Criterios verificables
Descripción: Cada historia incluye uno o más criterios observables y cada criterio declara una forma de verificación.
Ámbito: definición y trazabilidad.
Comprobación: structural
Evidencia: enlaces de historia a criterio y criterio a verificación.
Excepción: no admite excepción para aprobar una definición.
Origen: FR-015

## 6. Aclaración

### HCD-CLARIFY-01. Registrar aclaraciones bloqueantes
Descripción: Una ambigüedad bloqueante se registra como aclaración abierta; validar o aprobar el objeto afectado se bloquea hasta responderla.
Ámbito: definiciones y tareas.
Comprobación: structural
Evidencia: aclaración, respuesta y decisión asociada.
Excepción: no admite avance sobre el objeto bloqueado.
Origen: FR-016a

## 7. Aprobación citada

### HCD-APPROVAL-01. Registrar origen y cita
Descripción: El motor registra si la aprobación vino de conversación o terminal, junto con la cita literal, cliente, fecha y revisión.
Ámbito: aprobaciones de definición, excepción, cierre, cancelación, clasificación, adopción y migración.
Comprobación: structural
Evidencia: registro de aprobación visible en `verification.md`.
Excepción: no admite aprobación sin respuesta citada.
Origen: FR-017a–FR-017c

Riesgo residual: el registro demuestra qué texto recibió el motor, pero no prueba que una persona y no un agente lo haya producido. La garantía depende de la confianza en el agente y deja evidencia auditable.

## 8. Vertical slicing

### HCD-SLICE-01. Slice observable
Descripción: Cada slice declara comportamiento observable, historias y criterios, límites, impacto arquitectónico conocido, verificación esperada, incertidumbres y dependencias.
Ámbito: `delivery.md` y planificación incremental.
Comprobación: structural
Evidencia: slice enlazado con criterios y tareas.
Excepción: agrupaciones horizontales solo pueden ser habilitantes y justificar qué desbloquean.
Origen: FR-020

## 9. Tareas atómicas y trazabilidad

### HCD-TASK-01. Admitir solo tareas atómicas enlazadas
Descripción: Una tarea tiene verbo y objeto concretos, un resultado inspeccionable, verificación, alcance, exclusiones, dependencias resueltas, slice, justificación y reglas aplicables.
Ámbito: admisión e inicio de tareas.
Comprobación: structural
Evidencia: resultado de `check` y enlaces historia → criterio → slice → tarea.
Excepción: una tarea habilitante debe declarar qué slice u obligación desbloquea y cómo se verifica.
Origen: FR-021–FR-024, FR-036

### HCD-TRACE-02. Mantener trazabilidad navegable
Descripción: El estado de control enlaza historias, criterios, slices, tareas, controles, evidencias y decisiones sin duplicar estados en Markdown.
Ámbito: todo el ciclo de vida.
Comprobación: structural
Evidencia: recorrido del grafo HCD y `verification.md` generado.
Excepción: ninguna.
Origen: FR-022–FR-024

## 10. Pruebas, evidencia y vigencia

### HCD-EVIDENCE-01. Verificación con evidencia vigente
Descripción: Cada criterio se asocia a una prueba o control y el motor invalida evidencia cuando cambian sus revisiones de entrada.
Ámbito: ejecución, reporte y verificación.
Comprobación: structural
Evidencia: procedimiento reproducible, entorno, sujeto y resultado inspeccionable.
Excepción: evidencia obsoleta se conserva, pero no cuenta para PASS ni cierre.
Origen: FR-029–FR-032

### HCD-MEM-01. Cierre con memoria no disponible
Descripción: La sesión solo se cierra si la memoria requerida está disponible y sincronizada; ante fallo se conserva el artefacto y la sincronización queda pendiente.
Ámbito: cierre de sesión y sincronización de decisiones.
Comprobación: structural
Evidencia: estado de sincronización y reintento idempotente registrado.
Excepción: solo mediante una excepción HCD aprobada y vigente, con alcance y revisión explícitos.
Origen: FR-038–FR-043

## 11. Verificación y cierre

### HCD-CLOSE-01. Cierre desde verificación reproducible
Descripción: El estado global se calcula desde criterios, controles y evidencias vigentes; no se deriva de marcas manuales en `delivery.md`.
Ámbito: `verify`, `status` y cierre.
Comprobación: structural
Evidencia: `verification.md` generado con resultado, trazabilidad y acción requerida.
Excepción: cierre rechazado o cancelado requiere una aprobación citada cuando corresponda.
Origen: FR-033–FR-035

## 12. Reglas para agentes

### HCD-AGENT-01. Las skills delegan mutaciones al motor
Descripción: Las skills explican, solicitan aprobación cuando aplica y presentan respuestas; cada mutación se ejecuta exclusivamente mediante `mem hcd … --json` (o `./mem` si corresponde). Si el motor rechaza, no se edita el estado para compensarlo.
Ámbito: Claude Code, Codex, OpenCode, TUI y CLI.
Comprobación: structural
Evidencia: pruebas de contrato de las skills y del motor.
Excepción: ninguna.
Origen: `contracts/skills.md` §2

## 13. Memoria, unidad activa y heartbeat

### HCD-MEM-02. Unidad activa interna
Descripción: La unidad activa vive en el estado de control HCD y nunca en archivos de contexto del agente ni en marcadores `SPECKIT`.
Ámbito: arranque, reanudación y fin de turno.
Comprobación: structural
Evidencia: `hcd_focus` y heartbeat por entorno.
Excepción: ninguna.
Origen: FR-041–FR-043

### HCD-CLEAN-01. Dependencias hacia el dominio
Descripción: Las dependencias de código apuntan hacia el dominio, preservando sus reglas frente a detalles externos.
Ámbito: arquitectura de producción.
Comprobación: structural
Evidencia: `tests/contract/layer_rules_test.go` y verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; complementa §3

### HCD-CLEAN-02. Independencia del core
Descripción: Entidades y casos de uso permanecen independientes de frameworks, bases de datos, interfaces y agentes.
Ámbito: dominio y aplicación.
Comprobación: review
Evidencia: revisión técnica de dependencias y verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; complementa §3

### HCD-CLEAN-03. Límites mediante ports
Descripción: Los cruces entre capas se realizan mediante ports y adaptadores.
Ámbito: dominio, aplicación e infraestructura.
Comprobación: structural
Evidencia: guarda de capas y revisión de wiring en verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; remite a §3

### HCD-CLEAN-04. Datos simples en los límites
Descripción: Los datos que cruzan límites son estructuras simples y no exponen tipos de infraestructura.
Ámbito: interfaces de ports y casos de uso.
Comprobación: review
Evidencia: revisión de contratos y verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; remite a §3

### HCD-SOLID-SRP. Responsabilidad única
Descripción: Cada módulo mantiene una responsabilidad cohesionada y un motivo principal de cambio.
Ámbito: diseño de paquetes, tipos y funciones.
Comprobación: review
Evidencia: revisión técnica y verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; complementa §3

### HCD-SOLID-OCP. Abierto/cerrado
Descripción: Las extensiones previsibles no requieren modificar consumidores estables sin una razón justificada.
Ámbito: puntos de variación del dominio y aplicación.
Comprobación: review
Evidencia: revisión técnica y verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; complementa §3

### HCD-SOLID-LSP. Sustitución de Liskov
Descripción: Las implementaciones intercambiables respetan el contrato observable de sus abstracciones.
Ámbito: ports y adaptadores.
Comprobación: review
Evidencia: pruebas de contrato y verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; complementa §3

### HCD-SOLID-ISP. Segregación de interfaces
Descripción: Los consumidores dependen de contratos pequeños que contienen las operaciones que usan.
Ámbito: definición de ports.
Comprobación: review
Evidencia: revisión de contratos y verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; complementa §3

### HCD-SOLID-DIP. Inversión de dependencias
Descripción: Las políticas de alto nivel dependen de abstracciones estables y no de adaptadores concretos.
Ámbito: aplicación y composición.
Comprobación: structural
Evidencia: guarda de capas, revisión del composition root y verificación HCD.
Excepción: no puede relajar §3 de la Constitución.
Origen: Playbook; remite a §3

## Engineering Guardrails

| Constitución | Regla del Playbook | Ámbito | Control |
|---|---|---|---|
| §3 | HCD-CLEAN-01–04, HCD-SOLID-* | Capas y límites | Guarda estructural y revisión técnica |
| §5 | HCD-EVIDENCE-01 | Persistencia y transacciones | Pruebas de contrato e integración |
| §6 | HCD-EVIDENCE-01 | Migraciones | Prueba de migración aditiva |
| §10 | HCD-EVIDENCE-01 | Pruebas y cobertura | Evidencia reproducible |
| §16 | HCD-TRACE-01 | Cambios y commits | Revisión de entrega |
| §20 | HCD-AGENT-01 | Dependencias y secretos | Revisión de alcance y pruebas |
| §21 | HCD-TASK-01, HCD-TRACE-02 | Ciclo de vida | Contrato del motor y trazabilidad |
| §23 | HCD-CLOSE-01 | Excepciones | Registro aprobado y vigente |

La tabla remite a las secciones vigentes de la Constitución; no las reproduce ni las sustituye.

## 14. Migración desde Spec Kit

### HCD-MIGRATE-01. Migración explícita y no inferencial
Descripción: La propuesta de migración clasifica cada elemento y no convierte inferencias ni tareas terminadas de Spec Kit en decisiones o verificaciones aprobadas.
Ámbito: vista previa y activación de migración.
Comprobación: structural
Evidencia: propuesta, clasificación, referencias y aprobación citada de activación.
Excepción: ninguna.
Origen: FR-054–FR-056

## 15. Gobierno de excepciones y evolución del Playbook

### HCD-GOV-01. Precedencia normativa
Descripción: Se respeta la precedencia de seguridad y cumplimiento, Constitución, Playbook y extensiones aprobadas, decisiones de unidad, definición, slices y tareas.
Ámbito: todas las reglas y excepciones.
Comprobación: review
Evidencia: validación del Playbook, registro de excepción y verificación HCD.
Excepción: una regla constitucional solo se exceptúa por el mecanismo de la Constitución.
Origen: FR-011–FR-013, FR-037

### HCD-GOV-02. Adopción explícita de revisiones
Descripción: Cada unidad fija las revisiones normativa de Playbook y Constitución; una revisión nueva solo se adopta tras mostrar diferencias e impacto y registrar motivo, responsable y cita.
Ámbito: documentos normativos fijados.
Comprobación: structural
Evidencia: revisiones conservadas, diff mostrado y adopción por unidad.
Excepción: ninguna adopción implícita.
Origen: FR-012
