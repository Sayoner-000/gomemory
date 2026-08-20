# Fase 0 — Investigación: benchmark de tokens por sesión

**Feature**: 020-token-usage-benchmark · **Fecha**: 2026-08-20

Este documento resuelve las incógnitas técnicas del plan. Todo lo que aquí se afirma sobre el
código o sobre los datos se comprobó ejecutándolo, no leyendo el brief de entrada. Donde el brief y
la realidad discrepan, se dice cuál era la afirmación y qué se midió.

---

## 1. Dónde nace el registro, y cómo entra la etiqueta de canal

**Decisión**: el caso de uso llama a `ports.UsageRecorder.Record(operación, línea base, emitido)`.
La etiqueta de canal **no viaja en la llamada**: se fija al construir el grabador, en el
composition root, según qué proceso se está ejecutando.

**Razón**: cada proceso de gomemory es exactamente un canal. El servidor MCP es un proceso
(`mcp`), `mem context` es otro (`cli`), la interfaz interactiva es otro (`tui`). Fijar la etiqueta
en la construcción tiene tres consecuencias que interesan:

1. La firma del puerto queda libre de cualquier concepto de canal, que es lo que exige FR-003.
2. Añadir un canal emisor nuevo es construir el grabador con otra etiqueta en el composition root:
   **cero líneas** en `domain/`, en el puerto, en el caso de uso y en el formateador. Ese es
   literalmente el criterio de FR-017 y SC-005.
3. Como la etiqueta es un `string` que nadie valida, un canal desconocido se registra igual y
   aparece en el reporte (FR-004).

**Alternativas consideradas**:

- *Pasar el canal en cada llamada a `Record`*. Rechazada: obliga a cada caso de uso a conocer un
  concepto que no le pertenece, y reintroduce por la puerta de atrás la asimetría que FR-003
  quiere evitar.
- *Registrar en el middleware MCP, como proponía el brief*. Rechazada como mecanismo principal:
  deja fuera toda emisión que no pase por MCP. El middleware conserva un papel, pero secundario
  (ver §4).

**Hallazgo que condiciona el diseño**: `ports.ContextBuilder` solo expone `Build() (string, error)`
y `WriteFile() error`. No hay por dónde devolver estadísticas sin ampliar el puerto, y ampliarlo
obligaría a tocar a los cuatro llamadores existentes (`cmd_context.go:29`, `cmd_mcp.go:302`,
`cmd_hook.go:109`, `:201`, `:827`). Por eso el registro ocurre **dentro** de `Build()`, con el
grabador como campo opcional del `Builder`: el puerto no cambia y ningún llamador se entera.

---

## 2. Dónde vive la línea base dentro de la construcción de contexto

**Decisión**: dos contadores de caracteres en el `Builder`, incrementados en los dos únicos puntos
donde hoy se descarta contenido.

**Verificado en el código**:

- `Builder.acota()` (`build_context.go:122`) trunca el contenido a un extracto de 200 caracteres y
  le adosa el puntero `get_memory <id>` cuando hay presupuesto. Ahí la línea base es
  `len(m.Content)` íntegro, y lo emitido es lo que devuelve.
- `Builder.fits()` (`build_context.go:135`) decide si una línea cabe bajo el techo, con una reserva
  de 300 caracteres. Cuando devuelve `false`, ese contenido no se emite pero **sí cuenta** como
  línea base: es justamente lo que se ahorró.
- Hay un tercer punto que el brief no menciona y que resultó ser el más grande de todos: el tope
  `i >= 5` de la sección de actividad automática (`build_context.go:302`). Descarta 75 de 80
  registros cargados. Ver §5.

Contar en caracteres y convertir a tokens al final —con el único contador del proyecto,
`tokens.ApproximateTokenCounter`— evita construir el texto dos veces y mantiene una sola fuente de
verdad para el conteo (FR-008).

---

## 3. Cómo medir el costo de los descriptores que gomemory publica

**Decisión**: conectar un cliente al servidor real por un transporte en memoria, pedirle la lista
de operaciones, serializar cada descriptor a JSON y contarlo con el contador del proyecto.

**Razón**: es lo único que mide *lo que el servidor efectivamente publica*, y sigue siendo correcto
sin tocar nada cuando se añada una operación número veinte.

**Por qué se descartó lo que proponía el brief**: la tarea 3.1 del brief pedía «extraer las 19
llamadas `mcp.AddTool` a una función que devuelva los descriptores». Eso **no es viable** con el
SDK en uso. `mcp.AddTool` es una función genérica cuyo parámetro de tipo es una struct anónima
distinta en cada una de las 19 llamadas, y el segundo argumento es una clausura que captura `deps`
y `project`. No existe un tipo común en el que meterlas, y el esquema JSON no está escrito a mano:
lo infiere el SDK a partir de las etiquetas `jsonschema` de esas structs anónimas. Replicarlo a
mano produciría un número que se desincroniza del real en cuanto alguien edite una descripción.

