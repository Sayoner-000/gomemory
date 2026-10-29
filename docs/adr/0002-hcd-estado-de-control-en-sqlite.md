# ADR 0002: el estado de control HCD vive en SQLite

- **Estado**: propuesto
- **Fecha**: 2026-10-27
- **Responsable**: josegomezJ3810
- **Excepción de la constitución**: §25.2, composition root en `infrastructure/container.go` (pendiente de aprobación)
- **Referencias**: feature 045, `specs/045-hcd-protocol/`

## Contexto

La Constitución exige artefactos Spec Kit por feature (§18), mientras que HCD define sus propias unidades y artefactos. Spec Kit conserva los artefactos Markdown como fuente de contenido; HCD necesita además estado de control transaccional para revisiones, aprobaciones, tareas, ejecuciones, evidencias, memoria y unidad activa. El proyecto ya dispone de una base SQLite por proyecto y un único punto de composición en `infrastructure/container.go`.

## Decisión

HCD se implementará como motor interno `mem hcd`. El estado de control se guardará en tablas aditivas `hcd_*` de la SQLite del proyecto; los artefactos redactables permanecerán en Markdown bajo `hcd/`; el Playbook será un documento fijado del proyecto. La unidad activa se mantendrá en el control interno de HCD y no en archivos de contexto del agente. El cableado se realizará en `infrastructure/container.go`, sujeto a la excepción §25.2 propuesta.

## Alternativas descartadas

- **R1 — solo archivos Markdown**: no ofrece transacciones fiables para mutaciones concurrentes, idempotencia ni revisiones esperadas.
- **R2 — base de datos independiente para HCD**: duplica el ciclo de vida y las migraciones de la base por proyecto.
- **R3 — estado de control dentro de los artefactos**: mezcla contenido con estado mutable y dificulta detectar ediciones externas.
- **R15 — marca de feature en archivos de contexto del agente**: introduce estado duplicado y dependiente del runtime; HCD usa `hcd_focus`.
- **R18 — Playbook como archivo de configuración independiente**: el documento fijado permite compartir importación, exportación, restauración y huella de contenido con la Constitución.

## Composition root

El composition root vigente es `infrastructure/container.go` (`NewContainer`). Mantener allí el cableado HCD conserva el grafo de dependencias en un único lugar y evita una reorganización ajena al alcance de 045. Esta decisión respalda la excepción §25.2 propuesta, que requiere aprobación y registro en la Constitución antes de aplicarse.

## Consecuencias

- Las tablas HCD serán aditivas, versionadas y migradas al abrir la base según la excepción §25.1.
- Las mutaciones usarán `HCDUnitOfWork`; los artefactos se escribirán atómicamente bajo `hcd/`.
- La Constitución y el Playbook conservarán precedencia normativa sobre las reglas de unidades.
- La excepción §25.2 no se considera vigente hasta su aprobación formal.

## Responsable y fecha

Responsable: josegomezJ3810. Fecha de propuesta: 2026-10-27. La fecha de aceptación se registrará al aprobar este ADR.
