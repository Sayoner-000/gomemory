# Corregir hallazgos ACR+ADM 2026-11-27: revalidación --stale y contrato CodeGraphProvider

ID: HCD-070
Tipo: fix
Carril: fast
Motivo: Dos hallazgos acotados (medio y bajo), sin decisiones interdependientes: la Vía rápida basta y mantiene la única aprobación en el cierre

## Intención
Que `mem hcd verify --stale` no informe éxito cuando queden tareas caducadas cuya última evidencia verde fue manual: debe fallar cerrado antes de ejecutar ningún comando y nombrar esas tareas. Que el contrato de `CodeGraphProvider` describa fielmente el refresco desacoplado que inicializa e indexa el grafo externo, con su plazo largo y su opt-out. Normas aplicables: Constitución §8 (fallar rápido), §10 (pruebas tempranas), §16 (convenciones) y §18 (documentación en español); Playbook HCD-EVIDENCE-01 (evidencia vigente) y HCD-TDD-01 (rojo antes del verde).

## Reproducción
Construir una unidad HCD con dos tareas caducadas: T1 cuya última evidencia verde es manual y T2 cuya última evidencia verde es un comando. Ejecutar `HCD.Verify` con `Stale: true` y sin comando ni manual. Hoy `verifyStaleEach` agrupa T2, omite T1 en `manual` y devuelve éxito; T1 queda caducada sin aviso. La prueba `TestHCD070VerifyStaleMixtoFallaCerrado` (en `application/usecases/`) debe fallar antes del cambio con:

```sh
go test ./application/usecases -run '^TestHCD070VerifyStaleMixtoFallaCerrado$' -count=1
```

Para H-02, la contradicción se demuestra comparando `application/ports/code_graph_provider.go:17-20` con `adapters/secondary/codegraph/codegraphcli/provider.go:158-184`.

## Alcance
Incluido: `verifyStaleEach` en `application/usecases/hcd.go`; el comentario de contrato en `application/ports/code_graph_provider.go`; pruebas focalizadas en `application/usecases/`.
Excluido: la implementación del refresco en `adapters/secondary/codegraph/codegraphcli/provider.go`, cambios de esquema SQLite, el comportamiento de `verify --task`, HCD-069, la publicación de releases y el informe `docs/audits/2026-11-27-acr-adm-project.md`.

## ¿Qué pasa si no lo hacemos?
Una revalidación puede declarar verificada una unidad con tareas todavía caducadas, sin que la persona vea cuáles faltan: se rompe la garantía de evidencia vigente (Playbook HCD-EVIDENCE-01). El contrato del proveedor sigue prometiendo un sondeo corto sin indexación mientras el refresco detached escribe el índice y el snapshot: los consumidores razonan sobre un contrato falso.

## Alternativa más barata
Añadir un aviso en la salida y conservar la revalidación parcial: cuesta menos, pero sigue devolviendo éxito y deja la decisión de observar T1 a quien lee la consola; se rechaza por §8 (fallar rápido). Para H-02, no tocar el comentario: coste cero, pero deja el contrato materialmente incorrecto; se rechaza.


## Aclaraciones
- [x] Q1. ¿La tabla adr_sync_records se borra con una migración, se vacía o se deja huérfana, dado que la Constitución §25.1 exige esquema solo aditivo? → Fuera de alcance de HCD-070: la tabla adr_sync_records y su migración pertenecen a HCD-069. Esta unidad solo toca verify --stale y el contrato de CodeGraphProvider.

## Checklist
- [x] 1. Corregir `application/usecases/hcd.go` para que la reproducción ya no falla → re-ejecutar la reproducción
- [x] 2. Contrato de CodeGraphProvider actualizado → grep -q "inicializa e indexa" application/ports/code_graph_provider.go && go test ./application/ports ./adapters/secondary/codegraph/codegraphcli -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-27 06:07 -05:00 · tarea 1 · rojo observado · caducada · `go test ./application/usecases -run ^TestHCD070VerifyStaleMixtoFallaCerrado$ -count=1` (salida 1)
      --- FAIL: TestHCD070VerifyStaleMixtoFallaCerrado (0.50s)
          hcd070_stale_mixed_test.go:42: err = <nil>, want ErrValidation que nombre la tarea 1
      FAIL
      FAIL	mem/application/usecases	0.863s
      FAIL
- 2026-11-27 06:08 -05:00 · tarea 1 · verde · caducada · `go test ./application/usecases -run ^TestHCD070VerifyStaleMixtoFallaCerrado$ -count=1` (salida 0)
      ok  	mem/application/usecases	0.757s
- 2026-11-27 06:10 -05:00 · tarea 2 · verde · caducada · `grep -q "inicializa e indexa" application/ports/code_graph_provider.go && go test ./application/ports ./adapters/secondary/codegraph/codegraphcli -count=1` (salida 0)
      ?   	mem/application/ports	[no test files]
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.919s
- 2026-11-27 06:10 -05:00 · tarea 1 · verde · caducada · `go test ./application/usecases -run ^TestHCD070VerifyStaleMixtoFallaCerrado$ -count=1` (salida 0)
      ok  	mem/application/usecases	0.648s
- 2026-11-27 06:13 -05:00 · tarea 1 · verde · vigente · `go test ./application/usecases -run ^TestHCD070VerifyStaleMixtoFallaCerrado$ -count=1` (salida 0)
      ok  	mem/application/usecases	0.912s
- 2026-11-27 06:13 -05:00 · tarea 2 · verde · vigente · `grep -q "inicializa e indexa" application/ports/code_graph_provider.go && go test ./application/ports ./adapters/secondary/codegraph/codegraphcli -count=1` (salida 0)
      ?   	mem/application/ports	[no test files]
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.741s

## Siguiente
Cerrada con aprobación sobre la revisión 092aa6fec1e9.
