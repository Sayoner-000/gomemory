# Specification Quality Checklist: Diagnóstico accionable y vitalidad de los canales

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

**Dependencia declarada.** Esta especificación depende de la 022: FR-008 exige que cada canal
traiga consigo su efecto y su corrección, y eso solo es posible si existe la declaración única
de canales que la 022 establece. Planificar la 024 antes de cerrar la 022 obligaría a mantener
una lista propia de efectos y correcciones, que es exactamente el patrón que la 022 elimina.

**La cita literal del informe se conservó** en el contexto del problema aunque contiene
elementos de la salida real. Es evidencia de qué se lee hoy, y sustituirla por una paráfrasis
haría imposible juzgar si el problema está bien descrito. Las rutas concretas se sustituyeron
por marcadores.

**Decisiones que el planificador debe tomar:**

- SC-002 se verifica pidiendo a alguien ajeno al proyecto que clasifique cada línea del informe.
  Es un criterio medible pero no automatizable: el plan debe decidir si se ejecuta una vez como
  validación de diseño o si se sustituye por una regla estructural comprobable.
- FR-011 (distinguir un canal sin actividad por falta de sesiones de uno que no responde
  habiéndolas) requiere correlacionar actividad de canal con actividad de sesión. Es la parte
  con más plomería de la especificación y el plan debe dimensionarla explícitamente.
- El caso límite de la corrección que sobrescribiría algo personalizado por el equipo entra en
  conflicto directo con la garantía de que un documento reemplazado no se sobrescribe. El plan
  debe resolver qué corrección se propone en ese caso, o declarar que no se propone ninguna.

**Riesgo de alcance:** FR-012 (que las rutas de error dejen rastro) toca el complemento de un
agente externo, cuyo ciclo de publicación no controla el proyecto. Un complemento ya instalado
en una máquina no adquiere ese rastro hasta que se actualice. FR-015 lo mitiga parcialmente al
exigir que la detección de inactividad funcione sobre versiones anteriores, pero la causa
registrada del fallo solo estará disponible tras actualizar.
