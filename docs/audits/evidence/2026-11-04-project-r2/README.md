# Evidencias ACR + ADM — segunda ronda

Target: `4cfdd1cb0841f7d990135ea4125280d087319840` más worktree, 1737 archivos. Digest `0ee22e8a53dc1398e59a8f60e6b125f732603cafae23be4ed481e9691df020b2`. `target-manifest.json` conserva rutas y hashes; digest calculado con SHA-256 sobre la lista ordenada serializada con `json.dumps(rows, separators=(",", ":"))`.

## Reproducir hallazgos nuevos

Desde la raíz del repositorio, con Go y las dependencias existentes:

```sh
repro_dir=$(mktemp -d)
cp docs/audits/evidence/2026-11-04-project-r2/reviewer-a.go.txt "$repro_dir/reviewer-a.go"
go run "$repro_dir/reviewer-a.go"
cp docs/audits/evidence/2026-11-04-project-r2/reviewer-b.go.txt "$repro_dir/reviewer-b.go"
go run "$repro_dir/reviewer-b.go"
python3 docs/audits/evidence/2026-11-04-project-r2/invalid-target.py
```

Los programas Go se ejecutan por separado: cada uno contiene su propio `main`. Crean DB y archivos temporales, y limpian sus datos. Los archivos `.go.txt` no entran en `go test ./...`. La reproducción del borrado denegado usa chmod y requiere usuario normal, no root; `/usr/bin/true` actúa como identidad de proveedor en macOS/Linux. No ejecutan reindexados ni usan DB del usuario. El directorio que contiene las copias de los programas puede retirarse después de inspeccionarlo.

Resultados observados, no aserciones de éxito: los programas pueden terminar con código 0 mientras muestran un defecto. Tras corregir, deben cambiar las salidas y añadirse las validaciones de cierre del informe.

```text
code_fts: err=<nil> nodes_cleaned=1 orphan_nodes=0 stale_fts_rows=1 fts_optimized=true
invalid_target: err=<nil> report={...contadores cero...}
failed_snapshot_remove: err=<nil> cleaned=1 file_remains=true

code_search residual: err=<nil> cleaned_nodes=1 live_nodes=0 orphan_fts_rows=1 fts_optimized=true
✓ El almacén y grafos ya estaban optimizados — nada que sanear.
invalid target: maintenance_called=false handler_returned=true
failed removal: err=<nil> reported_cleaned=1 artifact_still_exists=true

invalid-target exit=0
✓ El almacén y grafos ya estaban optimizados — nada que sanear.
```

Salidas completas: [reviewer-a.log](reviewer-a.log), [reviewer-b.log](reviewer-b.log), [invalid-target.log](invalid-target.log).

## Verificación de cierres anteriores

Se ejecutaron los programas de [la primera ronda](../2026-11-04-project/README.md) contra la fuente actual. Logs de esta ronda:

- [prior-storage.log](prior-storage.log): conserva la clave ajena y elimina nodos/aristas en una pasada con esquema real.
- [prior-providers.log](prior-providers.log): dry-run sin indexado y snapshot ajeno conservado; incluye además la comprobación preliminar de almacenamiento con esquema mínimo.
- [prior-panic.log](prior-panic.log): ambos proveedores sin pánico ante arquitectura ausente. El programa captura pánicos para mostrar ambos proveedores; el dato relevante es `panic=<nil>`.

## Validaciones mecánicas actuales

Todos los comandos siguientes finalizaron con código 0:

| Comando | Log |
|---|---|
| `go test ./... -count=1` | [go-test.log](go-test.log) |
| `go test -race ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli ./adapters/primary/setup` | [go-race.log](go-race.log), usecases/setup desde caché válida |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | [plugin-node.log](plugin-node.log), 66/66 |
| `npm --prefix installer run typecheck` | [typecheck.log](typecheck.log) |
| `npm --prefix installer test` | [installer.log](installer.log), 24/24 |
| `bash scripts/test-install-uninstall.sh` | [install-uninstall.log](install-uninstall.log) |
| `bash scripts/test-purge-physical.sh` | [purge-physical.log](purge-physical.log) |

`go build ./...`, `go vet ./...`, `gofmt -l .`, `git diff --check` y `bash -n scripts/install.sh` finalizaron con código 0 sin diagnósticos; la lista de gofmt estaba vacía. No se ejecutaron CVE actuales, escaneo de secretos, Windows real ni el umbral combinado de cobertura.

Ledger: `acr_204d39cc-ded5-4dcc-b25e-461f6d775a40`; `APPROVED` con dos MEDIUM y un LOW abiertos, sin correcciones realizadas por esta auditoría. Los IDs C-001..C-003 de este ledger son nuevos y no reemplazan los IDs de la primera ronda.
