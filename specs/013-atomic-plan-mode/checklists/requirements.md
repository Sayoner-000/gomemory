# Specification Quality Checklist: Modo Plan Atómico con Memoria

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-08-06
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

**Estado: todos los ítems pasan.** La especificación está lista para `/speckit-plan`.

### Iteración 1 — redacción y primera validación

Correcciones aplicadas:

- Se retiraron nombres de archivo y rutas concretas del cuerpo de requisitos
  (`.memory/settings.json`, `.claude/skills/`, `.opencode/commands/`, `mem install`).
  Los requisitos hablan de "configuración del proyecto", "el paso de instalación de
  gomemory" e "interfaz de configuración". Las referencias concretas quedan solo en
  Contexto (para situar el origen del problema) y en Assumptions/Dependencies, donde
  documentan la reutilización decidida, no el diseño.
- Se retiró de los criterios de éxito toda métrica de sistema interna. SC-004, SC-005 y
  SC-010 miden tiempo y esfuerzo de la persona; SC-001 a SC-003 miden propiedades
  observables del plan entregado.
- Se añadió un requisito para la relación con el flujo SDD existente (hoy FR-019), que
  estaba enunciada solo como edge case sin requisito que la resolviera.

Cerró con 3 marcadores [NEEDS CLARIFICATION] (dentro del límite de 3), todos decisiones
de alcance sin valor por defecto razonable.

### Iteración 2 — resolución de clarificaciones

Las tres preguntas fueron respondidas por el usuario y quedaron incorporadas:

| # | Decisión del usuario | Impacto en la spec |
|---|----------------------|--------------------|
| Q1 | Atomicidad por **autovalidación del agente**, sin compuerta externa | FR-015 y FR-016; escenario 6 de la Historia 2; edge case de tarea no atomizable; SC-002 admite tarea marcada como no atómica con motivo; "validador externo" pasa a Out of Scope |
| Q2 | Alcance **solo planificación** (fases 1-2 del ADS) | FR-017 y FR-018; Historia 5 renombrada a "conservar el plan como contrato" para no prometer gobierno de la ejecución; fases 3 y 4 pasan explícitamente a Out of Scope |
| Q3 | **Ambos ámbitos**: global por defecto con override por proyecto | FR-021 a FR-023 y FR-029; Historia 3 reescrita alrededor del ámbito global; escenario 3 nuevo en Historia 4; SC-004 y SC-011; entidad "Ámbito de instalación" |

### Iteración 3 — dos precisiones del usuario a mitad de redacción

**a) Activación autónoma, para cualquier agente.** El usuario precisó que es el propio
agente quien invoca gomemory al entrar en modo plan, y que aplica a cualquier tipo de
agente. Esto cambió el modelo de activación de la spec: de "el entorno detecta modo plan e
inyecta contexto por el agente" a "una instrucción del protocolo del proyecto ordena al
agente cargar el contexto él mismo". El cambio amplía la cobertura de dos agentes a
cualquiera que lea el protocolo y alcance la memoria.

Impacto: FR-001 a FR-004 (nuevos, modelo de activación); FR-027 y FR-028 (distribución por
protocolo común, con formato propio de agente como opción); Historia 1 y Historia 3
reescritas; escenario 2 nuevo en la Historia 3 para agentes sin integración dedicada;
cuatro edge cases nuevos; SC-005 y SC-006; entidad "Instrucción de activación";
Assumptions distingue ahora "agentes de referencia para verificación" de "límite de
soporte"; Out of Scope excluye integraciones dedicadas por agente.

Contrapartida registrada en Assumptions: la fiabilidad de la activación pasa a depender
del cumplimiento del agente, no de una garantía del entorno. Es el mismo criterio del
protocolo de memoria que el proyecto ya opera hoy.

**b) Línea base del ADS ya optimizada.** El usuario aportó su versión optimizada del
método, conservada en `reference-ads-baseline.md` junto con el análisis de brecha frente a
los requisitos. Sustituye al documento original como punto de partida de `/speckit-plan`.
Se verificó que su rama "Modo ejecución" no contradice FR-020/FR-021: en modo plan, el
propio texto ordena entregar el árbol y detenerse.

### Verificación de integridad tras las ediciones

- Numeración contigua y sin huecos: FR-001…FR-036, SC-001…SC-013.
- Cero ocurrencias de `NEEDS CLARIFICATION` en `spec.md`.
- Referencias cruzadas comprobadas una a una tras las dos renumeraciones: la Historia 5
  apunta a FR-020/FR-021 (alcance del método) y la tabla de brecha de
  `reference-ads-baseline.md` apunta a FR-001, FR-002, FR-003, FR-016, FR-018 y FR-019,
  que son en efecto los requisitos que ese texto todavía no cubre.

### Puntos que la fase de planificación debe resolver temprano

1. **Configuración de usuario por agente.** El ámbito global (FR-024, FR-025) asume que
   los agentes admiten configuración de usuario además de la de proyecto. Es razonable y
   queda declarado en Assumptions, pero no está verificado. Si resultara falso para algún
   agente, FR-024 a FR-026 habría que replantearlos para ese caso. Condiciona la Historia
   3 completa.
2. **Detección de intención de planificación** (FR-004) para agentes sin modo plan nativo:
   es el requisito con más margen de interpretación de la spec y conviene acotarlo en el
   plan técnico antes de implementarlo.
3. **Presupuesto de contexto bajo activación autónoma** (FR-007): el presupuesto ya existe
   en gomemory, pero hasta ahora lo aplicaba una ruta controlada por el entorno. Conviene
   confirmar que sigue siendo efectivo cuando quien invoca es el agente.
