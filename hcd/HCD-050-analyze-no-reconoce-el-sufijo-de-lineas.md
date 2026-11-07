# analyze no reconoce el sufijo de líneas «:52/55» y da un PATH-MISSING falso

ID: HCD-050
Tipo: fix
Carril: fast
Motivo: Fix con reproducción clara y causa localizada en una expresión regular; sin decisiones abiertas

## Intención
Que las lentes de analyze quiten también el sufijo de líneas separado por barra («archivo.go:52/55»), igual que ya quitan «:12», «:152-159» y «:160,190», para que una ruta existente no se reporte como PATH-MISSING

## Reproducción
Al cerrar HCD-048, `mem hcd analyze` dio «PATH-MISSING: la ruta citada tests/integration/session_delta_test.go:52/55 no existe», y el archivo sí existe. `CitedPaths` (`domain/hcd_lens.go`) solo quita los sufijos que reconoce `lineSuffix` (`:\d+([-,]\d+)*(:\d+)?$`): «:12», «:152-159» y «:160,190», pero no «:52/55». Prueba: `go test ./domain -run TestHCDLens_CitedPathsSufijosDeLinea -count=1`.

## Alcance
Incluido: `lineSuffix` acepta «/» como separador de líneas, igual que «-» y «,»; la prueba cubre todos los sufijos admitidos. Como `pathLike` usa la misma expresión, `TASK-NO-FILE` también reconoce esas rutas.
Excluido: otras notaciones (por ejemplo «L52» o «#L52»); cambiar qué cuenta como ruta.

## ¿Qué pasa si no lo hacemos?
Cada vez que un documento cita varias líneas con «/», analyze da un aviso falso. Si se corrige el documento para quitarlo, la definición cambia y obliga a volver a aprobar y a verificar todas las tareas: con HCD-048 eso costó unos 10 minutos.

## Alternativa más barata
Escribir las líneas como «:52,55» en los documentos. Sirve, pero el aviso falso volverá con la siguiente persona que use «/», que es una notación común; arreglar la expresión cuesta un carácter.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./domain -run TestHCDLens_CitedPathsSufijosDeLinea -count=1` en `domain/hcd_lens_test.go`
- [x] 2. Mantener en verde las lentes y el motor HCD → `go test ./domain ./application/usecases ./adapters/primary/cli/hcd -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-07 05:51 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain -run TestHCDLens_CitedPathsSufijosDeLinea -count=1` (salida 1)
      --- FAIL: TestHCDLens_CitedPathsSufijosDeLinea (0.00s)
          hcd_lens_test.go:17: CitedPaths = ["a/cinco.go:1-4/9" "a/cuatro.go" "a/dos.go" "a/tres.go" "a/uno.go" "tests/integration/session_delta_test.go:52/55"], want ["a/cinco.go" "a/cuatro.go" "a/dos.go" "a/tres.go" "a/uno.go" "tests/integration/session_delta_test.go"]
      FAIL
      FAIL	mem/domain	0.176s
      FAIL
- 2026-11-07 05:51 -05:00 · tarea 1 · verde · vigente · `go test ./domain -run TestHCDLens_CitedPathsSufijosDeLinea -count=1` (salida 0)
      ok  	mem/domain	0.166s
- 2026-11-07 05:51 -05:00 · tarea 2 · verde · vigente · `go test ./domain ./application/usecases ./adapters/primary/cli/hcd -count=1` (salida 0)
      ok  	mem/domain	0.228s
      ok  	mem/application/usecases	7.679s
      ok  	mem/adapters/primary/cli/hcd	1.999s

## Siguiente
Cerrada con aprobación sobre la revisión bba8c00963d5.