**Verificado en el SDK** (`go-sdk@v1.6.1`):

- `(*Server).listTools` no está exportado, así que no se puede llamar directamente.
- `mcp.NewInMemoryTransports()` sí está exportado (`mcp/transport.go:147`) y permite levantar un
  par cliente/servidor sin tocar stdio ni abrir procesos.
- `mcp.Tool` (`mcp/protocol.go:1295`) lleva `Name`, `Description`, `InputSchema` y `OutputSchema`;
  serializarlo produce exactamente el descriptor que viaja al cliente.

**Recuento verificado**: 19 operaciones publicadas — 14 en `adapters/primary/cli/cmd_mcp.go` y 5 en
`adapters/primary/cli/cmd_mcp_code_tools.go`. La cifra del brief era correcta.

**Límite declarado**: se mide lo que gomemory publica. Cómo un cliente concreto empaqueta esos
descriptores en su propia solicitud es asunto del cliente, y no se presume.

---

## 4. Qué papel le queda al middleware MCP

**Decisión**: el middleware deja de ser la autoridad del registro y pasa a cubrir un hueco
concreto: las operaciones que **no** optimizan.

**Razón**: FR-005 exige que las operaciones sin optimización queden registradas con línea base
igual a lo emitido, para que el porcentaje global no se calcule sobre un subconjunto sesgado —solo
las que recortan—. Guardar una memoria no tiene línea base propia, pero sí ocupa contexto en la
respuesta. El middleware ya intercepta toda llamada y ya suma `callToolResultTextLen(res)`
(`cmd_mcp.go:46-56`) para la huella en caracteres de la feature 008; añadir ahí el registro de
respaldo no cuesta un mecanismo nuevo.

**Riesgo de doble conteo y su resolución**: una operación que el caso de uso ya reportó no debe
volver a registrarse en el middleware. Se resuelve con una marca por llamada, en el contexto de la
petición: el caso de uso la deja puesta al reportar, y el middleware solo registra si no está. El
contrato queda en [contracts/usage-recorder.md](./contracts/usage-recorder.md).

**Convivencia**: el contador de huella en caracteres (`.memory/.footprint`) **no se toca**. Lo
consumen el aviso de compactación y el enganche de fin de turno. El benchmark en tokens es una capa
al lado, no un reemplazo (FR-037).

---

## 5. La premisa de la fase B es falsa en los datos reales

Este es el hallazgo más importante de la fase 0, y cambia el blanco de una historia completa.

**Lo que decía el brief**: «el upsert por `topic_key` solo actualiza la última fila con ese tópico;
las N previas quedan vivas → esa es la fase B».

**Lo que se midió** contra la base real del proyecto
(`~/.local/share/gomemory/projects/go_memory-71967c70724078fd/mem.db`):

| Medición | Resultado |
|---|---|
| Memorias totales | 427 |
| Memorias con clave de tópico | 10 |
| **Grupos de clave de tópico con más de una fila** | **0** |

El mecanismo funciona. `findDuplicate` (`persistence/memory.go:317`) busca por clave de tópico,
toma la fila más reciente y la actualiza; no deja huérfanas. Consolidar por clave de tópico sobre
esta base **no eliminaría ninguna fila y el Δ sería exactamente cero**, con lo que FR-030 y SC-008
—que exigen una reducción *verificable*— no podrían cumplirse.

**Lo que sí se acumula**, midiendo las 100 memorias que `Builder.Build()` carga:

| Tipo | Filas cargadas | Caracteres | Participación |
|---|---:|---:|---:|
| **checkpoint** (actividad automática) | **80** | **103 022** | **69 %** |
| decision | 11 | 21 124 | 14 % |
| architecture | 3 | 10 614 | 7 % |
| discovery | 3 | 8 355 | 6 % |
| bugfix | 2 | 6 075 | 4 % |
| preference | 1 | 229 | <1 % |

Y sobre la redundancia dentro de ese 69 %:

| Medición | Resultado |
|---|---|
| Registros de actividad entre los 20 más recientes | 20 |
| De ellos, con contenido **distinto** | **9** |
| **Redundancia literal** | **55 %** |
| Caracteres de los 5 que sí se emiten | 8 888 (37 % del presupuesto de 24 000) |

