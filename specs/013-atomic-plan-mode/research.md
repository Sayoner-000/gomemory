# Investigación — Fase 0: Modo Plan Atómico con Memoria

**Feature**: 013-atomic-plan-mode
**Fecha**: 2026-08-06

Esta fase resuelve los tres puntos que el checklist de `spec.md` marcó para resolver
temprano, más las decisiones de diseño que se derivan de ellos. Todo lo afirmado aquí se
verificó leyendo el código actual del repositorio, no de memoria.

---

## D1 — ¿Los agentes admiten configuración de usuario además de la de proyecto?

**Pregunta abierta #1 del checklist.** Condiciona la Historia 3 completa (FR-024 a FR-026).

**Decisión**: Sí, y el proyecto ya lo explota. El ámbito global es viable sin
infraestructura nueva.

**Verificación realizada**:

- `adapters/primary/cli/cmd_mcp_setup.go:25` declara `globalScopeAgents` con `claude`,
  `codex` y `opencode` en `true`. El comentario del mismo archivo (líneas 21-24) deja
  constancia de que el comportamiento de OpenCode **se confirmó empíricamente** con
  `opencode debug config`: mergea `~/.config/opencode/opencode.json` (ámbito usuario) con
  el `opencode.json` del proyecto.
- `adapters/primary/setup/opencode_setup.go:32` implementa `InstallOpenCodeGlobal`, y
  `openCodeGlobalConfigPath()` (línea 48) resuelve `~/.config/opencode/opencode.json`.
- `setupClaudeGlobal` (`cmd_mcp_setup.go:203`) registra en el ámbito de usuario de Claude
  Code delegando en el propio CLI `claude mcp add`, en lugar de escribir el archivo a
  mano.
- El plugin de OpenCode ya se instala siempre en `~/.config/opencode/plugins/`, es decir,
  ya es global por naturaleza (`installOpenCodePlugin`, comentario en la línea 96).

**Consecuencia**: FR-024 a FR-026 se sostienen. El trabajo del ámbito global no es
"construir soporte global", sino **extender la ruta global existente** para que además de
registrar el servidor MCP escriba el bloque de protocolo en el archivo de instrucciones de
usuario de cada agente.

**Alternativas descartadas**:

- *Solo ámbito por proyecto*: descartada por decisión explícita del usuario en
  `/speckit-specify`.
- *Inventar un mecanismo global propio de gomemory* (un archivo central que los agentes
  leyeran): descartada — ningún agente lee un archivo arbitrario; hay que usar las
  ubicaciones que cada uno ya carga.

**Riesgo residual**: Cursor, Windsurf y Cline **no** están en `globalScopeAgents`; el
comentario del código indica que no tienen un ámbito de usuario equivalente. Para ellos,
la cobertura llega por ámbito de proyecto (su archivo de reglas, que `mem install` ya
escribe). Queda documentado, no bloquea nada.

---

## D2 — ¿Cómo se define "detectar intención de planificación" (FR-004)?

**Pregunta abierta #2 del checklist.** Era el requisito con más margen de interpretación.

**Decisión**: No se construye ningún detector. El disparador se expresa como una condición
declarativa en el texto del protocolo, con tres formas equivalentes, y es el agente quien
la evalúa:

1. El agente entra en un modo plan nativo (Claude Code, OpenCode).
2. La persona invoca explícitamente un comando de planificación.
3. La solicitud pide un plan, un enfoque o una estrategia antes de tocar código.

**Rationale**: la spec ya decidió (Assumptions, "Modelo de activación") que la activación
es autónoma del agente y dirigida por el protocolo. Un detector determinista viviría en el
entorno, que es justo el modelo que el usuario descartó. Además, un detector por patrones
sobre el texto de la solicitud sería frágil y produciría falsos positivos costosos
(cargar contexto en cada turno).

**Alternativas descartadas**:

- *Detección por hook `UserPromptSubmit`* con coincidencia de patrones sobre el prompt:
  descartada. Sería determinista pero solo funciona en Claude Code y OpenCode, que es
  exactamente la limitación que la corrección del usuario eliminó.
- *Detección por herramienta `EnterPlanMode`*: descartada. No todos los agentes entran en
  modo plan mediante una llamada de herramienta observable; en varios es un cambio de modo
  de la interfaz, invisible para los hooks.

**Cómo se verifica entonces**: los escenarios de aceptación se prueban ejerciendo las tres
formas del disparador contra los agentes de referencia, no inspeccionando un detector.

---

## D3 — ¿El presupuesto de contexto sigue siendo efectivo bajo invocación autónoma?

**Pregunta abierta #3 del checklist.** (FR-007)

**Decisión**: Sí, sin cambios. El presupuesto se aplica dentro del caso de uso, no en la
ruta que lo invoca.

