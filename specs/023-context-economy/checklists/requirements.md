# Specification Quality Checklist: Economía del contexto

**Purpose**: Validar que la especificación está completa antes de planificar
**Created**: 2026-08-23
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Sin detalles de implementación (lenguajes, frameworks, APIs, nombres de archivo)
- [x] Centrada en el valor para quien usa el sistema
- [x] Redactada para una persona no técnica
- [x] Todas las secciones obligatorias completas

## Requirement Completeness

- [x] No quedan marcadores [NEEDS CLARIFICATION]
- [x] Los requisitos son verificables y sin ambigüedad
- [x] Los criterios de éxito son medibles
- [x] Los criterios de éxito son agnósticos a la tecnología
- [x] Todos los escenarios de aceptación están definidos
- [x] Los casos límite están identificados
- [x] El alcance está acotado
- [x] Dependencias y supuestos identificados

## Feature Readiness

- [x] Cada requisito funcional tiene criterio de aceptación claro
- [x] Las historias cubren los flujos principales
- [x] La feature cumple los resultados medibles de Success Criteria
- [x] No se filtran detalles de implementación

## Notas de la validación

**Orden de prioridad deliberado.** La medición (Historia 3) quedó en P2 aunque intuitivamente
parezca el primer paso. La razón: el ahorro ya está identificado y cuantificado con mediciones
manuales, así que medir de nuevo no es requisito para actuar. La medición sirve para sostener el
ahorro en el tiempo y demostrarlo, no para descubrirlo.

**La Historia 1 se cierra editando un documento, no escribiendo código**, y es el mayor ahorro
medido de las tres. Por relación costo/beneficio debería ejecutarse primero dentro de esta
especificación.

**Tensiones que el planificador debe resolver:**

- FR-012 (supresión acotada a la sesión) contra el caso límite de compactación: compactar puede
  dejar al agente sin material que el registro cree entregado. FR-010 es la salida, pero el plan
  debe decidir si la recuperación se activa a mano o al detectar la compactación. Decidirlo por
  omisión produciría el peor resultado: un agente sin contexto que cree tenerlo.
- FR-005 (la condición de la persona prevalece sobre el texto por defecto) requiere una noción
  de precedencia que hoy no existe. El plan debe decidir si es un orden de presentación o una
  supresión efectiva del texto en conflicto.
- SC-001 fija una reducción del 70 % sobre trabajo delegado, pero esa cifra depende de cómo
  conduzca la sesión quien orquesta, no solo del documento de reglas. Es medible, y a la vez
  parcialmente fuera del control del sistema. Debe evaluarse sobre una tarea de referencia
  definida, no sobre cualquier sesión.

**Límite aceptado, no descuido:** la detección de duplicados por coincidencia literal no cubre
dos canales que entregan lo mismo con formato distinto. Está declarado en Assumptions.
