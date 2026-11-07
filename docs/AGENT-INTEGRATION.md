# Integración de un agente cualquiera con el modo plan atómico

Este es **el** contrato del modo plan atómico de gomemory. Los formatos propios de cada agente
(Claude Code, OpenCode y los que vengan) son traducciones de lo que está aquí, nunca su definición.
Si un agente puede cumplir lo que sigue, obtiene la garantía completa **sin que gomemory necesite
conocerlo de antemano**.

Está escrito para que alguien lo implemente sin leer el código de gomemory.

## Los tres niveles

Un agente pide lo que puede sostener. Ninguno es obligatorio salvo el tercero.

| Nivel | Qué debe poder hacer el agente | Qué obtiene |
|---|---|---|
| **1 — Garantía de forma** | Invocar un comando **antes** de mostrar el plan a la persona, y respetar la decisión que recibe | Un plan sin forma de árbol no llega a la persona |
| **2 — Contexto al planificar** | Inyectar texto en el contexto del modelo al entrar en modo plan | Método de descomposición e historial disponibles antes de redactar |
| **3 — Piso textual** | Recibir las instrucciones que el servidor MCP entrega al conectarse, o leer un archivo de instrucciones de nivel usuario | Protocolo, recordatorio por turno y envoltorio nativo |

## Nivel 1 — Garantía de forma

### Invocación

```bash
mem hook plan-guard                   # dialecto detectado automáticamente
mem hook plan-guard --emit=neutral    # dialecto forzado
```

**Momento**: inmediatamente antes de presentar el plan a la persona, y después de que el agente lo
haya redactado por completo.

### Entrada (stdin)

La forma mínima y siempre válida:

```json
{ "plan": "texto completo del plan tal como se presentaría" }
```

También se acepta texto plano por stdin: si el contenido no es JSON, se toma íntegro como el plan.
Un agente que ya tenga su propia envoltura puede enviarla: si contiene el plan en un campo conocido,
se reconoce (ver «Dialectos»).

### Salida — dialecto `neutral` (el que manda por defecto)

| Decisión | Código de salida | stdout | stderr |
|---|---|---|---|
| **Permitir** | `0` | vacío | vacío |
| **Devolver el plan** | `≠ 0` | vacío | motivo legible, para dárselo al modelo |

Eso es todo lo que un agente necesita implementar: *si el código de salida no es cero, no muestres el
plan; entrégale al modelo lo que vino por stderr y pídele que lo presente otra vez.*

### Salida — dialecto `json`

```json
{ "decision": "deny", "reason": "…" }
```

y `{ "decision": "allow" }` para permitir. Código de salida `0` en ambos casos: aquí la decisión
viaja en la salida, no en el código.

### Garantías que el agente puede dar por hechas

1. **Nunca bloquea por un fallo propio.** Payload ilegible, estado corrupto, memoria no inicializada,
   error interno → permitir.
2. **Como máximo una devolución por episodio de plan.** La persona no puede quedar atrapada en un
   bucle.
3. **Nunca sobre solicitudes triviales.** Un plan de un solo paso pasa siempre.
4. **Rápido.** No consulta la base de datos: presupuesto por debajo de 50 ms.
5. **Apagable por la persona** (`plan_guard_disabled` en la configuración), y apagado equivale a
   permitir siempre.

### Cierre del episodio

```bash
mem hook plan-approved    # con {"plan":"…"} por stdin
```

Se invoca cuando la persona **aprueba** el plan. Cumple dos funciones: guarda la decisión en la
memoria del proyecto y cierra el episodio, de modo que el siguiente plan vuelve a poder evaluarse. Un
agente que implemente el nivel 1 debería implementar también esta llamada; si no lo hace, el sistema
degrada hacia permitir.

## Nivel 2 — Contexto al planificar

```bash
mem hook plan-entered     # devuelve el documento en el dialecto que corresponda
mem plan-context          # equivalente sin envoltura de hook, para inyección directa
```

**Momento**: al entrar en modo plan, antes de que el modelo redacte.

**Salida**: el método de descomposición atómica seguido del historial del proyecto, ya **ajustado al
presupuesto del documento** (por defecto 9500 caracteres, configurable con `--budget`). El método va
siempre completo; el historial se recorta si hace falta y se indica cómo recuperar el resto. Nunca hay
cortes a mitad de frase. El hook adapta esa salida al dialecto del agente.

**Si el agente no puede inyectar en ese momento**: no pasa nada. El nivel 3 emite un recordatorio de
una línea en cada turno, que es lo que sostiene la cobertura mientras tanto.

## Nivel 3 — Piso textual (todo agente lo tiene)

| Vía | Comando | Cuándo |
|---|---|---|
| Bloque de protocolo en `initialize.instructions` | `mem mcp` (el propio servidor) | En cada conexión, sin instalar nada |
| Recordatorio por turno | `mem hook nudge` | En cada turno, si el agente puede inyectar texto |
| Envoltorio nativo del método | lo escribe la instalación | Una vez, si el agente tiene formato propio de habilidad o comando |

No existe una vía de archivos de instrucciones: gomemory no escribe `AGENTS.md` ni `CLAUDE.md`.
`mem setup-mcp --scope global` retira, con copia junto al archivo, los bloques completos que dejaron
versiones anteriores; un bloque sin marcador de cierre queda intacto y se informa.

El bloque que escribía en esos archivos era una segunda copia del que el servidor entrega en
`initialize.instructions`. Mantener dos copias del mismo texto permitía que una quedara
desactualizada respecto de la otra.

