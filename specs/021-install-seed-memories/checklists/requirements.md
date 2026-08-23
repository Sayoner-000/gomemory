# Specification Quality Checklist: Instalación sin artefactos — reglas y constitución como memorias semilla

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-08-23
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

- Iteración 1: la primera redacción arrastraba nombres de archivo, funciones y rutas
  del árbol de tareas de entrada (`GetMemoryByTopicKey`, `cmd_install.go`,
  `.windsurf/mcp_config.json`). Se reescribieron FR-001..FR-029 y los escenarios en
  términos de comportamiento observable: "archivos de instrucciones para agentes",
  "carpetas de configuración de agentes", "clave de tópico estable". El detalle
  técnico correspondiente pertenece a `plan.md`, no a la especificación.
- Las 4 decisiones abiertas (qué hacer con los archivos de agente existentes, cómo
  llegan las reglas al agente sin archivo, qué hacer con las carpetas de agentes no
  solicitados, y cómo se activa la constitución) ya fueron resueltas por el usuario
  antes de redactar, por eso no queda ningún marcador [NEEDS CLARIFICATION].
- Riesgo declarado y aceptado por el usuario (ver Assumptions): retirar los archivos
  de instrucciones elimina contenido propio del repositorio. FR-017 y FR-018 acotan
  el daño con respaldo previo y con la negativa a borrar si el respaldo falla.
- Iteración 2 (tras revisión del usuario): se incorporaron dos bloques que la primera
  redacción trataba como hallazgos pasivos en `research.md` en vez de como trabajo a
  ejecutar. Ahora son requisitos con criterio de aceptación propio:
  - **FR-030/FR-031** — el defecto latente de la clave de tópico y la dependencia de la
    ventana de recencia. Se corrigen en esta feature porque esta feature sería la
    primera en activarlos.
  - **FR-032/FR-033/FR-034** — siembra inerte. La auditoría de efectos colaterales había
    concluido "los cuatro son inocuos"; era falso para la publicación a documentación
    externa, que con la sincronización activada publicaría la constitución entera sin
    que nadie lo pidiera.
  Se añadieron SC-009..SC-011, cuatro casos borde y los escenarios 11-13 del quickstart.
  Total: 34 requisitos funcionales, 11 criterios de éxito, 12 casos borde.
- La regla de trabajo que originó esta iteración quedó escrita en
  `infrastructure/templates/agent-preamble.md` §7 y como preferencia en gomemory: un
  hallazgo se entrega con su propuesta de cierre, no solo declarado.
- Iteración 3: se añadió la **Historia 5 — documentos fijados** (FR-035..FR-046,
  SC-012..SC-015, 4 casos borde, escenarios 14-16). Cierra un sesgo que la spec no
  había nombrado: sembrar reglas y constitución sin vía de reemplazo convertía a la
  herramienta en autora de las normas del equipo. Las plantillas pasan a ser un
  *default* explícito, con export/import/restauración por consola y TUI sobre un
  catálogo table-driven.
  Total: 46 requisitos funcionales, 15 criterios de éxito, 16 casos borde,
  5 historias de usuario, 16 escenarios de validación.
