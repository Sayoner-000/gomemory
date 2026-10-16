# Specification Quality Checklist: Socratica — refinamiento de prompts y escrutinio socrático

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-10-15
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

- Iteración 1: un marcador [NEEDS CLARIFICATION] (generación fuera de un agente) resuelto con la persona → "Ambas con detección" (FR-032a, US4 escenarios 6–7).
- Excepciones aceptadas, exigidas por el propio borrador de la persona y no decisiones de diseño: nombres de comandos (`/socratica`, `/socratic-q`, `mem socratica`), formato abierto Agent Skills y la paleta hexadecimal del contrato visual (FR-035). No fijan lenguaje, almacenamiento ni esquema; tablas, migraciones y superficie MCP quedan para `/speckit-plan`.
- Pendiente para planificación: verificar contra los binarios instalados qué capacidades (comando con nombre, intercepción pre-envío, panel, portapapeles) ofrece cada agente, y fijar el límite de flujos pendientes.
- Revisión 2 (2026-10-15): spec ajustada tras /speckit-analyze (F1 on/off solo CLI/TUI, F2 un pendiente por agente, F3/U1 envío confirmado tras arranque y marca de origen, F5 capacidades evidenciadas, F6 `>` garantizado/por directiva, SC-005 conjunto fijo manual, G6 retomar pendiente, T1 equivalencia de modos). Sin marcadores pendientes; todos los ítems siguen en verde.
