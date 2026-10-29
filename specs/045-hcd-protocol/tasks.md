# Tasks: HCD v2

**Input**: [spec.md](spec.md), [plan.md](plan.md) · Tareas v1: `git show a0503b0:specs/045-hcd-protocol/tasks.md`.

## Fase 1: Alcance

- [X] T001 Reescribir `spec.md` (28 FR) con la ceremonia completa y la Vía rápida
- [X] T002 Reescribir `plan.md`, `tasks.md` y `checklists/requirements.md`

## Fase 2: Dominio

- [X] T003 Recuperar del intento v2 el núcleo de dominio (tipos, documento, revisión, huella, gate TDD)
- [X] T004 Añadir Historias, Aclaraciones y Slices al parser, y las dos revisiones (definición y trabajo)
- [X] T005 Derivar los pasos de los dos carriles (`StageFor`, `NextAction`) con tests por paso
- [X] T006 Implementar `Analyze` con tests por hallazgo

## Fase 3: Persistencia

- [X] T007 Schema v5 (3 tablas + DROP acotado), golden, test v4→v5 que conserva memorias y ADR 0003
- [X] T008 `HCDStore` con aprobaciones `approve`, `close` y `cancel`; borrar los repositorios v1

## Fase 4: Casos de uso

- [X] T009 Borrar casos de uso y puertos v1
- [X] T010 `usecases/hcd.go`: start, show, clarify, approve, verify, analyze, close, cancel y promote
- [X] T011 Tests: ceremonia completa paso a paso, Vía rápida, edición tras aprobar, evidencia caducada, rojo y análisis
- [X] T012 Adaptador `hcdfiles` (documento, huella, ejecución)

## Fase 5: Motor

- [X] T013 `cli/hcd` con los 10 comandos y `contracts/engine-cli.md`, con tests de contrato

## Fase 6: Skills, hook y TUI

- [X] T014 10 skills, wrappers OpenCode y `contracts/skills.md`; borrar las plantillas retiradas
- [X] T015 Retirar el heartbeat y `HCDSettings`; aviso pre-edición en `PreToolUse`
- [X] T016 Formulario «Especificación» de la TUI con carril

## Fase 7: Documentación

- [X] T017 MANUAL, ARQUITECTURA, DATABASE, CONFIGURATION, README, ADR 0002 y CHANGELOG
- [X] T018 §18 de la Constitución: artefacto HCD único (requiere aprobación de la persona)

## Fase 8: Verificación real

- [ ] T019 `go build ./... && go vet ./... && go test ./...` en verde
- [ ] T020 Migrar a v5 una copia de la BD real de gomemory y comprobar memorias intactas y 3 tablas `hcd_*`
- [ ] T021 Feature por la ceremonia completa con el binario compilado
- [ ] T022 Fix por la Vía rápida con el binario compilado
- [ ] T023 Medir antes/después (líneas, tablas, skills) y registrarlo en el CHANGELOG
