# Línea base — feature 037

Medida el 2026-10-11 sobre la revisión `30528515597292ecfcd42c7c08283cae0fbbbd12` (rama `main`), antes de cualquier cambio de código.

## Entorno

| Herramienta | Versión |
| --- | --- |
| Go | go1.27.2 darwin/arm64 |
| Node | v26.11.1 |
| Python por defecto (`python3`) | 3.9.6 — **por debajo del mínimo 3.12**; los runners usan `/opt/homebrew/bin/python3.13` |
| Sistema | macOS (Darwin 27.0.0, arm64) |

## Resultados

| Comando | Resultado |
| --- | --- |
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `go test ./... -count=1` | OK (todas las suites) |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | 22/24 — fallan `v2: session.created starts a session only for this directory` y `v2: session.created uses the session location, not the envelope` (H-09) |
| `npm --prefix installer ci && run typecheck` | OK |
| `npm --prefix installer test` | 20/20 |
| `scripts/coverage.sh` (combinada) | **78,6 %** — por debajo del umbral 80 %, el script sale 1 |

## Consecuencias

- La puerta de cobertura ya falla en la línea base. La entrega 037 debe dejarla en ≥80 %, no solo "no bajarla".
- El `python3` del sistema no cumple el mínimo de los runners; el preflight de T068 debe rechazarlo con `RUNTIME_UNSUPPORTED`.

## Resultado tras implementar (2026-10-11, árbol de trabajo sobre 3052851, sin commit)

| Comando | Línea base | Ahora |
| --- | --- | --- |
| `go vet ./...` | OK | OK |
| `go test ./... -count=1` | OK | OK |
| `go test -race` (usecases, persistence, cli) | no medido | OK, sin carreras |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | 22/24 | 39/39 |
| `npm --prefix installer ci && typecheck && test` | 20/20 | 20/20 |
| `scripts/coverage.sh` (combinada) | 78,6 % (falla) | 80,4 % (pasa) |
| `audit_037_smoke.py --case all` (Python 3.13) | — | 60/60 |
| `git diff --check` | — | OK |

Q6 (Windows) y Q7 (OpenCode v2 real) no se ejecutaron: no hay Windows ni OpenCode 2.x en esta máquina. Ver `closure.md`.
