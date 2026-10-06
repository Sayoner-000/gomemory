# Research: 036-compression-promise-gaps

**Fecha**: 2026-10-06 — Todas las incógnitas resueltas contra el código y la auditoría [378]. Sin NEEDS CLARIFICATION pendientes.

## R1 — Texto veraz del hook OpenCode (US2, FR-004)

- **Decision**: Cambiar `states["opencode"]` en `toolOutputHookStates` (`adapters/primary/cli/cmd_doctor_compression.go:49`) de `"v1: sin verificar en vivo · v2: no soportado"` a un estado de emisión best-effort, p. ej. `"activo (best-effort: emite reescritura, la sustitución depende del runtime)"`. La clave `opencode` y el tipo string en `--json` no cambian.
- **Rationale**: La auditoría 2026-10-06 verificó en vivo que el hook emite `{"output": …}` comprimido con ref recuperable (igual que Claude); lo no verificado es si el runtime aplica la sustitución, no si se emite. El texto actual contradice la evidencia y el diagnóstico lo mostraría como problema inexistente.
- **Alternatives considered**: Mantener el texto conservador de la feature 033 (rechazado: contradice la medición [378]); afirmar soporte total (rechazado: la aplicación por el runtime sigue sin verificar end-to-end).

## R2 — Punto de prueba para US2

- **Decision**: Extender el patrón de `TestToolOutputHookStates_CodexSoloEnOrigen` (`cmd_doctor_compression_codex_test.go:11`) con una aserción del valor opencode que contenga `best-effort`, sin congelar la cadena exacta salvo esa palabra clave.
- **Rationale**: El test existente solo cubre codex; el fixture `tests/contract/testdata/tool_output/claude-bash-doctor.json` es entrada, no aserción sobre esta cadena, así que el cambio no rompe dorados.
- **Alternatives considered**: Test nuevo en `tests/contract` (rechazado: el patrón unitario junto al caso codex es el sitio natural y ya existe).

## R3 — Superficies de documentación para US1/US3 (alcance B de la clarificación)

- **Decision**: `INSTALLATION.md` (sección de ahorro, ~línea 346: tabla por vía + comandos de reproducción) y `docs/MANUAL.md` (§10 `mem pack`, §16 `mem usage`, sección de hooks ~línea 285: casos intactos + límites por harness). TUI fuera de alcance.
- **Rationale**: Son las superficies que la historia P1 promete verificar; la ayuda en línea ya quedó cubierta por US4 (implementada).
- **Alternatives considered**: Incluir TUI (rechazado en clarificación, opción B).

## R4 — Cifras fechadas a publicar (opción A de la clarificación)

- **Decision**: Publicar con fecha 2026-10-06 y comando de reproducción cada una: `mem usage` 84.38% por emisión (sesión medida 432099→67495); `pack build` ~15–20% por presupuesto (≈+5pp con grafo: 20.8% vs 15.6%); motor nativo bimodal (`pack savings`: json 81.5%, listing 92.7%, table 74.9% en bajo volumen; structural/prose ~0.3% en ~1.1M tokens con fallback intacto); hook solo sobre ~2000 tokens con ganancia recuperable (JSON 30 elementos intacto, 60 elementos 20k→~1k).
- **Rationale**: Son las mediciones de la auditoría [378] reproducibles hoy con `mem pack savings`, `mem pack build --json` y `mem hook tool-output`.
- **Alternatives considered**: Cualitativo sin cifras (rechazado en clarificación, opción A).

## R5 — US4 ya implementada

- **Decision**: `Usage()` reagrupado en 8 flujos con `pack`/`seed`/`adr-sync` visibles (`cli.go`), contrato `TestUsage_MencionaPackYFlujos` en verde, `tests/contract` y `adapters/primary/cli` verdes. No entra en tasks salvo regresión.
- **Rationale**: Implementado y verificado con binario fresco el 2026-10-06 (memoria [381]).

## R6 — Revalidación T002 (2026-10-06, segunda medición)

- `pack savings`: json 82.7% (28 usos), listing 92.7%, table 71.6%, structural 0.3% (605 usos, 1080k→1077k), prose 0.5%. Mismo patrón bimodal que la auditoría.
- `pack build` (otra tarea): 8637→5314 (38.5%, saved 3323). El ahorro por presupuesto varía por tarea (~15–40% observado); documentar como rango con método, no cifra fija.
