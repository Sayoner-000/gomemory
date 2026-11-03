# Integración de Antigravity CLI (agy) como agente de gomemory

ID: HCD-015
Tipo: feature
Carril: full
Motivo: agy ya usa gomemory mediante hooks manuales con jq y dependencias de Claude; mem install, doctor y la retirada no lo conocen.

## Intención
Registrar agy como agente oficial: mem install --agents agy lo deja funcionando, visible en mem doctor y reversible, sin scripts jq, declarando lo que su host no permite: compresión automática de salidas y aviso tras compactar.


## Alcance
Incluido: registro de `agy` en `domain.KnownAgents`; dialecto `agy` en los hooks de `mem`; instalación idempotente con copia previa de `~/.gemini/config/mcp_config.json`, `hooks.json` y `skills.json`, conservando lo ajeno; migración del grupo manual `gomemory-lifecycle`; filas de `mem doctor`; retirada; instalador y documentación; y una sesión real de agy con el binario nuevo.
Excluido: compresión automática de salidas de agy, porque `PostToolUse` no recibe ni reemplaza la salida; aviso tras compactar, porque no hay evento de compactación; la aplicación Antigravity y su IDE sin comprobar por separado; plugin o marketplace de agy; cambios de proveedor, modelo o credenciales; el trabajo pendiente de HCD-014.

## ¿Qué pasa si no lo hacemos?
agy sigue funcionando con la conexión manual, pero queda fuera del producto: `mem install` no lo configura, `mem doctor` no informa de su estado, una actualización de gomemory no lo corrige y la retirada no lo limpia. Esa conexión depende de `jq`, de `~/.claude/hooks/` y de traducciones en shell que fingen eventos de Claude: si cambia el host o falta una de esas piezas, los hooks fallan sin aviso visible. Otra persona o máquina tendría que repetir el montaje a mano.

## Alternativa más barata
Conservar la conexión manual y documentarla. Cuesta casi nada y ya funciona en esta máquina, pero no da diagnóstico, actualización, retirada ni reproducibilidad. Solo se justifica si agy va a usarse aquí y de forma ocasional. La unidad supone que la persona quiere agy al mismo nivel que Codex, OpenCode y Claude, como pidió.

## Base comprobada y normas

- Revisión del 2026-11-02: `agy` 1.4.12 con autenticación por clave de API de Gemini. `~/.gemini/config/` contiene un registro MCP de gomemory, `skills.json` y el grupo `gomemory-lifecycle` con `SessionStart`, `PreInvocation`, `PreToolUse`, `PostToolUse` y `Stop`.
- Documentación oficial de hooks (https://antigravity.google/docs/hooks?tab=cli): configuración global en `~/.gemini/config/hooks.json` y de espacio de trabajo en `.agents/hooks.json`; campos en camelCase con `conversationId` y `workspacePaths`; `injectSteps` en `SessionStart`, `PreInvocation` y `PostInvocation`; `PostToolUse` responde `{}`; sin evento de compactación. Aún no se contrastó con eventos reales capturados.
- `adapters/primary/cli/cmd_mcp_setup.go`: `globalScopeAgents` y `runGlobalScopeAgent`. `domain/agents.go`: `KnownAgents`, registro único de capacidades con degradaciones declaradas. `adapters/primary/setup/`: instaladores por agente.
- Constitución `521cfe46f409`: §§3–4 (capas y composición), §§8–9 (errores y configuración), §10 (pruebas), §§15 y 20 (privacidad y seguridad), §18 (documento HCD único), §21 (simplicidad). Playbook `fd576035ef3f`: HCD-FLOW-01/02/03, HCD-EVIDENCE-01 y HCD-TDD-01.

## Historias
### US1. Instalar agy como agente oficial
Como persona que usa agy, quiero registrarlo con `mem install --agents agy`, para no montar su integración a mano.
- C-01: Dado un perfil de agy sin gomemory, cuando se ejecuta la instalación, entonces quedan registrados el MCP, los hooks y las skills, con copia previa de cada archivo que cambie.
- C-02: Dado un perfil ya instalado o con hooks y servidores ajenos, cuando se repite la instalación, entonces no se duplican entradas y lo ajeno se conserva intacto.
- C-03: Dado el grupo manual `gomemory-lifecycle`, cuando se instala, entonces se reemplaza por el grupo oficial, con copia previa del archivo, y los hooks no se ejecutan dos veces.

### US2. Usar gomemory durante una sesión de agy
Como persona que usa agy, quiero el contexto, las guardas y el registro de actividad de gomemory, para trabajar con la misma memoria que en los demás agentes.
- C-04: Dada una sesión real de agy con el binario nuevo, cuando empieza, entonces recibe el contexto del proyecto por `injectSteps` y la memoria usa el proyecto de `workspacePaths`.
- C-05: Dado un prompt o una herramienta bloqueada por gomemory, cuando se dispara `PreInvocation` o `PreToolUse`, entonces agy recibe el contexto o la denegación en su formato, sin `jq` ni rutas de Claude.
- C-06: Dado un fallo de `mem` en un hook, cuando agy lo ejecuta, entonces la sesión continúa sin bloquearse y sin salida inválida.
- C-07: Dado el cierre de un turno, cuando se dispara `Stop`, entonces queda registrada la actividad en la sesión de memoria asociada a `conversationId`.

### US3. Conocer y revertir la integración
Como persona responsable del entorno, quiero ver qué cubre agy y poder retirarlo, para no suponer capacidades que su host no ofrece.
- C-08: Dado agy instalado, cuando se ejecuta `mem doctor`, entonces muestra su estado por canal y declara con motivo que no hay compresión automática de salidas ni aviso tras compactar.
- C-09: Dado agy instalado, cuando se retira, entonces desaparecen solo las entradas de gomemory y la configuración ajena queda intacta.
- C-10: Dado un `agy` con versión no comprobada o con un contrato de evento distinto, cuando se ejecutan los hooks, entonces se degradan sin bloquear y `mem doctor` lo indica.

## Aclaraciones
- [x] Q1. Q1: ¿qué debe hacer mem install con el grupo manual gomemory-lifecycle de ~/.gemini/config/hooks.json? A) reemplazarlo por el oficial con copia previa; B) conservarlo y avisar. → La persona respondió literalmente: «A». Se reemplaza el grupo manual por el oficial, con copia previa.