**Verificación realizada**:

- `application/ports/settings_repository.go:23` define `Budget int` como "techo blando (en
  CARACTERES emitidos) de get_context. <=0 = sin límite".
- `application/usecases/build_context.go:93-113` aplica el techo dentro de `Build()`, con
  `budgetReserve = 300` de margen para las secciones de cierre.
- Tanto la herramienta MCP `get_context` (`cmd_mcp.go:296`) como el comando
  `mem context` (`cmd_context.go`) llaman al mismo `deps.ContextBuilder.Build()`.

**Consecuencia**: cualquier ruta nueva que reutilice `ContextBuilder.Build()` hereda el
presupuesto automáticamente. El requisito se cumple por construcción siempre que la nueva
funcionalidad **no** genere el contexto por su cuenta.

---

## D4 — Vehículo de la instrucción de activación: ¿dónde vive el disparador?

**Decisión**: en el bloque de protocolo que `mem install` ya escribe en
`AGENTS.md` / `CLAUDE.md` / `.cursorrules` / `.windsurfrules`, subiendo su versión de
`gomemory-protocol-v5` a `gomemory-protocol-v6`.

**Rationale**: es el único canal que **todos** los agentes ya leen, que es la condición que
FR-002 exige. Y el mecanismo de actualización ya está resuelto:
`versionMarkerPattern = regexp.MustCompile("<!-- gomemory-protocol-v\\d+ -->")` en
`cmd_install.go:311` localiza el bloque instalado sea cual sea su versión, y
`composeAgentFile` lo reemplaza entero. Subir el número de versión hace que las
instalaciones existentes se actualicen solas, sin dejar restos (FR-030) y sin escribir una
línea de migración.

**Alternativas descartadas**:

- *Un archivo nuevo propio* (`.memory/atomic-plan.md`) referenciado desde el protocolo:
  añade un artefacto y una indirección sin ganar cobertura.
- *Solo habilidades nativas por agente* (Claude skill, OpenCode command): descartada como
  vehículo principal. Cubre dos agentes y deja fuera al resto, contradiciendo FR-027.
  Se conserva como capa opcional (ver D6).

---

## D5 — Cómo llega el método al agente sin inflar el contexto permanente

**Tensión detectada**: el bloque de protocolo vive en el prompt de sistema de **todos** los
turnos, no solo los de planificación. El método completo ronda 1,5 KB sobre un bloque
actual de ~3 KB: un 50 % más de huella permanente para algo que solo sirve en una minoría
de turnos. La feature 008 del proyecto (`008-reduce-context-footprint`) se hizo
precisamente para reducir esta huella, así que inflarla aquí iría contra una decisión ya
tomada.

**Decisión**: una sola llamada devuelve método y contexto juntos. Se añade la herramienta
MCP `get_plan_context()` y su equivalente de línea de comandos `mem plan-context`. El
bloque de protocolo solo carga el disparador y el puntero (unas 8 líneas).

**Rationale**:

- **Una sola invocación al entrar en modo plan**, no dos (contexto por un lado, método por
  otro). Menos pasos que el agente pueda saltarse.
- **Fuente única de verdad**: el método vive embebido en el binario
  (`infrastructure/templates/`, ya cubierto por el `go:embed all:templates` existente), no
  duplicado en cada proyecto ni en cada archivo de agente. Se versiona con el binario.
- **Huella permanente mínima**: el método solo entra en el contexto cuando de verdad se va
  a planificar.
- **Presupuesto heredado**: la parte de contexto reutiliza `ContextBuilder.Build()`, así
  que D3 se cumple sin trabajo extra.
- **Doble vía**: MCP para los agentes que lo tengan, línea de comandos para el resto. FR-003
  se satisface con el mismo patrón de degradación que el protocolo ya declara hoy.

**Alternativas descartadas**:

- *Método completo en línea dentro del bloque de protocolo*: la más simple de construir,
  pero paga la huella permanente descrita arriba en todos los turnos y de todos los
  proyectos. Descartada por contradecir la feature 008.
- *Que `get_context()` devuelva el método cuando detecte modo plan*: `get_context` no tiene
  —ni debe tener— noción de modo plan, y contaminaría el arranque de sesión de todos los
  proyectos.
- *Comando separado `mem plan-method` + `mem context` por separado*: dos invocaciones donde
  basta una, y más superficie que el agente puede ejecutar a medias.

---

## D6 — Envoltorios nativos por agente (FR-028)

**Decisión**: se generan como capa **opcional y equivalente**, nunca como vehículo
principal: una habilidad para Claude Code y un comando para OpenCode, ambos generados
desde la misma plantilla embebida que alimenta `get_plan_context()`.

