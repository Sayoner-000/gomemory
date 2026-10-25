# Línea base previa a la implementación de 042

**Fecha**: 2026-10-24. **Commit**: `67d4e9b` (`main`). **Go**: go1.27.3 darwin/arm64.

## Pruebas

`go test ./... -count=1`: todos los paquetes `ok` (29 paquetes; 4 sin pruebas). Sin fallos previos que distinguir.

## Catálogo SQLite

`adapters/secondary/persistence/db.go`: `schemaVersion = 2`; `testdata/schema.golden` con `user_version=2`. 042 sube el catálogo a 3.

## Cobertura de los paquetes que modifica 042

| Paquete | Cobertura |
| --- | --- |
| `mem/domain` | 91.4 % |
| `mem/application/usecases` | 75.7 % |
| `mem/adapters/secondary/persistence` | 73.2 % |

T062 exige no bajar de estas cifras y un mínimo del 80 % en el código nuevo.

## Paridad de skills (canónica `skills/` frente a `infrastructure/templates/`)

| Skill | Estado |
| --- | --- |
| `adversarial-consensus-review` | idéntica |
| `defensive-qa-audit` | idéntica |
| `socratica` | idéntica |
