# La evidencia HCD protege la salida antes de recortarla

ID: HCD-043
Tipo: fix
Carril: full
Motivo: Privacidad, riesgo alto: un fallo filtra texto marcado como privado a SQLite, al documento HCD y a la CLI (auditoría 2026-11-06 H-01)

## Intención
Que `mem hcd verify` nunca guarde ni muestre texto privado, aunque la apertura `<private>` quede fuera del extracto de 2000 bytes / 20 líneas que conserva `Workspace.Run`.

## Reproducción
`Workspace.Run` (`adapters/secondary/hcdfiles/workspace.go:160`) recorta la salida con `tail` y solo después `HCD.Verify` (`application/usecases/hcd.go:309`) aplica `domain.ProtectText`. Con un comando que imprime `<private>`, 2500 bytes de relleno con el canario sintético `SYNTH-PRIVATE-0001` y `</private>`, el extracto empieza dentro del bloque: ya no contiene la apertura y `ProtectText` deja el canario visible.
Programa de la auditoría: `docs/audits/evidence/2026-11-06-project/private-tail.go.txt` → `excerpt_bytes=2000 contains_private_marker=false protected_leaks_synthetic_private=true`.

## Alcance
Incluido: proteger la salida completa de la comprobación antes de recortarla en `Workspace.Run`; mantener la segunda protección de `HCD.Verify`; pruebas con canarios sintéticos sobre el extracto de `Run` y sobre los destinos finales de `Verify` (evidencia guardada y documento de la unidad).
Excluido: cambiar el tamaño del extracto (2000 B / 20 líneas); otros campos de la evidencia (el comando y `--manual` ya se protegen enteros); el plazo de la comprobación y sus procesos descendientes (H-02, otra unidad); metadatos de memorias (H-03, otra unidad).

## ¿Qué pasa si no lo hacemos?
Cualquier comprobación cuya salida tenga un bloque privado de más de 2000 bytes o 20 líneas antes de su cierre deja el contenido privado en `hcd_evidence` (SQLite), en la sección Evidencia del documento versionado en git y en el JSON de la CLI. Es la garantía central del producto: lo marcado como `<private>` no se persiste.

## Alternativa más barata
No hay una más barata: el arreglo es una línea (`tail(domain.ProtectText(out.String()))`) y el paquete ya importa `mem/domain`. Quitar el recorte sí sería más simple, pero haría crecer la evidencia y el documento sin límite.

## Historias
### US1. La evidencia nunca contiene texto privado
Como persona que verifica tareas con `mem hcd verify`, quiero que la salida de la comprobación se proteja entera antes de recortarla, para que ningún texto marcado como privado quede en la base, el documento o la respuesta de la CLI.
- C-01: Dado un comando cuya salida abre `<private>` más de 2000 bytes antes del final y lo cierra al final, cuando `Workspace.Run` lo ejecuta, entonces el extracto no contiene el canario sintético.
- C-02: Dado un comando cuya salida abre `<private>` más de 20 líneas antes del final, cuando `Workspace.Run` lo ejecuta, entonces el extracto no contiene el canario.
- C-03: Dado un bloque privado largo seguido de texto público, cuando `Workspace.Run` lo ejecuta, entonces el texto público final sigue en el extracto.
- C-04: Dada una tarea verificada con un comando que imprime un bloque privado largo, cuando `HCD.Verify` registra la evidencia, entonces ni la evidencia guardada ni el documento de la unidad contienen el canario.

## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->

## Slices
### VS-01. La salida de una comprobación se protege entera antes de recortarla
Tareas: 1, 2, 3
Criterios: C-01, C-02, C-03, C-04

## Checklist
- [x] 1. La reproducción ya no falla: el extracto de `Workspace.Run` no contiene el canario si la apertura queda a más de 2000 bytes o de 20 líneas del final, y conserva el texto público final → `go test ./adapters/secondary/hcdfiles/ -run TestRunProtegeLaSalidaAntesDeRecortar -count=1`
- [x] 2. `HCD.Verify` no guarda el canario ni en la evidencia de SQLite ni en el documento de la unidad → `go test ./application/usecases/ -run TestHCDVerifyNoGuardaTextoPrivadoRecortado -count=1`
- [x] 3. El programa de la auditoría deja de filtrar el canario → `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/private-tail.go.txt $d/main.go && go run $d/main.go | grep -q 'protected_leaks_synthetic_private=false'`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-06 07:15 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/hcdfiles/ -run TestRunProtegeLaSalidaAntesDeRecortar -count=1` (salida 1)
                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        SYNTH-PRIVATE-0001\npublico-final"
          --- FAIL: TestRunProtegeLaSalidaAntesDeRecortar/apertura_a_más_de_20_líneas (0.01s)
              workspace_test.go:131: el extracto contiene el canario privado: "SYNTH-PRIVATE-0002-13\nSYNTH-PRIVATE-0002-14\nSYNTH-PRIVATE-0002-15\nSYNTH-PRIVATE-0002-16\nSYNTH-PRIVATE-0002-17\nSYNTH-PRIVATE-0002-18\nSYNTH-PRIVATE-0002-19\nSYNTH-PRIVATE-0002-20\nSYNTH-PRIVATE-0002-21\nSYNTH-PRIVATE-0002-22\nSYNTH-PRIVATE-0002-23\nSYNTH-PRIVATE-0002-24\nSYNTH-PRIVATE-0002-25\nSYNTH-PRIVATE-0002-26\nSYNTH-PRIVATE-0002-27\nSYNTH-PRIVATE-0002-28\nSYNTH-PRIVATE-0002-29\nSYNTH-PRIVATE-0002-30\n\npublico-final"
      FAIL
      FAIL	mem/adapters/secondary/hcdfiles	0.146s
      FAIL
- 2026-11-06 07:15 -05:00 · tarea 2 · rojo observado · caducada · `go test ./application/usecases/ -run TestHCDVerifyNoGuardaTextoPrivadoRecortado -count=1` (salida 1)
                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     SYNTH-PRIVATE-0043\npublico-final"
      FAIL
      FAIL	mem/application/usecases	0.348s
      FAIL
- 2026-11-06 07:15 -05:00 · tarea 3 · rojo observado · caducada · `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/private-tail.go.txt $d/main.go && go run $d/main.go | grep -q 'protected_leaks_synthetic_private=false'` (salida 1)
- 2026-11-06 07:16 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/secondary/hcdfiles/ -run TestRunProtegeLaSalidaAntesDeRecortar -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	0.173s
- 2026-11-06 07:16 -05:00 · tarea 2 · verde · vigente · `go test ./application/usecases/ -run TestHCDVerifyNoGuardaTextoPrivadoRecortado -count=1` (salida 0)
      ok  	mem/application/usecases	0.411s
- 2026-11-06 07:16 -05:00 · tarea 3 · verde · vigente · `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/private-tail.go.txt $d/main.go && go run $d/main.go | grep -q 'protected_leaks_synthetic_private=false'` (salida 0)

## Siguiente
Cerrada con aprobación sobre la revisión 1e81da37d094.
