# Propuesta: corrección de la §18 de la Constitución (FR-010)

**Estado**: propuesta, pendiente de la aprobación del Implementador. No se aplica hasta aprobarla.
**Vía de aplicación**: `mem constitution` (memoria fijada) y sincronización a `.specify/memory/constitution.md`. Nunca se edita solo la copia de `.specify`.

## Texto vigente (§18, «Por feature»)

> `spec.md`, `plan.md`, `tasks.md` y `checklists/requirements.md` son obligatorios. Los demás dependen del alcance.

## Texto propuesto

> **Por unidad de trabajo, según su enfoque registrado:**
>
> | Enfoque | Ubicación | Obligatorios | Opcionales |
> |---|---|---|---|
> | Spec Kit (legado) | `specs/{NNN}-{name}/` | `spec.md`, `plan.md`, `tasks.md`, `checklists/requirements.md` | `contracts/`, `data-model.md`, `research.md`, `quickstart.md` |
> | HCD | `hcd/features/{NNN}-{name}/` | `definition.md`, `delivery.md`, `verification.md` (generado por el motor) | Archivos que aporten una decisión o evidencia concreta, justificados en `delivery.md` |
>
> Una unidad tiene un solo enfoque. Una unidad Spec Kit no está obligada a tener artefactos HCD, ni una unidad HCD a tener artefactos Spec Kit. Las reglas de idioma de esta sección aplican a ambos enfoques. El protocolo, los controles y las excepciones de HCD se rigen por el Playbook HCD (documento fijado `playbook` de GoMemory, consultable con `mem playbook`), que remite a esta Constitución y no puede relajarla (FR-011).

## Impacto

- Las features `specs/001…044` no cambian (C-02); `specs/045-hcd-protocol/` solo recibe su propia evidencia.
- La correspondencia de §3, §5, §6, §10, §16, §20, §21 y §23 con reglas `HCD-*` vive en el Playbook (FR-009).
- La plantilla `.specify/templates/constitution-template.md` no se toca.

---

## Excepción §25.2 propuesta (misma ronda de aprobación): composition root en `infrastructure/container.go`

> ### 25.2 Composition root en `infrastructure/container.go` (relaja §4)
>
> - **Regla afectada:** §4 Composition root, que nombra `infrastructure/composition.go` para Go.
> - **Motivo:** el proyecto concentra el cableado en `infrastructure/container.go` (`NewContainer`) desde antes de la §4 vigente. La regla de fondo, un único composition root que construye el grafo completo y entrega dependencias por constructor, se cumple. Solo difiere el nombre del archivo.
> - **Alternativas descartadas:** renombrar el archivo dentro de la feature 045, porque es una reestructuración fuera de su alcance y toca la vía de arranque de todos los comandos; renombrarlo en un cambio aparte sigue disponible y cerraría esta excepción.
> - **Riesgo técnico:** ninguno nuevo; el riesgo que la §4 previene (cableado disperso) no existe, porque todo el cableado sigue en un único archivo.
> - **Riesgo de seguridad:** ninguno.
> - **Controles compensatorios:** el cableado de HCD se añade solo en `container.go` (tarea de fundamentos); la guarda de capas `tests/contract/layer_rules_test.go` impide importar `infrastructure` desde `domain` o `application`.
> - **Responsable:** josegomezJ3810.
> - **Fecha de aprobación:** la de la aprobación de esta propuesta (T004).
> - **Fecha de revisión:** 2027-01-23, con la renovación trimestral del catálogo (§0), igual que la §25.1.
> - **ADR:** `docs/adr/0002-hcd-estado-de-control-en-sqlite.md` (sección «Composition root»).
> - **Ticket:** sin ticket externo. El seguimiento es `specs/045-hcd-protocol/`.
