---

description: "Tareas de implementación de la feature 032: compatibilidad del plugin de OpenCode con v2 sin romper v1"
---

# Tasks: Compatibilidad del plugin de OpenCode con v2 sin romper v1.18

**Input**: Documentos de diseño en `specs/032-opencode-v2-plugin-compat/`

**Prerequisites**: plan.md, spec.md, research.md (R-1..R-11), data-model.md, contracts/opencode-plugin.md, quickstart.md

**Tests**: OBLIGATORIOS (constitución, principio III: TDD). Cada tarea de test se escribe primero y DEBE fallar antes de su implementación. Solo se modifican dos contratos existentes, `tests/contract/opencode_hooks_test.go` y `tests/contract/opencode_compaction_test.go`, y solo **añadiendo** aserciones (justificado en plan.md, Constitution Check III).

**Organization**: una fase por historia de spec.md (US1…US4). US1 y US2 son P1 y comparten el export dual, que va en Foundational.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede hacer en paralelo (otro archivo, sin depender de tareas incompletas)
- **[Story]**: historia de spec.md (US1…US4)

## Convenciones del repo (aplican a todas las tareas)

- Plugin: `infrastructure/plugin/opencode/gomemory.ts`. Tests del plugin: `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (Node ≥ 22 importa `.ts` quitando los tipos, así que no se puede usar sintaxis TS que no sea borrable: nada de `enum` ni `namespace`).
- **Sin imports de runtime** en el plugin salvo `node:*`. Los tipos van con `import type`.
- **Formato de la rama v1 congelado**: el objeto de ganchos v1 conserva sus claves a 4 espacios (`    "chat.message": async`) y su cierre `\n  };`, porque `blockBetween` y `hooksQueRegistraElComplemento` dependen de ese formato.
- **Delimitadores v2**: cada gancho v2 va precedido de una línea `// v2:<nombre>`, con estos nombres: `event`, `prompt`, `context`, `compaction`, `tool.execute.after` y `cleanup`. Los contratos Go recortan los bloques v2 entre un marcador `// v2:` y el siguiente.
- **Costura de pruebas**: `createV2Setup(execFactory)` se expone solo como `export const __testing = { createV2Setup }`, un objeto y no una función, para que ningún cargador 1.x la ejecute como plugin.
- Go: identificadores en inglés, comentarios en español y solo sobre el "por qué". Tests junto al código.
- Al cerrar cada fase: `gofmt -l .`, `go vet ./...`, `go test ./...` y `node --test infrastructure/plugin/opencode/gomemory.test.mjs`.

---

## Phase 1: Setup (línea base)

- [X] T001 Registrar la línea base: ejecutar `go test ./tests/contract/... ./adapters/primary/setup/... ./adapters/primary/cli/... -count=1` y `node --test infrastructure/plugin/opencode/gomemory.test.mjs`, y anotar el resultado (todo verde esperado) en la sección "Línea base" de este archivo
- [X] T002 [P] Guardar una copia de referencia del plugin v1 actual en `specs/032-opencode-v2-plugin-compat/baseline/gomemory.v1.ts` (copia literal de `git show HEAD:infrastructure/plugin/opencode/gomemory.ts`) para comparar comportamiento en US2

---

## Phase 2: Foundational (bloquea US1–US4)

**Objetivo**: extraer el núcleo compartido sin cambiar el comportamiento v1 y dejar el export dual en su sitio.

