# Evidencias ACR + ADM — 2026-11-04

Target: `4cfdd1cb0841f7d990135ea4125280d087319840` más worktree HCD-025. Identidad completa en `target-manifest.json`; cada entrada contiene ruta y SHA-256 de sus bytes al congelar. El digest se calcula con SHA-256 sobre `json.dumps(rows, separators=(",", ":"))`, con rutas ordenadas. Los archivos de auditoría se añadieron después del congelado.

## Reproducciones

Ejecutar desde la raíz del repositorio. Requieren el toolchain y las dependencias Go ya disponibles. Los scripts almacenados con extensión `.go.txt` no se compilan durante `go test ./...`; se copian a un temporal para ejecutarlos por separado.

```sh
repro_dir=$(mktemp -d)
cp docs/audits/evidence/2026-11-04-project/storage-real-schema.go.txt "$repro_dir/storage.go"
go run "$repro_dir/storage.go"
cp docs/audits/evidence/2026-11-04-project/providers-and-storage.go.txt "$repro_dir/providers.go"
go run "$repro_dir/providers.go"
cp docs/audits/evidence/2026-11-04-project/snapshot-panic.go.txt "$repro_dir/snapshot.go"
go run "$repro_dir/snapshot.go"
```

Los scripts crean y limpian sus datos temporales; no borran el directorio que contiene las copias anteriores. El segundo programa utiliza un esquema mínimo para su comprobación preliminar de almacenamiento; el primero utiliza `persistence.Open`, migraciones, índices y FTS reales y confirma C-001/C-002. Los proveedores se invocan con ejecutables sintéticos; no se reindexa el proyecto del usuario. El script de pánico captura los pánicos para poder mostrar ambos casos; por ello puede terminar con código 0 aunque haya demostrado los defectos.

Salida observada en el target auditado:

```text
schema: persistence.Open succeeded with real migrations and isolated GOMEMORY_DATA_HOME
A001 real_schema: err=<nil> counted=1 other_nodes_remaining=0 FTSOptimized=true DatabaseOptimized=true
A002 real_schema: err=<nil> counted_nodes=1 counted_edges=0 dangling_edges=1 FTSOptimized=true DatabaseOptimized=true
A002 second_pass: err=<nil> counted_edges=1 dangling_edges=0

cross_project: err=<nil> reported=1 other_nodes_remaining=0
new_orphans: err=<nil> reported_edges=0 dangling_edges=1
dry_run_reindex: err=<nil> reindexed=true command_executed=true
other_provider: err=<nil> cleaned=1 snapshot_deleted=true

codebase panic=runtime error: invalid memory address or nil pointer dereference
codegraph panic=runtime error: invalid memory address or nil pointer dereference
```

## Comprobaciones mecánicas

Todos los siguientes comandos finalizaron con código 0; sus salidas están en los archivos enlazados:

| Comando | Evidencia |
|---|---|
| `go test ./... -count=1` | [go-test.log](go-test.log) |
| `go test -race ./application/usecases ./adapters/secondary/persistence ./adapters/secondary/requestcompression ./adapters/primary/cli ./adapters/primary/setup` | [go-race.log](go-race.log) |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | [plugin-node.log](plugin-node.log) |
| `npm --prefix installer run typecheck` | [typecheck.log](typecheck.log) |
| `npm --prefix installer test` | [installer.log](installer.log) |
| `bash scripts/test-install-uninstall.sh` | [install-uninstall.log](install-uninstall.log) |
| `bash scripts/test-purge-physical.sh` | [purge-physical.log](purge-physical.log) |

`go build ./...`, `go vet ./...` y `bash -n scripts/install.sh` finalizaron con código 0 sin diagnósticos.

Comprobaciones pendientes de cierre:

```text
gofmt -l .
adapters/secondary/codegraph/codegraphcli/provider.go
domain/optimize.go

git diff --check
adapters/secondary/codegraph/codegraphcli/provider.go:297: new blank line at EOF.
```

`gofmt -l` devuelve código 0 aunque liste archivos: el workflow de CI transforma esa lista no vacía en fallo. `git diff --check` devolvió código 2.
