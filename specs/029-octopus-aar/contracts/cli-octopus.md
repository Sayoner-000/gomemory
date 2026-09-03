# Contrato: comando `mem octopus`

Cinco subcomandos. La línea de comandos es una comodidad, no un requisito del protocolo (FR-053): la política funciona igual sin ella.

```text
mem octopus plan [--file <ruta.json>] [--budget N] [--max-parallel N] [--max-agents N] [--json]
mem octopus route <objetivo> [--class C] [--deps a,b] [--files x,y] [--read-only] [--json]
mem octopus status [--json]
mem octopus usage [--json]
mem octopus history [-n N] [--class C] [--json]
```

## `plan`

Simulación por definición: describe qué quedaría inline, qué se delegaría, qué podría ejecutarse en paralelo, con qué presupuestos estimados y por qué en cada caso. **No inicia ningún subagente** (FR-044, AC-019).

Origen del grafo de tareas, en este orden: `--file` si se pasa; en su defecto, la funcionalidad activa de Spec Kit vía `ports.SpecKitReader`; si tampoco hay, termina pidiendo un origen explícito.

## `route`

Enruta una sola unidad descrita por argumentos. Es la vía rápida para comprobar una decisión sin armar un plan.

## `status`, `usage`, `history`

Estado del módulo y topes efectivos; agregados de consumo estimado frente a real; historial de decisiones con su resultado. `history` acepta filtro por clase de tarea.

## Reglas comunes

- Con el módulo apagado, cualquier subcomando responde que está desactivado, indica cómo activarlo y termina con código distinto de cero.
- `--json` emite la misma estructura que devuelve la tool MCP equivalente. Una sola forma de datos para las dos superficies.
- Toda cifra estimada se marca como tal en la salida legible (FR-033).
- El comando entra en el despachador (`case "octopus"`) y en la ayuda de uso, y toma el canal `cli` de forma natural, sin caso especial en `channelForCommand`.