- [X] T003 Test en rojo: añadir a `infrastructure/plugin/opencode/gomemory.test.mjs` una prueba que importe `default` y compruebe `default.id === "gomemory"`, `typeof default.setup === "function"` y `default.server === GomemoryPlugin`
- [X] T004 Test en rojo: crear `tests/contract/opencode_v2_shape_test.go` con `TestOpenCodeV2_ExportDefaultDual` (el texto contiene `export default {`, `id: "gomemory"`, `setup`, `server: GomemoryPlugin`, y se conserva `export const GomemoryPlugin`) y `TestOpenCodeV2_SinImportsDeRuntimeAjenos` (toda línea `import … from "x"` sin `import type` tiene `x` con prefijo `node:`)
- [X] T005 **(Rediseñada al implementar; ver "Desvío de diseño")** No se extrae un `createCore`: la fábrica v1 `GomemoryPlugin` queda intacta y la rama v2 la reutiliza como adaptador. Implementar en `infrastructure/plugin/opencode/gomemory.ts` `bunShellShim(exec)`, que emula el `$` de Bun (`$\`${BIN} ${args}\`.cwd(dir).quiet()`, `.stdin.getWriter()` y `.text()`, que rechaza la promesa si el comando falla), sobre una función `exec(bin, args, {cwd, input})`
- [X] T006 Añadir al final de `gomemory.ts` `createV2Setup(execFactory)`, que devuelve `setup(ctx)`, más `export default { id: "gomemory", setup: createV2Setup(execFileExec), server: GomemoryPlugin }` y `export const __testing = { createV2Setup }` → T003 y T004 en verde
- [X] T007 Ejecutar `go test ./tests/contract/... -run OpenCode -count=1` y `node --test …gomemory.test.mjs` → todo verde; si `blockBetween` o `hooksQueRegistraElComplemento` dejan de encontrar ganchos, el formato v1 se rompió y hay que corregir T005, no el test

**Checkpoint**: 1.x sigue igual y el archivo ya tiene la forma que v2 acepta.

---

## Phase 3: User Story 1 - El plugin carga sin error en OpenCode 2.x (Priority: P1) 🎯 MVP

**Goal**: en v2 el plugin carga, abre y cierra sesión, hace el checkpoint al quedar idle e inyecta el protocolo y el contexto.

**Independent Test**: quickstart Q1 (`opencode plugin list` muestra `gomemory` sin `failed to load plugin`) más los tests con `ctx` falso.

### Tests (rojo primero)

- [X] T008 [US1] En `infrastructure/plugin/opencode/gomemory.test.mjs`, crear la fixture `v2Fixture()`. Debe incluir: `ctx` falso con `location.directory = '/proj'`; `event.subscribe({signal})` como iterador asíncrono alimentado por `push(ev)`; `session.hook(name, fn)` y `tool.hook(name, fn)` que guardan los callbacks; `session.context` configurable; y un runner falso que registra `{args, input}`. La fixture llama a `__testing.createV2Setup(() => runner)(ctx)`
- [X] T009 [US1] Tests en rojo en el mismo archivo:
  - (a) `session.created` en `/proj` → `session start`; el mismo evento con `location.directory = '/otro'` → nada.
  - (b) `session.context` devuelve mensajes v2 (`assistant` con tools `write{filePath:'a.go'}` y `bash{command:'ls'}`, ambas `completed`) y llega `session.idle` con `data.sessionID='s1'` → exactamente un `hook turn-end` con `{"files":["a.go"],"commands":["ls"]}`; un segundo idle sin mensajes nuevos → ninguna llamada nueva.
  - (c) el hook `context` añade a `event.system` partes `{type:'text', text}` con el contexto (`context`) y el nudge.
  - (d) el cleanup aborta la suscripción, y los eventos posteriores no llaman a `mem`.

### Implementación

