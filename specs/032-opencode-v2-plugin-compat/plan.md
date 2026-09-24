# Implementation Plan: Compatibilidad del plugin de OpenCode con v2 sin romper v1.18

**Branch**: `032-opencode-v2-plugin-compat` (sin rama creada: se trabaja sobre `main`) | **Date**: 2026-09-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/032-opencode-v2-plugin-compat/spec.md`

## Summary

OpenCode 2.x rechaza `gomemory.ts` porque le falta un `export default { id, setup }`.
La investigación, hecha contra binarios reales (2.0.16, 1.18.32, 1.18.20 y 1.17.0),
llevó a una solución más pequeña de lo que suponía el spec:

1. **Un solo archivo dual**: `export default { id: "gomemory", setup, server }`,
   manteniendo además `export const GomemoryPlugin`. v2 llama a `setup`; toda
   la rama 1.x probada (desde 1.17.0) llama a `server`, y ninguna versión
   carga el plugin dos veces. **No hace falta** una variante solo v1 ni
   detectar la versión en el instalador (R-3).
2. **Núcleo compartido**: la lógica que hoy vive dentro de la fábrica v1
   (qué llamar a `mem` y cuándo) se extrae a un `GomemoryCore` que usan ambas
   ramas. Solo cambian el runner de `mem` (`$` en v1, `execFile` en v2) y
   el cableado de ganchos (R-4, R-5).
3. **Paridad en v2**: todas las capacidades tienen equivalente, incluida la
   compactación: `session.hook("compaction")` y el evento
   `session.compaction.ended`. Esto cierra el riesgo principal del spec.
   Dos lecturas de mensajes (resumen C6 y texto del modo plan) quedan como
   best-effort declarado (R-7).
4. **Sin cambios en `opencode.json`**: v2 traduce sola `mcp` y `permission`
   del formato v1 (R-8).
5. **Diagnóstico e higiene**: `mem doctor` informa la versión, la forma del
   plugin y los plugins ajenos con forma v1 (`cbm-augment.ts`). El instalador
   deja de copiar `gomemory.test.mjs` a la carpeta de plugins, un defecto
   previo que salió en la investigación (R-9).

## Technical Context

**Language/Version**: Go 1.27 (módulo `mem`, binario de `infrastructure/`); TypeScript del plugin ejecutado por el runtime de OpenCode (Bun en 1.x; Bun o Node en 2.x)

**Primary Dependencies**: ninguna nueva. El plugin solo usa `import type` y `node:child_process`

**Storage**: N/A (sin cambios en SQLite)

**Testing**: `go test` (contratos en `tests/contract/`, pruebas junto al código); `node --test infrastructure/plugin/opencode/gomemory.test.mjs` para el plugin; validación contra OpenCode real ([quickstart.md](quickstart.md))

**Target Platform**: OpenCode ≥ 1.17.0 (verificado) y 2.x (verificado con 2.0.16) en Linux/macOS/Windows

**Project Type**: CLI + servidor MCP con plugins por agente

**Performance Goals**: sin regresión. El hook `context` de v2 hace las mismas llamadas a `mem` que el `system.transform` de v1

**Constraints**: el plugin no puede requerir `npm install` en `~/.config/opencode/plugins/`; best-effort ante la falta de `mem`; el comportamiento en 1.x no cambia

**Scale/Scope**: 1 archivo TS (~475 líneas hoy), 1 test JS, instalador de OpenCode, doctor y 3 contratos Go

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Evaluación | Estado |
|-----------|------------|--------|
| I. Hexagonal | El plugin es un adaptador primario de infraestructura: solo traduce eventos de OpenCode a llamadas a `mem`, y la lógica sigue en Go (FR-006). La inspección de la forma del plugin para el doctor va en `adapters/primary/setup/` (junto a `activation_inspect.go`); el doctor solo la presenta. | ✅ |
| II. SQLite con SQL directo | Sin SQL. | ✅ |
| III. Testing first | Cada hoja de código empieza por su prueba en rojo: contratos Go sobre el texto del plugin, `node --test` con un `ctx` v2 falso y pruebas del instalador y del doctor. Se **modifican** 2 contratos existentes (`opencode_compaction_test.go`, `opencode_hooks_test.go`) porque recortan bloques por nombre de gancho v1; se amplían para exigir también el gancho v2 y no pierden ninguna aserción. | ✅ |
| IV. Configuración | Sin ajustes nuevos. El `id` `"gomemory"` y la tabla de nombres de tools son constantes del contrato con OpenCode. | ✅ |
| V. Operativos | Simplicidad: un archivo dual en vez de dos variantes más detección de versión. Sin parches: el archivo de pruebas se corrige en su causa (filtro del instalador). Idempotencia: la reinstalación no reescribe si nada cambia (se conserva el criterio de `InstallPlugin`). | ✅ |
| Documentación en español | Artefactos en español; identificadores en inglés. | ✅ |
| Prohibiciones | Nada se cachea en proceso salvo `pendingRecovery` y el acumulador de turno, que ya son estado efímero por sesión, con el mismo patrón que v1. | ✅ |

**Re-check post-diseño (Fase 1)**: sin cambios. Complexity Tracking queda vacío.

## Project Structure

### Documentation (this feature)

```text
specs/032-opencode-v2-plugin-compat/
├── spec.md
├── plan.md              # este archivo
├── research.md          # R-1..R-11, verificados contra binarios reales
├── data-model.md
├── quickstart.md        # validación contra OpenCode 1.x y 2.x reales
├── contracts/
│   └── opencode-plugin.md
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```text
infrastructure/plugin/opencode/
├── gomemory.ts                 # MODIFICA: GomemoryCore + rama v1 (server) + rama v2 (setup) + export default
└── gomemory.test.mjs           # MODIFICA: pruebas de la rama v2 con ctx falso; las v1 siguen igual

adapters/primary/setup/
├── setup.go                    # MODIFICA: InstallPlugin/copyFileOrDir excluyen *.test.*
├── opencode_setup.go           # MODIFICA: retira gomemory.test.mjs heredado del destino
├── opencode_plugin_inspect.go  # NUEVO: forma del plugin (dual|v1-only|missing), versión, plugins ajenos
└── opencode_plugin_inspect_test.go

adapters/primary/cli/
└── cmd_doctor.go               # MODIFICA: sección OpenCode (contrato C-4)

tests/contract/
├── opencode_hooks_test.go      # MODIFICA: exige export default {id, setup, server} y ganchos v2
├── opencode_compaction_test.go # MODIFICA: recorta también los bloques v2 (compaction, compaction.ended)
└── opencode_v2_shape_test.go   # NUEVO: sin imports de runtime fuera de node:*, id estable

INSTALLATION.md, docs/MANUAL.md, CHANGELOG.md   # MODIFICA: versiones soportadas (FR-013)
```

