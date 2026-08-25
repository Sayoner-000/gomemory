# Contrato de configuración consolidada de hooks

## Fuente única

Codex debe cargar hooks únicamente desde `/root/.codex/config.toml`. El archivo `/root/.codex/hooks.json` no debe
existir en su ruta reconocida después de completar la migración.

## Algoritmo requerido

1. Leer todos los eventos, grupos y acciones de `config.toml` y `hooks.json`.
2. Normalizar cada grupo por nombre de evento, `matcher` efectivo y colección ordenada de acciones.
3. Normalizar cada acción por todos sus campos compatibles, incluidos `type`, `command`, `timeout` y cualquier opción
   adicional presente.
4. Conservar la primera aparición de cada identidad normalizada y descartar apariciones posteriores equivalentes.
5. Mantener primero las definiciones únicas de `config.toml` en su orden original y anexar después las definiciones
   únicas procedentes de `hooks.json`.
6. Serializar el resultado en `config.toml` sin alterar secciones ajenas a hooks.

La ausencia de `matcher` es un valor semántico y no debe convertirse en un filtro explícito. Ninguna decisión puede
depender del nombre del proveedor, de fragmentos del comando ni de la ruta del destino.

## Estado observado como fixture

En el estado actual, aplicar el algoritmo produce una sección equivalente a:

```toml
[[hooks.SessionStart]]
matcher = "startup|resume|clear|compact"

[[hooks.SessionStart.hooks]]
type = "command"
command = '<aviso vigente de codebase-memory-mcp>'

[[hooks.SessionStart]]

[[hooks.SessionStart.hooks]]
type = "command"
command = "bash '/root/.codex/herdr-agent-state.sh' session"
timeout = 10
```

Los nombres Herdr y `codebase-memory-mcp` describen únicamente este fixture. Otro inventario debe producir un resultado
equivalente según sus propios datos.

## Estado de confianza durante la migración

- Se conserva un estado solo si fuente, evento, posición e identidad normalizada permanecen iguales.
- Se elimina cualquier estado cuya posición se retire, cambie de identidad o pase a representar otro hook.
- Se elimina todo estado cuyo identificador contenga `/root/.codex/hooks.json`.
- Cada hook migrado o reubicado se acepta únicamente si Codex genera o confirma su estado desde `config.toml`.
- No se calcula, copia ni codifica manualmente ningún `trusted_hash`.
- Tras autorizar, cada hook ejecutable debe tener un hash no vacío y estar habilitado conforme a la serialización
  efectiva de Codex.

## Invariantes

- `[features] hooks = true` permanece activo.
- No cambia ninguna sección ajena a `hooks.SessionStart` y `hooks.state`.
- El contrato debe superar una carga con `--strict-config`.
- Los conteos se validan con un lector TOML, no por coincidencias de texto que puedan incluir comentarios.
- Reaplicar la normalización produce el mismo resultado y no altera identidades únicas.
