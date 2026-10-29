# Contrato: enmienda de la §18 de la Constitución (v2)

**Cubre**: FR-024 (artefacto HCD único). **v1**: `git show a0503b0:specs/045-hcd-protocol/contracts/constitution-18-amendment.md`.

**Aprobada por la persona el 2026-10-29.** Versión PATCH: Constitución del proyecto 2.2.0 → 2.2.1; plantilla distribuida 2.1.0 → 2.1.1.

## Texto vigente

| Enfoque | Ubicación | Obligatorios | Opcionales |
|---|---|---|---|
| Spec Kit (legado) | `specs/{NNN}-{name}/` | `spec.md`, `plan.md`, `tasks.md`, `checklists/requirements.md` | `contracts/`, `data-model.md`, `research.md`, `quickstart.md` |
| HCD | `hcd/` | `HCD-NNN-<slug>.md`: un documento por unidad con intención, alcance, historias (carril completo), aclaraciones, slices, tareas y evidencia (la escribe el motor) | Archivos que aporten una decisión o evidencia concreta, citados en el documento de la unidad |

Una unidad tiene un solo enfoque. […] El protocolo y los controles de HCD se rigen por el Playbook HCD (documento fijado `playbook` de GoMemory, consultable con `mem playbook`), que remite a esta Constitución y no puede relajarla.

## Dónde se aplicó

| Copia | Estado |
|---|---|
| `.specify/memory/constitution.md` (proyecto, sin rastrear en git) | 2.2.1 |
| Constitución fijada en gomemory (`mem docs show constitution`) | 2.2.1, importada tras comprobar que no tenía ediciones propias |
| `infrastructure/templates/speckit-constitution-gen.md` (plantilla distribuida) | 2.1.1; la huella 2.1.0 pasa a `PreviousDefaultSHA256` para que las copias intactas se actualicen solas |

Cambios respecto de 2.2.0: la fila HCD pasa de `hcd/features/{NNN}-{name}/` con `definition.md`, `delivery.md` y `verification.md` a un documento único, y se retira «las excepciones» porque `hcd-exception` ya no existe.
