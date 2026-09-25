# Contrato: CLI y herramientas MCP de compresión

Toda la superficie nueva vive bajo `mem pack`, el namespace que ya existe
(`build | show | compress | stats`). Las herramientas MCP siguen el prefijo
`pack_`.

## CLI

### `mem pack compress [archivo|-] [--level none|structural|max] [--compare] [--json]` *(ampliado)*
- Sin `--level`: usa `context_compression_level` del proyecto.
- Salida en texto: el contenido comprimido **byte a byte por stdout**; la línea
  `tokens: <raw> → <final> (<compresor>, -<pct>%)` va por **stderr**, precedida
  de un salto de línea si el contenido no termina en uno. Corrige el defecto
  R0 (la línea salía pegada a la última del contenido) sin ensuciar stdout.
- `--compare`: sin contenido. Imprime una tabla con las tres salidas:
  ```
  nivel        tokens   ahorro   compresor
  ninguna      18788    —        none
  estructural  18062    3.9%     structural
  máxima        4210   77.6%     json
  ```
- `--json`: `{"content", "raw_tokens", "structural_tokens", "tokens", "compressor", "content_type", "refs": [], "fallback_reason"}`.
- Código de salida 0 incluso si hubo degradación (FR-006). Solo es distinto de
  0 ante un error de E/S del archivo de entrada.

### `mem pack retrieve <ref>` *(nuevo)*
- Imprime el original **byte a byte** por la salida estándar, sin añadir nada.
- Una ref desconocida o caducada devuelve el código 2 y este mensaje en
  stderr: `ref <ref> no encontrada o caducada; vuelve a pedir el contenido a su fuente`.
- Suma una recuperación a `(compressor, content_type)` del original (FR-027).

### `mem pack savings [--json]` *(nuevo)*
Informe por compresor del proyecto actual (FR-026):
```
compresor   usos   tokens antes → después   ahorro   recuperación   degradaciones   latencia media
json         42    310 204 → 61 877          80.1%    3.1%           1               2.4 ms
log          17     88 010 → 29 544          66.4%    0.0%           0               1.1 ms
…
Ajustes adaptativos: code/python agresividad 2 (recuperación 34% sobre 41 omisiones, 2026-09-30)
Originales: 18.2 MB de 256 MB · 311 refs · caducan a los 7 días sin uso
```

### `mem pack tune --reset [--type <content_type>]` *(nuevo)*
Restablece la agresividad a 3 para todos los tipos o para uno.

### `mem pack purge` *(nuevo)*
Borra los originales caducados y aplica el tope. Imprime `purgados N (X MB)`.

### `mem settings` *(ampliado)*
Hoy solo acepta `--auto-approve` y `--show` (verificado en 2.25.0). Se añaden:
`--compression-level=none|structural|max`, `--tool-output-compression=true|false`
y `--concise-output=true|false`. `--show` muestra los tres. Escribe los campos
en `SettingsData` **y** en el `Settings` de persistencia.

### `mem doctor` *(ampliado)*
Sección «Compresión» (FR-029), también en `--json` bajo la clave `compression`:
- nivel activo y su origen (ajuste, heredado o de fábrica);
- espacio de originales usado frente al tope; aviso a partir del 90 %;
- enganche de salidas de herramientas por runtime: `activo | inactivo | parcial (solo MCP) | no soportado`;
- ajustes adaptativos vigentes;
- con `--strict`, código distinto de 0 si el almacén no se puede escribir con el nivel en `max`.

## Herramientas MCP

### `pack_compress` *(ampliada, compatible hacia atrás)*
Entrada: `{ "text": string, "level"?: "none"|"structural"|"max", "compare"?: bool }`.
Salida: el texto comprimido (sin añadir nada, compatible con los clientes
actuales) y el resultado estructurado (`CompressionResult`: tokens por etapa,
compresor, refs y motivo de degradación). Con `compare=true`, la tabla
comparativa. *(Ajustado durante la implementación: no se añade la línea de
tokens al texto para no cambiar la salida de los clientes existentes.)*
Sin `level`, usa el ajuste del proyecto. Los clientes actuales, que solo
envían `text`, siguen funcionando.

### `pack_retrieve` *(nueva)*
Entrada: `{ "ref": string }`. Salida: el original exacto, o el error MCP
`ref no encontrada o caducada`.
Se auto-aprueba igual que `get_memory`, porque es de solo lectura: se añade a
`domain.MCPAutoApprovableToolsFor`.

### `pack_savings` *(nueva)*
Entrada: `{}`. Salida: el mismo informe que `mem pack savings`.

### Herramientas existentes afectadas (sin cambios de firma)
`get_context`, `get_plan_context`, `search_memories`, `search_code`,
`pack_build` y `octopus_*` (paquete delegado): su salida pasa por el motor
según el nivel. En `max` incluye deltas de sesión y marcadores.

## Línea de recuperación (nivel `max`)
*(Ajustado durante la implementación.)* En vez de condicionar el protocolo
—que se reproduce en hooks, plantillas y en el plugin de OpenCode— la línea va
**al final de cada salida que lleva marcas** (`get_context`, `mem context`,
búsquedas, contexto de arranque y de post-compact):
```
> Marcas ⟦mem⟧ … ref=X: contenido omitido para ahorrar tokens; recupéralo íntegro con pack_retrieve(ref=X) (CLI: mem pack retrieve X).
```
Solo aparece cuando hace falta, no altera el prefijo estable y es agnóstica al
agente. La descripción de la herramienta `pack_retrieve` repite la instrucción.

## Registro de uso existente
Cada compresión sigue registrándose en `usage_records` con
`domain.OpCompressPack` (o la operación de origen), así que `mem usage` refleja
el ahorro sin cambios en su contrato
([USAGE-REPORT-CONTRACT](../../../docs/USAGE-REPORT-CONTRACT.md)).
