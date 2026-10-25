# Contrato: estructura del informe

**Feature**: 043-vertical-slices-assessment | **Entidades**: [data-model.md](../data-model.md)

El informe es el único entregable. Se entrega en la conversación o en un archivo fuera del repositorio. Debe contener estas secciones, en este orden:

1. **Línea base** — `head`, `status` y fecha de captura (LíneaBase).
2. **Criterios** — referencia a [evaluation-criteria.md](evaluation-criteria.md) y, si alguno cambió, el cambio y su motivo.
3. **Mapa estructural** (C1) — tabla de PaqueteCapa con sus importaciones de producción entre capas y el comando que la reproduce.
4. **Flujos recorridos** (C1) — un FlujoRecorrido por slice, con los pasos en `archivo:línea`.
5. **Matriz criterio × slice** (C4) — resultado de cada celda.
6. **Hallazgos** (C2) — tabla de Hallazgo, con las categorías de la lista cerrada.
7. **Preferencias de diseño** (C2) — tabla de Preferencia, separada de los hallazgos.
8. **Prácticas a conservar** (C3) — tabla de PrácticaConservar.
9. **Conclusión** (C4) — una sola opción, `sostenida_por` y razonamiento.
10. **Roadmap** (C5) — tabla de PasoRoadmap, o la frase "Sin roadmap: …" con su motivo si la conclusión es `mantener`.
11. **Comprobación de solo lectura** (C6) — comparación de la línea base con el estado final y su resultado.

## Reglas de validez

- Toda cita `archivo:línea` usa rutas relativas a la raíz del repositorio y apunta al código del `head` de la línea base.
- Un hallazgo `verificado: no` aparece en el informe, pero no se cita en `sostenida_por`.
- No se incluye código nuevo, parches ni diffs de refactorización.