**Structure Decision**: se mantiene la estructura vigente. Solo se añade un
inspector en `setup/`, que es donde vive `activation_inspect.go`, el análogo
para otros agentes.

## Árbol de tareas atómicas

🎯 gomemory funciona en OpenCode 2.x y en 1.x (≥ 1.17.0) con un solo plugin instalado. Está hecho cuando Q1–Q6 del quickstart pasan contra binarios reales.

```text
├─ [1] Núcleo compartido (refactor sin cambio de comportamiento v1)
│  ├─ [1.1] ✓ Extraer GomemoryCore(runner, root) de la fábrica v1 → gomemory.test.mjs actual pasa sin tocarlo
│  └─ [1.2] ✓ Definir MemRunner v1 sobre `$` (run/mem/memWithStdin actuales) → mismas llamadas registradas en el test   (dep: 1.1)
├─ [2] Rama v2                                                                  (dep: 1)
│  ├─ [2.1] ✓ Escribir test node con ctx v2 falso (event.subscribe, session.hook, tool.hook, location) → rojo
│  ├─ [2.2] ✓ Implementar MemRunner v2 con node:child_process.execFile (cwd, stdin, timeout, null si falla) → test del runner verde
│  ├─ [2.3] ✓ Cablear session.created/idle/deleted/compaction.ended con filtro por location.directory → test verde   (dep: 2.2)
│  ├─ [2.4] ✓ Implementar TurnAccumulator desde tool.hook("execute.after") y vaciarlo en idle → turn-end con {files, commands}   (dep: 2.3)
│  ├─ [2.5] ✓ Cablear session.hook("prompt") → hook prompt con event.prompt.text   (dep: 2.2)
│  ├─ [2.6] ✓ Cablear session.hook("context") → system.push({type:"text"}) con systemParts() y pendingRecovery   (dep: 2.2)
│  ├─ [2.7] ✓ Cablear session.hook("compaction") → system.push(compactionContext())   (dep: 2.2)
│  ├─ [2.8] ✓ Cablear subagent-stop desde execute.after (task, completed, texto de Tool.Result)   (dep: 2.4)
│  ├─ [2.9] ✓ Leer C6 y el texto del modo plan con session.context() normalizado; si la forma es desconocida, channel-error → test verde   (dep: 2.3)
│  └─ [2.10] ✓ Devolver un cleanup que aborta la suscripción y vacía los acumuladores → test verde   (dep: 2.3)
├─ [3] Export dual                                                               (dep: 1, 2)
│  ├─ [3.1] ✓ Escribir tests/contract/opencode_v2_shape_test.go (export default, id "gomemory", setup, server, sin imports de runtime ajenos a node:*) → rojo
│  ├─ [3.2] ✓ Añadir `export default { id, setup, server: GomemoryPlugin }` conservando el export con nombre → 3.1 en verde
│  └─ [3.3] ✓ Ampliar opencode_hooks_test.go y opencode_compaction_test.go con las aserciones v2 → verde sin perder aserciones v1
├─ [4] Instalador (∥ 2, 3)
│  ├─ [4.1] ✓ Test: InstallPlugin omite *.test.* → rojo, luego verde
│  └─ [4.2] ✓ Retirar gomemory.test.mjs heredado en installOpenCodePlugin → test con carpeta temporal verde
├─ [5] Diagnóstico (∥ 2, 3)
│  ├─ [5.1] ✓ Implementar parseo de `opencode --version` (con o sin "v") y la forma del plugin (dual|v1-only|missing) con tests de tabla
│  ├─ [5.2] ✓ Detectar .ts/.js ajenos sin export default cuando Major ≥ 2 → test con fixture cbm-augment.ts   (dep: 5.1)
│  ├─ [5.3] ✓ Pintar la sección OpenCode en mem doctor según C-4 → test de salida   (dep: 5.1, 5.2)
│  └─ [5.4] ✓ Marcar los canales del plugin como rotos si la forma es v1-only en 2.x (FR-005) → test verde   (dep: 5.1)
├─ [6] Verificación contra OpenCode real                                         (dep: 3, 4, 5)
│  ├─ [6.1] ✓ Ejecutar quickstart Q0–Q2 y Q5–Q6 → resultados pegados en la sección de revisión
│  └─ [6.2] ⚠ no atómica → Q3/Q4 requieren credenciales de modelo en un HOME aislado; los ejecuta la persona o un entorno con auth
└─ [7] Documentación y release                                                   (dep: 6.1)
   ├─ [7.1] ✓ Actualizar INSTALLATION.md y docs/MANUAL.md con las versiones soportadas y la actualización 1.x→2.x
   └─ [7.2] ✓ Añadir la entrada al CHANGELOG (fix: compatibilidad OpenCode 2.x)
```

