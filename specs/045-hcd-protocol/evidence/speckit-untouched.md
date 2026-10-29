# Evidencia T124 — Spec Kit no cambió (C-24, SC-004)

Fecha: 2026-10-28. Línea base: `eb26453a38c09ae85c078b1897a4a7a508b293bf` (`evidence/baseline.md`).

## 1. `specs/` y adaptador Spec Kit

```text
git diff --stat eb26453 -- specs/ ':(exclude)specs/045-hcd-protocol' adapters/secondary/speckit
(sin salida)
git status --short -- specs/ adapters/secondary/speckit   (excluyendo specs/045-…)
(sin salida)
```

Cero diferencias en `specs/001…044` y en `adapters/secondary/speckit`, tanto contra el SHA como en el árbol de trabajo.

## 2. Huellas de skills instaladas

`baseline.md` registra 11 huellas, todas de `.opencode/commands/speckit.*.md`. Se recalcularon con SHA-256: **11 de 11 coinciden**, 0 discrepancias.

**`.claude/skills/speckit-*` y `.agents/skills/speckit-*` (2026-10-28).** `baseline.md` no registró sus huellas, y no se pueden reconstruir después sin inventarlas. Regenerarlas con `specify init` tampoco sirve: la CLI instalada es la 1.2.4 y las skills se generaron con otra versión. La evidencia usada es la fecha de modificación: los 21 archivos se modificaron por última vez entre el 2026-06-22 y el 2026-10-16, antes del commit base `eb26453` (2026-10-27T05:42). Ninguno cambió durante la 045. Quedan registradas sus huellas actuales como línea base para futuras comprobaciones:

| Archivo | Última modificación | SHA-256 |
|---|---|---|
| `.agents/skills/speckit-analyze/SKILL.md` | 2026-10-16T03:50:36 | `a84a54c4701706a711bf23c59817bc659bd6e484488fd7402a908496992af217` |
| `.agents/skills/speckit-checklist/SKILL.md` | 2026-10-16T03:50:36 | `2a10245ed6772e5e5c52bd723aab1e3eb975617fa9121610e72f4329b1dd1635` |
| `.agents/skills/speckit-clarify/SKILL.md` | 2026-10-16T03:50:36 | `d3aa120d6b5d7718c4c6d792dd41bee66254bd4bb82f88f3067f1d1c4e9d2dc2` |
| `.agents/skills/speckit-constitution/SKILL.md` | 2026-10-16T03:50:36 | `505d20a209c3f2027ffd2464786b8ef6ee7e3c4ab485397ad3fa794f1ce7d3c3` |
| `.agents/skills/speckit-converge/SKILL.md` | 2026-10-16T03:50:36 | `466e51121a590e3abd409fda063db6361c9a7e2b2cef927a75f5afb130c1eb88` |
| `.agents/skills/speckit-implement/SKILL.md` | 2026-10-16T03:50:36 | `1fc2cfc294a05b6a18d2dbbf7d37a63f2992f7f65e2bea8f022e169aab317fa4` |
| `.agents/skills/speckit-plan/SKILL.md` | 2026-10-16T03:50:36 | `b3dca1c679a2452e8e3aa84c96b42fc5cee42e8b0cb3c30fa77a60ff83db6d4d` |
| `.agents/skills/speckit-specify/SKILL.md` | 2026-10-16T03:50:36 | `88d7fab201de66297f16b6c271a418a0f2c198d714456dfd707e623348ec47c1` |
| `.agents/skills/speckit-tasks/SKILL.md` | 2026-10-16T03:50:36 | `e89dd51b27d0deeab99bad94233bbec57e2fcb053e21857eeb92cbef2d9b5be9` |
| `.agents/skills/speckit-taskstoissues/SKILL.md` | 2026-10-16T03:50:36 | `82a8defb6bfbdcd8adc5d3a442398a7acc3f975ad39624e0e0f84d6ffeada550` |
| `.claude/skills/speckit-analyze/SKILL.md` | 2026-06-22T10:24:47 | `0c6d0525b97b523851a3ea8fc0efdcf477dfa65bef8c4d976b6da2f220f32c86` |
| `.claude/skills/speckit-checklist/SKILL.md` | 2026-06-22T10:24:47 | `b05086a11d4b98dcb85822e8a468ef30b8353f132b6dac1359080b93833c4666` |
| `.claude/skills/speckit-clarify/SKILL.md` | 2026-06-22T10:24:47 | `eb8907c57e9af2c54952370a45e3fb66cb21a64dc7b3d361615bab914c13becf` |
| `.claude/skills/speckit-constitution/SKILL.md` | 2026-06-22T10:24:47 | `c1a044aba243ca6aff627fb5e4404feb6f1108d4f7dd174631bee3ae477d6c15` |
| `.claude/skills/speckit-converge/SKILL.md` | 2026-06-22T10:24:47 | `a6acccbb6630f04fa716967c68f3be9da96762805ddd2bf284fad36324c5c876` |
| `.claude/skills/speckit-gomemory-context-update/SKILL.md` | 2026-08-03T20:05:07 | `c59e4b46898b9c3485daa9a3adc59040d6eb4c1f3e2ab5056105d60135aea395` |
| `.claude/skills/speckit-implement/SKILL.md` | 2026-06-22T10:24:47 | `042a0415ce60a5b66adf039da431d9a98cb4897ff330635e0bab0becbbaf3cd2` |
| `.claude/skills/speckit-plan/SKILL.md` | 2026-06-22T10:24:47 | `fd0e312109109d09d6b691d232925b54df11681e79e1dc333ad49b210d5efb00` |
| `.claude/skills/speckit-specify/SKILL.md` | 2026-06-22T10:24:47 | `ebe19db388407058a6b078b3a66ea7198cb89795dc75680ba3c457f11bc8bf7a` |
| `.claude/skills/speckit-tasks/SKILL.md` | 2026-06-22T10:24:47 | `023b1c75a62dc6fd8bff67855ebb82ec54a8104e727571aa72200c8afcc3f99b` |
| `.claude/skills/speckit-taskstoissues/SKILL.md` | 2026-06-22T10:24:47 | `aa1151798a551fb52c439ca75c02d4f2e7b40c6c7d11fc025265cf1230c67cb1` |

## 3. Enfoque de las unidades existentes (SC-004)

`mem hcd list --json` sobre una copia de `specs/` (45 carpetas numeradas) en un proyecto temporal: **45 de 45 con `approach: speckit`**, ninguna `hcd` ni `unclassified`. Incluye `001…044` y la propia `045-hcd-protocol`.

## 4. Constitución

`git diff --stat eb26453 -- .specify/memory/constitution.md` → sin cambios. La §18 y la §25.2 se aprobaron (T004) y se aplicaron (T005) el 2026-10-28. Es el único cambio en `.specify/memory/constitution.md` y es la excepción explícita de FR-006; el diff completo está en `evidence/constitution-18-applied.md`. `.specify/` no está en git, así que no aparece en `git diff`.

La plantilla que gomemory distribuye a otros proyectos, `infrastructure/templates/speckit-constitution-gen.md`, recibió la misma §18 el 2026-10-28 (versión 2.0.0 → 2.1.0), sin la §25.2, que es una excepción propia de este proyecto. Su huella nueva está en `domain/seed.go` y la de la 2.0.0 queda como huella anterior, así que una constitución sembrada sin editar se actualiza sola. `start` responde `constitution_conflict` si la constitución vigente exige la carpeta de Spec Kit por unidad y no reconoce la de HCD. No es un archivo de Spec Kit: no altera `specs/`, el adaptador ni las skills `speckit-*`.