**Rationale**: mejoran la ergonomía (la persona puede invocarlos a mano) y aprovechan la
carga diferida nativa de cada agente, pero no son necesarios para que la funcionalidad
opere. El proyecto ya tiene el patrón exacto: `InstallSpeckitExtension`
(`adapters/primary/setup/speckit_extension.go`) distribuye hoy el brazo extensor a
`.claude/skills/` y `.opencode/commands/` reutilizando `InstallPlugin`, que solo reescribe
un archivo si su contenido difiere (idempotencia, FR-029).

**Riesgo controlado**: dos copias del método (binario + archivos distribuidos) podrían
divergir. Se mitiga generándolas siempre desde la misma plantilla embebida en cada
instalación, nunca editándolas a mano.

---

## D7 — Semántica del interruptor de apagado (FR-032 vs FR-034)

**Tensión detectada**: FR-034 exige que el método siga aplicándose aunque no haya contexto
que cargar; FR-032 exige poder apagar la funcionalidad. Son cosas distintas y el diseño
debe distinguirlas.

**Decisión**: dos comportamientos separados en la salida de `get_plan_context()`:

| Situación | Salida |
|-----------|--------|
| Todo normal | Método + contexto del proyecto |
| Memoria no inicializada, o fallo al construir el contexto | **Solo el método** (FR-034) |
| `atomic_plan_disabled: true` | **Nada**, con código de salida 0 (FR-032) |

**Rationale**: la ausencia de historial es una circunstancia, no una preferencia — degradar
a "solo método" conserva el valor de la Historia 2, que la spec declaró independiente de la
Historia 1. El apagado explícito sí es una preferencia y debe silenciarlo todo.

El patrón de "terminar sin salida y con código 0" ya está establecido y verificado en el
proyecto: es exactamente lo que hace hoy
`.specify/extensions/gomemory-context/scripts/bash/update-gomemory-context.sh` cuando
`speckit_context_disabled` está activo.

---

## D8 — Nombre y forma del ajuste de configuración

**Decisión**: `atomic_plan_disabled bool` en `ports.SettingsData`, con
`json:"atomic_plan_disabled,omitempty"`. Ausente o `false` = funcionalidad activa.

**Rationale**: replica exactamente el patrón ya usado por `speckit_context_disabled`
(feature 011), `synapse_disabled` y `code_graph_disabled`. La convención "el campo apaga,
el default enciende" mantiene la retrocompatibilidad de los archivos de configuración ya
escritos: un `settings.json` existente sigue siendo válido y la funcionalidad queda
encendida por defecto, que es lo que FR-025 pide.

**Ubicación**: `.memory/settings.json` del proyecto, que es donde `SettingsRepository` ya
lee y escribe. El apagado es por proyecto (FR-026), así que no hace falta un archivo de
configuración global nuevo.

---

## D9 — Captura del plan aprobado (FR-035, FR-036)

**Decisión**: no se construye nada. Ya existe y cumple los dos requisitos.

**Verificación realizada**: `hookPlanApproved` (`adapters/primary/cli/cmd_hook.go:323`)
persiste el plan aprobado como memoria `type=decision`. Su documentación en el código
confirma los dos extremos que la spec exige:

- Claude Code lo dispara con `PostToolUse` y matcher `ExitPlanMode`, y
  **`PostToolUse` solo dispara si la persona aprobó** — un plan rechazado no ejecuta la
  herramienta. Eso satisface FR-036 sin trabajo adicional.
- OpenCode y cualquier otro agente invocan `mem hook plan-approved` con `{"plan":"..."}`
  por entrada estándar; `extractPlanFromPayload` acepta ambas formas.

Como el plan atómico es texto, su descomposición en tareas queda conservada tal cual dentro
del contenido de la memoria, que es lo que FR-035 pide ("conservando su descomposición").

**Único ajuste pendiente**: verificar que el árbol con caracteres de dibujo sobrevive el
viaje sin mutilarse, y que `planTitle` produce un título razonable a partir de una primera
línea que ahora empieza por un emoji de objetivo.

---

## D10 — Permisos de la herramienta nueva en Claude Code

**Decisión**: añadir `mcp__gomemory__get_plan_context` a `ClaudeAutoAllowTools`
(`adapters/primary/setup/claude_code_setup.go:103`).

**Rationale**: el comentario de esa lista establece el criterio — se pre-aprueban las
herramientas "de solo lectura, o de escritura acotada y reversible".
`get_plan_context` es de solo lectura pura. Y el propio código advierte que la falta de
pre-aprobación "es la causa más común de que el protocolo de memoria no se aplique
automáticamente" (`writeClaudePermissions`, línea ~131). Sin este paso, la activación
autónoma quedaría bloqueada pidiendo permiso en cada planificación, que es precisamente el
fallo que la feature busca evitar.

---

## D11 — Pre-aprobación en OpenCode: hueco verificado, no existe hoy