26 hojas: 25 atómicas y 1 declarada no atómica (6.2).

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Nombres de tools v2 (`bash`, `edit`, `write`, `task`) distintos a v1 | Tabla única de nombres en el core; Q3 la valida y, si difieren, se corrige en un solo sitio |
| Forma de `SessionMessageInfo` desconocida (C6, modo plan) | Normalizador tolerante con `channel-error` explícito; C6 ya tenía el respaldo `RecoverySteps` |
| Acción de permisos MCP con otro nombre en v2 | Q4. Si falla, se añade la regla con el nombre v2 sin tocar la de v1 |
| El hook `context` corre en cada llamada al modelo del bucle del agente | Igual que `system.transform` en v1; sin cambio de coste. `channel-fired` sigue siendo fire-and-forget |
| Cargadores 1.x anteriores a 1.17.0 | El export con nombre se conserva; se documenta 1.17.0 como piso verificado |
| Plugins ajenos (`cbm-augment.ts`) siguen mostrando errores | Fuera de alcance; el doctor lo explica y se recomienda un issue en codebase-memory-mcp |

## Agent context update

No aplica: el repo no tiene `.specify/scripts`. El hallazgo principal quedó
registrado en gomemory (memoria 58), que se actualiza con los resultados de este plan.

## Complexity Tracking

Sin violaciones de la constitución.
