# Quickstart — validación de la feature 020

**Feature**: 020-token-usage-benchmark · **Fecha**: 2026-08-20

Guía para comprobar que la feature funciona **de verdad**, no solo en verde. Sigue la regla de campo
número 2 del proyecto: un test vale lo que vale su fixture, así que la validación decisiva es contra
el binario y la base reales, no contra mocks.

## Prerrequisitos

```bash
cd /Users/josegomezj/home/rcw/go_memory
go build -o mem .          # el repositorio no versiona el binario
```

---

## V1 · Puerta de calidad

```bash
go build ./... && go vet ./... && go test ./...
go test -cover ./domain/... ./application/... ./adapters/secondary/persistence/... ./adapters/secondary/usage/...
```

**Esperado**: todo en verde y cobertura ≥ 80 % en los paquetes nuevos (Principio III).
Ningún test preexistente debe aparecer como modificado en el diff.

---

## V2 · La migración es idempotente

```bash
cp ~/.local/share/gomemory/projects/<proyecto>/mem.db /tmp/mem-antes.db
./mem migrate && ./mem migrate          # dos veces seguidas
sqlite3 ~/.local/share/gomemory/projects/<proyecto>/mem.db ".schema usage_records"
sqlite3 /tmp/mem-antes.db "SELECT COUNT(*) FROM memories;"
sqlite3 ~/.local/share/gomemory/projects/<proyecto>/mem.db "SELECT COUNT(*) FROM memories;"
```

**Esperado**: la segunda corrida termina sin error, `usage_records` existe con su índice, y el conteo
de memorias es idéntico antes y después (SC-011).

---

## V3 · Medición en vivo contra el servidor MCP

No basta con los tests. Reiniciar el servidor, ejecutar una secuencia **conocida** y contrastar.

1. Reiniciar el servidor MCP.
2. Ejecutar, en este orden: `get_context`, `search_memories` (dos veces), `pack_build`.
3. Comprobar:

```bash
./mem usage
```

**Esperado**: cuatro llamadas con desglose por operación, y línea base mayor que lo emitido en las
tres primeras (SC-001). El desglose por canal muestra `mcp`.

---

## V4 · La salida legible por máquina cuadra

```bash
./mem usage --json | python3 -c '
import json,sys
r = json.load(sys.stdin)
assert r["baseline_tokens"] - r["saved_tokens"] == r["emitted_tokens"], "G1 rota"
assert sum(b["baseline_tokens"] for b in r["by_operation"]) == r["baseline_tokens"], "G3 rota"
assert sum(b["calls"] for b in r["by_channel"]) == r["calls"], "G4 rota"
assert (r["window_ratio"] is None) == (r["window_tokens"] == 0), "G5 rota"
print("garantias G1..G5 OK · contract_version =", r["contract_version"])'
```

**Esperado**: imprime la línea de confirmación. Ver [contracts/usage-report.md](./contracts/usage-report.md).

---

## V5 · La pantalla de la interfaz interactiva

```bash
./mem tui
```

1. Pulsar `u` → se abre la pantalla de uso.
2. **Sección [1]**: debe coincidir cifra por cifra con `./mem usage` para la misma sesión (SC-006).
3. **Sección [2]**: escribir una tarea y un presupuesto, disparar el cálculo, ver tokens antes y
   después, porcentaje y conteo por importancia.
4. Salir con `esc` y volver a entrar con `u`: **el snapshot no debe reaparecer** (SC-007, FR-023).
5. Probar tarea vacía y presupuesto no entero: mensaje de validación claro, sin disparar el cálculo.
6. Probar un presupuesto ridículamente bajo: mensaje comprensible, no un error crudo.

---

## V6 · El costo de los descriptores cuadra con lo publicado

```bash
./mem usage --json | python3 -c 'import json,sys; r=json.load(sys.stdin); print(r["schema_operations"], r["schema_tokens"])'
grep -c "AddTool(" adapters/primary/cli/cmd_mcp.go adapters/primary/cli/cmd_mcp_code_tools.go
```

**Esperado**: `schema_operations` coincide con el número real de operaciones publicadas —hoy 19— y
`schema_tokens` es mayor que cero (SC-004). Si mañana se añade una operación número veinte, esta
comprobación debe seguir cuadrando **sin tocar código de medición**.

---

## V7 · Agnosticismo: el canal de línea de comandos, sin servidor MCP

Esta es la validación que protege la invariante central de la feature.