## Slices
### VS-01. Atender una sesión de agy con hooks nativos de gomemory
Tareas: 1, 2, 3, 4, 5
Criterios: C-04, C-05, C-06, C-07

### VS-02. Instalar agy y migrar el grupo manual
Tareas: 6, 7, 8
Criterios: C-01, C-02, C-03

### VS-03. Diagnosticar y retirar la integración
Tareas: 9, 10
Criterios: C-08, C-09, C-10

### VS-04. Comprobar la integración completa en agy real
Tareas: 11, 12
Criterios: C-01, C-04, C-05, C-07, C-08

## Checklist
- [x] 1. Capturar los eventos reales de agy 1.4.12 en `docs/evidence/hcd015/agy-native.md` → observación manual de una sesión aislada con hook de registro: stdin de `SessionStart`, `PreInvocation`, `PreToolUse`, `PostToolUse` y `Stop`, respuesta consumida de `injectSteps` y `decision`, sin credenciales ni historial.
- [x] 2. Congelar esos eventos en `tests/fixtures/hcd015/events.json` (dep: 1) → `go test ./tests/contract -run '^TestHCD015AgyEventFixtures$' -count=1`, que exige los cinco eventos y sus campos `conversationId` y `workspacePaths`.
- [x] 3. Traducir la entrada de agy al contrato neutral de `mem hook` (dep: 2) → `go test ./adapters/primary/cli -run '^TestHCD015Agy(Input|StopRecordsTurnActivity)$' -count=1`, con proyecto tomado de `workspacePaths`, identidad de `conversationId` y actividad del turno leída del transcript de agy.
- [x] 4. Emitir la salida de agy desde el dialecto `agy` (dep: 3) → `go test ./adapters/primary/cli -run '^TestHCD015AgyOutput$' -count=1`, con `injectSteps` en contexto y `decision`/`reason` en guardas.
- [x] 5. Degradar sin bloquear ante un fallo de `mem` o un evento desconocido (dep: 4) → `go test ./adapters/primary/cli -run '^TestHCD015AgyFailOpen$' -count=1`, con JSON válido y salida 0.
- [x] 6. Registrar `agy` en `domain.KnownAgents` y en `globalScopeAgents` (dep: 4) → `go test ./tests/contract -run '^TestHCD015AgyCapabilities$' -count=1`, con cada capacidad ausente declarada con su motivo.
- [x] 7. Instalar MCP, hooks y skills en `~/.gemini/config` (dep: 6) → `go test ./adapters/primary/setup -run '^TestHCD015InstallAgy$' -count=1`, con copia previa, idempotencia y conservación de lo ajeno.
- [x] 8. Reemplazar el grupo manual `gomemory-lifecycle` por el oficial (dep: 7) → `go test ./adapters/primary/setup -run '^TestHCD015MigrateManualGroup$' -count=1`, con copia previa y sin hooks duplicados.
- [x] 9. Mostrar agy en `mem doctor` con sus límites y su versión comprobada (dep: 6) → `go test ./adapters/primary/cli -run '^TestHCD015DoctorAgy$' -count=1`, incluida una versión no comprobada.
- [x] 10. Retirar solo las entradas de gomemory en agy (dep: 7) → `go test ./adapters/primary/cli -run '^TestHCD015UninstallAgy$' -count=1`, con configuración ajena intacta.
- [x] 11. Actualizar instalador y documentación de agentes (dep: 6) → `go test ./tests/contract -run '^TestHCD015AgentSurfaces$' -count=1`, sin prometer compresión automática en agy.
- [x] 12. Comprobar la integración en agy real con el binario nuevo (dep: 5, 8, 9, 10) → observación manual en `docs/evidence/hcd015/agy-session.md`: instalación, contexto recibido, guarda aplicada, actividad registrada y `mem doctor`, en perfil aislado si la autenticación lo permite.

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-02 23:27 · tarea 1 · manual · caducada
      agy 1.4.12 -p con hook de registro: observados SessionStart, PreInvocation, PostInvocation, PreToolUse, PostToolUse y Stop; PostToolUse sin salida; deny consumido; allow insuficiente en -p; injectSteps con systemMessage de cadena ignorado y con objeto anidado o ephemeralMessage consumido en transcript. Detalle y límites en docs/evidence/hcd015/agy-native.md.