- [X] T010 [US1] Implementar `execFileExec(bin, args, {cwd, input})` con `node:child_process.execFile` (timeout de 30 s y `input` escrito en stdin), que resuelve stdout y rechaza si hay error; es la `execFactory` por defecto
- [X] T011 [US1] En `createV2Setup`: construir los ganchos v1 con `GomemoryPlugin({ $: bunShellShim(exec), directory: ctx.location.directory, client: clientShim(ctx) })`. En el bloque `// v2:event`, suscribirse con `ctx.event.subscribe({signal})`, descartar los eventos de otro `location.directory` y traducir `session.created`, `session.idle` y `session.deleted` (`data.sessionID`) a la forma v1 `{event:{type, properties:{sessionID, info:{id}}}}` → T009a en verde
- [X] T012 [US1] Implementar `clientShim(ctx).session.messages({path:{id}})` → `{data: normalizeMessagesV2(await ctx.session.context({sessionID:id}))}`. `normalizeMessagesV2` traduce `assistant` a `{info:{id, role:"assistant", mode: agent}, parts}` (tool → `{type:"tool", tool:name, state:{status, input}}`), `compaction` completada a `{info:{id, summary:true}, parts:[{type:"text", text:summary}]}`, `user` a `{info:{id, role:"user"}, parts:[{type:"text", text}]}` y el resto a partes vacías → T009b en verde (turn-end sale del escaneo v1 existente)
- [X] T013 [US1] Implementar el bloque `// v2:context`: `ctx.session.hook("context", ev => …)` llama al gancho v1 `experimental.chat.system.transform` con `{sessionID}` y `out={system:[]}`, y hace `ev.system.push({type:"text", text})` por cada cadena → T009c en verde
- [X] T014 [US1] Implementar el bloque `// v2:cleanup`: `setup` devuelve una función que aborta la suscripción y llama al `dispose` v1 → T009d en verde
- [X] T015 [US1] Añadir a `tests/contract/opencode_v2_shape_test.go` `TestOpenCodeV2_GanchosRegistrados`, que exige los marcadores `// v2:event`, `// v2:context` y `// v2:tool.execute.after` y las cadenas `ctx.event.subscribe`, `session.hook("context"` y `tool.hook("execute.after"`

**Checkpoint**: US1 cumplida en tests; la validación real va en T036.

---

## Phase 4: User Story 2 - Quien sigue en OpenCode 1.18 no nota ningún cambio (Priority: P1)

**Goal**: la rama v1 es byte a byte la de antes en comportamiento.

**Independent Test**: los 3 tests v1 existentes del `.mjs` pasan sin modificarlos, más quickstart Q2 con 1.17.0 y 1.18.32.

- [X] T016 [P] [US2] **(Reemplazada por una garantía más fuerte)** `TestOpenCodeV2_FabricaV1IdenticaALaReferencia` en `tests/contract/opencode_v2_shape_test.go` exige que la fábrica v1 sea texto a texto la de `baseline/gomemory.v1.ts`. Descripción original: con la fixture v1 existente, recorrer un guion fijo (created, prompt, transform, idle con mensajes, compacted, transform, task after, compacting) y comparar la lista de `calls` contra la esperada, escrita a mano a partir del comportamiento de `baseline/gomemory.v1.ts`
- [X] T017 [US2] No hubo diferencias que corregir: la fábrica v1 no se tocó. Descripción original: corregir en `gomemory.ts` cualquier diferencia que T016 detecte en la rama v1 (orden de llamadas, payloads) hasta que pase, sin tocar el test
- [X] T018 [P] [US2] Añadir a `tests/contract/opencode_v2_shape_test.go` `TestOpenCodeV2_RamaV1ConservaGanchos`: `hooksQueRegistraElComplemento` sigue devolviendo exactamente `chat.message`, `experimental.chat.system.transform`, `experimental.session.compacting` y `tool.execute.after` (más `event`, que no lleva comillas)

---

## Phase 5: User Story 3 - Paridad de capacidades en v2, o degradación declarada (Priority: P2)

**Goal**: procedencia del prompt, subagentes, compactación (C1/C2/C6) y texto del modo plan en v2.

**Independent Test**: tests con `ctx` falso más quickstart Q3 (con credenciales).

### Tests (rojo primero)