El piso textual lo cubre MCP: el cliente recibe el bloque al conectarse, sin instalar nada. Algunos
clientes truncan ese texto, por eso las reglas van antes que la lista de herramientas. El baseline
universal llega en el contexto de sesión. Por eso `mem doctor` reporta el canal de instrucciones como
«no aplica», con el motivo declarado, y solo avisa si queda un bloque heredado.

`mem hook nudge` existe precisamente para agentes sin el sistema de eventos de Claude Code: escribe en
stdout el texto del turno (recordatorio de guardado, de compactación, de modo plan) o nada. Se invoca
al comienzo de cada turno y su salida se inyecta como contexto.

## Dialectos

El dialecto se detecta a partir de lo que el agente envía y se puede forzar con `--emit`:

| Dialecto | Se detecta por | Devolver el plan se expresa como |
|---|---|---|
| `neutral` | nada reconocible (**default**) | código de salida ≠ 0 + motivo por stderr |
| `json` | `--emit=json` | `{"decision":"deny","reason":"…"}`, código 0 |
| `claude` | payload con la envoltura de eventos de Claude Code | `hookSpecificOutput.permissionDecision: "deny"` + motivo, código 0 |
| `text` | `--emit=text` | motivo por stdout, código 0 |

Añadir un dialecto nuevo es añadir una traducción de salida: el motor de decisión no cambia.

## Declararse en el registro (opcional pero recomendable)

Un agente **no** necesita estar en el registro de capacidades para usar el contrato: si invoca los
comandos, funciona. Declararse sirve para dos cosas: que la instalación le escriba el piso textual en
las rutas correctas, y que `mem doctor` lo enumere con su estado real en lugar de omitirlo. Lo que se
declara: nombre, dialecto, niveles soportados y ámbitos.

## Ejemplo mínimo: agente ficticio en 12 líneas

```bash
#!/usr/bin/env bash
# Nivel 1 completo para un agente imaginario que llama a este script
# antes de mostrar un plan y después de aprobarlo.
plan_file="$1"; phase="$2"

case "$phase" in
  before-present)
    if ! reason=$(mem hook plan-guard < "$plan_file" 2>&1 >/dev/null); then
      printf 'PLAN DEVUELTO: %s\n' "$reason"   # dáselo al modelo y pídele otro intento
      exit 1
    fi ;;
  approved)
    jq -Rs '{plan:.}' < "$plan_file" | mem hook plan-approved ;;
esac
exit 0
```

Ese script es la base del test de agnosticismo del proyecto
(`tests/integration/foreign_agent_test.go`): un cliente que imita a un agente desconocido obtiene la
garantía completa sin una sola línea modificada en gomemory.

## Antigravity CLI (`agy`)

`mem install --agents agy` escribe el MCP y los hooks en `~/.gemini/config` e instala allí las skills. Guarda antes una copia de cada archivo y reemplaza el grupo manual `gomemory-lifecycle`. Cada hook ejecuta `mem hook agy <evento>`.

| Evento de agy | gomemory |
|---|---|
| `SessionStart` | contexto de arranque |
| `PreInvocation` (primera del turno) | contexto del prompt |
| `PreToolUse` | reglas de `expand`; responde `allow` o `deny` |
| `PostToolUse` | actividad; no recibe la salida de la herramienta |
| `Stop` | checkpoint del turno |

En agy 1.4.12 no hay compresión automática de salidas ni aviso tras compactar. `allow` no aprueba nada: la decisión sigue siendo de los permisos de agy. `mem doctor` marca como desactualizada cualquier otra versión. Las skills se instalan en `~/.gemini/config/skills`, la carpeta global de agy. La instalación también convierte a rutas absolutas las entradas de `skills.json` que empiezan por `~`, porque agy las descarta. Evidencia: `docs/evidence/hcd015/`.

## Aislamiento de Subagentes Revisor sin Contexto (`splash`)

Cuando un agente lanza un subagente de tipo `splash` (el revisor sin contexto bajo el modelo Elephant-Goldfish, HCD-047 y HCD-051), el runtime o el envoltorio debe cumplir las siguientes reglas de aislamiento:

1. **Sin Contexto de Arranque:** Si `agent_type == "splash"` o el subagente se identifica con ese nombre, `SubagentStart` o el hook de inicio no debe inyectar memorias previas, instrucciones globales ni el bloque de integración MCP.
2. **Sin Herramientas MCP de Memoria:** El subagente debe disponer únicamente de herramientas del sistema de archivos de solo lectura (`read`, `grep`, `glob`). No debe tener acceso a herramientas como `save_memory` o `search_memories`.
3. **Sin Herencia de Conversación:** En agentes que soporten bifurcación de contexto (como `spawn_agent` en Codex), la invocación debe forzar `fork_turns: "none"` para garantizar que el revisor evalúe la especificación de manera estrictamente imparcial.

`mem setup` instala automáticamente las plantillas correspondientes en Claude Code (`~/.claude/agents/splash.md`), OpenCode (`~/.config/opencode/agents/splash.md`), agy (`~/.gemini/config/agents/splash/agent.md`) y Codex (`~/.codex/agents/splash.toml`).

## Ver también

- `mem doctor [--json] [--strict]` — reporte de cobertura de canales por agente.
- [MANUAL.md](./MANUAL.md) — comandos del CLI en general.
- [architecture.md](./architecture.md) — cómo encajan los hooks y el registro de capacidades.
