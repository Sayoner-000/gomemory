# Quickstart: Reindexado dual de grafos de código + edición de huella de contexto en TUI

Guía de validación manual end-to-end. Referencias: [contracts/code-graph-indexer.md](./contracts/code-graph-indexer.md),
[data-model.md](./data-model.md).

## Prerrequisitos

- Compilar el binario con los cambios: `go build -o mem .` desde la raíz del repo.
- Un proyecto de prueba con `.memory/` inicializado (`mem init` si hace falta).
- Para los escenarios "CON binario": `codebase-memory-mcp` resuelto en `PATH` o vía el override
  de settings ya soportado por el proveedor.
- Para los escenarios "SIN binario": renombrar temporalmente el binario o apuntar el override a
  una ruta inexistente.

## Escenario 1 — CLI, con proveedor externo instalado (Historia 1, FR-001/002/005/006)

```bash
cd /ruta/al/proyecto/de/prueba
mem index
```

**Esperado**: bloque de indexado nativo Go idéntico al comportamiento actual, seguido de
`🔗 Indexando grafo externo (...)...` y una línea final con nodos/aristas reales. `echo $?` → `0`.

## Escenario 2 — CLI, sin proveedor externo instalado (FR-003)

```bash
mem index   # con el binario externo no resuelto
```

**Esperado**: bloque nativo Go intacto + línea informativa "grafo externo: ... no está instalado
en PATH, omitido". `echo $?` → `0`.

## Escenario 3 — CLI, omitir el grafo externo a propósito (FR-002)

```bash
mem index --skip-graph
```

**Esperado**: solo el bloque nativo Go; ninguna línea relacionada con el grafo externo.

## Escenario 4 — CLI, proveedor instalado pero el reindexado falla (FR-004)

Simular un fallo del proceso externo (por ejemplo, apuntar el override a un binario que devuelve
un JSON inválido o sale con error).

**Esperado**: bloque nativo Go intacto + línea `⚠️ grafo externo: ...` con el error. `echo $?` → `0`
(el fallo del grafo externo no hace fallar el comando).

## Escenario 5 — TUI, acción de reindexado con proveedor instalado (Historia 2, FR-007/008/009)

```bash
mem   # modo interactivo, en el proyecto de prueba
```

1. Navegar a Configuración.
2. Confirmar la fila "Reindexar grafo externo (codebase-memory-mcp)" al final del menú, sin
   desplazar las filas existentes.
3. Seleccionarla y presionar Enter.

**Esperado**: el status cambia de inmediato a "Indexando..."; la TUI sigue respondiendo a otras
teclas (navegar a otra pantalla y volver) mientras el proceso corre en segundo plano; al terminar,
el status muestra "Grafo externo reindexado: N nodos, M aristas".

## Escenario 6 — TUI, disparo repetido mientras hay uno en curso (FR-011)

Repetir el paso 3 del escenario 5 inmediatamente después del primer disparo, antes de que termine.

**Esperado**: no se inicia un segundo reindexado; el status deja claro que ya hay uno en curso.

## Escenario 7 — TUI, acción de reindexado sin proveedor instalado (FR-010)

Repetir el escenario 5 con el binario externo no resuelto.

**Esperado**: status = "codebase-memory-mcp no disponible", sin colgar la TUI ni intentar el
reindexado.

## Escenario 8 — TUI, editar los tres ajustes de huella de contexto (Historia 3, FR-012 a FR-017)

Para cada uno de Budget / CompactThreshold / DedupWindowDays:

1. En Configuración, seleccionar la fila del ajuste.
2. Confirmar que el input llega precargado con el valor actual.
3. Probar los tres casos:
   - Valor entero positivo → guarda, vuelve a Configuración, el resumen refleja el nuevo valor.
   - `0` → guarda igual (semántica: usa el valor por defecto).
   - Negativo → guarda igual (semántica: desactivación explícita).
4. Verificar en disco: `cat .memory/settings.json` refleja el nuevo valor tras cada guardado.

**Esperado**: los tres ajustes se editan de forma independiente, cada guardado no afecta a los
otros dos.

## Escenario 9 — TUI, validación de entrada inválida (FR-014)

En la pantalla de edición de cualquiera de los tres ajustes, ingresar un valor vacío, texto no
numérico, o un decimal (`3.5`), y confirmar.

**Esperado**: mensaje de error claro, el usuario permanece en la pantalla de edición, el valor
guardado no cambia.

## Escenario 10 — TUI, cancelar edición (FR-016)

En la pantalla de edición, cambiar el valor mostrado pero presionar Esc antes de confirmar.

**Esperado**: vuelve a Configuración sin guardar; `.memory/settings.json` no cambia.

## Verificación de regresión

```bash
go build ./...
go vet ./...
go test ./adapters/secondary/codegraph/codebasememory/...
go test ./adapters/primary/cli/...
go test ./adapters/primary/tui/...
go test ./...
git diff --stat   # confirmar que solo se tocaron los archivos listados en plan.md
```