- [X] T019 [US3] Tests en rojo en `gomemory.test.mjs` (fixture v2):
  - (a) el hook `prompt` con `ev.prompt.text = 'hola'` → `hook prompt` con stdin `{"prompt":"hola"}`; un texto vacío → nada.
  - (b) el hook `compaction` → `ev.system` recibe `{type:'text', text:<salida de hook compaction-context>}`.
  - (c) `session.compaction.ended` para s1 → `hook post-compact`; el siguiente `context` de s1 incluye la recuperación una sola vez y el de s2 no.
  - (d) `execute.after` con `tool:'task', status:'completed', result:{output:'X'}` → `hook subagent-stop` con `{"last_assistant_message":"X"}`.
  - (e) `session.context` devuelve una forma no reconocida → `hook channel-error opencode user …` y ningún `compact-summary`.

### Implementación

- [X] T020 [US3] Implementar el bloque `// v2:prompt`: `ctx.session.hook("prompt")` → gancho v1 `chat.message` con `{parts:[{type:"text", text: ev.prompt?.text ?? ""}]}` → T019a en verde
- [X] T021 [US3] Implementar el bloque `// v2:compaction`: `ctx.session.hook("compaction")` → gancho v1 `experimental.session.compacting` con `out={context:[]}`, y cada cadena va a `ev.system.push({type:"text", text})` → T019b en verde
- [X] T022 [US3] En `// v2:event`, traducir `session.compaction.ended` a `session.compacted` v1 (con `properties.sessionID`) → T019c en verde
- [X] T023 [US3] Implementar el bloque `// v2:tool.execute.after`: `ctx.tool.hook("execute.after")` con `status === "completed"` → gancho v1 `tool.execute.after` con `({tool}, {output: resultText(ev.result)})`, donde `resultText` toma `content` (texto o partes `text`) o, en su defecto, `output` → T019d en verde
- [X] T024 [US3] Hacer que `normalizeMessagesV2` lance un error ante una respuesta que no sea un arreglo, para que el `catch` v1 existente emita `channel-error` (FR-005) en lugar de un checkpoint vacío silencioso → T019e en verde
- [X] T025 [P] [US3] Ampliar `tests/contract/opencode_compaction_test.go` con funciones nuevas al final: `TestOpenCodeCompactionV2_CompactionPideCompactionContext` (bloque `// v2:compaction` contiene `["hook", "compaction-context"]` y no contiene `"post-compact"`) y `TestOpenCodeCompactionV2_CompactionEndedLlamaPostCompact` (bloque `// v2:event` contiene `"session.compaction.ended"` y `["hook", "post-compact"]`), con un helper `v2Block(t, texto, nombre)` que recorta entre `// v2:<nombre>` y el siguiente `// v2:`
- [X] T026 [P] [US3] Ampliar `tests/contract/opencode_hooks_test.go` con `TestOpenCodeV2_ObtieneLaPoliticaOctopus`: `createCore` contiene `["hook", "octopus-delegation-policy", "opencode"]`. Se conserva intacta la aserción v1 `output.system.push(octopusPolicy)`

---

## Phase 6: User Story 4 - El diagnóstico explica qué versión y qué plugin fallan (Priority: P3)

**Goal**: `mem doctor` informa la versión, la forma del plugin y los plugins ajenos, y el instalador deja de copiar el archivo de pruebas.

**Independent Test**: tests Go de tabla más quickstart Q5 y Q6.

### Tests (rojo primero)

- [X] T027 [P] [US4] Crear `adapters/primary/setup/opencode_plugin_inspect_test.go` con tests de tabla:
  - `ParseOpenCodeVersion`: `"opencode v2.0.16"`→(2,"2.0.16"), `"1.18.32"`→(1,"1.18.32"), `""`→(0,"").
  - `DetectPluginShape`: un fixture dual → `dual`, el texto de `baseline/gomemory.v1.ts` → `v1-only`, un archivo ausente → `missing`.
  - `ForeignV1Plugins`: un directorio temporal con `cbm-augment.ts` sin `export default` y `ok.ts` con él → solo `cbm-augment.ts`; con Major 1 → vacío.
