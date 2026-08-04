# Contrato: `mem settings` — nuevo flag `--speckit-context`

Extiende el contrato ya existente de `adapters/primary/cli/cmd_settings.go`
(mismo patrón que `--code-graph`, `--adr-sync`) con un flag para el
interruptor de la Historia 4.

## Flag nuevo

| Flag | Tipo | Default | Efecto |
|------|------|---------|--------|
| `--speckit-context` | `bool` | `true` (activado) | `false` ⇒ `settings.SpeckitContextDisabled = true` |

Sigue la misma convención que `--code-graph` (donde el flag expresa el
estado "activado" y se invierte internamente al campo `*Disabled`), para
que `mem settings --speckit-context=false` sea simétrico con `mem settings
--code-graph=false`.

## `--show` / salida sin flags

`printSettings()` agrega una línea:

```text
Brazo extensor spec-kit: <true|false>
```

en el mismo bloque que ya imprime `Grafo de código externo:` y
`Sincronización de ADR:`.

## Contrato de la fila en la TUI (`adapters/primary/tui/tui.go`)

- `configOptions` pasa de `5` a `6`.
- Nueva fila (índice 5): `"Brazo extensor spec-kit: " + onOff(!s.SpeckitContextDisabled)`.
- `enter`/`espacio` sobre esa fila alterna `s.SpeckitContextDisabled`,
  escribe con `m.settingsRepo.Write`, y muestra `m.statusMsg` — mismo
  patrón exacto que el `case 0` (grafo de código externo) y `case 4`
  (sinapsis) ya implementados en `updateConfig()`.
- La fila se muestra **siempre**, exista o no `.specify/` en el proyecto
  (ver research.md #5) — sin lógica condicional de visibilidad.

## Compatibilidad hacia atrás

- Un `settings.json` sin la clave `speckit_context_disabled` se interpreta
  como `false` (activado) — mismo criterio `omitempty` que el resto de los
  campos `*Disabled`, sin migración necesaria.