```bash
pkill -f "mem mcp" || true          # asegurarse de que NO hay servidor MCP
./mem context > /dev/null            # emitir por el canal de línea de comandos
./mem usage --json | python3 -c '
import json,sys
r=json.load(sys.stdin)
canales = {b["key"] for b in r["by_channel"]}
assert "cli" in canales, f"el canal cli no aparece: {canales}"
print("canal cli registrado OK")'
```

Y comprobar el diff del trabajo:

```bash
git diff --stat -- domain/usage.go application/ports/usage_repository.go \
    application/ports/usage_recorder.go application/usecases/build_usage_report.go \
    adapters/primary/cli/cmd_usage.go
```

**Esperado**: la emisión aparece etiquetada `cli`. Si para lograrlo hubo que modificar alguno de esos
cinco archivos, **la desviación volvió a colarse** y hay que rehacer el cableado (SC-005).

---

## V8 · La ventana de referencia

```bash
./mem usage | grep -i "ventana" ; echo "salida: $?"      # con el valor por defecto (0)
```

**Esperado**: no encuentra nada. Con el valor por defecto, **todo lo impreso es medido** (SC-003).

Luego, configurando una ventana mayor que cero en los ajustes:

```bash
./mem usage | grep -i "estimado"
```

**Esperado**: aparece una única línea, rotulada `(estimado)` (FR-015).

---

## V9 · Fase B — el Δ de la consolidación

Antes de consolidar, tomar la línea base:

```bash
./mem context > /dev/null && ./mem usage --json | python3 -c 'import json,sys; print("antes:", json.load(sys.stdin)["baseline_tokens"])'
```

Previsualizar, aplicar y volver a medir:

```bash
./mem gc consolidate                 # previsualización: NO debe modificar nada
sqlite3 <db> "SELECT COUNT(*) FROM memories;"     # igual que antes de previsualizar
./mem gc consolidate --apply
./mem context > /dev/null && ./mem usage --json | python3 -c 'import json,sys; print("despues:", json.load(sys.stdin)["baseline_tokens"])'
```

**Esperado**: la previsualización no cambia el conteo de filas; tras aplicar, queda una sola memoria
por grupo sin pérdida de contenido, y la línea base baja de forma verificable (SC-008).

> **Contexto que conviene tener presente**: medido sobre la base real, consolidar **solo** por clave
> de tópico da Δ cero —hay 0 grupos con más de una fila—. El Δ observable proviene de los registros
> automáticos de actividad, de los que el 55 % de los recientes son byte a byte idénticos. Ver
> [research.md §5](./research.md).

---

## V10 · Fase C — el modo índice

Con el modo índice activo:

```bash
./mem context | grep -c "get_memory"        # debe haber una entrada por memoria
./mem context | head -40                     # el protocolo debe verse íntegro
```

**Esperado**: aparecen todos los identificadores y ningún cuerpo de memoria (SC-009); el protocolo de
trabajo se emite completo, sin recortes (FR-032).

Reversibilidad:

```bash
./mem context > /tmp/completo-antes.txt      # modo completo
# activar modo índice, luego desactivarlo
./mem context > /tmp/completo-despues.txt
diff /tmp/completo-antes.txt /tmp/completo-despues.txt
```

**Esperado**: sin diferencias (SC-010).

---

## V11 · La librería de la interfaz interactiva

```bash
go get github.com/charmbracelet/bubbletea@v1.3.10
go get github.com/charmbracelet/bubbles@v1.0.0
go get github.com/charmbracelet/lipgloss@v1.1.0
go mod tidy && go build ./... && go test ./...
```

**Esperado**: compila y la suite pasa **sin haber modificado ninguna prueba existente** (SC-012).
Después, recorrer a mano las pantallas previas —lista, detalle, guardar, mantenimiento,
configuración, importar, optimizar— y confirmar que ninguna cambió de aspecto ni de comportamiento
(FR-039).

> No se migra a `bubbletea/v2`: cambia la ruta del módulo y rompe la interfaz de programación.
> Declarado fuera de alcance en la spec y justificado en Complexity Tracking de
> [plan.md](./plan.md).

---

## Resumen de trazabilidad

| Validación | Cubre |
|---|---|
| V1 | Principio III, cobertura |
| V2 | FR-009, SC-011 |
| V3 | FR-001..FR-005, SC-001 |
| V4 | FR-012, SC-002, garantías G1–G5 |
| V5 | FR-018..FR-025, SC-006, SC-007 |
| V6 | FR-007, SC-004 |
| V7 | FR-003, FR-004, FR-017, SC-005 |
| V8 | FR-013..FR-016, SC-003 |
| V9 | FR-026..FR-030, SC-008 |
| V10 | FR-031..FR-035, SC-009, SC-010 |
| V11 | FR-038, FR-039, SC-012 |
