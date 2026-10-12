# Validación 038 — informe saneado

**Estado: `blocked`** (`measurement_unavailable`). Sin capturas fabricadas; T025–T028 sin marcar.

## Procedencia

- Revisión: `79c5351238782de005f7b8ed67b1322a177e933c`
- Manifiesto: `specs/038-fix-codex-compression/evidence/query-manifest.json`, SHA-256 `f990fc8d9ea8285f5ed69d58b09570f17f7c540c3f8b115d69393e611d3b7b49` (3 consultas elegibles prefijadas).
- Negativo: `specs/038-fix-codex-compression/evidence/fallback-manifest.json`, SHA-256 `c8e521c0152ec0b3349d0639d6389df812bd82df9bcab8dfb0e2213317f11587`.
- Entorno: `.memory/validation/038/environment.json` (local, no versionado).

## Motivo

Codex 0.162.1 no expone una interfaz verificable por llamada para resultados de herramientas en TUI ni un oráculo independiente de entrada del modelo (`codex debug prompt-input` solo declara la lista inicial; `codex exec --json` es no interactivo). Sin adaptador certificado no hay pares comparables; el evaluador rechazaría cualquier entrada con exit 2. No se ejecutó el evaluador sobre evidencia real porque no existe.

## Qué sí está demostrado

- Instrucciones consistentes v9 y migración preservadora (T010/T017, tests en verde).
- Evaluador offline 21/21 con pytest y ejemplo sintético → `blocked` exit 2 (T019–T022).
- Puerta CI pytest + auditoría (T030) y controles Go (T030/T031).

## Cierre

FR-007–011 y SC-002–005 quedan pendientes de una TUI con captura certificable. Reabrir por T024 con nueva versión de Codex y nueva certificación; conservar este informe y generar uno nuevo.
