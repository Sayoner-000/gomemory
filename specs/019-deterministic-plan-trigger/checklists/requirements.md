# Specification Quality Checklist: Activación determinista del modo plan atómico

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-08-17
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Notes

- **Iteración 1 — correcciones aplicadas antes de cerrar la validación**:
  - Se retiraron del cuerpo de la especificación los nombres de eventos de hook, rutas de archivos,
    nombres de funciones y versiones concretas del marcador de protocolo que venían en la revisión
    de origen. Quedan descritos como "señal observable de entrada al modo plan", "canal de
    activación", "archivo de instrucciones de usuario" y "versión vigente del protocolo". El
    material técnico específico pertenece a `/speckit-plan`, no aquí.
  - La comprobación técnica pendiente (si el canal de entrada del agente admite devolver contenido)
    se registró como supuesto con camino de degradación declarado (FR-003) en lugar de un marcador
    de aclaración: no cambia el alcance de la feature, solo qué canal la implementa, y se resuelve
    con una verificación en vivo durante la planificación.
- **Iteración 2 — revisión de estrategia (no dañar el brazo extensor + determinismo real)**:
  - Se descartó el encuadre de "subir el volumen del disparador de plan" del FR-011 original. Era una
    guerra de imperativos cuya única forma de ganar era debilitar al brazo extensor. Sustituido por
    composición de roles (FR-010): el grafo es el instrumento de exploración, el árbol atómico es la
    forma de la salida; se enuncian secuenciados en una sola instrucción.
  - Se añadió la sección **Invariantes de convivencia** (INV-1..INV-5) como restricción previa a los
    requisitos: gomemory solo administra lo propio, no toca canales ni mensajes del extensor, no
    añade restricciones sobre las herramientas de exploración, y con el extensor ausente se comporta
    igual y en silencio.
  - Se trasladó el determinismo del borde de **entrada** al borde de **salida** (nueva Historia 1,
    P1): el borde de entrada dependía de una capacidad no verificada; la presentación del plan tiene
    señal observable y mecanismo documentado. La inyección en la entrada baja a "mejor esfuerzo" y
    pasa a la Historia 2, donde aporta calidad (historial) en vez de determinismo.
  - La exigencia de forma se acotó con salvaguardas explícitas (FR-002..FR-004): una devolución por
    episodio, nunca sobre solicitudes triviales, sesgo a permitir ante la duda, y apagable.
  - La regresión (Historia 5) ahora cubre **ambos brazos**: falla también si la activación del brazo
    extensor deja de producirse, y si una reinstalación duplica entradas.
  - Base factual verificada antes de reescribir: la fusión de configuración preserva entradas ajenas
    y los canales de ambos brazos coexisten sin pisarse; el riesgo real es la duplicación de entradas
    propias al reinstalar, no la colisión.
- **Iteración 3 — corrección de agnosticismo (el usuario señaló la desviación)**:
  - Diagnóstico: la especificación estaba agnóstica ("señal observable"), pero el plan y las tareas
    anclaron el mecanismo en un dialecto concreto (`permissionDecision` de Claude Code) y trataron al
    resto de agentes como degradación. Eso reproducía en el mecanismo determinista la misma asimetría
    que la feature venía a corregir, con el agravante de ser estructural y no de redacción.
  - Añadido **INV-6**: ningún agente es el de referencia; los formatos por agente son traducciones de
    un contrato neutral, y conectar un agente nuevo no debe exigir cambios en gomemory.
  - Añadidos **FR-A1..FR-A5**: contrato neutral, selección de dialecto con neutral por defecto,
    integración sin cambios de código, registro único de capacidades y piso textual garantizado para
    todo agente.
  - US3 reescrita: pasa de «misma experiencia en dos agentes» a «cualquier agente, presente o futuro»,
    con escenario de aceptación para un integrador externo.
  - Añadidos **SC-A1..SC-A3**, incluido el criterio verificable de 0 líneas de gomemory modificadas
    para conectar un agente desconocido.
  - Publicado `contracts/agent-integration.md` como **el** contrato (tres niveles, cuatro dialectos,
    ejemplo mínimo ejecutable); los contratos de hook quedan rotulados como traducciones a Claude Code.
- **0 marcadores [NEEDS CLARIFICATION]**: las decisiones abiertas de la revisión de origen se
  resolvieron con supuestos documentados (prioridad método > historial ante el límite de tamaño,
  refuerzo por turno de una sola línea, alcance de agentes sin ámbito de usuario, inclusión de la
  corrección del borrado de contenido).
- Items marked incomplete require spec updates before `/speckit-clarify` or `/speckit-plan`
