# Feature Specification: HCD — ceremonia completa sobre núcleo ligero, con Vía rápida

**Feature Branch**: `045-hcd-protocol`

**Created**: 2026-10-27 · **Revisada**: 2026-10-29 (v2)

**Status**: Draft

**Input**: User description: "HCD (Harness Continual Development) es el protocolo propio de gomemory para llevar una unidad de trabajo (una corrección, una feature, una evolución o una mejora técnica) desde la intención hasta una entrega verificada y cerrada con tu aprobación."

## Contexto y trazabilidad

- La v1 (72 FR, 16 tablas) está en el commit `a0503b0`: `git show a0503b0:specs/045-hcd-protocol/spec.md`. La persona la juzgó sobreingenierizada el 2026-10-29. **La sobreingeniería estaba en la maquinaria, no en la ceremonia**: esta v2 conserva los pasos y reduce lo que hay debajo.
- **La ceremonia manda.** La Vía rápida es un atajo dentro de HCD para no recorrer la ceremonia completa en el trabajo ordinario; no la sustituye. Spec Kit coexiste aparte y no forma parte de HCD.
- Ideas tomadas de [ericrisco/rsc-harness](https://github.com/ericrisco/rsc-harness): decisor de carril, documento único, «la casilla no es la prueba», promover sin reiniciar y las preguntas «¿qué pasa si no lo hacemos?» y «alternativa más barata».
- Lo que gomemory añade: el **motor** `mem hcd` aplica los gates (aprobación de revisión exacta, evidencia vigente, rojo TDD, análisis de coherencia) igual en Claude Code, Codex, OpenCode y la TUI.
- Único actor humano: el **Implementador**. El agente asiste y nunca convierte sus inferencias en aprobaciones.

## Clarifications

### Session 2026-10-29

- Q: ¿Se conserva la ceremonia de HCD? → A: Sí, completa: `/hcd → define → clarify → approve → slice → tasks → implement → verify → close`, más `/hcd-analyze` para confirmar la coherencia.
- Q: ¿Qué es la Vía rápida? → A: Un carril de HCD que omite `approve` y `slice`: `/hcd → define → (clarify) → tasks → implement → verify → close`. La única aprobación es la del cierre.
- Q: ¿Qué se simplifica? → A: La maquinaria: de 16 tablas a 3; sin operations, leases/executions, heartbeats, prompts de aprobación de un solo uso, revisiones normativas fijadas ni adopciones del Playbook.
- Q: ¿Una unidad es una historia de usuario? → A: No. Una unidad equivale a una feature. En el carril completo contiene historias (US) con criterios (C); en la Vía rápida, el checklist hace de criterio.
- Q: ¿Dónde vive el documento? → A: (D2) Dentro del repo, en `hcd/HCD-NNN-<slug>.md`, para que viaje en el PR.
- Q: ¿Se avisa al editar código sin unidad activa? → A: (D3) Sí, con un aviso no bloqueante del hook, una vez por sesión.
- Q: ¿HCD aplica TDD? → A: En `fix` es obligatorio observar el rojo: antes del cambio, `verify --expect-fail` registra que la reproducción falla; el cierre exige ese rojo y un verde posterior. En los demás tipos, `--expect-fail` es opcional por tarea.
- Q: ¿Qué pasa con `hcd-exception`, `hcd-playbook` y `hcd-migrate`? → A: Se retiran como parte de la maquinaria. El Playbook sigue como documento fijado (`mem playbook`).
- Q: ¿La Constitución que se carga por la TUI aplica a HCD? → A: Sí, de forma transversal (FR-029): regla común en las skills y gate ligero en el motor (huellas de las normas en cada aprobación; cambio tras aprobar → `HIGH`, y `CRITICAL` al cerrar hasta reaprobar).
- Q: ¿Cómo se trata el schema v4 que el binario de la rama ya aplicó a 85 BD? → A: Schema v5 con una excepción acotada a «solo aditivo»: DROP de las 16 tablas `hcd_*` v1 (nunca publicadas) y creación de las 3 nuevas en la misma transacción. Las memorias no se tocan (ADR 0003).

### Session 2026-10-27 (vigente de v1)

- La invocación principal es conversacional (`/hcd`, `$hcd`); el motor `mem hcd … --json` lo llaman las skills por dentro.
- Las decisiones se guardan en la memoria existente de GoMemory, no en un sistema paralelo.
- HCD no escribe archivos de contexto del agente ni marcadores como los `SPECKIT`.
- La aprobación conversacional se basa en confianza y es auditable: la skill pide una elección explícita y el motor guarda la cita de la persona.
- El Playbook es un documento fijado (como la Constitución), con Arquitectura Limpia y SOLID. Las skills lo leen como guía.
- Antes de decidir por probabilidad, el protocolo pregunta (`hcd-clarify`).

### Principios HCD

1. **Intención antes que solución.**
2. **Comportamiento verificable:** cada historia tiene criterios observables; cada tarea dice qué la demuestra.
3. **La casilla no es la prueba:** solo cuenta la comprobación observada por el motor.
4. **Fricción proporcional al riesgo:** la Vía rápida para lo ordinario, la ceremonia completa donde el fallo es caro.
5. **El tamaño no elige el carril:** lo elige la ambigüedad o el riesgo de decisiones interdependientes.
6. **Promover, no reiniciar.**
7. **Un gate que no comprueba nada es peor que no tener gate:** cada control lo aplica el motor o no existe.
8. **Alcance explícito.**

### Terminología

| Término | Definición |
|---|---|
| Unidad de trabajo | Feature, evolución, corrección (`fix`) o mejora técnica gestionada por HCD. |
| Carril | `fast` (Vía rápida) o `full` (ceremonia completa). El decisor también puede responder sin crear unidad. |
| Documento de la unidad | `hcd/HCD-NNN-<slug>.md`, único artefacto de la unidad en ambos carriles. |
| Revisión de la definición | SHA-256 normalizado de Intención, Reproducción, Alcance, Historias y Aclaraciones. Es lo que se aprueba en `approve`. |
| Revisión de trabajo | Revisión de la definición más Slices y Checklist, sin casillas. La evidencia y el cierre se ligan a ella. |
| Huella del código | `HEAD` más el contenido de los archivos cambiados y sin rastrear, excluyendo `hcd/`. Fuera de git, `nogit`. |
| Evidencia | Registro del motor: comando, código de salida, extracto, revisión de trabajo y huella; o una observación `manual`. |
| Análisis | Consulta de coherencia del motor (`analyze`) con hallazgos de severidad `critical`, `high` o `medium`. |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Recorrer la ceremonia completa (Priority: P1)

Como Implementador, quiero conducir una feature por `define → clarify → approve → slice → tasks → implement → verify → close`, con el motor diciendo el paso y la siguiente skill, para entregar trabajo complejo con alcance aprobado y evidencia.

**Independent Test**: crear una feature `full`, dejar una aclaración abierta, resolverla, aprobar la definición, definir un slice y sus tareas, verificarlas y cerrar. Comprobar en cada paso el `stage` y la `next_action`.

**Acceptance Scenarios**:

1. **Given** una unidad `full` con secciones obligatorias vacías, **When** se consulta, **Then** el paso es `define` y se listan los faltantes, incluidas las historias con criterios.
2. **Given** una aclaración abierta (`- [ ] Q1.` o un `[NEEDS CLARIFICATION]`), **When** se consulta, **Then** el paso es `clarify` y la siguiente skill es `hcd-clarify`.
3. **Given** `clarify --id Q1 --answer "…"`, **When** se ejecuta, **Then** la línea queda resuelta en el documento y se guarda una memoria `decision`.
4. **Given** una definición completa sin aprobación vigente, **When** se consulta, **Then** el paso es `approve`; `approve --quote` registra la aprobación sobre la revisión exacta de la definición.
5. **Given** una definición aprobada que después se edita, **When** se consulta, **Then** vuelve al paso `approve`.
6. **Given** una definición aprobada sin slices, **When** se consulta, **Then** el paso es `slice`; con slices pero sin tareas, `tasks`.
7. **Given** tareas sin hacer, **When** se consulta, **Then** el paso es `implement`; con tareas hechas sin evidencia vigente, `verify`; con todo verificado y sin hallazgos `critical`, `close`.

---

### User Story 2 - Vía rápida (Priority: P1)

Como Implementador, quiero que un fix o un cambio ordinario recorra `define → tasks → implement → verify → close` con un solo documento, sin `approve` ni `slice`, para no pagar la ceremonia completa en lo trivial.

**Independent Test**: crear un `fix`, comprobar que el documento exige Reproducción y trae la tarea 1 «la reproducción ya no falla», observar el rojo, corregir, verificar el verde y cerrar.

**Acceptance Scenarios**:

1. **Given** una unidad `fix` `fast`, **When** se crea, **Then** el documento tiene Reproducción y la tarea 1 «La reproducción ya no falla → re-ejecutar la reproducción», y no tiene Historias ni Slices.
2. **Given** un documento `fast` válido, **When** se consulta, **Then** pasa de `define` a `implement` sin pedir aprobación previa.
3. **Given** un `[NEEDS CLARIFICATION]` en una sección obligatoria, **When** se consulta, **Then** el paso es `clarify`.

---

### User Story 3 - Gates de evidencia, TDD y cierre (Priority: P1)

Como Implementador, quiero que solo cuente como hecho lo que se observó y que el cierre requiera mi aprobación sobre lo que realmente se evaluó.

**Independent Test**: registrar evidencia, modificar el código, comprobar que caduca; cerrar un fix sin rojo previo y comprobar que se rechaza.

**Acceptance Scenarios**:

1. **Given** `verify --task N -- <comando>`, **When** termina, **Then** se guarda la evidencia y el bloque Evidencia del documento se reescribe; si pasó, la tarea se marca `[x]`.
2. **Given** un código de salida distinto de 0, **When** se registra, **Then** la tarea queda fallida, no verificada.
3. **Given** evidencia registrada, **When** cambia la huella del código o la revisión de trabajo, **Then** la evidencia caduca.
4. **Given** un `fix` sin rojo observado en la tarea 1, **When** todo lo demás está verificado, **Then** el paso es `verify` y el cierre se rechaza.
5. **Given** todo verificado y una cita válida, **When** se cierra, **Then** la aprobación guarda revisión de trabajo, huella, origen y cita protegida, y la unidad queda `closed`.

---

### User Story 4 - Analizar la coherencia (Priority: P1)

Como Implementador, quiero un análisis que confirme que todo está bien antes de implementar y antes de cerrar.

**Independent Test**: crear una unidad `full` con un criterio sin slice y una tarea sin slice; `analyze` los reporta; corregirlos deja el análisis sin hallazgos `critical`.

**Acceptance Scenarios**:

1. **Given** una historia sin criterios, un criterio que ningún slice cubre o una tarea fuera de todo slice, **When** se ejecuta `analyze`, **Then** cada caso aparece como hallazgo con su severidad y su referencia.
2. **Given** una aprobación caducada, aclaraciones abiertas, tareas sin evidencia vigente o rojo pendiente, **When** se ejecuta `analyze`, **Then** aparecen como hallazgos.
3. **Given** `analyze`, **When** se ejecuta, **Then** no escribe estado, documento ni evidencia.
4. **Given** un hallazgo `critical`, **When** se intenta cerrar, **Then** el cierre se rechaza.

---

### User Story 5 - Promover sin reiniciar (Priority: P2)

Como Implementador, quiero pasar una unidad de la Vía rápida a la ceremonia completa conservando lo hecho.

**Acceptance Scenarios**:

1. **Given** una unidad `fast` con evidencia, **When** se promueve, **Then** el carril pasa a `full`, el documento recibe las secciones Historias y Slices, la evidencia se conserva y el paso pasa a `define` o `approve` según lo que falte.

---

### User Story 6 - Retomar en cualquier agente (Priority: P2)

**Acceptance Scenarios**:

1. **Given** una unidad activa, **When** arranca una sesión en cualquier agente, **Then** el contexto incluye «HCD activo» con unidad, carril, paso, tareas sin verificar y siguiente skill.
2. **Given** una consulta (`show`, `next`, `analyze`), **When** se ejecuta, **Then** no cambia estado, revisión ni evidencia.

---

### User Story 7 - Aviso al editar sin unidad activa (Priority: P3)

**Acceptance Scenarios**:

1. **Given** ninguna unidad activa, **When** el agente edita un archivo de código en Claude Code, **Then** el hook avisa una vez por sesión y permite la edición.

### Edge Cases

- Edición manual del documento: se valida en la siguiente consulta; nunca cuenta como aprobación ni como evidencia.
- Evidencia `manual`: queda marcada como manual en el documento, el análisis y el cierre.
- Repositorio sin git: la huella es `nogit`; el rojo se acepta si es anterior al verde y la respuesta lo avisa.
- Dos unidades activas: no se admite; actuar sobre una la deja activa.

## Requirements *(mandatory)*

### Functional Requirements

**Unidad, carril y documento**

- **FR-001**: La skill `hcd` DEBE clasificar el pedido (responder sin unidad, `fast` o `full`) y anunciar el carril con su motivo. `full` se elige por decisiones abiertas interdependientes, riesgo alto o petición de la persona; el tamaño no cuenta.
- **FR-002**: `start --kind --lane --title --why` DEBE crear la unidad, su documento y dejarla activa; sin motivo se rechaza.
- **FR-003**: Tipos: `feature`, `evolution`, `fix`, `technical`. Solo `fix` exige Reproducción.
- **FR-004**: Como máximo una unidad activa por proyecto, compartida por agentes y TUI.
- **FR-005**: El documento DEBE tener Intención, Reproducción (`fix`), Alcance (Incluido/Excluido), «¿Qué pasa si no lo hacemos?», «Alternativa más barata», Aclaraciones, Checklist, Evidencia y Siguiente. En `full` añade Historias y Slices.
- **FR-006**: Historias (`full`): `### USn. título`, texto «Como…, quiero…, para…» y criterios `- C-NN: Dado…, cuando…, entonces…`. Cada historia necesita al menos un criterio.
- **FR-007**: Checklist: `- [ ] N. tarea → comprobación`. Una línea sin comprobación es un faltante. Un `fix` nace con la tarea 1 «La reproducción ya no falla → re-ejecutar la reproducción».
- **FR-008**: La validación DEBE enumerar los faltantes (secciones vacías o con marcadores de pendiente) e informar lo que no comprueba: que el contenido sea correcto.

**Ceremonia**

- **FR-009**: El paso DEBE derivarse del documento, las aprobaciones y la evidencia, nunca fijarse desde un adaptador. Carril `full`: `define → clarify → approve → slice → tasks → implement → verify → close`. Carril `fast`: `define → clarify (si hay aclaraciones abiertas) → tasks → implement → verify → close`.
- **FR-010**: Una aclaración está abierta si su línea es `- [ ] Qn. pregunta` o si una sección normativa contiene `[NEEDS CLARIFICATION: …]`. `clarify --ask "…"` añade una pregunta; `clarify --id Qn --answer "…"` la resuelve en el documento y guarda una memoria `decision`.
- **FR-011**: `approve --quote` (solo `full`) DEBE registrar la aprobación sobre la revisión exacta de la definición. Si la definición cambia, la aprobación deja de estar vigente y el paso vuelve a `approve`.
- **FR-012**: Slices (`full`): `### VS-NN. comportamiento` con `Tareas: 1, 2` y `Criterios: C-01`. Sin slices, el paso es `slice`; sin tareas, `tasks`.

**Gates**

- **FR-013**: `verify --task N[,M] [--expect-fail] -- <comando>` DEBE ejecutar el comando y guardar código de salida, extracto acotado, revisión de trabajo y huella. `--manual "<observación>"` registra evidencia manual.
- **FR-014**: Una tarea está verificada solo si su evidencia más reciente vigente pasó. Una evidencia roja nunca verifica. La evidencia caducada se conserva.
- **FR-015**: Gate TDD: en `fix`, la tarea 1 exige un rojo observado (`--expect-fail` con salida ≠ 0) sobre una huella distinta de la de su verde; fuera de git, anterior al verde. En los demás tipos, solo las tareas con un intento rojo registrado.
- **FR-016**: El motor DEBE reescribir el bloque Evidencia y marcar `[x]` las tareas que pasaron. El texto escrito a mano no cuenta.
- **FR-017**: `analyze` DEBE ser una consulta que reporte hallazgos con severidad y referencia: historia sin criterio, criterio sin slice, tarea sin slice o slice con referencias inexistentes (`full`); aclaración abierta; aprobación ausente o caducada (`full`); faltantes; tareas sin evidencia vigente o fallidas; rojo pendiente.
- **FR-018**: `close --quote` DEBE exigir el paso `close`, ningún hallazgo `critical` y una cita válida; guarda revisión de trabajo, huella, origen (`conversation`/`terminal`), cliente y cita protegida con `ProtectText`.
- **FR-019**: `cancel --quote` DEBE exigir cita y dejar la unidad `cancelled`.
- **FR-020**: `promote` DEBE pasar una unidad `fast` a `full` añadiendo Historias y Slices al documento, sin perder evidencia.

**Continuidad, interfaces y esquema**

- **FR-021**: El contexto de arranque y `get_context` DEBEN incluir «HCD activo».
- **FR-022**: Las decisiones (carril, aclaraciones, aprobación, promoción, cierre) DEBEN guardarse como memorias `decision` enlazadas al documento. No hay tabla propia de decisiones.
- **FR-023**: El motor `mem hcd` DEBE ofrecer `start`, `show`, `next`, `clarify`, `approve`, `verify`, `analyze`, `close`, `cancel` y `promote`, con `--json` estable. Las consultas no escriben.
- **FR-024**: El sistema DEBE instalar 10 skills: `hcd`, `hcd-define`, `hcd-clarify`, `hcd-approve`, `hcd-slice`, `hcd-tasks`, `hcd-implement`, `hcd-verify`, `hcd-analyze` y `hcd-close`. `hcd-exception`, `hcd-playbook` y `hcd-migrate` se retiran.
- **FR-025**: En Claude Code, el hook pre-edición DEBE avisar, sin bloquear y una vez por sesión, cuando se edita código sin unidad activa. En Codex y OpenCode la regla viaja en la skill `hcd`.
- **FR-026**: HCD NO DEBE escribir fuera de `hcd/` ni en archivos de contexto del agente, y NO DEBE guardar secretos.
- **FR-027**: Spec Kit, sus skills y `adapters/secondary/speckit` NO DEBEN modificarse.
- **FR-028**: El esquema DEBE pasar a v6: desde v4 se eliminan las 16 tablas `hcd_*` v1 y se crean `hcd_units`, `hcd_evidence` y `hcd_approvals` en la misma transacción, sin tocar otras tablas; la v6 añade a `hcd_approvals` las huellas de la Constitución y del Playbook (FR-029).
- **FR-029**: La Constitución y el Playbook DEBEN aplicar de forma transversal: cada skill los lee antes de definir, aprobar, implementar y cerrar; cada aprobación guarda las huellas de sus versiones vigentes; si cambian después de aprobar la definición (carril `full`), `analyze` lo reporta como `HIGH` mientras se trabaja y `CRITICAL` en el paso `close`, y volver a aprobar la definición con las normas nuevas lo resuelve.

### Key Entities

- **Unidad** (`hcd_units`): id, número, título, slug, tipo, carril, motivo, estado (`active`, `closed`, `cancelled`), activa, ruta del documento.
- **Evidencia** (`hcd_evidence`): unidad, tareas, tipo, rojo esperado, comando u observación, código de salida, extracto, revisión de trabajo, huella, fecha.
- **Aprobación** (`hcd_approvals`): unidad, acción (`approve`, `close`, `cancel`), revisión, huella, origen, cliente, cita protegida, huellas de la Constitución y del Playbook, fecha.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Una feature `full` recorre los 8 pasos de la ceremonia con el binario compilado, y el motor da en cada uno la skill siguiente correcta.
- **SC-002**: Un `fix` `fast` recorre `define → implement → verify → close` con un artefacto y una sola aprobación.
- **SC-003**: El esquema tiene exactamente 3 tablas `hcd_*` y la instalación deja exactamente 10 skills HCD.
- **SC-004**: Editar la definición después de aprobarla devuelve el paso a `approve` en el 100 % de los casos probados.
- **SC-005**: Cambiar el código después de registrar evidencia la caduca en el 100 % de los casos probados.
- **SC-006**: El cierre de un `fix` sin rojo observado y el de una unidad con hallazgos `critical` se rechazan en el 100 % de los casos probados.
- **SC-007**: Ninguna consulta (`show`, `next`, `analyze`) cambia estado, documento ni evidencia.
- **SC-008**: Una BD v4 con el catálogo v1 migra a v5 conservando el 100 % de las memorias.
- **SC-009**: Spec Kit queda con cero bytes modificados y ningún archivo fuera de `hcd/` cambia por causa de HCD.
- **SC-010**: El código de producción HCD se mide antes y después y se publica en el CHANGELOG.
- **SC-011**: Cambiar la Constitución o el Playbook después de aprobar la definición bloquea el cierre en el 100 % de los casos probados hasta reaprobarla.

## Assumptions

- El schema v4 nunca se publicó (`v2.40.3` usa v3), pero el binario de la rama lo aplicó a 85 BD globales; la v5 lo corrige sin perder memorias.
- La §18 de la Constitución 2.2.0 ya admite «artefactos por enfoque»; solo cambia la lista del enfoque HCD: un documento único.
- Esta feature 045 se gestiona con Spec Kit.

### Fuera de alcance

- Operations e idempotencia por operación, executions con vencimiento, heartbeats, prompts de aprobación de un solo uso, revisiones normativas fijadas por unidad, adopción del Playbook, excepciones, bloqueos persistidos y migración desde Spec Kit.
- Retirar o modificar Spec Kit.
- Nuevas herramientas MCP o un binario independiente.
