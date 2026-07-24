# Research: Evolución de la Integración con Grafo de Código Externo

**Feature**: `010-codegraph-integration-evolution` | **Fecha**: 2026-07-23

Este documento resuelve las incógnitas técnicas necesarias para diseñar las
tres historias del spec sobre la base ya existente (`CodeGraphProvider`,
`adapters/secondary/codegraph/codebasememory/provider.go`,
`application/usecases/build_context.go`).

## 1. Impacto por archivo (Historia 1): el snapshot actual no alcanza

**Pregunta**: `domain.CodeHotspot` (usado hoy en el snapshot cacheado) solo
tiene `Name` + `FanIn` — sin `File`. Para anotar impacto al guardar una
memoria con `filepath=X`, hace falta saber qué símbolos viven en `X`.
¿Cómo se resuelve sin romper el patrón no-bloqueante?

**Decisión (verificada contra el CLI real durante la implementación, no
solo supuesta)**: se probó en vivo `get_architecture` sobre este mismo
repo — sus hotspots solo traen `name`, `qualified_name` y `fan_in`, **sin**
`file`. En cambio `search_code` (mismo CLI) sí devuelve `file` por
resultado, casando por `qualified_name` exacto (confirmado con `WriteFile`,
`FindRoot`, `ProjectKey` contra el fixture real de este repo). Por eso el
archivo de cada hotspot se resuelve con una llamada extra a `search_code`
por `qualified_name`, no leyendo un campo que `get_architecture` no expone.

Para no pagar ese costo en el hot path, la resolución ocurre **solo dentro
de `Refresh()`** (proceso detached, fuera del camino de guardado/arranque):
tras `parseArchitecture` (que se deja intacta, sigue siendo pura y con su
test existente sin tocar), un helper nuevo y separado
(`hotspotQualifiedNames`) relee el mismo JSON de `get_architecture` para
obtener `qualified_name` por hotspot (dato que `parseArchitecture` no
expone en el tipo de dominio, porque no hace falta después de resolver el
archivo), y `resolveHotspotFiles` hace un `search_code` por hotspot
(acotado a `maxHotspots`=6) para completar `CodeHotspot.File`. Best-effort
por hotspot: si uno falla o no matchea, ese hotspot queda sin `File` y
simplemente no participa del match por archivo — degrada igual que hoy con
datos ausentes, sin abortar el resto del refresco. `ImpactFor()` (el que sí
corre en el hot path) solo lee `Snapshot()` ya resuelto — cero llamadas al
proveedor en el momento de guardar.

La consulta de impacto (`ImpactFor(filepath string)`) se resuelve **siempre
contra el snapshot ya cacheado en disco** (`.memory/code_provider_snapshot.json`),
igual que `Snapshot()` hoy: nunca dispara una llamada nueva al proveedor
en el momento de guardar. Esto preserva sin excepción el contrato de
no-bloqueo ya establecido (FR-002, SC-002).

**Alternativas consideradas**:
- *Consultar `search_code` en vivo por archivo al guardar*: descartada —
  rompería el principio "ninguna consulta al proveedor bloquea el guardado"
  (edge case ya cubierto en el spec: "nunca se espera al refresco").
- *Indexar file↔hotspot en una tabla propia de gomemory*: descartada por
  redundante — gomemory no debe mantener una copia paralela del grafo del
  proveedor; el snapshot condensado ya es la única fuente de verdad cacheada.

## 2. Gestión de ADR (Historia 2): qué expone `manage_adr` REALMENTE

**Verificado en vivo** (no solo supuesto): `manage_adr` llamado contra los
dos proyectos indexados en esta sesión (`go_memory`, `kolmena_core_oci`)
NO es un CRUD de múltiples ADR con ID individual. Es **un documento único
por proyecto**, con 6 secciones fijas (`PURPOSE, STACK, ARCHITECTURE,
PATTERNS, TRADEOFFS, PHILOSOPHY`), con 3 modos: `get` (lee el documento
completo), `update` (escribe/reemplaza `content`), `sections` (lista los
nombres de sección presentes). Sin `list` de ADRs separados, sin
`external_adr_id`, sin timestamp por entrada — el spec y el primer borrador
de este research asumían una forma que la API real no tiene.

**Decisión (corregida con el usuario, ver spec — se mantiene bidireccional
por decisión explícita)**: cada memoria `architecture`/`decision` se
representa como un **bloque marcado** dentro de la sección que le
corresponde del documento único:

