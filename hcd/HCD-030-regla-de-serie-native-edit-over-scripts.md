# Regla de serie native-edit-over-scripts en motor expand

ID: HCD-030
Tipo: feature
Carril: fast
Motivo: Vía rápida: feature acotada con comportamiento determinista en hooks, sin decisiones abiertas interdependientes

## Intención
Incorporar una regla de serie nativa en el core de gomemory (`domain/` y motor `expand`) que detecte e intercepte en el hook `PreToolUse:Bash` el uso de scripts inline de consola (Python, Node/Bun/Deno, Ruby, Perl, sed, awk) y comodines de shell (`cat >`, `echo >`, `printf >`, `tee`) que intentan editar o sobreescribir archivos de código fuente en los lenguajes estándar de desarrollo del mercado, advirtiendo/bloqueando al agente y guiándolo al uso de herramientas nativas de edición (`edit_file`, `Edit`, `write_file`) para preservar diffs auditables y activar el contexto de grafo (HCD-028).

## Alcance
Incluido: una tercera regla de serie en el motor de `mem expand` (`domain.BuiltinRules`, junto a `destructive-shell` y `graph-before-grep`), evaluada en el `PreToolUse` de shell de los cuatro runtimes (Claude `Bash`, Codex `Bash`, agy `run_command`, OpenCode `tool.execute.before`). Cubre las extensiones de desarrollo del mercado (.ts, .tsx, .js, .jsx, .mjs, .cjs, .vue, .svelte, .astro, .go, .py, .rs, .java, .kt, .kts, .scala, .cs, .fs, .c, .cpp, .cc, .cxx, .h, .hpp, .php, .rb, .swift, .sh, .bash, .zsh, .dart, .ex, .exs, .sql) e intercepta comandos comodines de modificación de código: scripts inline de intérpretes (python, python3, node, bun, deno, ruby, perl), herramientas de reemplazo in-place (sed -i, awk inplace) y comodines de sobreescritura de terminal (cat >, echo >, printf >, tee). Emite veredicto pedagógico orientando al agente a usar herramientas nativas (edit_file/Edit) para diffs limpios y activar contexto de grafo HCD-028, con soporte de anulación vía overrides o mem expand off rules.
Excluido: ejecuciones de pruebas unitarias y suites (pytest, python -m unittest, go test, npm test, cargo test, vitest), ejecución de scripts desde archivos existentes sin mutar código (python3 script.py, node server.js), comandos de lectura pasiva o tuberías hacia paginadores (cat | grep, head) y modificaciones sobre archivos fuera de código (documentación .md, configuración .json/.yaml, .env, logs).

## ¿Qué pasa si no lo hacemos?
Los agentes continuarán usando scripts ad-hoc de Python, sed o comodines de terminal (`cat >`, `echo >`) para sobreescribir archivos de código en proyectos nuevos, eludiendo la inyección de dependencias de grafo (HCD-028), produciendo cambios sin diffs atómicos visibles para el usuario y aumentando el riesgo de corrupciones sintácticas o de codificación.

## Alternativa más barata
Confiar únicamente en memorias de preferencia (`type=preference`) o instrucciones textuales añadidas manualmente por cada usuario. Es más barata a corto plazo pero ineficaz entre implementadores y proyectos nuevos, ya que no previene físicamente que el agente tome atajos en la terminal.

## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->

## Checklist
- [x] 1. Añadir la regla de serie `native-edit-over-scripts` en `domain.BuiltinRules` con reconocimiento de extensiones del mercado y comandos comodines (python, node, ruby, perl, sed, cat>, echo>, tee) → `go test ./domain -run TestNativeEditOverScripts_Detecta`
- [x] 2. La regla no interviene en ejecución de pruebas unitarias (`pytest`, `python -m unittest`, `npm test`, `go test`) ni lecturas pasivas → `go test ./domain -run TestNativeEditOverScripts_NoInterviene`
- [x] 3. Cablear la regla en el hook `PreToolUse` para los cuatro runtimes y permitir anulación con `overrides` o `mem expand off rules` → `go test ./adapters/primary/cli -run TestNativeEditOverScripts_Hook`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 13:59 -05:00 · tarea 1 · rojo NO observado (el comando pasó) · caducada · `go test ./domain -run TestNativeEditOverScripts_Detecta` (salida 0)
      go: downloading go1.27.3 (linux/amd64)
      ok  	mem/domain	0.006s [no tests to run]
- 2026-11-05 14:01 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain -run TestNativeEditOverScripts_Detecta` (salida 1)
      # mem/domain [mem/domain.test]
      domain/rules_edit_test.go:34:41: undefined: NativeEditOverScriptsRule
      domain/rules_edit_test.go:35:66: undefined: NativeEditOverScriptsRule
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 14:04 -05:00 · tarea 1 · verde · caducada · `go test ./domain -run TestNativeEditOverScripts_Detecta` (salida 0)
      ok  	mem/domain	0.003s
- 2026-11-05 14:04 -05:00 · tarea 2 · verde · caducada · `go test ./domain -run TestNativeEditOverScripts_NoInterviene` (salida 0)
      ok  	mem/domain	0.003s
- 2026-11-05 14:08 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli -run TestNativeEditOverScripts_Hook` (salida 0)
      ok  	mem/adapters/primary/cli	0.285s
- 2026-11-05 14:08 -05:00 · tarea 1 · verde · vigente · `go test ./domain -run TestNativeEditOverScripts_Detecta` (salida 0)
      ok  	mem/domain	(cached)
- 2026-11-05 14:09 -05:00 · tarea 2 · verde · vigente · `go test ./domain -run TestNativeEditOverScripts_NoInterviene` (salida 0)
      ok  	mem/domain	(cached)

## Siguiente
Cerrada con aprobación sobre la revisión 66ae65ae944d.
