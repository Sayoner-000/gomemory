# Quickstart: validar la distribución del brazo extensor vía `mem install`

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Guía de validación manual end-to-end. No incluye código de implementación.

## Prerrequisitos

- Build de gomemory con los cambios de esta feature (`InstallSpeckitExtension`
  llamado desde `cmd_install.go`, plantillas embebidas en
  `infrastructure/templates/gomemory-context/`).
- Dos proyectos de prueba en directorios temporales: uno con `.specify/`
  ya inicializado (`specify init . --integration claude` o `--integration
  opencode`), otro sin `.specify/`.

## Escenario 1 — Historia 1: Claude Code recibe el brazo extensor solo con `mem install`

1. En el proyecto de prueba CON `.specify/`, correr el binario nuevo de
   gomemory: `mem install`.
2. **Esperado**: al terminar existen `.specify/extensions/gomemory-context/`
   (extension.yml, README.md, commands/, scripts/) y
   `.claude/skills/speckit-gomemory-context-update/SKILL.md`.
3. Verificar que el script bash tiene permiso de ejecución:
   `test -x .specify/extensions/gomemory-context/scripts/bash/update-gomemory-context.sh`.
4. Sin haber ejecutado `specify extension add` manualmente en ningún
   momento — confirma FR-007.

## Escenario 2 — Historia 2: paridad con OpenCode

1. Repetir el Escenario 1 en un proyecto de prueba inicializado con
   `--integration opencode`.
2. **Esperado**: además de los archivos del Escenario 1, existe
   `.opencode/commands/speckit.gomemory-context.update.md`.
3. Confirmar que ambos artefactos (Claude y OpenCode) se crean en el mismo
   `mem install`, sin importar cuál integración esté activa en
   `.specify/integration.json` — ambos son incondicionales mientras exista
   `.specify/` (ver Acceptance Scenario 2 de Historia 2 en `spec.md`).

## Escenario 3 — Historia 3: proyectos sin spec-kit quedan intactos

1. En el proyecto de prueba SIN `.specify/`, correr `mem install`.
2. **Esperado**: no aparece ningún archivo bajo `.specify/extensions/
   gomemory-context/`, `.claude/skills/speckit-gomemory-context-update/`
   ni `.opencode/commands/speckit.gomemory-context.update.md`.

## Escenario 4 — Historia 4: las correcciones futuras se propagan

1. Sobre el proyecto del Escenario 1 (ya con la extensión instalada),
   modificar a mano un byte de
   `.specify/extensions/gomemory-context/README.md` (simula una versión
   anterior desactualizada).
2. Correr `mem install` de nuevo con el mismo binario.
3. **Esperado**: el archivo modificado vuelve a coincidir con la versión
   embebida (se sobrescribió). Los archivos que NO se tocaron a mano no
   reportan escritura en la segunda corrida (verificar con `mtime` o con
   los mensajes de la CLI, según cómo quede instrumentado el paso).

## Escenario 5 — idempotencia

1. Correr `mem install` dos veces seguidas sobre el mismo proyecto (sin
   tocar nada entre medio).
2. **Esperado**: la segunda corrida no reescribe ningún archivo del brazo
   extensor (mismo contenido, mismo `mtime`).

## Criterio de éxito del quickstart

Los cinco escenarios cubren, en conjunto, todos los Acceptance Scenarios
de `spec.md` (Historias 1–4) y los Success Criteria SC-001 a SC-005.