- [X] T028 [P] [US4] En `adapters/primary/setup/setup_test.go`, crear el test `TestInstallPlugin_OmiteArchivosDePrueba`: un `fstest.MapFS` con `plugin/opencode/gomemory.ts` y `gomemory.test.mjs` → en el destino solo queda `gomemory.ts`
- [X] T029 [P] [US4] En `adapters/primary/setup/opencode_global_test.go`, crear un test que siembre `gomemory.test.mjs` en `~/.config/opencode/plugins/` (HOME temporal), ejecute `installOpenCodePlugin` y compruebe que ese archivo desaparece mientras `cbm-augment.ts` sigue intacto
- [X] T030 [P] [US4] Test de salida del doctor en un archivo nuevo `adapters/primary/cli/cmd_doctor_opencode_test.go` (mismo estilo que `cmd_doctor_compaction_test.go`): con un `OpenCodeInstallStatus` inyectado, las líneas coinciden con contracts/opencode-plugin.md C-4 en los casos dual/v2, v1-only/v2, versión desconocida, plugin ajeno y artefacto de pruebas

### Implementación

- [X] T031 [US4] Crear `adapters/primary/setup/opencode_plugin_inspect.go` con `ParseOpenCodeVersion(out string) (major int, version string)`, `DetectPluginShape(path string) string`, `ForeignV1Plugins(dir string, major int) []string` e `InspectOpenCode() OpenCodeInstallStatus` (ejecuta `opencode --version` con un timeout de 5 s y absorbe el error) → T027 en verde
- [X] T032 [US4] En `adapters/primary/setup/setup.go`, hacer que `copyFileOrDir`/`InstallPlugin` omitan los nombres que contienen `.test.` → T028 en verde
- [X] T033 [US4] En `adapters/primary/setup/opencode_setup.go` (`installOpenCodePlugin`), junto a la limpieza de `plugins/gomemory/`, añadir `os.Remove(filepath.Join(pluginsDir, "gomemory.test.mjs"))` con un comentario sobre el porqué (research R-9) → T029 en verde
- [X] T034 [US4] En `adapters/primary/cli/cmd_doctor.go`, añadir la sección OpenCode que pinta `InspectOpenCode()` según C-4. Si la forma es `v1-only` y Major ≥ 2, los canales `opencode/user/plan_entry` y `turn_reminder` se reportan rotos con la causa "plugin v1 en OpenCode 2.x" (FR-005) → T030 en verde

---

## Phase 7: Polish & validación contra el sistema real

- [X] T035 Ejecutar `gofmt -l .`, `go vet ./...`, `go test ./... -count=1` y `node --test infrastructure/plugin/opencode/gomemory.test.mjs` → todo verde, con la salida pegada en "Evidencia"
- [X] T036 Ejecutar quickstart Q1, Q2, Q5 y Q6 contra binarios reales (`@opencode/cli@2.0.16`, `opencode-ai@1.17.0` y el `opencode` 1.18.32 local, cada uno con un HOME aislado) y pegar la salida en "Evidencia". Si Q1 muestra `failed to load plugin` para gomemory, la feature no está hecha
- [X] T037 Ejecutar quickstart Q3 y Q4. **Se pudo ejecutar aquí**: OpenCode 2.0.16 trae un modelo gratuito por defecto. Si los nombres de tools v2 difieren, corregir solo `TOOL_NAMES` en `gomemory.ts`; si `gomemory_*` no aplica como acción de permiso en v2, añadir la regla con el nombre v2 en `adapters/primary/setup/opencode_setup.go` sin quitar la v1
- [X] T038 [P] Actualizar `INSTALLATION.md` y `docs/MANUAL.md`: versiones soportadas (1.17.0+ verificada, 2.x), que no hay pasos extra al pasar de 1.x a 2.x (basta `mem setup-mcp --scope global --agents opencode` si el doctor lo pide), y la nota sobre `cbm-augment.ts`
- [X] T039 [P] Añadir la entrada en `CHANGELOG.md` (sección sin publicar): "fix(opencode): plugin compatible con OpenCode 2.x manteniendo 1.x; el instalador ya no copia gomemory.test.mjs; mem doctor informa la versión y la forma del plugin"
- [X] T040 Guardar en gomemory (`save_memory`, topic_key `opencode-v2-plugin-compat`) el resultado de la validación real y los nombres de tools v2 confirmados