```
## ARCHITECTURE

<!-- gomemory:id=42 -->
### Usar SQLite WAL
decisión de concurrencia de escritura

### Convención de logging (escrito directo en el proveedor)
...sin marcador: se importa a gomemory como memoria nueva...
```

- **Mapeo tipo→sección**: `architecture` → sección `ARCHITECTURE`;
  `decision` → sección `TRADEOFFS` (heurística documentada, no hay una
  sección "decisions" en el esquema fijo del proveedor).
- **Export** (gomemory→proveedor): `get` el documento, parsear en
  secciones/bloques (dominio puro, ver `data-model.md`), upsert del bloque
  de esa memoria por su marcador `<!-- gomemory:id=N -->` (reemplaza si ya
  existía, agrega si no), reserializar las 6 secciones en orden fijo,
  `update` con el documento completo.
- **Import** (proveedor→gomemory): `get`, parsear, cualquier bloque **sin**
  marcador `gomemory:id` es de origen `provider` — su identidad estable es
  `section + heading` (no hay ID real del proveedor que usar), y se crea/
  actualiza como memoria `architecture` en gomemory.
- Todo el parseo/render vive en `domain/` (funciones puras, testeables con
  fixtures de texto, sin CLI); el adaptador (`adrsync/codebasememory/`)
  solo sabe `GetDocument`/`UpdateDocument` (todo el documento, sin lógica de
  bloques) — así si mañana aparece un proveedor con CRUD real por ADR, el
  puerto no cambia, solo el adaptador.

**Alternativas consideradas**: espejo de documento completo (una sola
memoria = todo el documento) — más simple, pero pierde "cada decisión es su
propio ADR" que el spec pide explícitamente; se descartó porque el usuario
priorizó mantener esa granularidad al elegir esta opción.

## 3. Prevención de bucles de sincronización

**Pregunta**: ¿cómo evitar que un bloque exportado desde gomemory se
reimporte como memoria nueva, o que una memoria importada se re-exporte
como bloque duplicado?

**Decisión**: cada relación memoria↔bloque se persiste en una tabla nueva
(`adr_sync_records`, ver `data-model.md`) con `origin` (`gomemory` |
`provider`) y `block_key` (el marcador `id=N` para bloques propios, o un
hash estable de `section+heading` para bloques ajenos). Antes de exportar,
se busca el registro por `memory_id`; antes de importar un bloque sin
marcador, se busca por `block_key`. Mismo patrón de idempotencia que
`formSynapse()`/`GetRelationByPair()` ya usa para no duplicar sinapsis.

## 4. Múltiples proveedores (Historia 3): forma del puerto

**Pregunta**: `CodeGraphProvider` es una interfaz de un solo proveedor
concreto. ¿Se extiende la interfaz o se agrega una capa de selección?

**Corrección tras inspeccionar el código actual** (importante: invalida un
supuesto del primer borrador de este research): `application/usecases/
build_context.go` **ya** recibe `CodeProviders []ports.CodeGraphProvider`
(plural) y ya itera sobre todos, escribiendo una sección de contexto por
cada uno cuyo snapshot esté `Available=true` (`writeCodeProviderSection`,
rotulada con `snap.Provider`) — hoy la lista solo tiene un elemento
(`infrastructure/container.go:56-61`, construida a partir de
`settings.CodeGraphCommand`, singular), pero el mecanismo de "N proveedores,
cada uno se salta en silencio si no está disponible" **ya existe y ya está
probado**. No hace falta un adaptador Composite nuevo que reimplemente esa
selección puertas adentro de la interfaz.

**Decisión (corregida)**: no se agrega ningún `registry.go`/Composite. Se
extiende únicamente el **wiring** en `infrastructure/container.go` para
construir `codeProviders` a partir de `settings.CodeGraphProviders`
(plural, ordenada) en vez de un solo `settings.CodeGraphCommand` — cada
comando de la lista se instancia como un `codebasememory.New(...)`
independiente, y el loop ya existente en `build_context.go` se encarga de
mostrarlos (o saltarlos) tal como ya hace hoy con uno solo.

