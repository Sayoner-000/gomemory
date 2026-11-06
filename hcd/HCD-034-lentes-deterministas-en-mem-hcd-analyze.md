# Lentes deterministas en mem hcd analyze: tareas sin archivo o sin comprobación, rutas citadas inexistentes y Alternativa más barata vacía

ID: HCD-034
Tipo: evolution
Carril: fast
Motivo: analyze es de solo lectura y las lentes añaden avisos sin bloquear; cambio acotado al dominio, sin decisiones interdependientes

## Intención
Que mem hcd analyze detecte sin coste en tokens lo que una lente de revisión encontraría: tareas del checklist que no nombran un archivo o no tienen comprobación, rutas citadas en el documento que no existen en el repositorio, y una «Alternativa más barata» vacía o de relleno.


## Alcance
Incluido: tres lentes nuevas en `HCDState.Analyze` (`domain/hcd_stage.go`), todas de severidad MEDIUM (aviso, nunca bloquean approve ni close según HCD-CLOSE-01): (1) `TASK-NO-FILE`, tarea del checklist cuyo texto ni comprobación nombran un archivo o directorio; (2) `PATH-MISSING`, ruta citada entre comillas invertidas en el documento que no existe en el repositorio; (3) `CHEAPEST-FILLER`, «Alternativa más barata» respondida con relleno. La existencia de rutas se consulta con un método nuevo del port `HCDWorkspace` (Constitución §3), confinado a la raíz del proyecto; el dominio recibe solo la lista de rutas ausentes. Pruebas de dominio, de caso de uso y del adaptador.
Excluido: la lente de comprensión con subagente (punto [4] del plan EGM); bloquear approve o close con estas lentes; comprobar símbolos en el grafo de código (solo rutas de archivo); cambiar `CHECK-MISSING` y `DOC-MISSING`, que ya cubren la tarea sin comprobación y la sección vacía o con `pendiente`; ACR.

## ¿Qué pasa si no lo hacemos?
Una tarea que no dice qué archivo toca, una ruta mal escrita o un «ninguna» en la Alternativa más barata pasan el motor sin aviso, y solo los descubre una revisión con modelo (ACR o la persona), que cuesta tokens y rondas. Es justo el gasto que motivó HCD-032: detectar tarde lo que una regla determinista ve gratis.

## Alternativa más barata
Añadir estas tres comprobaciones como instrucción en las skills `hcd-define` y `hcd-tasks`, sin código. Se descarta porque depende de que el modelo las aplique (no es determinista ni auditable) y gasta tokens en cada unidad; la comprobación en el motor cuesta cero tokens y `analyze` la reporta siempre.


## Aclaraciones
- [x] Q1. PATH-MISSING: una ruta citada que la unidad todavía no ha creado (archivo nuevo) no existe en disco durante define/tasks/implement. ¿Desde qué paso se avisa? → Desde verify (Recommended): PATH-MISSING solo se emite en los pasos verify y close; antes no, para no avisar de archivos que la unidad aún va a crear
- [x] Q2. CHEAPEST-FILLER: ¿qué cuenta como relleno en «Alternativa más barata»? → Negación sin motivo (Recommended): relleno = la sección completa es una negación escueta (ninguna, no hay, n/a, no aplica, nada, -, no), sin distinguir mayúsculas ni puntuación final; con motivo no avisa