---

## Dependencies & Execution Order

### Phase Dependencies

- Setup (T001–T002) → Foundational (T003–T007) → US1, US2, US3 y US4.
- US3 depende de US1 (reusa `v2Fixture`, el bloque `// v2:event` y el acumulador).
- US2 y US4 solo dependen de Foundational.
- Polish depende de todas las historias.

### Conflictos de archivo (serializar aunque las historias sean independientes)

- `gomemory.ts`: T005, T006, T010–T014, T017 y T020–T024 van en serie.
- `gomemory.test.mjs`: T003, T008, T009, T016 y T019 van en serie.
- `tests/contract/opencode_v2_shape_test.go`: T004, T015 y T018 van en serie.

### Within Each User Story

Test en rojo → implementación → test en verde → checkpoint de la fase.

## Parallel Example: tras Foundational

```text
Hilo A (plugin):  T008 → T009 → T010 … T015 → T019 … T024
Hilo B (Go):      T027 ∥ T028 ∥ T029 ∥ T030 → T031 → T032 → T033 → T034
Hilo C (contratos): T018, T025, T026 (cuando existan los marcadores v2)
```

## Implementation Strategy

### MVP First (US1 + US2)

Foundational + US1 + US2 cierran el incidente reportado: el plugin carga en v2 y 1.x no cambia. Con eso se puede publicar un parche validado con T036 (Q1/Q2).

### Incremental Delivery

1. MVP (T001–T018) → publicar el parche.
2. US3 (T019–T026) → paridad completa en v2.
3. US4 (T027–T034) → diagnóstico e higiene del instalador.
4. Polish (T035–T040).

### Reglas de la persona que aplican durante la ejecución

- "Verde en tests" no es "funciona": T036 contra binarios reales es obligatorio antes de decir "listo".
- Todo hallazgo nuevo se cierra en el mismo cambio (regla 7).

## Desvío de diseño (2026-09-24, al implementar)

plan.md y data-model.md §3 proponían extraer un `GomemoryCore` compartido. Al implementar se comprobó que 7 aserciones de contrato existentes (`opencode_compaction_test.go` y `opencode_hooks_test.go`) exigen las llamadas literales a `mem` **dentro** de cada gancho v1, así que extraerlas obligaba a reescribir esos contratos. La rama v2 pasa a ser un **adaptador** sobre la fábrica v1 intacta: le inyecta un `$` hecho sobre `execFile` y un `client` hecho sobre `ctx.session.context`, y traduce cada gancho y evento v2 al v1. El resultado es el mismo contrato C-2, con cero lógica duplicada, la rama v1 byte a byte igual (FR-002) y ningún test existente modificado. También desaparece el `TurnAccumulator`: el checkpoint de turno sale del escaneo de mensajes v1 existente sobre los mensajes v2 normalizados, cuya forma (`assistant.content[].tool{name,state}`, `compaction.summary`) se verificó en `@opencode/client@2.0.16`.

## Línea base

2026-09-24, antes de cambiar código: `go test ./tests/contract/... ./adapters/primary/setup/... ./adapters/primary/cli/... -count=1` → `ok` en los 3 paquetes (contract 93.7s, setup 0.2s, cli 24.1s); `node --test …gomemory.test.mjs` → 5/5 pass.

## Evidencia

Todo contra binarios reales en un `HOME` aislado del scratchpad, con `mem` compilado del árbol y envuelto en un script que registra cada invocación.

**T035**: `gofmt -l` vacío (fuera de `specs/`); `go vet ./...` limpio; `go test ./... -count=1` → 13 paquetes `ok`, 0 fallos; `node --test gomemory.test.mjs` → 21/21.

