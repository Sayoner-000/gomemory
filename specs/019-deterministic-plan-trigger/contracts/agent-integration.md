# Contrato — Integración de un agente cualquiera con el modo plan atómico

Este es **el** contrato. Los formatos propios de cada agente (Claude Code, OpenCode y los que
vengan) son traducciones de lo que está aquí, nunca su definición. Si un agente puede cumplir lo que
sigue, obtiene la garantía completa **sin que gomemory necesite conocerlo** (FR-A1, FR-A3, INV-6).

Está escrito para que alguien lo implemente sin leer el código de gomemory.

## Los tres niveles

Un agente pide lo que puede sostener. Ninguno es obligatorio salvo el tercero.

| Nivel | Qué debe poder hacer el agente | Qué obtiene |
|---|---|---|
| **1 — Garantía de forma** | Invocar un comando **antes** de mostrar el plan a la persona, y respetar la decisión que recibe | Un plan sin forma de árbol no llega a la persona |
| **2 — Contexto al planificar** | Inyectar texto en el contexto del modelo al entrar en modo plan | Método de descomposición e historial disponibles antes de redactar |
| **3 — Piso textual** | Leer un archivo de instrucciones del proyecto o del usuario | Protocolo, recordatorio por turno y envoltorio nativo |

## Nivel 1 — Garantía de forma

### Invocación

```bash
mem hook plan-guard            # dialecto detectado automáticamente
mem hook plan-guard --emit=neutral   # dialecto forzado
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
5. **Apagable por la persona**, y apagado equivale a permitir siempre.

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
presupuesto** del canal (por defecto 9 500 caracteres, configurable con `--budget`). El método va
siempre completo; el historial se recorta si hace falta y se indica cómo recuperar el resto. Nunca hay
cortes a mitad de frase.

**Si el agente no puede inyectar en ese momento**: no pasa nada. El nivel 3 emite un recordatorio de
una línea en cada turno, que es lo que sostiene la cobertura mientras tanto.

## Nivel 3 — Piso textual (todo agente lo tiene)

| Vía | Comando | Cuándo |
|---|---|---|
| Bloque de protocolo en el archivo de instrucciones | `mem install --target <dir>` | Una vez por proyecto |
| Bloque de protocolo de nivel usuario | `mem setup-mcp --scope global` | Una vez por máquina |
| Recordatorio por turno | `mem hook nudge` | En cada turno, si el agente puede inyectar texto |
| Envoltorio nativo del método | lo escribe la instalación | Una vez, si el agente tiene formato propio de habilidad o comando |

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
declara: nombre, dialecto, niveles soportados, ámbitos, archivos de instrucciones y formato del
envoltorio nativo.

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

Ese script es también la base de la prueba de agnosticismo: un cliente que imita a un agente
desconocido y obtiene la garantía completa **sin una sola línea modificada en gomemory** (SC-A1).

## Pruebas de contrato exigidas

- Cliente de prueba que usa solo el dialecto `neutral` (stdin + código de salida): obtiene devolución
  con plan en prosa y permiso con plan en árbol.
- Texto plano por stdin (sin JSON) → mismo veredicto que la forma `{"plan":"…"}`.
- Sin `--emit` y sin envoltura reconocible → responde en `neutral`, nunca en el dialecto de un agente
  concreto.
- `--emit=json`, `--emit=text` y `--emit=claude` producen las tres formas documentadas con el mismo
  veredicto para el mismo plan.
- Un agente ausente del registro obtiene la garantía igual.
- Añadir una entrada al registro hace que `mem doctor` la enumere sin tocar el reporte ni el script de
  verificación.