## Checklist
- [x] 1. Crear en `domain/hcd_lens.go` la detección pura: rutas citadas entre comillas invertidas, tarea que nombra archivo y Alternativa de relleno (Q2) → go test ./domain/ -run TestHCDLens
- [x] 2. Emitir en `domain/hcd_stage.go` TASK-NO-FILE, CHEAPEST-FILLER y PATH-MISSING (solo en verify y close, Q1) como MEDIUM → go test ./domain/ -run TestAnalyzeLenses
- [x] 3. Añadir `PathExists` al port `application/ports/hcd.go` e implementarlo en `adapters/secondary/hcdfiles/workspace.go` confinado a la raíz → go test ./adapters/secondary/hcdfiles/ -run TestWorkspacePathExists
- [x] 4. Rellenar las rutas ausentes en `application/usecases/hcd.go` para que analyze las reporte → go test ./application/usecases/ -run TestHCDAnalyzeLenses
- [x] 5. Comprobar el binario real sobre una unidad con los tres defectos en `hcd/` de un repositorio temporal → go test ./tests/integration/ -run TestHCD034Lentes

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 19:29 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain/ -run TestHCDLens` (salida 1)
      # mem/domain [mem/domain.test]
      domain/hcd_lens_test.go:17:9: undefined: CitedPaths
      domain/hcd_lens_test.go:36:13: undefined: TaskNamesFile
      domain/hcd_lens_test.go:44:7: undefined: IsCheapestFiller
      domain/hcd_lens_test.go:49:6: undefined: IsCheapestFiller
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 19:29 -05:00 · tarea 1 · verde · caducada · `go test ./domain/ -run TestHCDLens` (salida 0)
      ok  	mem/domain	0.155s
- 2026-11-05 19:30 -05:00 · tarea 2 · rojo observado · caducada · `go test ./domain/ -run TestAnalyzeLenses` (salida 1)
      # mem/domain [mem/domain.test]
      domain/hcd_lens_test.go:15:4: s.MissingPaths undefined (type HCDState has no field or method MissingPaths)
      domain/hcd_lens_test.go:29:4: s.MissingPaths undefined (type HCDState has no field or method MissingPaths)
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 19:30 -05:00 · tarea 2 · verde · caducada · `go test ./domain/ -run TestAnalyzeLenses` (salida 0)
      ok  	mem/domain	0.134s
- 2026-11-05 19:30 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/secondary/hcdfiles/ -run TestWorkspacePathExists` (salida 1)
      # mem/adapters/secondary/hcdfiles [mem/adapters/secondary/hcdfiles.test]
      adapters/secondary/hcdfiles/workspace_test.go:143:15: w.PathExists undefined (type Workspace has no field or method PathExists)
      FAIL	mem/adapters/secondary/hcdfiles [build failed]
      FAIL
- 2026-11-05 19:30 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/secondary/hcdfiles/ -run TestWorkspacePathExists` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	0.180s
- 2026-11-05 19:31 -05:00 · tarea 4 · rojo observado · caducada · `go test ./application/usecases/ -run TestHCDAnalyzeLenses` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd_lens_test.go:22:14: assignment mismatch: 2 variables but env.uc.Verify returns 3 values
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-05 19:31 -05:00 · tarea 4 · rojo observado · caducada · `go test ./application/usecases/ -run TestHCDAnalyzeLenses` (salida 1)
      --- FAIL: TestHCDAnalyzeLenses (0.17s)
          hcd_lens_test.go:24: action not allowed: la unidad está en define: falta la sección «Intención»
      FAIL
      FAIL	mem/application/usecases	0.406s
      FAIL
- 2026-11-05 19:31 -05:00 · tarea 4 · rojo observado · caducada · `go test ./application/usecases/ -run TestHCDAnalyzeLenses` (salida 1)
      --- FAIL: TestHCDAnalyzeLenses (0.26s)
          hcd_lens_test.go:38: PATH-MISSING: map[CHEAPEST-FILLER:Alternativa más barata:MEDIUM]
      FAIL
      FAIL	mem/application/usecases	0.460s
      FAIL
- 2026-11-05 19:31 -05:00 · tarea 4 · verde · caducada · `go test ./application/usecases/ -run TestHCDAnalyzeLenses` (salida 0)
      ok  	mem/application/usecases	0.470s
- 2026-11-05 19:32 -05:00 · tarea 1 · verde · vigente · `go test ./domain/ -run TestHCDLens` (salida 0)
      ok  	mem/domain	0.130s
- 2026-11-05 19:32 -05:00 · tarea 2 · verde · vigente · `go test ./domain/ -run TestAnalyzeLenses` (salida 0)
      ok  	mem/domain	(cached)
- 2026-11-05 19:32 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/secondary/hcdfiles/ -run TestWorkspacePathExists` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	(cached)
- 2026-11-05 19:32 -05:00 · tarea 4 · verde · vigente · `go test ./application/usecases/ -run TestHCDAnalyzeLenses` (salida 0)
      ok  	mem/application/usecases	(cached)
- 2026-11-05 19:32 -05:00 · tarea 5 · verde · vigente · `go test ./tests/integration/ -run TestHCD034Lentes` (salida 0)
      ok  	mem/tests/integration	1.723s

## Siguiente
Cerrada con aprobación sobre la revisión 806211f554b5.
