# Data Model: Informe de evaluación

**Feature**: 043-vertical-slices-assessment

Son entidades lógicas del informe, no persistidas. Cada una corresponde a una entidad clave de la spec.

## LíneaBase

| Campo | Descripción | Regla |
|---|---|---|
| `head` | `git rev-parse HEAD` al empezar | Obligatorio |
| `status` | Salida de `git status --porcelain` al empezar | Obligatorio; puede no estar vacía |
| `capturada_en` | Fecha y hora | Obligatorio |

Transición: `capturada` → `comparada`. Al terminar, `head` y `status` deben ser idénticos (FR-010).

## PaqueteCapa (C1, FR-001)

| Campo | Descripción | Regla |
|---|---|---|
| `ruta` | Ruta de importación sin el prefijo `mem/` | Única |
| `capa` | `domain` · `ports` · `usecases` · `adapter-primary` · `adapter-secondary` · `infrastructure` · `otro` | Se deduce de la ruta; `otro` exige una nota (p. ej., `version/`, `assets/`, `tests/…`) |
| `importa` | Lista de `ruta` del módulo que importa en producción | Procede de `go list` (research R1) |
| `importa_en_pruebas` | Importaciones adicionales de `_test` | Se informa aparte; no cuenta como acoplamiento de producción |

## FlujoRecorrido (C1, FR-002)

| Campo | Descripción | Regla |
|---|---|---|
| `slice` | Socratica · Review · Instincts (research R2) | Mínimo 3 |
| `entrada` | CLI · MCP · TUI · hook | Entre todos los flujos deben aparecer los cuatro tipos |
| `pasos[]` | Secuencia `{capa, símbolo, archivo:línea}` desde la entrada hasta persistencia o adaptador externo | Cada paso comprobable; sin saltos de capa sin explicar |

## Hallazgo (C2, FR-003)

| Campo | Descripción | Regla |
|---|---|---|
| `id` | `H-NN` | Único |
| `categoría` | acoplamiento entre capas · dependencia invertida o incorrecta · lógica repartida · dificultad de prueba · límite de módulo · punto de entrada | Lista cerrada |
| `criterio` | Identificador del criterio incumplido (L1…T3) | Obligatorio (research R3, condición 1) |
| `evidencia` | `archivo:línea` o comando reproducible con su salida relevante | Obligatorio |
| `consecuencia` | Efecto concreto para desarrollar por slices | Obligatorio (research R3, condición 3) |
| `verificado` | sí · no (motivo) | Si es `no`, no puede sostener la conclusión |
| `riesgo` / `beneficio` | alto · medio · bajo | Se usan para ordenar el roadmap |
| `cierre` | Propuesta de cierre, sin implementarla | Obligatorio (regla de trabajo "cierra los hallazgos") |

## Preferencia (C2, FR-004)

| Campo | Descripción |
|---|---|
| `id` | `P-NN` |
| `observación` | Qué se ve |
| `por_qué_no_es_defecto` | Qué condición de research R3 falta |

## PrácticaConservar (C3, FR-005)

| Campo | Descripción | Regla |
|---|---|---|
| `id` | `B-NN` | Único |
| `evidencia` | `archivo:línea` | Obligatorio |
| `por_qué_favorece` | Motivo ligado a slices o a la separación hexagonal | Obligatorio |

## CriterioEvaluación (C4, FR-006)

Están definidos en [contracts/evaluation-criteria.md](contracts/evaluation-criteria.md). Cada uno tiene `id`, `pregunta`, `cómo se comprueba` y `umbral`. El informe incluye una matriz **criterio × slice** con el resultado de cada celda: cumple · no cumple (H-NN) · no aplica.

## Conclusión (C4, FR-007)

| Campo | Regla |
|---|---|
| `opción` | Exactamente una: `refactorizar` · `mantener` · `cambios-puntuales` |
| `sostenida_por` | Al menos un H-NN o B-NN, solo de los verificados |
| `razonamiento` | Por qué no se eligen las otras dos opciones |

## PasoRoadmap (C5, FR-008/FR-009)

Existe solo si `opción` ≠ `mantener`.

| Campo | Regla |
|---|---|
| `orden` | Ordenado por riesgo ascendente y beneficio descendente |
| `slice_candidato` | Nombre del slice o del límite afectado |
| `resuelve` | Lista de H-NN |
| `criterio_éxito` | Comando ejecutable o comprobación medible (p. ej., "`go list` no muestra importaciones de `adapters/` desde `application/`") |
| `riesgo` / `beneficio` | Heredados de los hallazgos que resuelve |
