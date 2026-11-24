# La skill ACR mezcla saltos de línea CRLF y LF

ID: HCD-062
Tipo: fix
Carril: fast
Motivo: fix con reproducción clara (git ls-files --eol marca i/mixed) y sin decisiones abiertas

## Intención
Que las dos copias de la skill adversarial-consensus-review usen solo LF como el resto del repositorio y que un test impida volver a versionar CR en plantillas y skills

## Reproducción
`git ls-files --eol` marca `i/mixed` en `infrastructure/templates/adversarial-consensus-review/SKILL.md` y `skills/adversarial-consensus-review/SKILL.md`: la skill de terceros llegó con CRLF y las ediciones de gomemory (HCD-033 y reglas de trabajo v3) añadieron líneas con LF. La excepción `-text` de `.gitattributes` evita que git la normalice.

## Alcance
Incluido: convertir a LF las dos copias de la skill ACR; retirar de `.gitattributes` la excepción `-text`, que ya no protege un original byte a byte; un test de contrato que falle si una plantilla de `infrastructure/templates/` o una skill de `skills/` contiene CR.
Excluido: cambiar el contenido de la skill; otros archivos con CR fuera de plantillas y skills (hoy no hay).

## ¿Qué pasa si no lo hacemos?
Cada agente recibe una skill con finales de línea mezclados; los diffs de cualquier edición futura muestran el archivo entero como cambiado (pi-gomemory vio 2.939 líneas para 2 reales) y cada copia instalada difiere en bytes de una versión normalizada, lo que confunde las comparaciones de desfase de HCD-073.

## Alternativa más barata
Pasar todo el archivo a CRLF conservaría el estilo del original, pero contradice la regla `eol=lf` del repositorio y el original ya no existe byte a byte. Normalizar solo en los instaladores (como hace Pi) deja la fuente mezclada.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Escribir en tests/contract un test que falle si una plantilla o skill versionada contiene CR → go test ./tests/contract -run TestHCD062SinCRLF -count=1
- [x] 2. Convertir a LF las dos copias de la skill ACR y retirar su excepción -text de .gitattributes → go test ./tests/contract -run 'TestHCD062SinCRLF|TestSkillSourcesInSync' -count=1 && test -z "$(git ls-files --eol | grep -E 'w/(crlf|mixed)')"
- [x] 3. Ejecutar la regresión de setup, contract y cli → go test ./adapters/primary/setup ./tests/contract ./adapters/primary/cli -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-23 19:31 -05:00 · tarea 1 · rojo observado · caducada · `go test ./tests/contract -run TestHCD062SinCRLF -count=1` (salida 1)
      --- FAIL: TestHCD062SinCRLF (0.01s)
          hcd062_crlf_test.go:41: contienen CR (usa solo LF): infrastructure/templates/adversarial-consensus-review/SKILL.md, skills/adversarial-consensus-review/SKILL.md
      FAIL
      FAIL	mem/tests/contract	0.802s
      FAIL
- 2026-11-23 19:31 -05:00 · tarea 1,2 · verde · vigente · `go test ./tests/contract -run 'TestHCD062SinCRLF|TestSkillSourcesInSync' -count=1 && test -z "$(git ls-files --eol | grep -E 'w/(crlf|mixed)')"` (salida 0)
      ok  	mem/tests/contract	0.263s
- 2026-11-23 19:32 -05:00 · tarea 1,2 · verde · vigente · `go test ./tests/contract -run 'TestHCD062SinCRLF|TestSkillSourcesInSync' -count=1 && test -z "$(git ls-files --eol | grep -E 'w/(crlf|mixed)')"` (salida 0)
      ok  	mem/tests/contract	0.237s
- 2026-11-23 19:32 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/setup ./tests/contract ./adapters/primary/cli -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	2.501s
      ok  	mem/tests/contract	92.175s
      ok  	mem/adapters/primary/cli	18.712s

## Siguiente
Cerrada con aprobación sobre la revisión 74c35bc966ef.
