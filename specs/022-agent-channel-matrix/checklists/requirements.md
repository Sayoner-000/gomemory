# Specification Quality Checklist: Matriz de canales como fuente única

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

Esta especificación **reemplaza** un borrador previo de la misma carpeta que mezclaba tres
problemas con ciclos de vida distintos: la matriz de canales, el costo del contexto y la
degradación silenciosa. Los otros dos pasaron a las especificaciones 023 y 024.

**Correcciones aplicadas durante la redacción:**

1. *Nombres de implementación retirados.* El borrador citaba funciones, archivos y agentes
   concretos. La versión final describe actividades del ciclo de vida y canales por su
   comportamiento. Las cifras medidas se conservan porque son evidencia.
2. *La tabla de defectos se reordenó por causa, no por descubrimiento.* Cada fila contrasta la
   actividad que conocía la celda con la que no, que es la forma común de los cuatro.
3. *Se añadió alcance excluido* para separar explícitamente lo que corresponde a 023 y 024.

**Observaciones para el planificador:**

- La Historia 4 es un **defecto ya reproducido**. La regla 1 del proyecto dice que un bug se
  repara contra el sistema en ejecución y no espera al ciclo de especificación. Está aquí para
  fijar la regresión y para que la matriz lo absorba; su corrección puede adelantarse.
- FR-006 tiene una alternativa deliberada: o añadir un agente no requiere edición adicional, o
  la verificación la exige. El plan debe elegir. Exigirla es más simple y más honesto que
  intentar generar los mecanismos por agente automáticamente.
- FR-016 (la batería no modifica el entorno de quien la ejecuta) tiene un costo de arranque: hoy
  hay pruebas que ejercen actividades de ciclo de vida sin aislar el entorno de la persona.
  Cerrarlo obliga a revisarlas todas, no solo a añadir la verificación.
- El caso límite del artefacto compartido por dos agentes no tiene respuesta en la spec. Es
  deliberado: hoy no ocurre, y resolverlo por adelantado sería diseñar sin caso. El plan debe
  dejarlo declarado como límite conocido.