**Q1 (v2.0.16)**: `opencode plugin list` → `gomemory  local  …/plugins/gomemory.ts`; ninguna línea `failed to load plugin` de gomemory (antes: `Missing key at ["default"]`).

**Q3 (v2.0.16, modelo gratuito por defecto)**, llamadas a `mem` observadas:
- `session.create` (con `location` = proyecto) → `session start`, con `cwd` = proyecto.
- Prompt → `hook prompt`, y en cada llamada al modelo `hook channel-fired … plan_entry`, `hook octopus-delegation-policy opencode`, `context`, `hook nudge` y `hook agent-notice`.
- Turno con `write` + `shell` → memoria `Checkpoint automático — Editó: chau.txt. Comandos: echo listo`.
- Subagente (`subagent`) → `hook subagent-stop`.
- `session.compact` → `hook compaction-context` (gancho `compaction`), `hook compact-summary` (resumen leído de `ctx.session.context`) y `hook post-compact` (`session.compaction.ended`).

**Q4 (v2.0.16)**: pedir `forget_memory` dejó un permiso pendiente con `action: "gomemory_forget_memory"`; la regla `ask` se respeta también cuando el agente invoca la tool vía `execute` (codemode).

**Q2 (1.18.32 real, `opencode run`)**: el plugin nuevo y el original (`baseline/gomemory.v1.ts`) producen **exactamente** las mismas llamadas: `session start`, 3× (`channel-fired`, `octopus-delegation-policy`, `context`, `nudge`, `agent-notice`) y `session end`. No hay regresión. La carga por `server()` también se verificó en 1.17.0 y 1.18.20 (research R-3).

**Q5**: `mem doctor` con el plugin v1 original sobre v2 → `solo v1 ❌ → ejecuta mem setup-mcp …`, el aviso de `cbm-augment.ts` y el del artefacto `gomemory.test.mjs`; `--strict` → rc=1. Tras reinstalar → `dual (v1+v2) ✅`; `gomemory.test.mjs` retirado y `cbm-augment.ts` intacto.

**Q6**: dos reinstalaciones seguidas → mismos sha1 de `opencode.json` y `gomemory.ts`.

### Defectos que solo aparecieron contra el binario real (corregidos en el mismo cambio)

1. **OpenCode 2.0.16 no emite `session.idle`**: el fin de turno es `session.execution.succeeded|failed|interrupted` (sin `location`). Con el mapeo del plan, el checkpoint nunca corría en v2. Se traducen a `session.idle`; es idempotente por el marcador de mensajes v1.
2. **Nombres de tools distintos en v2**: `shell` (antes `bash`) y `subagent` (antes `task`), leídos del gancho `context`. Sin traducción, los comandos no llegaban al checkpoint y la captura de subagentes no se disparaba. Se añadió `V1_TOOL_NAMES`.
3. **El instalador copiaba `gomemory.test.mjs`** a la carpeta de plugins (R-9). Confirmado en la instalación real.

### Observación previa, fuera del alcance

En OpenCode 1.18.32 con `opencode run` (no interactivo), ni el plugin original ni el nuevo reciben `chat.message` ni `session.idle` antes del `dispose`, así que no hay `hook prompt` ni checkpoint en ese modo. Es comportamiento previo de 1.x, no introducido aquí; en la TUI esos ganchos sí se disparan.

## Notes

- Total: 40 tareas (Setup 2, Foundational 5, US1 8, US2 3, US3 8, US4 8, Polish 6).
- T037 no es atómica: depende de credenciales de modelo (plan 6.2).

## Revisión ACR (2026-09-24)

Veredicto APPROVED (0 confirmados, 3 sospechas; independencia degradada: mismo modelo). Las tres sospechas se verificaron contra el código, eran reales y se corrigieron con TDD:

