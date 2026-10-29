# Evidencia T005 — §18 y §25.2 aplicadas

Fecha: 2026-10-28. Aprobación: `evidence/constitution-18-approval.md`.

## Vía

1. `mem docs import constitution <archivo>`: constitución fijada en la memoria del proyecto.
2. `mem constitution --sync`: copia a `.specify/memory/constitution.md` (`syncSpeckitConstitution`).
3. Comprobación: `mem docs export constitution` y `.specify/memory/constitution.md` son idénticos byte a byte (`cmp`), 755 líneas.

## Divergencia previa que se corrigió

Antes del cambio, las dos copias no coincidían: la fijada estaba en la versión 2.0.0, sin la §25; la de `.specify` estaba en la 2.1.0, con la §25.1 (aprobada el 2026-10-23, feature 041). Alguien había editado solo la copia de `.specify`. Sincronizar la fijada tal cual habría borrado la §25.1, así que la enmienda partió de la copia 2.1.0. Ahora la versión fijada es la 2.2.0 y las dos copias coinciden.

## Diff aplicado (`.specify/memory/constitution.md`, 2.1.0 → 2.2.0)

```diff
@@ -1,8 +1,8 @@
 # Constitución Técnica
 
-**Versión:** 2.1.0
+**Versión:** 2.2.0
 **Fecha de corte:** 2026-09-24
-**Última enmienda:** 2026-10-23 (§25, excepciones del proyecto gomemory)
+**Última enmienda:** 2026-10-28 (§18 por enfoque Spec Kit/HCD y §25.2, feature 045)
 
 > Esta constitución aplica a todo proyecto nuevo. Codifica lecciones aprendidas en proyectos anteriores y define valores predeterminados técnicos, arquitectónicos, operativos y de seguridad. Un proyecto puede extender estas reglas, pero no relajarlas sin una excepción explícita, fechada y justificada en su propio `constitution.md`.
 
@@ -570,22 +570,14 @@
 | `docs/OPERACION.md` | despliegue, observabilidad, SLO y recuperación |
 | `docs/adr/` | decisiones arquitectónicas fechadas |
 
-Por feature:
+Por unidad de trabajo, según su enfoque registrado:
 
-```text
-specs/{NNN}-{name}/
-├── spec.md
-├── plan.md
-├── tasks.md
-├── checklists/
-│   └── requirements.md
-├── contracts/
-├── data-model.md
-├── research.md
-└── quickstart.md
-```
+| Enfoque | Ubicación | Obligatorios | Opcionales |
+|---|---|---|---|
+| Spec Kit (legado) | `specs/{NNN}-{name}/` | `spec.md`, `plan.md`, `tasks.md`, `checklists/requirements.md` | `contracts/`, `data-model.md`, `research.md`, `quickstart.md` |
+| HCD | `hcd/features/{NNN}-{name}/` | `definition.md`, `delivery.md`, `verification.md` (generado por el motor) | Archivos que aporten una decisión o evidencia concreta, justificados en `delivery.md` |
 
-`spec.md`, `plan.md`, `tasks.md` y `checklists/requirements.md` son obligatorios. Los demás dependen del alcance.
+Una unidad tiene un solo enfoque. Una unidad Spec Kit no está obligada a tener artefactos HCD, ni una unidad HCD a tener artefactos Spec Kit. Las reglas de idioma de esta sección aplican a ambos enfoques. El protocolo, los controles y las excepciones de HCD se rigen por el Playbook HCD (documento fijado `playbook` de GoMemory, consultable con `mem playbook`), que remite a esta Constitución y no puede relajarla (FR-011).
 
 Reglas de idioma:
 
@@ -748,3 +740,16 @@
 - **ADR:** [docs/adr/0001-migracion-al-abrir-la-base.md](../../docs/adr/0001-migracion-al-abrir-la-base.md).
 - **Ticket:** sin ticket externo. El seguimiento es `specs/041-migracion-transaccional/` (deuda C1 del análisis del 2026-10-23).
 
+### 25.2 Composition root en `infrastructure/container.go` (relaja §4)
+
+- **Regla afectada:** §4 Composition root, que nombra `infrastructure/composition.go` para Go.
+- **Motivo:** el proyecto concentra el cableado en `infrastructure/container.go` (`NewContainer`) desde antes de la §4 vigente. La regla de fondo, un único composition root que construye el grafo completo y entrega dependencias por constructor, se cumple. Solo difiere el nombre del archivo.
+- **Alternativas descartadas:** renombrar el archivo dentro de la feature 045, porque es una reestructuración fuera de su alcance y toca la vía de arranque de todos los comandos; renombrarlo en un cambio aparte sigue disponible y cerraría esta excepción.
+- **Riesgo técnico:** ninguno nuevo; el riesgo que la §4 previene (cableado disperso) no existe, porque todo el cableado sigue en un único archivo.
+- **Riesgo de seguridad:** ninguno.
+- **Controles compensatorios:** el cableado de HCD se añade solo en `container.go` (tarea de fundamentos); la guarda de capas `tests/contract/layer_rules_test.go` impide importar `infrastructure` desde `domain` o `application`.
+- **Responsable:** josegomezJ3810.
+- **Fecha de aprobación:** 2026-10-28.
+- **Fecha de revisión:** 2027-01-23, con la renovación trimestral del catálogo (§0), igual que la §25.1.
+- **ADR:** [docs/adr/0002-hcd-estado-de-control-en-sqlite.md](../../docs/adr/0002-hcd-estado-de-control-en-sqlite.md) (sección «Composition root»).
+- **Ticket:** sin ticket externo. El seguimiento es `specs/045-hcd-protocol/`.
```