- 2026-11-02 23:27 · tarea 2 · rojo observado · caducada · `go test ./tests/contract -run '^TestHCD015AgyEventFixtures$' -count=1` (salida 1)
      --- FAIL: TestHCD015AgyEventFixtures (0.00s)
          hcd015_agy_events_test.go:12: open ../fixtures/hcd015/events.json: no such file or directory
      FAIL
      FAIL	mem/tests/contract	0.583s
      FAIL
- 2026-11-02 23:27 · tarea 2 · verde · caducada · `go test ./tests/contract -run '^TestHCD015AgyEventFixtures$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.275s
- 2026-11-02 23:28 · tarea 3,4,5 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD015Agy(Input|Output|FailOpen)$' -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/hcd015_agy_test.go:34:11: undefined: planAgyHook
      adapters/primary/cli/hcd015_agy_test.go:56:21: undefined: planAgyHook
      adapters/primary/cli/hcd015_agy_test.go:61:21: undefined: planAgyHook
      adapters/primary/cli/hcd015_agy_test.go:67:14: undefined: planAgyHook
      adapters/primary/cli/hcd015_agy_test.go:73:28: undefined: agyRunner
      adapters/primary/cli/hcd015_agy_test.go:74:23: undefined: agyCall
      adapters/primary/cli/hcd015_agy_test.go:86:20: undefined: agyHookResponse
      adapters/primary/cli/hcd015_agy_test.go:93:23: undefined: agyCall
      adapters/primary/cli/hcd015_agy_test.go:105:10: undefined: agyHookResponse
      adapters/primary/cli/hcd015_agy_test.go:110:25: undefined: agyCall
      adapters/primary/cli/hcd015_agy_test.go:110:25: too many errors
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-02 23:28 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD015AgyInput$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.422s
- 2026-11-02 23:28 · tarea 4 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD015AgyOutput$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.424s
- 2026-11-02 23:28 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD015AgyFailOpen$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.373s
- 2026-11-02 23:29 · tarea 6 · rojo observado · caducada · `go test ./tests/contract -run '^TestHCD015AgyCapabilities$' -count=1` (salida 1)
      --- FAIL: TestHCD015AgyCapabilities (0.00s)
          hcd015_agy_capabilities_test.go:18: agy no figura en domain.KnownAgents
      FAIL
      FAIL	mem/tests/contract	0.394s
      FAIL
- 2026-11-02 23:30 · tarea 6 · verde · caducada · `go test ./tests/contract -run '^TestHCD015AgyCapabilities$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.347s
- 2026-11-02 23:37 · tarea 3 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD015Agy(Input|StopRecordsTurnActivity)$' -count=1` (salida 1)
      --- FAIL: TestHCD015AgyStopRecordsTurnActivity (0.00s)
          hcd015_agy_test.go:107: actividad del turno: {Files:[] Commands:[] Transcript:/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestHCD015AgyStopRecordsTurnActivity2853476918/001/transcript_full.jsonl}
      FAIL
      FAIL	mem/adapters/primary/cli	0.444s
      FAIL
