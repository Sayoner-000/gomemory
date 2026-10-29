# Evidencia T118 — suite completa y cobertura (§10)

Fecha: 2026-10-28. Rama `045-hcd-protocol` (árbol de trabajo, con cambios sin confirmar) frente a `main` en `eb26453` (worktree temporal, mismo comando).

```text
go vet ./...                                          → exit 0, sin avisos
go test -count=1 -coverprofile=… ./...                → exit 0, 0 paquetes en FAIL
go tool cover -func=… | tail -1
  main   (eb26453): total 69.9 % de sentencias
  rama   (045):     total 70.6 % de sentencias
```

**La línea base no baja** (+0,7 puntos). El mínimo global de 80 % de la §10 **no se alcanza ni en `main`** (69,9 %); esta rama no lo empeora pero tampoco lo cierra.

## Paquetes cuya cobertura cambió (porcentaje de sentencias)

| Paquete | main | rama |
|---|---|---|
| `mem/adapters/primary/cli` | 51.4 | 51.9 |
| `mem/adapters/primary/cli/hcd` | — | 59.9 |
| `mem/adapters/primary/setup` | 72.0 | 72.1 |
| `mem/adapters/primary/tui` | 75.3 | 75.0 |
| `mem/adapters/secondary/hcdfiles` | — | 63.0 |
| `mem/adapters/secondary/persistence` | 75.2 | 75.7 |
| `mem/application/usecases` | 77.4 | 77.3 |
| `mem/domain` | 92.4 | 91.4 |
| `mem/infrastructure` | 48.9 | 52.9 |

## Historia de la medición

Una primera medición dio 68,6 % (por debajo de `main`). La causa: el CLI `adapters/primary/cli/hcd` (445 líneas) solo se ejercitaba con el binario, que `go test` no cuenta, y el almacén `persistence/hcd*.go` solo se cubría desde otros paquetes. Se añadieron pruebas en proceso (`adapters/primary/cli/hcd/execute_test.go`, `adapters/secondary/persistence/hcd_store_test.go`, `domain/hcd_migrate_test.go`) y la cobertura subió sin cambiar código de producción.

Una segunda medición intermedia falló en `tests/contract` con «compilar binario» porque se lanzó mientras se editaba `parse.go`; se repitió completa tras terminar las ediciones y pasó.

## Defectos hallados al contrastar con el binario real (todos cerrados)

1. `hcd playbook adopt` y `hcd exception` no parseaban `--expected-revision` → `revision_conflict` (`TestHCDPlaybookExportImportAdopt`).
2. `Store.Delete` dejaba la carpeta vacía de la unidad, que aparecía como `unclassified` tras un rollback.
3. La plantilla de definición sin editar validaba como completa: `isHCDPending` no reconocía `…` ni `{…}`.
4. `ParseDefinition` ignoraba «Problema técnico»: las unidades `technical` no podían pasar `validate`.
5. `mem hcd` faltaba en el índice de `mem help` (`TestHelpIndexListaCadaComandoConAyuda`).
