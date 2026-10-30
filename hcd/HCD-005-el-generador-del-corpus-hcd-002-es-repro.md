# El generador del corpus HCD-002 es reproducible en Windows

ID: HCD-005
Tipo: fix
Carril: fast
Motivo: Fix con causa raíz localizada (stdout en modo texto traduce LF a CRLF en Windows) y sin decisiones abiertas interdependientes

## Intención
TestHCD002CorpusContract falla en Windows porque scripts/hcd002_corpus.py escribe en sys.stdout en modo texto y la salida regenerada lleva CRLF; debe producir los mismos bytes que tests/fixtures/hcd002/corpus.json en cualquier sistema.

## Reproducción
En la CI: el job «Windows (Go y binario del release)» del workflow `test`, run 40988514031 (v2.42.0), falla con `--- FAIL: TestHCD002CorpusContract`, en `hcd002_corpus_test.go:64`, con el mensaje «el corpus no es reproducible desde su generador: <nil>». Los jobs de Linux y macOS pasan.

En local, simulando la salida estándar de Windows (traducción de `\n` a `\r\n` y la codificación de consola `cp1252` de los runners cuando stdout es un pipe):

`python3 -c "import io,sys,runpy; sys.stdout=io.TextIOWrapper(sys.stdout.buffer, encoding='cp1252', newline='\r\n'); runpy.run_path('scripts/hcd002_corpus.py', run_name='__main__')" | cmp -s - tests/fixtures/hcd002/corpus.json`

Observado el 2026-10-29: sin simular, `cmp` sale con 0; con la simulación, sale con 1 y la primera diferencia está en el carácter 2, el salto de línea tras `{`.

Causa: `scripts/hcd002_corpus.py:120-121` escribe con `json.dump(…, sys.stdout, ensure_ascii=False)` y `sys.stdout.write("\n")`, es decir, en modo texto. En Windows, Python traduce cada `\n` a `\r\n` y codifica con la página de códigos de la consola. El fixture se versiona en LF y UTF-8 (`.gitattributes`: `* text=auto eol=lf`), así que `bytes.Equal` falla. El `err` es `<nil>` porque el script termina bien: solo cambian los bytes.

## Alcance
Incluido: que `scripts/hcd002_corpus.py` emita siempre UTF-8 con LF, escribiendo bytes en `sys.stdout.buffer`, de modo que su salida sea idéntica a `tests/fixtures/hcd002/corpus.json` en Windows, Linux y macOS. Se comprueba con la reproducción simulada y con el job de Windows en la CI.
Excluido: el contenido del corpus, que no cambia ni un byte en Linux y macOS. Tampoco entran los demás scripts de evaluación (`hcd001_evaluate.py`, `hcd002_evaluate.py`): sus pruebas de contrato solo ejecutan `--self-test` y no comparan bytes. Ni cambios en `TestHCD002CorpusContract` ni en `.gitattributes`.

## ¿Qué pasa si no lo hacemos?
El workflow `test` de `main` sigue en rojo en Windows desde v2.42.0, y cada push lo repite. Un fallo real de Windows quedaría escondido detrás de este fallo conocido. La prueba que garantiza que el corpus de HCD-002 es reproducible, y por tanto que la medición de 24,88% también lo es, no protege nada en ese sistema.

## Alternativa más barata
Normalizar en la prueba Go (quitar `\r` antes de comparar) o saltar la comparación en Windows con `skipOnWindows`. Cuestan lo mismo o más y debilitan la garantía: la primera sigue sin detectar el cambio de codificación y la segunda deja de comprobar el sistema donde falla. Escribir bytes en el generador es el cambio más pequeño, porque ataca la causa.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → la reproducción simulada de Windows (`cmp -s` contra `tests/fixtures/hcd002/corpus.json`) sale con 0, y `go test ./tests/contract/ -run TestHCD002CorpusContract -count=1` pasa
- [ ] 2. El job de Windows del workflow `test` pasa `TestHCD002CorpusContract` en la CI sobre el commit del fix (dep: 1) → `gh run view` del run `test` de ese commit

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-30 02:43 · tarea 1 · rojo observado · caducada · `python3 -c "import io,sys,runpy; sys.stdout=io.TextIOWrapper(sys.stdout.buffer, encoding=\"cp1252\", newline=\"\r\n\"); runpy.run_path(\"scripts/hcd002_corpus.py\", run_name=\"__main__\")" 2>/dev/null | cmp -s - tests/fixtures/hcd002/corpus.json && go test ./tests/contract/ -run TestHCD002CorpusContract -count=1` (salida 1)
- 2026-10-30 02:44 · tarea 1 · verde · vigente · `python3 -c "import io,sys,runpy; sys.stdout=io.TextIOWrapper(sys.stdout.buffer, encoding=\"cp1252\", newline=\"\r\n\"); runpy.run_path(\"scripts/hcd002_corpus.py\", run_name=\"__main__\")" 2>/dev/null | cmp -s - tests/fixtures/hcd002/corpus.json && go test ./tests/contract/ -run TestHCD002CorpusContract -count=1` (salida 0)
      ok  	mem/tests/contract	0.943s

## Siguiente
pendiente
