# mem search no oculta la fila que coincide con la consulta

ID: HCD-004
Tipo: fix
Carril: fast
Motivo: Fix con reproducción clara (recuperacion-03 de HCD-002) y enfoque ya decidido por la persona, sin decisiones abiertas interdependientes

## Intención
La compresión de la tabla de mem search oculta tras una marca ⟦mem⟧ la fila cuyo título coincide con la consulta; esa fila debe quedar siempre visible.

## Reproducción
Montaje aislado igual al de `scripts/hcd002_evaluate.py` (caso `recuperacion-03` de `tests/fixtures/hcd002/corpus.json`): almacén temporal (`GOMEMORY_DATA_HOME`, `HOME`), `context_compression_level: max`, `git init`, `mem session start`, `mem import` del bundle `representativo`, y luego:

`mem search "representativo decision 0146: sobre observación omisión delta arr"`

Observado el 2026-10-29 con el binario de `413a84d`: la salida (stdout no TTY) muestra cabecera, tres checkpoints que contienen «0146», `⟦mem⟧ ×15 filas · ref=…` y dos filas finales. La memoria `decision` cuyo título coincide exactamente con la consulta queda dentro de las 15 filas omitidas. El evaluador no la encuentra y el caso falla en los dos brazos de HCD-002.

Causa: `compressTable` (`adapters/secondary/compression/native/log.go:215`) conserva las tres primeras filas, las dos últimas y las protegidas (errores y fallos), y omite las intermedias. No conoce la consulta. `CmdSearch` (`adapters/primary/cli/cmd_search.go:56`) le pasa la tabla por `compressDeliveredContext` sin indicarle qué fila se busca.

## Alcance
Incluido: la salida no TTY de `mem search` con nivel `max`. La fila cuya memoria coincide con la consulta sigue visible, con su ID, aunque el resto de la tabla se comprima. Incluye una prueba que fija la reproducción y la reejecución de `recuperacion-03`. Criterio de «coincide» (Q1): el título es igual a la consulta, sin distinguir mayúsculas ni espacios extremos. Una búsqueda que no coincide con ningún título conserva la compresión de hoy.
Excluido: el orden por relevancia de `SearchMemories` (FTS5/bm25), aunque aquí los checkpoints con «0146» salgan antes que la coincidencia exacta; el panel TTY (`memoryPanel`), que no comprime; los niveles `none` y `structural`; y el resto de compresores del motor. También queda fuera la tool MCP `search_memories` (Q2). Entrega bloques `[id] tipo | título`, no una tabla, y en la sonda del 2026-10-29 (mismo corpus, nivel `max`) devolvió los 20 resultados sin marcas `⟦mem⟧` y con la coincidencia visible.

## ¿Qué pasa si no lo hacemos?
Con `max`, un agente que busca una memoria por su título puede no recibir su ID. Tiene que seguir una marca `⟦mem⟧` (el protocolo no lo hace) o concluir que la memoria no existe. Así se pierde la recuperación de lo omitido que HCD-002 prometía con `get_memory`. Además, `recuperacion-03` seguirá fallando en cualquier medición futura.

## Alternativa más barata
No comprimir nunca la salida de `mem search`. Es un cambio de una línea, pero renuncia al ahorro de `max` en todas las búsquedas para arreglar una sola fila. Otra opción sería que el evaluador siguiera la marca con `mem pack retrieve`, pero eso solo cambia la medición: el agente real seguiría sin ver la fila. Ninguna de las dos es mejor que proteger solo la fila coincidente.


## Aclaraciones
- [x] Q1. ¿Qué filas de mem search cuentan como coincidentes con la consulta y no se omiten: solo el título igual a la consulta (sin distinguir mayúsculas), o también los títulos que contienen la consulta? → Título igual (Recomendado): solo el título igual a la consulta, sin distinguir mayúsculas ni espacios extremos.
- [x] Q2. ¿La tool MCP search_memories entra en HCD-004? Sonda del 2026-10-29 con el corpus de recuperacion-03 y nivel max: entrega los 20 resultados sin marcas ⟦mem⟧ y la coincidencia es visible. → Excluirla (Recomendado): search_memories no oculta la coincidencia según la sonda; la unidad queda en la salida no TTY de mem search.

## Checklist
- [x] 1. La reproducción ya no falla → `go test ./adapters/primary/cli/ -run TestHCD004 -count=1`
- [x] 2. En el binario compilado, el evaluador de HCD-002 pasa las 100 observaciones, incluidas las 5 de `recuperacion-03` (dep: 1) → `scripts/hcd002_evaluate.py --arm improved` sobre `go build ./infrastructure`, sin casos con `quality_passed: false`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-30 02:21 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run TestHCD004 -count=1` (salida 1)
      --- FAIL: TestHCD004SearchKeepsExactTitleRow (0.02s)
          hcd004_search_match_test.go:62: la fila 510, cuyo título es igual a la consulta, quedó oculta:
              ID Tipo Título Contenido
              -- ---- ------ ---------
              500 checkpoint checkpoint representativo 0146 CHECKPOINT-0146 motor delta huella contrato aislamiento s...
              501 checkpoint checkpoint representativo 0147 CHECKPOINT-0147 motor delta huella contrato aislamiento s...
              502 checkpoint checkpoint representativo 0148 CHECKPOINT-0148 motor delta huella contrato aislamiento s...
              ⟦mem⟧ ×15 filas · ref=90e6e31de59c
              518 checkpoint checkpoint representativo 0164 CHECKPOINT-0164 motor delta huella contrato aislamiento s...
              519 checkpoint checkpoint representativo 0165 CHECKPOINT-0165 motor delta huella contrato aislamiento s...
              > Marcas ⟦mem⟧ … ref=X: contenido omitido para ahorrar tokens; recupéralo íntegro con pack_retrieve(ref=X) (CLI: mem pack retrieve X).
              (20 resultados)
      FAIL
      FAIL	mem/adapters/primary/cli	0.320s
      FAIL
- 2026-10-30 02:27 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli/ -run TestHCD004 -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	1.420s
- 2026-10-30 02:27 · tarea 2 · verde · vigente · `d=$(mktemp -d) && go build -o "$d/mem" ./infrastructure && python3 scripts/hcd002_evaluate.py --binary "$d/mem" --arm improved --output "$d/r.json" && python3 -c "import json,sys; r=json.load(open(sys.argv[1]))[\"results\"]; bad=sorted({x[\"case\"] for x in r if not x[\"quality_passed\"]}); ok=sum(x[\"quality_passed\"] for x in r); print(ok, \"/\", len(r), \"observaciones con calidad; casos fallidos:\", bad); sys.exit(0 if len(r)==100 and not bad else 1)" "$d/r.json"` (salida 0)
      improved: repetition 1/5 captured
      improved: repetition 2/5 captured
      improved: repetition 3/5 captured
      improved: repetition 4/5 captured
      improved: repetition 5/5 captured
      100 / 100 observaciones con calidad; casos fallidos: []

## Siguiente
Cerrada con aprobación sobre la revisión 79a87f89c685.
