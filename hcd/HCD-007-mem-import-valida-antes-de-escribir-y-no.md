# mem import valida antes de escribir y no duplica

ID: HCD-007
Tipo: fix
Carril: fast
Motivo: Fix con reproducción clara en binario real, sin decisiones interdependientes

## Intención
Un bundle inválido se rechaza antes de escribir nada, un fallo de lectura de relaciones no se ignora, reimportar no duplica memorias con secretos y un fallo a mitad informa lo ya importado (C-005, C-006)

## Reproducción
`mem import` de un bundle con [nueva-A, una memoria cuyo contenido es solo `<private>s</private>`, nueva-C]. Hoy falla: aborta con `contenido vacío tras redactar <private>`, deja `nueva-A` escrita, no informa de ella y `nueva-C` no entra en ningún reintento. Las pruebas `TestImportBundle_*` fijan además el error ignorado de `GetByPair` y el dedup con secretos.

## Alcance
Incluido: validar las memorias antes de la primera escritura, propagar el error de `GetByPair`, comparar hashes sobre texto protegido e incluir lo ya importado en el error (C-005, C-006).
Excluido: transacción entre repositorios (cambia los puertos y no arregla la fila inválida); el tipo de relación desconocido (C-007, cerrado sin código).

## ¿Qué pasa si no lo hacemos?
Una fila inválida deja imports parciales imposibles de completar y sin informe, y reimportar un bundle antiguo con secretos duplica memorias.

## Alternativa más barata
Corregir solo el `_` de `GetByPair`: una línea, pero deja el import parcial reproducido. Los cuatro cambios caben en `ImportBundle` sin tocar puertos.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./application/usecases/ -run 'TestImportBundle_RechazaAntesDeEscribir$' -count=1`
- [x] 2. Un fallo de `GetByPair` aborta e informa lo ya importado (dep: 1) → `go test ./application/usecases/ -run 'TestImportBundle_PropagaFalloDeGetByPair$' -count=1`
- [x] 3. Reimportar un bundle con secretos no duplica (dep: 1) → `go test ./application/usecases/ -run 'TestImportBundle_ReimportarConSecretoNoDuplica$' -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-31 15:06 · tarea 1 · rojo observado · caducada · `go test ./application/usecases/ -run 'TestImportBundle_RechazaAntesDeEscribir$' -count=1` (salida 1)
      --- FAIL: TestImportBundle_RechazaAntesDeEscribir (0.01s)
          portability_import_test.go:37: err = import memory: contenido vacío tras redactar 
- 2026-10-31 15:06 · tarea 2 · rojo observado · caducada · `go test ./application/usecases/ -run 'TestImportBundle_PropagaFalloDeGetByPair$' -count=1` (salida 1)
      --- FAIL: TestImportBundle_PropagaFalloDeGetByPair (0.01s)
          portability_import_test.go:62: err = <nil>, quiero el fallo de GetByPair
      FAIL
      FAIL	mem/application/usecases	0.162s
      FAIL
- 2026-10-31 15:06 · tarea 3 · rojo observado · caducada · `go test ./application/usecases/ -run 'TestImportBundle_ReimportarConSecretoNoDuplica$' -count=1` (salida 1)
      --- FAIL: TestImportBundle_ReimportarConSecretoNoDuplica (0.01s)
          portability_import_test.go:94: memorias = 2, quiero 1
      FAIL
      FAIL	mem/application/usecases	0.161s
      FAIL
- 2026-10-31 15:07 · tarea 1 · verde · vigente · `go test ./application/usecases/ -run 'TestImportBundle_RechazaAntesDeEscribir$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.195s
- 2026-10-31 15:07 · tarea 2 · verde · vigente · `go test ./application/usecases/ -run 'TestImportBundle_PropagaFalloDeGetByPair$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.166s
- 2026-10-31 15:07 · tarea 3 · verde · vigente · `go test ./application/usecases/ -run 'TestImportBundle_ReimportarConSecretoNoDuplica$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.166s

## Siguiente
Cerrada con aprobación sobre la revisión 0c38b6b63031.
