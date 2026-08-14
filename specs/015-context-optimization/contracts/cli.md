# Contrato CLI: `mem pack`

**Feature**: [../spec.md](../spec.md) · Ver decisión de nombre en [../research.md](../research.md) §8.

Sigue el patrón de subcomando de `mem session` (`cmd_session.go`): `CmdPack` despacha
por el primer argumento posicional. `mem context` (comando existente, resumen de
`get_context()`) no se toca.

## `mem pack build`

```text
mem pack build --task "<descripción de la tarea>" --max-tokens <N> [flags]

Flags:
  --task string           Obligatorio. Descripción de la tarea (alimenta el retrieval).
  --max-tokens int         Obligatorio. Presupuesto total, debe ser > 0.
  --project string        Opcional. Default: proyecto detectado por FindRoot().
  --min-relevance float    Opcional. Default: SettingsData / valor de fábrica.
  --max-items int          Opcional. Tope de candidatos antes de rankear.
  --no-speckit             Opcional. Desactiva IncludeSpecKit para esta llamada.
  --no-compress            Opcional. Compression=None en vez de Structural.
  --json                   Opcional. Emite el ContextPack como JSON en vez de texto plano.
```

**Salida (texto plano, default)**: el contenido concatenado de `ContextPack.Items` en
Markdown, en el mismo estilo que produce hoy `mem context` (encabezados por sección),
seguido de un bloque de estadísticas (ver `mem pack stats`).

**Salida (`--json`)**: serialización de `domain.ContextPack` completo (incluye
`Stats`), pensada para consumo programático desde otro proceso.

**Códigos de salida**: `0` en éxito. `1` en `ErrCriticalContextOverflow` (mensaje
explícito: qué items críticos no cupieron y por cuántos tokens se excedió). `1` en
`ErrInvalidContextRequest` (mensaje: qué campo falta o es inválido).

## `mem pack show`

```text
mem pack show <archivo.json>
mem pack show -   # lee de stdin
```

`ContextPack` es efímero — no queda guardado en disco entre invocaciones (ver
research.md §6). "Inspeccionar un paquete previamente construido" (FR-013) significa
reformatear en Markdown legible el JSON que `mem pack build --json` ya emitió y que la
persona guardó o encadenó por su cuenta, p. ej.:

```text
mem pack build --task "..." --max-tokens 4000 --json > pack.json
mem pack show pack.json
```

No hay estado oculto entre invocaciones: el mismo archivo `pack.json` siempre produce
la misma salida de `show`.

## `mem pack compress`

```text
mem pack compress <archivo>
mem pack compress -   # lee de stdin
```

Corre solo el paso de compresión estructural (sin retrieval, sin budget) sobre el
contenido de `<archivo>` (o stdin) y devuelve el resultado comprimido a stdout, más un
resumen de tokens antes/después a stderr (para no ensuciar la salida si se está
encadenando el contenido comprimido a otro comando).

## `mem pack stats`

```text
mem pack stats <archivo.json>
mem pack stats -   # lee de stdin
```

Mismo insumo que `mem pack show` (un `ContextPack` en JSON, propio o encadenado), pero
imprime solo el bloque `ContextStats`, en el formato:

```text
GoMemory Context Optimization

Raw tokens:          N
Final tokens:        N
Reducción:            NN.NN%
Ahorrados:            N

Items críticos:       N
Items relevantes:     N
Items opcionales:     N
Duplicados removidos: N
Descartados:          N
```
