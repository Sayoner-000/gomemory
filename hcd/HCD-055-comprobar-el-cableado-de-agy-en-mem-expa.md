# Comprobar el cableado de agy en mem expand status

ID: HCD-055
Tipo: fix
Carril: fast
Motivo: Omisión reproducida y acotada del diagnóstico de hooks, sin decisiones interdependientes ni cambios del runtime.

## Intención
Que mem expand status compruebe el ciclo de hooks de agy y muestre su cableado en texto y JSON, detectando configuración ausente, ilegible o incompleta.

## Reproducción
Con agy instalado, comprobar que `~/.gemini/config/hooks.json` registra el grupo habilitado `gomemory-lifecycle`, con `mem hook agy` para SessionStart, PreInvocation, PreToolUse, PostToolUse y Stop. Ejecutar `mem expand status` y `mem expand status --json`: la salida humana omite agy y el objeto `wiring` tampoco contiene esa clave, aunque las activaciones históricas incluyen agy.

La comprobación automatizada de regresión usará un HOME temporal con los artefactos producidos por el instalador, sin tocar la configuración real. Verificará que ambas salidas incluyen agy conectado; debe fallar antes de la corrección y pasar después.

## Alcance
Incluido: diagnosticar el cableado de agy leyendo el mismo archivo y contrato que escribe el instalador; añadir `agy` al mapa de ExpandWiring, al objeto JSON `wiring` y a la lista humana de harness. Cubrir con pruebas las configuraciones completa, ausente, ilegible, deshabilitada e incompleta y conservar el diagnóstico de Claude, Codex y OpenCode.

Excluido: modificar la instalación de hooks, ejecutar o cambiar los hooks de agy, comprobar su versión o la disponibilidad de su ejecutable, alterar reglas o supertools, cambiar los otros diagnósticos y publicar una versión.

### Criterios de aceptación
- C-01: `mem expand status` muestra una línea `agy ✓` cuando el ciclo requerido está conectado, y `agy ✗` cuando no lo está.
- C-02: `mem expand status --json` incluye siempre `wiring.agy` como booleano, conservando las claves existentes.
- C-03: solo se informa conectado cuando el grupo `gomemory-lifecycle` está habilitado y los cinco eventos tienen los comandos correspondientes de gomemory. Los matchers de herramienta deben cubrir las herramientas requeridas por el instalador; un grupo deshabilitado, un evento ausente o un comando ajeno no prueban cableado completo.
- C-04: un archivo ausente, no legible o con JSON inválido produce `wiring.agy=false`, sin reescribir configuración ni impedir el diagnóstico de los otros harness.
- C-05: el diagnóstico reconoce los comandos generados por el instalador, incluyendo rutas de binario con espacios, y no confunde texto incidental o comandos ajenos con hooks válidos.

### Evidencia previa y normas
La validación del 2026-11-08 y la memoria 2099 documentan la omisión. ExpandWiring enumera tres harness y la salida humana repite esa lista; el instalador de agy mantiene su contrato en agyLifecycleGroup. `mem hcd history --file adapters/primary/setup/expand_wiring.go` no devolvió fichas; no demuestra ausencia de antecedentes porque el índice de historial puede estar incompleto.

Aplican Constitución §3 (capas), §10 (pruebas), §18 (documento HCD único) y §21 (simplicidad y causa raíz); Playbook HCD-DEF-01, HCD-TASK-01, HCD-TDD-01 y HCD-EVIDENCE-01. Se reutilizará el contrato del instalador para evitar declaraciones divergentes y no se añadirán dependencias.

## ¿Qué pasa si no lo hacemos?
El estado seguirá ocultando el cableado de agy. Las activaciones históricas pueden dar una impresión de funcionamiento aunque la configuración actual esté incompleta o deshabilitada, y la persona tendrá que inspeccionar el JSON manualmente.

## Alternativa más barata
Inspeccionar manualmente `~/.gemini/config/hooks.json` y sus cinco eventos. Sirve como comprobación puntual, pero no corrige la omisión del comando ni permite que los consumidores de JSON diagnostiquen los cuatro harness de forma uniforme. El cambio acotado en el diagnóstico evita esa revisión manual recurrente.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla: añadir una prueba de regresión de status con agy instalado en un HOME temporal, observar el rojo antes del cambio y corregir la presencia de agy en texto y JSON → go test ./adapters/primary/cli -run '^TestExpandStatusAgy$' -count=1
- [x] 2. Validar el ciclo de hooks de agy reutilizando el contrato del instalador y cubrir configuración completa, ausente, ilegible, inválida, deshabilitada e incompleta, comandos ajenos, matchers insuficientes y rutas con espacios → go test ./adapters/primary/setup -run '^TestExpandWiringAgy$' -count=1
- [x] 3. Verificar la compatibilidad del diagnóstico de los cuatro harness y documentar agy en la explicación de expand → go test ./adapters/primary/setup ./adapters/primary/cli -run 'Test.*Expand' -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-08 07:22 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestExpandStatusAgy$' -count=1` (salida 1)
              regla: graph-before-grep (builtin)
              regla: native-edit-over-scripts (builtin)
              claude expand.block: 2
          expand_status_agy_test.go:42: status JSON installed=true: code=0 stderr= wiring=map[claude:false codex:false opencode:false]
          expand_status_agy_test.go:31: status installed=false: code=0 stderr= output=claude ✗
              codex ✗
              opencode ✗
              rules: true
              shield: true
              flow: true
              instincts: true
              context: true
              regla: destructive-shell (builtin)
              regla: graph-before-grep (builtin)
              regla: native-edit-over-scripts (builtin)
              claude expand.block: 2
          expand_status_agy_test.go:42: status JSON installed=false: code=0 stderr= wiring=map[claude:false codex:false opencode:false]
      FAIL
      FAIL	mem/adapters/primary/cli	0.807s
      FAIL
- 2026-11-08 07:23 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run '^TestExpandStatusAgy$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.334s
- 2026-11-08 07:23 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/setup -run '^TestExpandWiringAgy$' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.255s
- 2026-11-08 07:25 -05:00 · tarea 1,2,3 · verde · vigente · `go test -race ./adapters/primary/setup ./adapters/primary/cli -run 'Test.*Expand' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	1.408s
      ok  	mem/adapters/primary/cli	1.933s

## Siguiente
Cerrada con aprobación sobre la revisión d2e319c725bc.
