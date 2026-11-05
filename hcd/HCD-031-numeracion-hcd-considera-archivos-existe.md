# Numeración HCD considera archivos existentes en disco además de SQLite

ID: HCD-031
Tipo: fix
Carril: fast
Motivo: Vía rápida: fix acotado para cálculo del siguiente número de unidad considerando el disco

## Intención
Hacer que mem hcd start calcule el siguiente número de unidad como MAX(max_sqlite, max_disco) + 1 escaneando los archivos hcd/HCD-*.md del workspace para evitar colisiones en clones frescos del repositorio

## Reproducción
En un repositorio con unidades existentes en `hcd/` pero con la tabla `hcd_units` vacía en la base de datos SQLite local, `mem hcd start` asigna `HCD-001` en vez de continuar a partir del número máximo existente en disco.

## Alcance
Incluido: escaneo del directorio `hcd/` en el workspace para extraer los prefijos numéricos `HCD-NNN` y calcular `max(sqlite, disk) + 1` en `Start`/`CreateActive`, garantizando correlatividad en cualquier clon nuevo sin depender de la base de datos local previa.
Excluido: no modifica los nombres ni contenidos de los archivos HCD existentes, no altera el esquema de tablas de SQLite ni crea subdirectorios por fecha.

## ¿Qué pasa si no lo hacemos?
Cada desarrollador que clone el repositorio o corra en un entorno con base de datos SQLite nueva verá sus unidades reiniciarse en `HCD-001`, colisionando con las unidades históricas existentes en Git.

## Alternativa más barata
Sincronizar manualmente la base de datos SQLite antes de empezar a trabajar. Es inaceptable porque es manual, propenso a errores y rompe la automatización en entornos efímeros (Docker, CI, agentes).

## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->

## Checklist
- [x] 1. Extracción del número máximo de unidad desde los archivos `hcd/HCD-*.md` en el workspace → `go test ./adapters/secondary/hcdfiles -run TestWorkspace_MaxUnitNumber`
- [x] 2. `HCD.Start` asigna `MAX(sqlite, disk) + 1` cuando SQLite está vacío o por detrás del disco → `go test ./application/usecases -run TestHCDStart_ContinuaDesdeDisco`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 14:24 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/hcdfiles -run TestWorkspace_MaxUnitNumber` (salida 1)
      # mem/adapters/secondary/hcdfiles [mem/adapters/secondary/hcdfiles.test]
      adapters/secondary/hcdfiles/workspace_test.go:109:22: w.MaxUnitNumber undefined (type Workspace has no field or method MaxUnitNumber)
      adapters/secondary/hcdfiles/workspace_test.go:118:19: w.MaxUnitNumber undefined (type Workspace has no field or method MaxUnitNumber)
      FAIL	mem/adapters/secondary/hcdfiles [build failed]
      FAIL
- 2026-11-05 14:26 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/hcdfiles -run TestWorkspace_MaxUnitNumber` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	0.003s
- 2026-11-05 14:28 -05:00 · tarea 2 · rojo observado · caducada · `go test ./application/usecases -run TestHCDStart_ContinuaDesdeDisco` (salida 1)
      --- FAIL: TestHCDStart_ContinuaDesdeDisco (0.05s)
          hcd_test.go:458: expected HCD-030 and number 30; got HCD-001 (1)
      FAIL
      FAIL	mem/application/usecases	0.053s
      FAIL
- 2026-11-05 14:29 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/secondary/hcdfiles -run TestWorkspace_MaxUnitNumber` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	(cached)
- 2026-11-05 14:30 -05:00 · tarea 2 · verde · vigente · `go test ./application/usecases -run TestHCDStart_ContinuaDesdeDisco` (salida 0)
      ok  	mem/application/usecases	0.338s

## Siguiente
Cerrada con aprobación sobre la revisión 1ad03d9b2752.
