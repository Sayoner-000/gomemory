# Contrato: router de supertools

`domain.RouteSupertools(sig ExpandSignals, sw ExpandSwitches) []SupertoolAction`

## Invariantes

- **I1 Pureza**: no hace E/S y no usa reloj, azar ni variables globales mutables. La misma entrada produce la misma salida, byte a byte.
- **I2 Interruptor**: si `sw.Disabled`, devuelve `nil`. Si `sw.Off[x]`, no aparece ninguna acción de la supertool `x`. `flow` cubre todas las acciones `suggest`.
- **I3 Evento desconocido**: si `sig.Event` es inválido, devuelve `nil`.
- **I4 Orden**: primero `rules.eval`, luego `shield.scan`, `suggest`, `instincts.observe`, `instincts.fold` y por último los resets. El router no produce ninguna acción de inyección de instincts: esa inyección la hace `build_context` (research R9).
- **I5 Una decisión por evento**: el adaptador llama al router una sola vez por invocación del hook (como `DecideInterception`, memoria 557).
- **I6 Silencio ante la duda**: una señal ambigua (herramienta `other`, comando vacío, prompt sin intención clara) no genera `suggest`.

## Tabla de verdad mínima (pruebas de tabla obligatorias)

| # | Event | Señales | Switches | Acciones esperadas |
|---|---|---|---|---|
| 1 | pre-tool | Tool=shell, Command=`ls` | — | `[rules.eval(bash)]` |
| 2 | pre-tool | Tool=shell, Command=`git commit -m x` | — | `[rules.eval(bash), suggest(verify-loop)]` |
| 3 | pre-tool | Tool=shell, Command=`gh pr create` | — | `[rules.eval(bash), suggest(verify-loop)]` |
| 4 | pre-tool | Tool=file | — | `[rules.eval(file)]` |
| 5 | pre-tool | Tool=delegate | — | `[suggest(iterative-retrieval)]` |
| 6 | pre-tool | Tool=shell, ToolCount=50, Threshold=50 | — | `[rules.eval(bash), suggest(strategic-compact)]` |
| 7 | pre-tool | Tool=other | — | `[]` |
| 8 | post-tool | Tool=shell | — | `[instincts.observe]` |
| 9 | post-tool | Tool=file | — | `[]` |
| 10 | prompt | Prompt=`añade la librería zod` | — | `[rules.eval(prompt), suggest(search-first), instincts.observe]` |
| 11 | prompt | Prompt=`explica este archivo` | — | `[rules.eval(prompt), instincts.observe]` |
| 12 | session-start | Source=startup, hash igual | — | `[instincts.fold]` |
| 13 | session-start | Source=startup, hash distinto | — | `[shield.scan, instincts.fold]` |
| 14 | session-start | Source=compact | — | `[instincts.fold, counter.reset, episode.reset]` |
| 15 | stop | — | — | `[rules.eval(stop), instincts.fold]` |
| 16 | pre-tool | Tool=shell, Command=`git commit` | Off={flow} | `[rules.eval(bash)]` |
| 16b | pre-tool | Tool=shell, Command=`git commit` | Off={rules} | `[suggest(verify-loop)]` |
| 16c | session-start | Source=startup, hash distinto | Off={shield, instincts} | `[]` |
| 17 | cualquiera | — | Disabled | `nil` |
| 18 | `bogus` | — | — | `nil` |

## Funciones auxiliares puras

- `ClassifyTool(name string) ToolKind`: tabla cerrada para los tres harnesses; los nombres desconocidos devuelven `other`.
- `IsCommitCommand(cmd string) bool`: tokenizado (R8). Reconoce `git commit`, `gh pr create` y `glab mr create`, también dentro de cadenas con `&&`.
- `MatchesDependencyIntent(prompt string) bool`: R9. Debe tener al menos 10 casos positivos y 10 negativos en la prueba de tabla.
