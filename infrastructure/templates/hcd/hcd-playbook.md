# Playbook HCD

## 1. Propósito

Este Playbook fija cómo se recorre la ceremonia HCD de GoMemory, desde la intención hasta la entrega verificada y cerrada con la aprobación del Implementador. Las skills acompañan cada paso; el motor `mem hcd` deriva el paso, aplica los gates y es el único que registra aclaraciones, aprobaciones, evidencia y cierres. El Playbook remite a la Constitución y no puede relajarla.

## 2. Principios HCD y terminología

1. Intención antes que solución: definir el resultado antes del diseño técnico.
2. Comportamiento verificable: cada historia tiene criterios observables y cada tarea dice qué la demuestra.
3. La casilla no es la prueba: solo cuenta la comprobación observada por el motor.
4. Fricción proporcional al riesgo: la Vía rápida para lo ordinario, la ceremonia completa donde el fallo es caro.
5. El tamaño no elige el carril: lo elige la ambigüedad o el riesgo de decisiones interdependientes.
6. Entrega incremental: preparar el siguiente slice sin diseñar toda la unidad.
7. Atomicidad antes de ejecutar: no iniciar tareas ambiguas ni agrupar resultados independientes.
8. Promover, no reiniciar: lo hecho en la Vía rápida pasa entero a la ceremonia completa.
9. Memoria con procedencia: cada decisión persistida identifica unidad y origen.
10. Alcance explícito: no añadir capacidades por conveniencia técnica.

Unidad de trabajo: feature, evolución, corrección o mejora técnica. Carril: `full` (ceremonia completa: `define → clarify → approve → slice → tasks → implement → verify → close`) o `fast` (Vía rápida, sin `approve` ni `slice`). Documento de la unidad: `hcd/HCD-NNN-<slug>.md`, único artefacto. Vertical slice: incremento comprobable que atraviesa las capas necesarias. Tarea atómica: acción acotada con resultado y comprobación concretos. Evidencia: comprobación ejecutada por el motor, o una observación manual declarada como tal. Aprobación: decisión explícita del Implementador sobre una revisión determinada.

## 3. Carril y ceremonia

### HCD-FLOW-01. Elegir el carril por riesgo, no por tamaño
Descripción: Cada unidad declara su carril y el motivo. La ceremonia completa se elige por decisiones abiertas que se afectan entre sí, riesgo alto o petición del Implementador; el tamaño no cuenta. Una unidad rápida que descubre ambigüedad se promueve sin reiniciar.
Ámbito: `mem hcd start` y `mem hcd promote`.
Comprobación: structural
Evidencia: carril y motivo en `hcd_units` y en la memoria `decision` del carril.
Excepción: ninguna.
Origen: FR-001, FR-002, FR-020

### HCD-FLOW-02. Aclarar antes de inferir
Descripción: Las ambigüedades que afectan alcance, comportamiento, datos, seguridad o contrato se convierten en aclaraciones (`- [ ] Qn.` o `[NEEDS CLARIFICATION: …]`) y se resuelven con el Implementador, una pregunta a la vez.
Ámbito: definición, slices y tareas.
Comprobación: structural
Evidencia: aclaración resuelta en el documento y memoria `decision` con la respuesta.
Excepción: no admite avance mientras haya aclaraciones abiertas.
Origen: FR-010

### HCD-FLOW-03. Aprobación citada
Descripción: Una aprobación liga la respuesta literal del Implementador a una revisión exacta: la de la definición en `approve` y la de trabajo en `close`. Registra origen, cliente, fecha y las normas vigentes. La garantía es auditable y se basa en la confianza en el agente.
Ámbito: `approve`, `close` y `cancel`.
Comprobación: structural
Evidencia: registro en `hcd_approvals`.
Excepción: no admite aprobación sin cita.
Origen: FR-011, FR-018, FR-019

Riesgo residual: el registro demuestra qué texto recibió el motor, pero no prueba que lo haya escrito una persona y no un agente.

## 4. Unidades de trabajo y sus tipos

### HCD-DEF-01. Tipo y secciones obligatorias
Descripción: Cada unidad declara `feature`, `evolution`, `fix` o `technical` y completa Intención, Alcance, «¿Qué pasa si no lo hacemos?» y «Alternativa más barata». Un `fix` añade Reproducción y nace con la tarea «La reproducción ya no falla».
Ámbito: documento de la unidad.
Comprobación: structural
Evidencia: `pending_checks` vacío en `mem hcd show`.
Excepción: ninguna.
Origen: FR-003, FR-005, FR-007

## 5. Historias y criterios

### HCD-TRACE-01. Criterios verificables
Descripción: En la ceremonia completa, cada historia incluye uno o más criterios observables («Dado…, cuando…, entonces…»).
Ámbito: sección Historias.
Comprobación: structural
Evidencia: `pending_checks` y `analyze` sin historias sin criterio.
Excepción: no admite excepción para aprobar una definición.
Origen: FR-006, FR-017

## 6. Vertical slicing

