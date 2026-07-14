# Quickstart — Validación end-to-end

Guía para **demostrar que la feature funciona** contra el binario real, no solo
en unit tests (regla de campo #2). Referencias: [contracts/mcp-tools.md](./contracts/mcp-tools.md),
[data-model.md](./data-model.md).

## Prerrequisitos

- Go >= 1.22, proyecto compilable: `go build -o mem .`
- Un proyecto con **≥100 memorias** (para ver el efecto del presupuesto). Si no
  lo hay, usar la BD real del propio gomemory (ya tiene >100).

## Escenario 1 — get_context bajo presupuesto (P1, SC-001)

```sh
# Baseline: tamaño actual del contexto
./mem context | wc -c        # baseline medido = 85888 (~86KB)

# Tras implementar, con Budget por defecto (~6k tokens):
./mem context | wc -c        # esperado ≤ ~24000
./mem context | grep -c "get_memory"   # esperado > 0 (punteros a detalle)
./mem context | grep -i "Conflictos"   # los conflictos siguen presentes
```
**Éxito**: salida ≤ techo, con punteros `get_memory <id>`, protocolo y conflictos
intactos. Con `Budget=0` en settings.json ⇒ salida idéntica a la baseline.

## Escenario 2 — search_memories compacto (P2, FR-005)

```sh
# Antes: volcaba el content íntegro. Después: extracto + id.
./mem search "arquitectura" | head -30
```
**Éxito**: cada resultado es `[id] tipo | título` + extracto corto (no el cuerpo
completo). Verificar contra el MCP en ejecución con un cliente MCP real, no solo
el CLI.

## Escenario 3 — get_memory íntegro (capa 3)

```sh
./mem get <id>    # o get_memory por MCP
```
**Éxito**: devuelve el `content` completo sin truncar.

## Escenario 4 — dedup/upsert en la fuente (P4, SC-007)

```sh
# Contar filas antes
sqlite3 <ruta-mem.db> "SELECT count(*) FROM memories WHERE project=?"

# Guardar 3 veces una memoria equivalente (mismo tipo+título)
./mem save -t "Título repetido" -y decision "contenido A"
./mem save -t "Título repetido" -y decision "contenido A"
./mem save -t "Título repetido" -y decision "contenido A"

# Contar filas después
sqlite3 <ruta-mem.db> "SELECT count(*) FROM memories WHERE project=?"
```
**Éxito**: el conteo sube en **1**, no en 3. La fila consolidada tiene
`updated_at` reciente.

## Escenario 5 — recordatorio neutral de compactación (P3, FR-008/FR-011)

```sh
# Simular huella emitida por encima del umbral en una sesión y correr el hook:
./mem hook turn-end        # (o nudge, según integración)
```
**Éxito**: cuando la huella emitida supera `CompactThreshold`, aparece **una vez**
un mensaje neutral que sugiere compactar el contexto **sin nombrar `/compact` ni
comando de ningún agente**. Por debajo del umbral: sin mensaje. Segundo turno sin
cruzar un nuevo umbral: sin repetición (debounce).

## Escenario 6 — agnosticismo (SC-006)

```sh
# El acotado ocurre en capas compartidas: misma salida por CLI y por MCP.
diff <(./mem context) <(<cliente-mcp> get_context)
```
**Éxito**: el texto acotado es equivalente por ambas vías (el acotado no depende
del cliente).

## Comprobaciones de no-regresión

- `go test ./...` verde (tests existentes intocables).
- `go build` produce binario autocontenido.
- Ninguna memoria persistida fue borrada ni mutada por generar contexto
  (comparar conteo/contenido de `memories` antes/después de `./mem context`).
