# Fase 0 — Investigación: Señal de grafo de código en Retrieval de ContextPack

**Feature**: [spec.md](./spec.md)

El Technical Context del plan no dejó ningún `NEEDS CLARIFICATION` — la exploración
previa a la escritura del spec (lectura directa de `code_graph_provider.go`,
`build_context.go`, `build_context_pack.go`, `domain/code_provider.go`,
`provider_selection.go`, `cmd_pack.go`, `cmd_mcp.go`) ya resolvió las decisiones de
diseño. Este documento registra esas decisiones y sus alternativas, no incógnitas
abiertas.

## §1. Reusar el contrato existente vs. crear uno nuevo

**Decisión**: Reusar `ports.CodeGraphProvider` tal cual (`Snapshot()`, `MaybeRefresh()`,
`ImpactFor(filepath)`), sin agregar métodos ni un puerto paralelo.

**Rationale**: El puerto ya documenta, en su propio comentario, el contrato exacto que
esta feature necesita: "brazo extensor enchufable, no un requisito... Snapshot() SOLO lee
el estado cacheado en disco: instantáneo, nunca invoca al proveedor externo ni bloquea el
hot path". `BuildContextPack` es, para efectos de este contrato, un hot path más (igual
que `get_context`) — no hay ninguna necesidad no cubierta por la interfaz actual.

**Alternativas consideradas**:
- *Nuevo método `SearchByTask(task string)` en el puerto*, para que el grafo pudiera
  aportar candidatos más allá de hotspots ligados a un `Filepath` ya conocido. Rechazado:
  requeriría una llamada en vivo al proveedor externo (viola el contrato de no-bloqueo) o
  un índice semántico local nuevo — alcance muy superior al de esta feature, y el spec no
  lo pide (User Story 1/2 solo hablan de boost por hotspot y resumen de arquitectura, ambos
  ya resolubles con el snapshot cacheado tal como es hoy).

## §2. Selección de proveedor para el ítem de arquitectura

**Decisión**: Usar `FirstAvailable(providers)` (ya existente en
`usecases/provider_selection.go`) para elegir el proveedor cuyo snapshot alimenta el ítem
de orientación arquitectónica, igual que ya hace la anotación de impacto (Historia 1 de la
feature 010).

**Rationale**: Con múltiples proveedores configurados (feature 010, Historia 3), mostrar
un resumen de arquitectura por cada uno duplicaría contenido de forma poco útil dentro de
un ContextPack acotado a tokens — a diferencia de `mem context`, que sí itera todos
porque no tiene presupuesto que cuidar. `FirstAvailable` ya es la función que el proyecto
usa exactamente para este tipo de consumidor ("necesita una única fuente inequívoca", según
su propio comentario).

**Alternativas consideradas**:
- *Iterar todos los proveedores y agregar un ítem de arquitectura por cada uno*: rechazado
  por lo anterior — no aporta valor proporcional al costo de tokens dentro de un
  presupuesto acotado.
- *Fusionar los snapshots de todos los proveedores en un solo resumen*: rechazado por
  complejidad innecesaria (mezclar clusters/hotspots de proveedores potencialmente
  distintos) sin que el spec lo pida.

Para el **boost de prioridad por hotspot** (Historia 1), en cambio, se consulta `ImpactFor`
contra **todos** los proveedores configurados, no solo el primero disponible — mismo
criterio que ya usa `build_context.go` en la sección "🔥 Memoria conectada a código activo"
(itera `b.CodeProviders` completo). Un archivo puede ser hotspot según un proveedor y no
según otro; para el boost (que solo puede ayudar, nunca perjudicar — FR-004) tiene sentido
ser generoso, no elegir uno solo.

## §3. Extraer `formatCodeArchitecture` en vez de duplicar el formato

**Decisión**: Extraer el cuerpo de `writeCodeProviderSection` (`build_context.go:29`) a una
función pura `formatCodeArchitecture(snap domain.CodeProviderSnapshot) string`, que
`writeCodeProviderSection` invoca por dentro sin cambiar su firma ni su comportamiento
observable.

**Rationale**: Evita mantener dos textos de "resumen de arquitectura" ligeramente
distintos en dos pipelines — riesgo real de deriva (uno se actualiza, el otro no). La
extracción es mecánica (mover el cuerpo a una función que devuelve `string` en vez de
escribir a un `*strings.Builder`) y los tests existentes (`TestBuild_HotCodeSection_*`) la
cubren por transitividad: si el output de `writeCodeProviderSection` cambiara por error, esos
tests fallarían.

