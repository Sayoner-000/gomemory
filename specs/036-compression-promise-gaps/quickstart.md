# Quickstart: 036-compression-promise-gaps

Guía de validación end-to-end. Prerrequisito: binario fresco (`go build -o /tmp/mem036 ./infrastructure/`).

## Q1 — Tabla por vía localizable (SC-001, US1)

1. Buscar "cuánto ahorra" en `INSTALLATION.md` y `docs/MANUAL.md`.
2. Esperado: tabla con las tres vías, cifras con fecha 2026-10-06 y comando de reproducción de cada una.

## Q2 — Diagnóstico veraz en OpenCode (SC-002, US2)

1. `/tmp/mem036 doctor` con el plugin instalado.
2. Esperado: `hook de salidas (opencode)` con `best-effort`, cero problemas atribuidos a ese hook. Ver contrato en [contracts/doctor-opencode-hook.md](./contracts/doctor-opencode-hook.md).

## Q3 — Reescritura con recuperación (SC-003)

1. Salida sintética JSON de 60 elementos por `mem hook tool-output claude` y `opencode` → reescritura con `ref`.
2. `mem pack retrieve <ref>` → original íntegro (método de la auditoría [378]).

## Q4 — Límite Codex intacto y documentado (SC-004, US3)

1. Misma salida por `mem hook tool-output codex` → sin emisión, salida 0.
2. `MANUAL.md` declara el límite del runtime con sus dos vías de ahorro.

## Q5 — Sin regresiones (SC-005, SC-006, US4)

1. `go test ./tests/contract/ ./adapters/primary/cli/` en verde.
2. `/tmp/mem036 --help` muestra los 8 flujos y menciona `pack`, `seed`, `adr-sync`.
