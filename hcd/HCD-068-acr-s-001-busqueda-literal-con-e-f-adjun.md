# ACR S-001: búsqueda literal con -e/-f adjuntos no se reconoce en graph-before-grep

ID: HCD-068
Tipo: fix
Carril: fast
Motivo: Fallo acotado y reproducible en domain/rules_graph.go, sin decisiones interdependientes; HCD-066 usó el mismo carril para esta regla

## Intención
Que graph-before-grep reconozca las formas adjuntas -ePATTERN y -fFILE de grep/rg/ag/git grep como patrones explícitos, para que un texto literal se avise en vez de bloquearse, igual que con -e PATTERN.

## Reproducción
Con el código actual, `EvaluateRules` sobre `grep -e'no disponible' --include=*.go src` devuelve `block` de `graph-before-grep`, mientras que `grep -e "no disponible" --include=*.go src` devuelve `warn`. Igual diferencia con `rg -e'no disponible' -g'*.go' src` (adjunto: `block`; separado: `warn`).

Ejecutado el 2026-11-27 sobre HEAD con un test temporal inyectado por `go test -overlay` (sin modificar el repositorio):

    grep -e'no disponible' --include=*.go src  → block · "buscar código .go con grep cuando el grafo ya lo indexa"
    grep -e "no disponible" --include=*.go src → warn  · "buscar texto literal en código .go con grep"
    rg -e'no disponible' -g'*.go' src          → block · "buscar código .go con rg cuando el grafo ya lo indexa"
    grep -e'no disponible' --include=*.go .     → warn  · avisa solo porque "." cuenta como puntuación literal

Causa: `searchPatterns` (domain/rules_graph.go:278-314) solo empareja los tokens exactos `-e`, `--regexp`, `--regexp=`, `-f` y `--file`; cualquier otro token que empiece por `-` se descarta como opción. El patrón adjunto queda fuera y se toma como patrón el primer argumento posicional, que suele ser una ruta sin puntuación (`src`). `literalSearch` (255-273) ve esa ruta, no el texto literal, y devuelve false. En cambio `flagValue` (471-487) ya admite formas adjuntas de opciones cortas (`-g'*.go'`), así que `codeSearchTarget` detecta la extensión indexada y la regla bloquea.

Precisión sobre el hallazgo original: el ejemplo del revisor B (`grep -e'no disponible' --include=*.go .`) avisa en realidad por el punto de la ruta `.`; el fallo se reproduce con una ruta sin puntuación, como `src`.

## Alcance
Incluido: reconocer `-ePATTERN` y `-e=PATTERN` como patrones explícitos en `searchPatterns`, `searchPaths` y `grepTarget`, y evaluar la literalidad sobre el patrón adjunto; reconocer `-fFILE` como fuente explícita de patrón sin confundir la ruta del archivo con patrón ni con ruta de búsqueda; añadir casos de regresión en `domain/rules_graph_test.go` para las formas adjuntas y separadas de grep y rg.
Excluido: leer el contenido de los archivos de patrones de `-f` para decidir literalidad (el hogar del hook no tiene ese contenido y HCD-066 definió la literalidad por puntuación del patrón); cambiar el veredicto de `-fFILE`, que seguirá bloqueando como hoy; modificar otras reglas de expand, los adaptadores, la documentación publicada ni el comportamiento de las formas separadas.

## ¿Qué pasa si no lo hacemos?
Un agente en un harness sin Grep nativo que busque texto literal con la forma adjunta sigue recibiendo `block` de graph-before-grep, aunque el grafo no indexe comentarios ni cadenas: la vía legítima prometida por HCD-066 queda incompleta. Para seguir trabajando tiene que saltar a consultar el grafo (que no le devolverá el texto) o desactivar las reglas con `mem expand off rules`, con lo que se pierde también la protección de las demás reglas de expand.

## Alternativa más barata
Documentar en `docs/expand.md` que `-e` debe escribirse separado del patrón. Cuesta una línea y ningún código, pero depende de que el agente recuerde la forma correcta sin ningún control automático —justo el fallo que HCD-065 ya intentó resolver con documentación— y no corrige la detección de la regla. Se descarta por la misma razón que HCD-066 descartó documentar la edición nativa.

## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->

Decisión de alcance registrada: `-fFILE` se reconoce solo en el parser para no confundir la ruta del archivo con un patrón posicional; su veredicto no cambia porque el contenido del archivo no está disponible al evaluar la regla y HCD-066 (Q2) definió la literalidad por el texto del patrón.

## Checklist
- [x] 1. La reproducción ya no falla → re-ejecutar la reproducción

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-26 22:41 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain -run TestGraphBeforeGrep_FormasAdjuntas -count=1` (salida 1)
      --- FAIL: TestGraphBeforeGrep_FormasAdjuntas (0.00s)
          rules_graph_test.go:233: "grep -e'no disponible' --include=*.go src" → block (gomemory expand · regla graph-before-grep: buscar código .go con grep cuando el grafo ya lo indexa; usa codegraph_explore (o search_code/get_symbol de gomemory), que devuelve el código con sus llamadores y su radio de impacto. Si es intencionado, ejecútalo tú en tu terminal o desactiva la regla (mem expand off rules).); esperaba warn
          rules_graph_test.go:233: "rg -e'no disponible' -g'*.go' src" → block (gomemory expand · regla graph-before-grep: buscar código .go con rg cuando el grafo ya lo indexa; usa codegraph_explore (o search_code/get_symbol de gomemory), que devuelve el código con sus llamadores y su radio de impacto. Si es intencionado, ejecútalo tú en tu terminal o desactiva la regla (mem expand off rules).); esperaba warn
      FAIL
      FAIL	mem/domain	0.280s
      FAIL
- 2026-11-26 22:42 -05:00 · tarea 1 · verde · vigente · `go test ./domain -run TestGraphBeforeGrep_FormasAdjuntas -count=1` (salida 0)
      ok  	mem/domain	0.187s

## Siguiente
Cerrada con aprobación sobre la revisión d99ef14aa714.