**Alternativas consideradas**:
- *Duplicar un formato más corto para `mem pack build`*: rechazado — el spec (Historia 2)
  pide "el mismo resumen compacto que hoy solo se ve en `mem context`", no uno nuevo.

## §4. Semántica de `IncludeCodeGraph` en el `ContextRequest`

**Decisión**: Campo `IncludeCodeGraph bool` en `ContextRequest`, con el mismo patrón que
`IncludeSpecKit`; el **CLI** (`cmd_pack.go`) lo traduce con default `true` vía un flag
`--no-code-graph` (opt-out), igual que ya hace `--no-speckit`.

**Rationale**: FR-007 exige "activado por defecto" para el CLI, y ese es exactamente el
patrón que `--no-speckit` ya estableció — cero superficie nueva de decisión de diseño ahí.

**Hallazgo relevante para el wiring MCP** (§5 abajo): la tool MCP `pack_build` **no**
replica ese default hoy. Su parámetro `include_speckit bool json:"...,omitempty"` usa el
zero-value de Go (`false`) como default — si el cliente MCP no manda el campo, Spec Kit
queda **desactivado**, al revés que el CLI. Es una inconsistencia preexistente entre los
dos call sites de la feature 015, fuera del alcance de este spec (nadie lo pidió corregir,
y tocarlo sería scope creep no relacionado con el grafo de código). Se documenta aquí
únicamente porque condiciona la decisión de nombre del parámetro nuevo en §5 — no se
modifica `include_speckit` en esta feature.

## §5. Nombre del parámetro nuevo en la tool MCP `pack_build`

**Decisión**: `no_code_graph bool json:"no_code_graph,omitempty"` (mismo signo que el flag
CLI `--no-code-graph`), **no** `include_code_graph`.

**Rationale**: Un bool con `omitempty` y zero-value `false` significa "si el cliente MCP no
manda el campo, se comporta como `false`". Si el parámetro se llamara `include_code_graph`,
un cliente que no lo conozca (la inmensa mayoría, al ser un campo nuevo) terminaría con el
grafo de código **desactivado por defecto** — reproduciendo exactamente la inconsistencia
de §4 en vez de evitarla. Nombrándolo en negativo (`no_code_graph`), el mismo zero-value
`false` significa "no desactivar" → el grafo queda activado por defecto también en MCP,
cumpliendo FR-007 en los dos call sites por igual.

**Alternativas consideradas**:
- *`include_code_graph` (paralelo a `include_speckit`)*: rechazado por la razón de arriba —
  perpetuaría un default apagado no pedido por el spec.
- *Corregir también `include_speckit` a `no_speckit` en esta feature*: rechazado — cambiaría
  el contrato público de una tool MCP ya en uso (`pack_build`) por un motivo ajeno a este
  spec; si se decide corregir, debe ser su propia feature con su propio spec.

## §6. Mecánica exacta del boost de prioridad

**Decisión**: `boostHotspotCandidates` solo puede mover una prioridad de
`PriorityOptional` a `PriorityRelevant`. Nunca toca `PriorityCritical` (ya garantizado por
tipo, FR-005 de la feature 015) ni introduce un cuarto nivel de prioridad.

**Rationale**: FR-004 de este spec es explícito: "el sistema NUNCA DEBE reducir la
prioridad... la señal solo puede aumentar la chance de inclusión, nunca disminuirla". Un
salto directo a `PriorityCritical` sobre-representaría la señal del grafo de código frente
a las reglas de negocio ya establecidas (tipo de memoria) — el grafo informa relevancia,
no criticidad.

## §7. Rendimiento (SC-002)

**Decisión**: No se introduce ninguna llamada nueva a un proceso o red durante
`BuildContextPack`. `Snapshot()` es una lectura de un valor ya cacheado en memoria/disco
(mismo camino que hoy usa `mem context` en cada invocación); `ImpactFor()` es una consulta
en memoria contra ese mismo snapshot. El límite de <50ms de sobrecarga (SC-002) es, por
construcción, un techo muy generoso frente al costo real (lectura de un archivo JSON
pequeño ya parseado en memoria del proceso, sin I/O de red).

**Rationale**: Coherente con el Principio Operativo #8 de la constitución ("Cache de
lectura opcional: TTL fijo, invalidación explícita en escritura, fallback transparente") —
el snapshot ya es ese cache; esta feature solo lo consulta desde un segundo punto del
código.
