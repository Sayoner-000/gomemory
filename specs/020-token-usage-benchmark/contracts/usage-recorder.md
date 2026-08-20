# Contrato: puerto de registro de uso y etiqueta de canal

**Feature**: 020-token-usage-benchmark · **Fecha**: 2026-08-20

Contrato interno entre quien emite contexto y quien lo mide. Existe para que la medición sea
simétrica entre canales y para que añadir uno nuevo no obligue a tocar el núcleo.

---

## 1. El puerto

```go
// ports.UsageRecorder registra una emisión de contexto ya medida.
// Implementación fire-and-forget: NUNCA devuelve error (FR-006).
type UsageRecorder interface {
    Record(operation string, baselineTokens, emittedTokens int)
}
```

### Por qué la firma no lleva canal

La etiqueta de canal **se fija al construir el grabador**, no en cada llamada. Cada proceso de
gomemory es exactamente un canal: el servidor MCP es un proceso, `mem context` es otro, la interfaz
interactiva es otro. Con la etiqueta en la construcción se consigue lo que exige FR-003: el caso de
uso reporta lo que sabe —qué operación hizo, cuánto habría costado, cuánto costó— y no necesita
conocer un concepto que no le pertenece.

### Por qué no devuelve error

FR-006: medir nunca puede impedir emitir. El grabador traga cualquier fallo de persistencia. Quien
emite no tiene nada que hacer con ese error y no debe verse obligado a manejarlo.

### Nil es un valor válido

El campo es opcional en todas las dependencias. Con el grabador en nulo, todo emisor funciona
exactamente igual, sin medición. Esto mantiene compilando y pasando el wiring y los tests
existentes que no lo configuran.

---

## 2. Quién llama a `Record`

| Llamador | Cuándo | De dónde saca las cifras |
|---|---|---|
| Construcción de contexto | Al terminar de construir la salida | Contadores acumulados en los puntos de descarte |
| Búsqueda y listado de memorias | Tras aplicar la divulgación progresiva | Contenido íntegro frente a lo entregado |
| Construcción de paquete de contexto | Tras calcular sus estadísticas | Las cifras de reducción que ya calcula |
| Compresión de paquete | Tras comprimir | El resultado de compresión que ya produce |
| **Middleware de canal** | Solo si nadie reportó esa llamada | Longitud del texto emitido, con línea base igual (FR-005) |

**Regla de precedencia**: cuando un caso de uso ya reportó una llamada, el middleware **no** vuelve
a registrarla. La marca de «ya reportado» viaja en el contexto de la petición; el middleware la
consulta antes de registrar su respaldo. Sin esta regla, las operaciones que optimizan quedarían
contadas dos veces y el porcentaje global saldría inflado.

**Por qué existe el respaldo del middleware**: sin él, solo se registrarían las operaciones que
recortan, y el porcentaje de reducción se calcularía sobre un subconjunto sesgado. Guardar una
memoria no ahorra nada, pero ocupa contexto: debe aparecer, con línea base igual a lo emitido y
ahorro cero (FR-005).

---

## 3. La etiqueta de canal

```go
// adapters/secondary/usage.NewRecorder construye un grabador atado a un canal.
func NewRecorder(
    repo    ports.UsageRepository,
    project string,
    channel string,          // etiqueta libre. NO se valida.
    session func() string,   // id de sesión activa, o cadena vacía
) ports.UsageRecorder
```

> Sin `TokenCounter`: `Record()` ya recibe los tokens calculados por quien emite (el caso de uso
> decide cómo convertir caracteres a tokens, con el único contador del proyecto). El grabador no
> vuelve a contar nada — solo etiqueta y persiste.

| Regla | Detalle |
|---|---|
| **No se valida** | `channel` es texto libre. No hay lista de valores permitidos, ni en el código ni en el esquema de la base (`TEXT NOT NULL`, sin `CHECK`) |
| **Es descriptiva, no autorizadora** | Un canal que el sistema no conozca se registra con su etiqueta y aparece en el reporte, igual que los conocidos (FR-004) |
| **Valores actuales** | `mcp`, `cli`, `tui`. La lista es de hecho, no de derecho: no está codificada en ninguna parte como conjunto cerrado |
| **Dónde se fija** | Solo en el composition root (`infrastructure/container.go`), según qué comando se está ejecutando |

---

## 4. Nombres de operación

`operation` nombra la operación **de dominio**, no la función de un protocolo. Constantes del
dominio:

```
build_context · search_memories · list_memories · get_memory
build_pack · compress_pack · plan_context · save_memory · other
```

Un canal que exponga la misma operación con otro nombre traduce **en el adaptador**. El dominio no
conoce el vocabulario de ningún canal. `other` es el destino de lo que el middleware registra como
respaldo y no sabe clasificar: es un valor legítimo, no un error.

---

## 5. Criterio de agnosticismo verificable

Añadir un canal emisor nuevo debe requerir **exactamente un cambio**: construir el grabador con
otra etiqueta en el composition root.

Estos cinco archivos **no se tocan** al añadir un canal:

```
domain/usage.go
application/ports/usage_repository.go
application/ports/usage_recorder.go
application/usecases/build_usage_report.go
adapters/primary/cli/cmd_usage.go   (el formateador)
```

Si alguno hiciera falta modificarlo, el cableado está mal: la capacidad volvió a definirse en el
formato de un canal, que es justamente lo que este contrato existe para impedir. Se verifica
emitiendo por el canal de línea de comandos con el servidor MCP apagado y comprobando que la
emisión aparece en `mem usage` etiquetada `cli`.
