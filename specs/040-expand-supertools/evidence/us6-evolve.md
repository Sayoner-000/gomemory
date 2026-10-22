# US6 — evolve real (T073)

2026-10-21. Binario 2.35.0 recompilado con 040; repositorio y almacén temporales; TTY real mediante PTY.

1. Se generaron 16 observaciones de `go test ./...` y se plegaron con el binario real.
2. `mem expand instincts evolve cmd:go-test` mostró el catálogo de harnesses disponibles.
3. Se eligió Codex 0.163.0. El generador de Socratica produjo un `SKILL.md` con nombre `instinct-cmd-go-test`, capacidades, procedimiento, criterio de salida y atribución ECC; pasó `ValidateSkillBody`.
4. Se mostró la propuesta completa. Ante «¿Instalar esta skill en los harnesses presentes? [s/N]» se respondió `n`.
5. Código de salida 3. No se llamó al instalador ni se creó la skill. Las pruebas unitarias cubren además la aprobación afirmativa y el cuerpo exacto entregado al instalador.

Registro literal transitorio: `/private/tmp/expand040-evolve-outzrv7l/evolve.log`. Los intentos previos con Claude terminaron con error antes de ofrecer instalación; no se ocultaron como verificaciones exitosas. Se mejoró el adaptador de generación para conservar los errores enviados por el proveedor en stdout cuando stderr está vacío.
