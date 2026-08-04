# Research: gomemory como brazo extensor de contexto histórico para /speckit

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

No quedaron marcadores `[NEEDS CLARIFICATION]` en `spec.md` — este documento
registra las decisiones técnicas necesarias para pasar de la especificación
al diseño (Phase 1), verificadas contra el código actual del repositorio
(`git log`/lectura directa, no supuestos).

## 1. Fuente del resumen de historial

**Decision**: reutilizar `get_context()` (MCP) / `mem context` (CLI) tal
como existen hoy, sin construir un endpoint o comando nuevo.

**Rationale**: `application/usecases/build_context.go` ya produce
exactamente el contenido que Historias 1 y 2 piden — secciones separadas
por tipo de memoria (`Decisiones de Arquitectura`, `Decisiones Técnicas`,
`Patrones`, `Bugfixes`, …) y, cuando hay un proveedor externo de grafo de
código conectado, una sección aparte y rotulada (`## Grafo de código
externo (<provider>)`) con una nota explícita que ya distingue el "por qué"
(gomemory) del "qué/cómo" (grafo externo) — ver
`writeCodeProviderSection()` en ese mismo archivo. Ya está acotado por
`Budget` (feature 008) y ya degrada en silencio si no hay memorias o no hay
proveedor externo. Construir un mecanismo paralelo duplicaría lógica ya
probada en producción (feature 010) y violaría el principio de simplicidad
del proyecto.

**Alternatives considered**:
- Nuevo comando MCP `get_spec_history` con formato dedicado — rechazado:
  no aporta nada que `get_context` no tenga ya, y añade una segunda fuente
  de verdad a mantener sincronizada con `Build()`.

## 2. Tipo de hook: mandatorio, no opcional

**Decision**: `hooks.before_specify` en el `extension.yml` de la nueva
extensión se declara `optional: false`.

**Rationale**: FR-001 exige que el resumen se incorpore "sin que la persona
tenga que solicitarlo explícitamente". El mecanismo de hooks opcionales que
ya usa `agent-context` (`after_specify`/`after_plan`, ambos `optional:
true`) solo *imprime una sugerencia* y espera que alguien ejecute el
comando a mano (confirmado leyendo el bloque `Optional Hook` que la propia
skill `speckit-specify` emitió en la sesión anterior de esta spec) — eso no
cumple "automático". Un hook mandatorio, en cambio, hace que la skill emita
`EXECUTE_COMMAND` y lo corra sin pedir confirmación.

**Alternatives considered**:
- Hook opcional (como `agent-context`) — rechazado: no satisface FR-001.
- Instrucción embebida directamente en el texto de la skill oficial
  `speckit-specify` — rechazado: acoplaría gomemory a un archivo que no le
  pertenece y que una actualización de spec-kit puede sobrescribir; el
  mecanismo de extensiones existe exactamente para evitar esto.

## 3. Implementación del comando del hook: script determinista, no instrucción para el LLM

**Decision**: el comando `speckit.gomemory-context.update` se implementa
como script (`bash`/`powershell`, mismo patrón que
`update-agent-context.sh`) que invoca el binario `./mem context` (con
fallback a `mem` en `PATH`), en vez de una instrucción markdown que le pida
al agente LLM que llame la tool MCP `get_context`.

**Rationale**: un hook mandatorio necesita un resultado determinista y
verificable — depender de que el LLM "recuerde" invocar la tool correcta en
cada corrida es frágil (el `EXECUTE_COMMAND` de spec-kit espera el
resultado de ejecutar un comando, no una decisión del modelo). El binario
`./mem` ya es la vía CLI equivalente y documentada de `get_context`
(confirmado en `adapters/primary/cli/cmd_context.go`: `mem context`
invoca el mismo `ContextBuilder.Build()` que expone el MCP), y ya es el
artefacto que `mem install` deja en la raíz del proyecto.

**Alternatives considered**:
- Instrucción markdown pidiéndole al agente que llame
  `mcp__gomemory__get_context` — se mantiene como *fallback documentado* en
  el `README.md` de la extensión para el caso borde de un entorno donde
  gomemory está disponible solo vía MCP (sin el binario `mem` instalado
  localmente), pero no es el mecanismo primario.

## 4. Dónde vive el interruptor de encendido/apagado

**Decision**: nuevo campo `SpeckitContextDisabled bool` (default
`false` = activado) en `Settings`/`SettingsData`
(`.memory/settings.json`), leído por el script del hook **antes** de
invocar `mem context` — si está en `true`, el script termina sin salida.

**Rationale**: el pedido explícito fue un interruptor visible y editable
"desde la propia TUI de gomemory", no solo un opt-out del lado de spec-kit.
Es exactamente el mismo patrón ya verificado en código para
`CodeGraphDisabled` (`application/ports/settings_repository.go:8`,
alternado en `adapters/primary/tui/tui.go:724`) — mismo tipo de campo,
mismo lugar de persistencia, mismo flujo de lectura en el composition root.

**Alternatives considered**:
- Usar únicamente `specify extension disable gomemory-context` (mecanismo
  nativo de spec-kit, visto en el `README.md` de `agent-context`) —
  rechazado como *única* vía porque no es visible/editable desde la TUI de
  gomemory; se documenta como vía complementaria ya existente (alguien
  puede seguir usándola si administra spec-kit directamente).

## 5. Sin detección activa de spec-kit en el arranque de gomemory

**Decision**: gomemory no escanea el filesystem buscando `.specify/` para
decidir nada en tiempo de arranque. La fila de la TUI se muestra siempre
(igual que la fila "Grafo de código externo" se muestra aunque el binario
externo no esté instalado — ver `tuiProvider()` en
`infrastructure/container.go:139`), y el gate real de "no pasa nada si no
hay spec-kit" (SC-006) ocurre de forma pasiva: si `.specify/extensions/
gomemory-context/` no está instalada, spec-kit nunca dispara el hook,
así que el script nunca corre.

**Rationale**: agregar una detección activa de `.specify/` en cada arranque
de gomemory sería I/O redundante para un caso que el propio mecanismo de
hooks de spec-kit ya resuelve gratis — violaría el principio de simplicidad
("impacto mínimo") sin aportar nada que el gate pasivo no dé ya.

**Alternatives considered**:
- Chequeo activo de `.specify/` en `NewContainer()` para ocultar/mostrar la
  fila de la TUI condicionalmente — rechazado: I/O extra en cada arranque,
  inconsistente con el patrón ya establecido para el grafo de código
  externo (fila siempre visible, estado "no disponible" implícito).

## 6. Registro de la extensión en spec-kit

**Decision**: la extensión se empaqueta (`extension.yml`, `README.md`,
`commands/`, `scripts/`) pero **no** se edita a mano
`.specify/extensions/.registry` ni `.specify/extensions.yml` — esos
archivos incluyen metadata gestionada por la herramienta (`manifest_hash`,
`installed_at`, `registered_commands` por integración) que la CLI `specify`
calcula al instalar una extensión.

**Rationale**: `.registry` (visto en `.specify/extensions/.registry`)
contiene un hash del manifiesto y timestamps que deben coincidir con lo que
la herramienta `specify` espera; escribirlos a mano arriesga un registro
inconsistente que rompa el loader de extensiones. La instalación formal
(`specify extension install .specify/extensions/gomemory-context` o el
comando equivalente de la versión de `specify` instalada) queda como tarea
explícita de la fase de implementación, no del diseño.

**Alternatives considered**: ninguna — es un paso mecánico de herramienta,
no una decisión de diseño.
