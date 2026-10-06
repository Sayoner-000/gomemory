# Data Model: 036-compression-promise-gaps

Esta feature no crea ni migra datos. Las entidades son conceptuales (vocabulario de la documentación y el diagnóstico), sin almacenamiento nuevo.

## Entidades

- **Vía de ahorro**: uno de los tres mecanismos (contexto emitido, paquete de contexto bajo presupuesto, hook de salidas).
  - Atributos: nombre, métrica (qué resta reporta), comando de reproducción, orden de magnitud medido con fecha.
  - Regla: las tres vías nunca se mezclan en una sola cifra (FR-003).
- **Estado de hook por harness**: valor legible del diagnóstico por asistente (`activo`, `solo en origen…`, `activo (best-effort…)`).
  - Regla: cada estado corresponde a un comportamiento verificado en vivo, no a documentación del proveedor (edge case del spec).

## Sin transiciones ni validaciones de datos

No hay cambios de esquema, repos ni formatos JSON (solo cambia un valor string legible existente).
