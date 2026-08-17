# Contrato — `mem doctor` (reporte de cobertura de los dos brazos)

Responde "¿está esto realmente activo, y dónde no?" por agente y por canal, de los dos brazos:
gomemory (que administra) y el grafo de código externo (que solo observa). Cubre FR-017 y sirve de
fuente de datos al script de regresión de FR-018.

## Invocación

```bash
mem doctor              # reporte legible, exit 0 siempre
mem doctor --json       # misma información, estable y comparable
mem doctor --strict     # exit != 0 si Problems > 0 (uso en CI)
```

`--json` y `--strict` son combinables. Sin `--strict`, el código de salida es 0 incluso con problemas:
un diagnóstico no rompe el flujo de quien lo consulta.

## Salida `--json`

```json
{
  "version": "2.7.0",
  "problems": 1,
  "channels": [
    {
      "arm": "gomemory",
      "agent": "claude",
      "scope": "user",
      "kind": "instructions",
      "state": "outdated",
      "detail": "protocolo v4 encontrado; vigente v8 (~/.claude/CLAUDE.md)"
    },
    {
      "arm": "gomemory",
      "agent": "claude",
      "scope": "user",
      "kind": "plan_guard",
      "state": "ok",
      "detail": "PreToolUse(ExitPlanMode) → mem hook plan-guard"
    },
    {
      "arm": "codegraph",
      "agent": "claude",
      "scope": "user",
      "kind": "instructions",
      "state": "ok",
      "detail": "activación presente"
    },
    {
      "arm": "gomemory",
      "agent": "opencode",
      "scope": "user",
      "kind": "plan_guard",
      "state": "not_applicable",
      "detail": "el ciclo del agente no ofrece decisión antes de presentar el plan"
    }
  ],
  "degradations": [
    "opencode: sin devolución en el borde de salida; la forma del plan se pide por texto en cada turno"
  ]
}
```

## Origen de los datos

El reporte **se genera a partir del registro único de capacidades por agente**
([data-model.md](./data-model.md) §5), no de listas escritas dentro del propio reporte. Añadir una
entrada al registro basta para que ese agente aparezca aquí y en la verificación de regresión, sin
tocar ninguno de los dos (FR-A4, SC-A2). Los agentes que no están en el registro no se inventan: se
omiten del reporte, lo que **no** les impide usar el contrato neutral.

## Reglas del contrato

1. **Orden determinista**: los canales se ordenan por `arm`, `agent`, `scope`, `kind`. Dos ejecuciones
   sin cambios producen JSON idéntico, byte a byte, para que un script pueda comparar.
2. **`problems`** cuenta únicamente los estados `outdated`, `duplicated` y `missing`. Las degradaciones
   declaradas **no** son problemas.
3. **`not_applicable`** solo para "agente no instalado en esta máquina" o "el agente no soporta este
   tipo de canal". Nunca para tapar un canal roto (FR-019).
4. **`duplicated`** es un estado propio: es la regresión que produce una reinstalación con el filtro de
   idempotencia incompleto.
5. **Los canales `codegraph` son de solo lectura**: se reportan, jamás se escriben ni se corrigen
   (INV-1). Si el brazo extensor no está instalado, sus canales se omiten del reporte y **no** se
   emite ningún aviso por su ausencia (INV-4, FR-013).
6. **Sin proyecto resoluble**: el reporte se limita a los canales de ámbito usuario y lo dice; no es un
   error.
7. **`version`** es la del binario, para que un reporte pegado en un issue sea interpretable.

## Tipos de canal inspeccionados

| `kind` | Qué comprueba |
|---|---|
| `plan_entry` | Hook de entrada al modo plan registrado y apuntando al binario correcto |
| `plan_guard` | Hook de borde de salida registrado y apuntando al binario correcto |
| `turn_reminder` | El recordatorio por turno está activo en el canal del agente |
| `instructions` | Bloque de protocolo presente y en la versión vigente |
| `native_wrapper` | Envoltorio nativo del método (habilidad o comando del agente) presente y al día |
| `mcp_instructions` | Instrucciones del servidor MCP incluyen el disparador de modo plan |

## Pruebas de contrato exigidas

- Reporte con un archivo de instrucciones en versión anterior → ese canal en `outdated` y
  `problems ≥ 1`.
- Entrada de hook duplicada → estado `duplicated`.
- Agente ausente de la máquina → `not_applicable` y `problems` sin incrementar.
- Brazo extensor ausente → sin canales `codegraph` y sin avisos.
- `--strict` con `problems > 0` → exit distinto de cero; con `problems == 0` → exit 0.
- Dos ejecuciones consecutivas sin cambios → salida `--json` idéntica.
- Añadir una entrada al registro de capacidades (un agente ficticio, en el test) hace que aparezca en
  el reporte **sin modificar el reporte ni el script de verificación**.
- Un agente declarado con solo `text_floor` aparece con sus canales deterministas en
  `not_applicable` y ninguna degradación oculta.