Para los dos consumidores NUEVOS que sí necesitan una única fuente
inequívoca (Historia 1 — anotar impacto por archivo; Historia 2 — exportar/
importar ADR, donde mezclar dos proveedores rompería la resolución de
conflictos por timestamp), se agrega una función pura y pequeña,
`firstAvailable(providers []ports.CodeGraphProvider) ports.CodeGraphProvider`,
que devuelve el primer proveedor de la lista cuyo snapshot cacheado esté
`Available=true` (o `nil` si ninguno lo está). Vive junto a los casos de uso
que la consumen (`application/usecases/`), no como un tipo nuevo que
implemente `CodeGraphProvider` — no necesita implementar la interfaz
completa, solo resolver "¿cuál es el activo ahora?" a partir de datos que
`Snapshot()` ya expone.

**Alternativas consideradas**:
- *Adaptador Composite (`registry.go`) implementando `CodeGraphProvider`*:
  es la decisión del primer borrador de este research — descartada al
  confirmar que `build_context.go` ya resuelve la pluralidad en su propio
  loop; el Composite hubiera sido una segunda implementación de exactamente
  la misma lógica de "saltar los no disponibles" que ya existe y ya se
  prueba en `application/usecases/build_context_test.go`.
- *Extender `CodeGraphProvider` con `Priority()`/`IsAlive()`*: descartada —
  ensucia el contrato ya usado por el único adaptador concreto existente sin
  necesidad.

## 5. Configuración

**Decisión**: extender `persistence.Settings` (mismo archivo
`.memory/settings.json` por proyecto) con:
- `code_graph_providers []string` (comandos en orden de prioridad) —
  **retrocompatible**: si está vacío pero `code_graph_command` (el campo ya
  existente) tiene valor, se trata como lista de un elemento; si ambos están
  vacíos, se busca `codebase-memory-mcp` en el PATH como hoy.
- `adr_sync_enabled bool` (default `false`, opt-in explícito).
- `code_impact_annotation_enabled bool` (default `true` — sigue el mismo
  criterio que hoy tiene `code_graph_disabled=false` por defecto: si hay
  proveedor disponible, se aprovecha; se puede apagar sin apagar todo el
  grafo externo).

Se expone por el mismo comando ya existente: `mem settings
--code-graph-providers=cmd1,cmd2 --adr-sync=true|false
--code-impact-annotation=true|false`, siguiendo la convención de flags ya
usada por `--code-graph=…`/`--code-graph-command`.

## 6. Persistencia y no-bloqueo del registro de sincronización

**Decisión**: `adr_sync_records` es una tabla más en el mismo `mem.db`
(mismo patrón que `memory_relations`: SQL directo, `modernc.org/sqlite`,
WAL). Los intentos de sincronización (exportar/importar) corren **fuera**
del hot path de guardado: al guardar una memoria `architecture`/`decision`
con `adr_sync_enabled=true`, el guardado en SQLite se confirma primero (como
hoy) y la sincronización con el proveedor se dispara como un paso
best-effort posterior (mismo patrón *fire-and-forget* que `formSynapse()` —
un fallo ahí nunca hace fallar `InsertMemory`). La importación de ADRs
nuevos del proveedor se resuelve en el mismo ciclo de refresco detached que
ya usa `MaybeRefresh()`/`mem code-refresh`, no en un proceso nuevo.

## Resumen de decisiones

| Incógnita | Decisión |
|---|---|
| Impacto por archivo sin romper no-bloqueo | Extender `CodeHotspot.File`; `ImpactFor()` solo lee el snapshot cacheado |
| Forma de `manage_adr` | CRUD vía CLI del proveedor, mismo patrón que `get_architecture`; degrada a solo-exportación si no hay `list` |
| Evitar bucles de sync | Tabla `adr_sync_records` con campo `origin`, mismo patrón de idempotencia que `GetRelationByPair` |
| Múltiples proveedores | Reusar el loop `[]ports.CodeGraphProvider` que `build_context.go` ya tiene; wiring en `container.go` a partir de `settings.CodeGraphProviders`; `firstAvailable()` puro para los consumidores que necesitan una única fuente (Historias 1 y 2). Sin tipo nuevo, interfaz sin cambios |
| Configuración | Extiende `persistence.Settings`, retrocompatible con `code_graph_command` |
| No-bloqueo de la sincronización | Fire-and-forget posterior al `commit` de la memoria, en el mismo ciclo detached de refresco |

No quedan `NEEDS CLARIFICATION` pendientes.