### HCD-SLICE-01. Slice observable y trazable
Descripción: Cada slice entrega un comportamiento observable y cita sus tareas y criterios. Entre todos cubren todos los criterios.
Ámbito: sección Slices (ceremonia completa).
Comprobación: structural
Evidencia: `analyze` sin criterios sin cubrir, tareas huérfanas ni referencias inexistentes.
Excepción: agrupaciones horizontales solo como habilitantes que digan qué desbloquean.
Origen: FR-012, FR-017

## 7. Tareas atómicas

### HCD-TASK-01. Tareas atómicas con comprobación
Descripción: Una tarea tiene verbo y objeto concretos, un resultado inspeccionable y la comprobación que la demuestra (`- [ ] N. tarea → comprobación`).
Ámbito: sección Checklist.
Comprobación: structural
Evidencia: `pending_checks` sin `CHECK-MISSING`.
Excepción: ninguna.
Origen: FR-007

## 8. Pruebas, evidencia y vigencia

### HCD-EVIDENCE-01. Evidencia vigente
Descripción: Una tarea solo está verificada con evidencia del motor cuya revisión de trabajo y huella del código coinciden con las actuales. La evidencia caducada se conserva y no cuenta.
Ámbito: `mem hcd verify` y cierre.
Comprobación: structural
Evidencia: bloque Evidencia del documento y `hcd_evidence`.
Excepción: la evidencia manual se admite, marcada como tal y reportada por `analyze`.
Origen: FR-013, FR-014, FR-016

### HCD-TDD-01. Rojo antes del verde
Descripción: En un `fix`, la comprobación de la tarea 1 se ejecuta con `--expect-fail` sobre el código sin el cambio y debe fallar; el verde llega después sobre el código corregido. En los demás tipos, toda tarea con un intento rojo debe tener un rojo real.
Ámbito: `hcd-implement` y `hcd-verify`.
Comprobación: structural
Evidencia: `red_missing` vacío al cerrar.
Excepción: fuera de git el rojo se acepta por orden, con aviso.
Origen: FR-015

## 9. Análisis y cierre

### HCD-CLOSE-01. Cierre sin hallazgos críticos
Descripción: Solo se cierra en el paso `close`, sin hallazgos `CRITICAL` de `analyze` y con la aprobación citada sobre la revisión de trabajo.
Ámbito: `mem hcd analyze` y `mem hcd close`.
Comprobación: structural
Evidencia: aprobación `close` en `hcd_approvals` y memoria `decision` del cierre.
Excepción: cancelar requiere una cita con el motivo.
Origen: FR-017, FR-018, FR-019

## 10. Reglas para agentes

### HCD-AGENT-01. Las skills delegan en el motor
Descripción: Las skills redactan el documento y conversan; cada aclaración, aprobación, evidencia y cierre pasa por `mem hcd … --json` (o `./mem`). Si el motor rechaza, no se edita el estado para compensarlo.
Ámbito: Claude Code, Codex, OpenCode, TUI y CLI.
Comprobación: structural
Evidencia: pruebas de contrato de las skills y del motor.
Excepción: ninguna.
Origen: FR-023, FR-024, `contracts/skills.md` §2

### HCD-AGENT-02. Unidad activa interna
Descripción: La unidad activa vive en `hcd_units` y nunca en archivos de contexto del agente ni en marcadores `SPECKIT`. En Claude Code, editar código sin unidad activa avisa una vez por sesión, sin bloquear.
Ámbito: arranque, reanudación y edición.
Comprobación: structural
Evidencia: sección «HCD activo» del contexto y aviso del hook `expand pre-tool`.
Excepción: ninguna.
Origen: FR-021, FR-025, FR-026

## 11. Diseño

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
| §10 | HCD-EVIDENCE-01, HCD-TDD-01 | Pruebas y cobertura | Evidencia vigente y rojo observado |
| §16 | HCD-TRACE-01 | Cambios y commits | Revisión de entrega |
| §18 | HCD-DEF-01 | Artefactos por enfoque | Documento único `hcd/HCD-NNN-<slug>.md` |
| §20 | HCD-AGENT-01 | Dependencias y secretos | Revisión de alcance y pruebas |
| §21 | HCD-TASK-01, HCD-SLICE-01 | Ciclo de vida | Contrato del motor y `analyze` |

La tabla remite a las secciones vigentes de la Constitución; no las reproduce ni las sustituye.

## 12. Precedencia normativa

### HCD-GOV-01. Precedencia y cambio de normas
Descripción: Se respeta la precedencia de seguridad y cumplimiento, Constitución, Playbook, decisiones de la unidad, definición, slices y tareas. Cada aprobación guarda la revisión de la Constitución y del Playbook vigentes; si cambian tras aprobar la definición, la unidad se revisa con las reglas nuevas y se reaprueba antes de cerrar.
Ámbito: todas las reglas y unidades.
Comprobación: structural
Evidencia: `norms_changed` y el hallazgo `NORMS-CHANGED` de `analyze`.
Excepción: una regla constitucional solo se exceptúa por el mecanismo de la Constitución.
Origen: FR-029