1. `OpenCodeInstallStatus.Compatible()` daba por compatible un plugin solo v1 cuando no se detectaba la versión (`Major 0 < 2`). Ahora solo la forma dual es compatible sin versión conocida; el doctor muestra `solo v1 ❌ (OpenCode 2.x no lo carga) → …`. Test: `TestCompatible_VersionDesconocida`, `TestPrintDoctorOpenCode_V1SinVersionNoEsVerde`.
2. La salida humana de `mem doctor` contaba `report.Problems()` sin el problema de OpenCode y podía decir "✅ Sin problemas" mientras `--strict` salía con 1. `printDoctorHuman` y `printDoctorRemedies` reciben ahora el total que calcula `CmdDoctor`. Test: `TestPrintDoctorHuman_CuentaElProblemaDeOpenCode`. Verificado con el binario: salida humana = `--json` = 12 problemas y `--strict` rc=1, con y sin OpenCode en el PATH.
3. CHANGELOG: queda explícito que los plugins ajenos solo se avisan y no cuentan para `--strict`.

Batería completa tras los cambios: gofmt y vet limpios, 13 paquetes `ok`, node 21/21.

## Revisión ACR 2 (acr_1f27e836, 2026-09-24): ESCALATED → corregida

Los dos CONFIRMED y la SUSPECT se verificaron contra el código y OpenCode real antes de corregir.

- **C-001 HIGH, aislamiento entre proyectos.** Reproducido con OpenCode 2.0.16 real y tres ubicaciones en un mismo servicio: la instancia de projB ejecutó `hook turn-end` para la sesión de projA (y guardó `Editó: a.txt` en la memoria de projB), y la instancia de `proj` ejecutó incluso los ganchos `context`/`nudge` de esa sesión. Era más grave de lo que decía la ACR: los **ganchos** de sesión también se reparten entre todas las instancias, no solo los eventos sin `location`. Fix: `ownsSession` resuelve el directorio dueño de cada sesión (del evento, o de `ctx.session.get().location.directory`), lo recuerda y se aplica en el bloque `event` y en los ganchos `prompt`, `context`, `compaction` y `tool.execute.after`; una sesión que no se puede resolver se ignora. Verificado con el binario: tras el fix, todas las llamadas del turno de projA salen con `cwd` de projA y projB no recibe nada. Tests: 3 JS nuevos más `TestOpenCodeV2_GanchosFiltranPorDueñoDeSesion`. Además se corrigió un fallo del fixture: `emit(…, undefined)` activaba el valor por defecto `/proj`, así que las pruebas "sin location" no lo eran; ahora `null` significa sin `location`.
- **C-002 HIGH, canales en `ok` con un plugin que no carga.** T034 estaba marcada sin implementar la parte de canales; fue un error de ejecución. Fix: `ActivationInspector.openCodePluginProblem` marca `outdated` los canales `plan_entry` y `turn_reminder` de opencode/user cuando el plugin es solo v1 y la versión detectada no lo carga (o no se detecta), con la causa en `detail`. La versión se consulta como mucho una vez y solo si el plugin es v1. Tests: `TestActivationInspect_OpenCodePluginV1EnV2EsOutdated` (4 casos) y el contrato de extremo a extremo `TestDoctor_PluginV1EnOpenCode2MarcaCanalesYCuentaProblems` (con un `opencode` falso en el PATH), que falla si se neutraliza el fix.
- **C-003 MEDIUM, `problems` fuera del contrato 019.** Desaparece con C-002: se quitó el `problems++` y `problems` vuelve a ser `report.Problems()`. El parámetro extra de `printDoctorHuman` que había añadido la ACR 1 sobra y se retiró. El contrato 019 documenta la sección opcional `opencode`.

Verificación final con el binario (plugin v1 y OpenCode 2.0.16 en el PATH): `13 problema(s)` = 13 canales con problema, los dos canales de opencode en ❌ con la causa y el remedio agrupado. Batería completa: gofmt y vet limpios, 13 paquetes `ok`, node 24/24.