- 2026-11-02 23:37 · tarea 4 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD015AgyOutput$' -count=1` (salida 1)
      --- FAIL: TestHCD015AgyOutput (0.00s)
          hcd015_agy_test.go:115: el sobre de Claude llega al modelo: {"injectSteps":[{"ephemeralMessage":"{\"hookSpecificOutput\":{\"hookEventName\":\"UserPromptSubmit\",\"additionalContext\":\"contexto real\"}}\n\n{\"hookSpecificOutput\":{\"hookEventName\":\"UserPromptSubmit\",\"additionalContext\":\"contexto real\"}}"}]}
      FAIL
      FAIL	mem/adapters/primary/cli	0.598s
      FAIL
- 2026-11-02 23:40 · tarea 2 · verde · caducada · `go test ./tests/contract -run '^TestHCD015AgyEventFixtures$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.360s
- 2026-11-02 23:40 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD015Agy(Input|StopRecordsTurnActivity)$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.383s
- 2026-11-02 23:40 · tarea 4 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD015AgyOutput$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.288s
- 2026-11-02 23:40 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD015AgyFailOpen$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.340s
- 2026-11-02 23:40 · tarea 6 · verde · caducada · `go test ./tests/contract -run '^TestHCD015AgyCapabilities$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.462s
- 2026-11-02 23:40 · tarea 7 · verde · caducada · `go test ./adapters/primary/setup -run '^TestHCD015InstallAgy$' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.208s
- 2026-11-02 23:40 · tarea 8 · verde · caducada · `go test ./adapters/primary/setup -run '^TestHCD015MigrateManualGroup$' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.486s
- 2026-11-02 23:40 · tarea 9 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD015DoctorAgy$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.396s
- 2026-11-02 23:40 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli -run '^TestHCD015UninstallAgy$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.312s
- 2026-11-02 23:40 · tarea 11 · verde · caducada · `go test ./tests/contract -run '^TestHCD015AgentSurfaces$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.259s
- 2026-11-02 23:40 · tarea 1 · manual · caducada
      agy 1.4.12 -p: eventos, respuestas y discrepancia de systemMessage en docs/evidence/hcd015/agy-native.md.
- 2026-11-02 23:40 · tarea 12 · manual · caducada
      agy 1.4.12 -p real con build 42b7fef5: instalación en HOME temporal, contexto citado por el modelo, deny de git reset --hard, una inyección por turno, checkpoint de Stop y mem doctor. Sin instalar en el perfil real; límites en docs/evidence/hcd015/agy-session.md.
- 2026-11-02 23:40 · tarea 2 · verde · vigente · `go test ./tests/contract -run '^TestHCD015AgyEventFixtures$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.272s
- 2026-11-02 23:40 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD015Agy(Input|StopRecordsTurnActivity)$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.299s
- 2026-11-02 23:40 · tarea 4 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD015AgyOutput$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.296s
- 2026-11-02 23:40 · tarea 5 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD015AgyFailOpen$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.296s
- 2026-11-02 23:40 · tarea 6 · verde · vigente · `go test ./tests/contract -run '^TestHCD015AgyCapabilities$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.249s
- 2026-11-02 23:40 · tarea 7 · verde · vigente · `go test ./adapters/primary/setup -run '^TestHCD015InstallAgy$' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.146s
- 2026-11-02 23:40 · tarea 8 · verde · vigente · `go test ./adapters/primary/setup -run '^TestHCD015MigrateManualGroup$' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.168s
- 2026-11-02 23:40 · tarea 9 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD015DoctorAgy$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.292s
- 2026-11-02 23:40 · tarea 10 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD015UninstallAgy$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.307s
- 2026-11-02 23:40 · tarea 11 · verde · vigente · `go test ./tests/contract -run '^TestHCD015AgentSurfaces$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.293s
- 2026-11-02 23:40 · tarea 1 · manual · vigente
      agy 1.4.12 -p: eventos, respuestas y discrepancia de systemMessage en docs/evidence/hcd015/agy-native.md.
- 2026-11-02 23:40 · tarea 12 · manual · vigente
      agy 1.4.12 -p real con build 42b7fef5: instalación en HOME temporal, contexto citado por el modelo, deny de git reset --hard, una inyección por turno, checkpoint de Stop y mem doctor. Sin instalar en el perfil real; límites en docs/evidence/hcd015/agy-session.md.

## Siguiente
Cerrada con aprobación sobre la revisión de2c99beb179.