**Origen**: el usuario pidió explícitamente que la pre-aprobación quedara cubierta también
para OpenCode, no solo para Claude Code.

**Hallazgo**: OpenCode **no tiene hoy ninguna gestión de permisos en gomemory**. Es un
hueco real, verificado en el código:

- `writeOpenCodeMCPFile` (`adapters/primary/setup/opencode_setup.go:117`) escribe
  únicamente `{type, command, enabled}` en la entrada `mcp.gomemory`. Ninguna clave de
  permisos.
- `ApplyAutoApprove` (`adapters/secondary/persistence/settings.go:130-193`) recorre
  exactamente cuatro rutas: `.mcp.json`, `.cursor/mcp.json`, `.windsurf/mcp_config.json` y
  `.cline/mcp_settings.json`. **`opencode.json` no está en la lista.**

Es decir: la advertencia de `writeClaudePermissions` —que la falta de pre-aprobación es
"la causa más común de que el protocolo de memoria no se aplique automáticamente"— aplica
hoy a OpenCode sin ninguna mitigación.

**Esquema correcto de OpenCode** (consultado en la documentación oficial, no asumido):
clave `permission` de primer nivel, con valores `allow` / `ask` / `deny`, y comodines para
controlar de golpe las herramientas de un servidor MCP.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "gomemory_*": "allow",
    "gomemory_forget_memory": "ask"
  }
}
```

La documentación muestra que una clave específica prevalece sobre el comodín
(`{"*": "ask", "bash": "allow"}`), lo que permite abrir el conjunto y cerrar la excepción.

**Decisión**: añadir `writeOpenCodePermissions`, análoga a `writeClaudePermissions`,
invocada desde `InstallOpenCode` (ámbito proyecto) e `InstallOpenCodeGlobal` (ámbito
global), ya que `writeOpenCodeMCPFile` documenta que "el esquema es idéntico en ambos
scopes".

**`forget_memory` queda deliberadamente fuera de la pre-aprobación**, en `ask`. Replica la
decisión ya tomada del lado de Claude Code, donde el comentario de `ClaudeAutoAllowTools`
la excluye "por ser destructiva/irreversible". Un comodín plano `"gomemory_*": "allow"`
habría abierto en silencio una herramienta irreversible: sería un retroceso de seguridad
introducido de pasada.

**Trampa a evitar — no extender `ApplyAutoApprove` con `opencode.json`**: sería la
solución aparentemente obvia, y estaría mal. Esa función escribe la forma
`mcpServers[].autoApprove`, que OpenCode **no** entiende. El propio código del proyecto
tiene la cicatriz de ese error: el comentario de `WriteOpenCodeMCP`
(`opencode_setup.go:100`) explica que una configuración previa usaba un esquema "que
OpenCode ignora por completo (de ahí que las tools nunca aparecieran)". Escribir permisos
con la forma equivocada reproduciría exactamente ese fallo: cero errores visibles y cero
efecto.

**Verificación exigida**: por la regla de trabajo del proyecto ("verde en tests no es
funciona"), esta decisión no se da por cerrada con pruebas unitarias. Hay que comprobar
contra OpenCode en ejecución que las herramientas de gomemory no piden aprobación —
`opencode debug config` es la vía que el proyecto ya usó para confirmar empíricamente el
comportamiento de configuración de OpenCode en la feature 005.

---

## Resumen de decisiones

| # | Decisión | Efecto sobre el trabajo |
|---|----------|-------------------------|
| D1 | El ámbito global ya existe y está verificado | Extender la ruta global, no construirla |
| D2 | El disparador es declarativo, evaluado por el agente | No se construye detector |
| D3 | El presupuesto se hereda de `ContextBuilder.Build()` | Cero trabajo, siempre que se reutilice |
| D4 | El disparador viaja en el bloque de protocolo, v5 → v6 | La actualización de instalaciones existentes es automática |
| D5 | `get_plan_context()` devuelve método + contexto en una llamada | Una herramienta MCP y un comando nuevos |
| D6 | Envoltorios nativos como capa opcional equivalente | Reutiliza `InstallPlugin`, ya idempotente |
| D7 | Sin memoria → solo método; apagado → nada | Dos ramas explícitas de degradación |
| D8 | `atomic_plan_disabled` siguiendo el patrón establecido | Un campo, retrocompatible |
| D9 | La captura del plan aprobado ya cumple FR-035/FR-036 | Solo verificar, no construir |
| D10 | Pre-aprobar la herramienta nueva en Claude Code | Una línea, pero crítica para la activación |
| D11 | OpenCode no tiene pre-aprobación en absoluto: hay que construirla | Función nueva, ambos ámbitos, con `forget_memory` excluida |

**Marcadores `NEEDS CLARIFICATION` restantes: ninguno.**
