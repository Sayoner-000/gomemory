# Implementation Plan: HCD v2 — ceremonia completa sobre núcleo ligero, con Vía rápida

**Branch**: `045-hcd-protocol` | **Date**: 2026-10-29 | **Spec**: [spec.md](spec.md) | **v1**: commit `a0503b0`

## Summary

Conservar la ceremonia de HCD (10 skills) y sustituir la maquinaria v1 por un
núcleo ligero: un documento por unidad, tres tablas y un motor que deriva el
paso del documento, las aprobaciones y la evidencia. La Vía rápida es el mismo
motor sin `approve` ni `slice`.

## Technical Context

Go, SQLite (schema v5) y la arquitectura hexagonal vigente. Sin dependencias
nuevas. El núcleo parte del intento v2 respaldado en
`../gomemory-045-hcd-v2-attempt.tgz` (documento, revisión, huella, gate TDD,
schema v5 y ADR 0003), con la cadena completa encima.

## Constitution Check

- §18 (2.2.0), artefactos por enfoque: el enfoque HCD pasa a un documento único, `hcd/HCD-NNN-<slug>.md`. Se ajusta en la fase 7 con aprobación de la persona.
- §6 y ADR 0001 («nunca DROP»): excepción acotada en la ADR 0003.
- Capas: parser, revisiones, pasos y análisis son puros (`domain`); git, archivos y ejecución son adaptadores secundarios.

## Diseño

### Documento

```markdown
# Título
ID: HCD-001 · Tipo: feature · Carril: full · Motivo: …
## Intención
## Reproducción            (fix)
## Alcance                 Incluido: … / Excluido: …
## ¿Qué pasa si no lo hacemos?
## Alternativa más barata
## Historias               (full)  ### US1. … / Como …, quiero …, para … / - C-01: Dado …, cuando …, entonces …
## Aclaraciones            - [ ] Q1. pregunta  |  - [x] Q1. pregunta → respuesta
## Slices                  (full)  ### VS-01. … / Tareas: 1, 2 / Criterios: C-01
## Checklist               - [ ] 1. tarea → comprobación
## Evidencia               (la escribe el motor)
## Siguiente
```

### Pasos (domain)

| Condición, en orden | `full` | `fast` |
|---|---|---|
| unidad no activa | `closed` | `closed` |
| faltantes de definición | `define` | `define` |
| aclaración abierta | `clarify` | `clarify` |
| sin aprobación vigente de la revisión de la definición | `approve` | — |
| sin slices | `slice` | — |
| sin tareas | `tasks` | `tasks` |
| alguna tarea sin hacer | `implement` | `implement` |
| tareas hechas sin evidencia vigente, fallidas o con rojo pendiente | `verify` | `verify` |
| todo verificado | `close` | `close` |

### Revisiones

- **Definición**: Intención, Reproducción, Alcance, Historias y Aclaraciones (sin casillas). La aprueba `approve`.
- **Trabajo**: definición más Slices y Checklist (sin casillas). La usan la evidencia y el cierre.

### Análisis (`domain.Analyze`)

| Hallazgo | Severidad |
|---|---|
| faltante de definición, aclaración abierta | critical |
| aprobación ausente o caducada (`full`) | critical |
| historia sin criterio, criterio sin slice, tarea sin slice, referencia inexistente en un slice (`full`) | high |
| tarea sin evidencia vigente, fallida o con rojo pendiente | critical en el paso `close`; medium antes |
| evidencia manual | medium |

`close` exige cero hallazgos `critical`.

### Persistencia

`hcd_units`, `hcd_evidence` y `hcd_approvals` (acción `approve`, `close` o `cancel`). Schema v5 con DROP acotado del catálogo v1 (ADR 0003).

### Motor y skills

- `mem hcd start|show|next|clarify|approve|verify|analyze|close|cancel|promote [--json]`.
- Skills: `hcd`, `hcd-define`, `hcd-clarify`, `hcd-approve`, `hcd-slice`, `hcd-tasks`, `hcd-implement`, `hcd-verify`, `hcd-analyze` y `hcd-close`. Se retiran `hcd-exception`, `hcd-playbook` y `hcd-migrate`.
- Hook pre-edición (D3), TUI (formulario de alta) y sección «HCD activo» del contexto.

### Lo que se elimina de v1

13 de 16 tablas, `HCDSettings`, el heartbeat, operations, executions, approval_prompts, normative_revisions, playbook_adoptions, exceptions, blocks, migración y clasificación de Spec Kit, `delivery.md` y `verification.md` como archivos separados.
