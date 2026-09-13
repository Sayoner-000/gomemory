# Quickstart: validar la feature 031 de extremo a extremo

Guía de validación. No contiene implementación. Los formatos esperados están en
[contracts/](contracts/) y las reglas en [data-model.md](data-model.md).

## Prerrequisitos

- Go 1.27 y golangci-lint v2.13.2 (memoria 172: v1.64.8 no sirve con Go 1.27).
- Almacén real: `~/.local/share/gomemory/projects/gomemory-72eb21d7fd6b9e68/mem.db`. Todas las consultas directas son `sqlite3 -readonly`.
- Prerrequisito P0 cerrado (research.md R1): la anomalía 207/209 está explicada y, si hubo defecto, corregida en su propio commit.

## V1. Calidad estática y tests

```bash
gofmt -l . && go vet ./...
golangci-lint run ./...
go test ./...
go test -race ./adapters/secondary/persistence/... ./application/...
```

Esperado: sin salida de `gofmt`, `vet`/`lint` limpios y todos los tests en verde.

## V2. El artefacto servido es el nuevo

```bash
go build -o /tmp/mem-031 . && /tmp/mem-031 version
which mem && mem version        # binario que usa el MCP
```

El servidor MCP en ejecución sigue sirviendo el binario anterior hasta que se
reinicie. Instala el nuevo, reinicia el cliente y confirma que `mem version`
coincide ANTES de V3–V5 (regla de trabajo 3).

## V3. Calibración del gate (antes de fijar las constantes)

```bash
GOMEMORY_CALIBRATION_DB=~/.local/share/gomemory/projects/gomemory-72eb21d7fd6b9e68/mem.db \
  go test -tags calibration -run TestSaveGateCalibration -v ./tests/integration/
```

Esperado: la tabla `umbral × avisos (%) × recall`. Los umbrales elegidos
cumplen ≤ 10 % de avisos y el 100 % de los pares confirmados (SC-002), o la
persona decide el compromiso. El mtime de `mem.db` no cambia.

## V4. Gate contra el sistema real (US1)

1. Con el MCP reiniciado: `save_memory` con el título, tipo y contenido de la memoria 209. Esperado: `✓ Memoria guardada (id=X)` y `⚠ Posible duplicado de #207` (o `#209`). La memoria X existe.
   - Si X coincide con 209 (upsert por título exacto), es correcto que no haya aviso. En ese caso, repite con el título ligeramente cambiado para ejercitar el gate.
2. `save_memory` con un tema nuevo. Esperado: sin líneas `⚠`.
3. Borrar las memorias de prueba con `forget_memory`, **solo con la aprobación de la persona**.

## V5. Evidencia de anclas (US2)

```bash
mem context | sed -n '/## 🧭 Anclas sin evidencia/,/^## /p'
```

Para cada ruta listada, confirma a mano que `ls <ruta>` falla. Esperado: 0
falsos positivos, y la memoria 148 (ruta absoluta fuera del repo) no aparece.

## V6. Masa (US3, US4)

```bash
mem mass --top 15 > /tmp/m1; mem mass --top 15 > /tmp/m2; diff /tmp/m1 /tmp/m2 && echo idénticas
mem mass --task "sinapsis cache" --top 10
mem context | sed -n '/## 🔗 Sinapsis/,/^## /p'
```

Esperado:

- Salidas idénticas y ningún checkpoint en el ranking.
- Con la tarea (SC-006 enmendado), aparecen memorias enlazadas a los resultados que la búsqueda sola no devuelve, como la 155 vía la 154. Las que no tienen relaciones (197, 200) quedan con su cuota de semilla.
- La sección Sinapsis ya no muestra `[178..180] ↔ 172` y termina con la leyenda de masa.

Pack:

```bash
mem pack build --task "sinapsis cache" --max-tokens 4000 --json | jq '[.items[] | select(.priority=="optional") | .id]'
```

Esperado: aparecen vecinos `memory:<N>` que la búsqueda sola no devolvía (como mucho 5).

## V7. Cierre

- Guardar con `save_memory` los umbrales medidos y las reglas de aristas, como decisiones.
- `end_session` con el resumen.
