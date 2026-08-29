# Contratos — CLI `mem review` (Fase 1)

Nuevo comando `mem review`, despachado desde `adapters/primary/cli/dispatcher.go` hacia `CmdReview` en `adapters/primary/cli/cmd_review.go`, siguiendo el mismo patrón que `cmd_compare.go`/`CmdCompare` (`compare`, `judge`). La CLI **no ejecuta modelos**: solo resuelve la identidad local del target (git/archivo) y delega el resto a los mismos casos de uso que las tools MCP (FR-037).

## `mem review --diff [<rango>]` / `mem review --commit <sha>` / `mem review --file <ruta>`

Inicia una revisión (equivalente CLI de `review_start`).

- **Resolución de target**: `--commit`/`--diff` usan el SHA que provee `git`; `--file` calcula el digest SHA-256 de los archivos indicados (ver `research.md`, Decisión 2).
- **Salida**: `review_id` impreso, y el nivel de independencia si se configuraron revisores en `.memory/settings.json`.
- **Contrato de test**: invocar dos veces sobre el mismo commit sin cambios produce dos `review_id` distintos (cada invocación es una revisión nueva) pero ambos con el mismo `target_digest`.

## `mem review status [<review-id>]`

Sin argumento, muestra la revisión activa del proyecto (si existe). Con `<review-id>`, su estado puntual.

- **Salida**: etapa del protocolo (`frozen`, `awaiting_reviewers`, `consensus_ready`, `fixing`, `rejudging`) o veredicto terminal.
- **Contrato de test**: sobre una revisión recién creada, la salida indica `frozen`/`awaiting_reviewers`, nunca un veredicto.

## `mem review history [--limit N]`

Lista revisiones pasadas del proyecto, más recientes primero.

- **Salida por fila**: `review_id`, tipo y revisión del target, veredicto final (o etapa si sigue abierta).
- **Contrato de test**: tras aprobar una revisión y escalar otra, ambas aparecen con su veredicto correcto.

## `mem review show <review-id>`

Detalle completo de una revisión: target, resultados de cada revisor por ronda, hallazgos, consenso, rondas de corrección y veredicto — el linaje descrito en `data-model.md`.

- **Contrato de test**: el detalle de una revisión con un hallazgo confirmado y corregido muestra la cadena completa: hallazgo de A, hallazgo de B, `consensus_finding` CONFIRMED que los referencia, el `fix_round` que lo aborda y el estado `RESOLVED` de la re-revisión.

## Errores comunes (mismo formato que el resto de la CLI)

- Target inaccesible (`--file` a una ruta inexistente, `--commit` a un SHA que no existe en el repo): error claro antes de llamar a `review_start`, sin crear una revisión a medias.
- `mem review status`/`show` sobre un `review-id` inexistente: error, no una revisión vacía.
