# Corpus de referencia de compresión (feature 033)

Muestras reales usadas por `TestCompressionCorpus` y por los benchmarks del
motor nativo. Capturadas el 2026-09-25 y anonimizadas: correos, autores y rutas
absolutas sustituidos por marcadores neutros (`autor@example.com`, `/repo`,
`/work` o `/tmp/scratch`).

| Muestra | Origen |
|---|---|
| `golist.json` | `go list -json ./... \| jq -s .` sobre este repositorio (array de objetos) |
| `gotest.jsonl` | `go test -json -count=1 ./domain/` (JSON Lines) |
| `gotest-fail.log` | `go test -v` de un módulo de prueba con un fallo inducido y un `panic` |
| `go-panic.log` | traza de `panic` de Go reproducida en un módulo de prueba y anonimizada |
| `py-traceback.log` | script Python con `logging` que termina en `ValueError` con traceback |
| `gitlog.txt` | `git log -120 --stat` de este repositorio |
| `repo.diff` | `git show 5dece4b` de este repositorio |
| `sample.go` | copia de `application/usecases/build_context.go` (v2.25.0) |
| `sample.py`, `sample.ts`, `sample.java` | código escrito para el corpus, sin terceros |
| `listing.txt` | `mem list -n 80` |
| `mem-context.md` | `mem context` de este proyecto |
| `search-results.txt` | `mem search compresión` |
| `prose.md` | tres memorias largas del proyecto (`mem get 58 64 71`) |

Los mínimos que exige cada muestra están en `expectations.json`. Si se
sustituye una muestra, hay que revisar su mínimo y dejar el motivo en el
commit.