**Causa raíz**: `findDuplicate` excluye deliberadamente los checkpoints del dedup por identidad,
con este comentario en el código: *«NUNCA aplica a checkpoints (su contenido varía por turno)»*.
Los datos contradicen esa suposición: más de la mitad son byte a byte idénticos. Dos enganches
distintos —`recordActivityCheckpoint(deps, "Checkpoint automático")` (`cmd_hook.go:402`) y
`recordActivityCheckpoint(deps, "Checkpoint de subagente")` (`cmd_hook.go:442`)— registran la misma
actividad del mismo turno y producen filas gemelas.

**Decisión**: la fase B conserva FR-026 —consolidar por clave de tópico sigue implementándose,
íntegro— y **añade un segundo criterio de agrupación**: registros de actividad con contenido
idéntico dentro de un proyecto. Ambos criterios corren bajo la misma operación, con la misma
previsualización obligatoria y el mismo camino de aplicación. Con eso, el Δ que exige FR-030 pasa a
ser medible en vez de nulo.

**Alternativas consideradas**:

- *Implementar solo la clave de tópico, como decía el brief.* Rechazada: entrega una función
  correcta que no ahorra nada y una historia que no puede cumplir su propio criterio de éxito.
- *Atacar solo la causa raíz, evitando escribir el checkpoint duplicado en el enganche.* Es una
  buena idea y debería hacerse, pero no basta: no toca las 352 filas ya escritas, que son las que
  hoy pesan sobre el contexto. Se recomienda como feature aparte; esta consolida lo existente.
- *Subir el tope de 5 registros de actividad, o bajarlo.* Rechazada: mueve el síntoma sin tocar la
  redundancia, y toca un comportamiento que ninguna historia pidió cambiar.

> **Esta desviación está declarada** en la sección Complexity Tracking de [plan.md](./plan.md) y
> debe confirmarla el usuario antes de implementar la fase B. Las fases A y C no dependen de ella.

---

## 6. Dónde entra la pantalla en la interfaz interactiva

**Decisión**: `screenUsage` se añade **al final** del `enum screen`, y se abre con la tecla `u`.

**Verificado**: `adapters/primary/tui/tui.go:24-38` define nueve pantallas apiladas en un `enum`,
terminando en `screenEditSetting`; no hay pestañas. Añadir al final evita desplazar los valores
existentes, que es la convención que el propio archivo ya sigue.

**Teclas ocupadas** en `updateList` (`tui.go:544-603`): `q`, `/`, `j`, `k`, `enter`, `s`, `a`, `m`,
`c`, `o`. **`u` está libre** y es mnemotécnica.

**Reutilización**: la sección de snapshot usa `FormatContextStats` (`cmd_pack.go:219`) sin
modificarlo —ya está compartido entre línea de comandos y MCP— y `BuildContextPack` como motor. No
se duplica ni el formato ni el cálculo.

---

## 7. Versiones de la librería de interfaz interactiva

**Consultado con `go list -m -versions`** el 2026-08-20:

| Módulo | En uso | Línea v1 vigente | Línea v2 |
|---|---|---|---|
| `charmbracelet/bubbletea` | v0.26.1 | **v1.3.10** | v2.0.9 (ruta `/v2`) |
| `charmbracelet/bubbles` | v0.18.0 | **v1.0.0** | — |
| `charmbracelet/lipgloss` | v1.0.0 | **v1.1.0** | — |

**Decisión**: subir a la línea v1. v2 cambia la ruta del módulo y rompe la interfaz de programación,
lo que choca de frente con FR-039 («no debe cambiar el comportamiento observable de ninguna
pantalla existente, ni requerir modificar pruebas ya escritas») y con el Principio III, que declara
los tests existentes intocables.

**Nota de gobernanza**: la constitución dice «TUI `charmbracelet/bubbletea` — última» y también
«si una librería tiene más de 2 versiones menores detrás, debe actualizarse». La segunda regla es
la que hace obligatorio este trabajo; la primera se cumple dentro de la línea v1 y la desviación
queda registrada en Complexity Tracking.

---

## 8. Modo índice: qué se emite y qué no

**Decisión**: en modo índice, el protocolo de trabajo va íntegro y cada memoria seleccionada se
reduce a una línea con identificador, tipo y título. El detalle se recupera con la capacidad de
lectura por identificador que **ya existe en todos los canales**; no se introduce ninguna capacidad
nueva (FR-033).

**Por qué el protocolo no se recorta**: ya es la regla vigente del `Builder` —el comentario del
campo `Budget` (`build_context.go:103-108`) dice que «protocolo y conflictos NUNCA se recortan»—.
El modo índice la conserva, y FR-032 la vuelve exigible con un test de regresión.

**Valor por defecto**: modo completo, es decir, el comportamiento actual. FR-034 obliga a que
activarlo y desactivarlo devuelva la emisión a un resultado idéntico al de partida (SC-010).
