# Manual de usuario

Guía para instalar, configurar y usar gomemory con los agentes compatibles.

## Índice

1. [Instalación Rápida](#1-instalación-rápida)
2. [Plugin para OpenCode](#2-plugin-para-opencode)
3. [Plugin para Claude Code](#3-plugin-para-claude-code)
4. [Servidor MCP por stdio](#4-servidor-mcp-por-stdio)
5. [Verificación](#5-verificación)
6. [Solución de Problemas](#6-solución-de-problemas)
7. [Memory Protocol](#7-memory-protocol)
8. [Mantenimiento de Memoria](#8-mantenimiento-de-memoria)
9. [Grafo de Código (mem index)](#9-grafo-de-código-mem-index)
10. [Optimización de Contexto (mem pack)](#10-optimización-de-contexto-mem-pack)
11. [Registro Multi-Agente (detalle completo)](#11-registro-multi-agente-detalle-completo)
12. [Seguridad](#12-seguridad)
13. [Stack Técnico](#13-stack-técnico)
14. [Portabilidad](#14-portabilidad)
15. [Modo Plan Determinista (mem doctor)](#15-modo-plan-determinista-mem-doctor)
16. [Benchmark de tokens (mem usage)](#16-benchmark-de-tokens-mem-usage)
17. [Revisión adversarial por consenso](#17-revisión-adversarial-por-consenso-mem-review)
18. [Octopus AAR](#18-octopus-aar)

---

## 1. Instalación Rápida

### Opción A — Instalador universal de consola (recomendado)

Deja el binario `mem` en el PATH, sin compilar. Linux, macOS y Windows.

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.sh | bash
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.ps1 | iex
```

Luego, en tu proyecto:

```bash
cd tu-proyecto
mem setup-mcp --scope global --agents claude,codex,opencode
mem --help
```

El registro global configura MCP y las integraciones de ciclo de vida disponibles
una sola vez para todos los proyectos. El almacén global y las memorias iniciales
del proyecto se crean de forma diferida en el primer uso.

Cursor, Windsurf y Cline no tienen un ámbito global compatible. Configúralos en
el proyecto que los use:

```bash
mem setup-mcp --scope project --agents cursor,windsurf,cline --target .
```

`mem install .` sigue disponible como flujo autocontenido por proyecto. Copia el
binario, configura MCP, instala los plugins y hooks compatibles, inicializa las
reglas de trabajo y la constitución en memoria, y distribuye las extensiones
opcionales. No crea archivos `AGENTS.md` ni `CLAUDE.md`. Si encuentra artefactos
gestionados por instalaciones antiguas, los respalda y retira.

Las reglas llegan al agente **íntegras** en cada `get_context()`; la constitución
se consulta bajo demanda con `mem constitution` o el atajo `/constitution`.
Ambas son del equipo, no de la herramienta: `mem docs` las exporta, importa y
restaura (ver *Documentos fijados*).

Para una integración por proyecto de Claude Code u OpenCode también puedes usar
`mem setup claude-code` o `mem setup opencode`.

### Opción B — Desde el fuente

```bash
go build -o mem ./infrastructure/
./mem setup opencode      # o claude-code
./mem --help
```

### Hooks portables

Los hooks son subcomandos del binario, como `mem hook session-start`,
`post-compact`, `turn-end` y `subagent-stop`. No usan scripts shell, `curl` ni
un servidor HTTP. `mem install .` o `mem setup claude-code` los registra en el
proyecto; `mem setup-mcp --scope global` registra los disponibles en el ámbito
del usuario.

### Prerrequisitos

- Para la Opción A: ninguno (binario autocontenido).
- Para la Opción B: Go 1.27+.
- OpenCode 0.70+ (para plugin OpenCode), Claude Code (para hooks/plugin).
- No se necesita CGO.

---

## 2. Plugin para OpenCode

### Instalación

```bash
./mem setup opencode
```

Esto:

1. Copia `infrastructure/plugin/opencode/gomemory.ts` a `~/.config/opencode/plugins/gomemory.ts` (archivo suelto — OpenCode auto-descubre plugins como archivos en `plugins/`, no en subcarpetas)
2. Crea/actualiza el `opencode.json` **del proyecto actual** con la entrada `mcp.gomemory` (usa `mem setup-mcp --scope global --agents opencode` en vez de esto para registrar el MCP una sola vez, en `~/.config/opencode/opencode.json`, para todos los proyectos)
3. El plugin se activa automáticamente al iniciar OpenCode — OpenCode lo descubre solo, sin que ningún `opencode.json` lo referencie explícitamente

### Verificación

```bash
ls ~/.config/opencode/plugins/
# Debería ver: gomemory.ts

cat opencode.json   # o ~/.config/opencode/opencode.json si usaste --scope global
# Buscar la clave "mcp" conteniendo "gomemory"
```

### Qué Hace el Plugin

- **Hooks directos**: invoca `mem hook <evento>` (sin servidor HTTP ni puerto)
- **Session lifecycle**: Crea sesión al iniciar, cierra al terminar
- **Memory Protocol**: Inyecta reglas de memoria en el system prompt
- **Context injection**: Provee contexto de sesiones previas al arrancar
- **Compaction recovery**: Recupera estado después de compactación
- **Context enrichment**: ToolSearch instruction en el primer prompt

### Eventos de OpenCode — para qué sirve cada uno

El plugin `gomemory.ts` mapea los eventos de OpenCode al binario `mem`, en
paralelo a los hooks de Claude Code (misma lógica, resuelta en Go):

| Evento OpenCode | Invoca | Función |
|-----------------|--------|---------|
| `session.created` | `mem session start` | Abre la sesión de gomemory |
| `session.idle` | `mem hook turn-end` (stdin) | Checkpoint de actividad del turno (archivos/comandos) |
| `session.compacted` | `mem hook compact-summary` + `mem hook post-compact` | Guarda el resumen disponible y prepara la recuperación posterior |
| `session.deleted` | — | Libera el estado temporal que el plugin mantenía para esa sesión |
| `chat.message` | `mem hook prompt` (stdin) | Persiste el prompt del turno como provenance (`origin_prompt`) |
| `experimental.chat.system.transform` | `mem context` + hooks de recordatorio | Inyecta protocolo, contexto, recuperación pendiente y avisos del turno |
| `experimental.session.compacting` | `mem hook compaction-context` | Entrega al compresor la memoria de la sesión activa antes de compactar |
| `tool.execute.after` (`task`) | `mem hook subagent-stop` | Captura aprendizajes estructurados del resultado de un subagente |
| `dispose` | `mem session end` | Cierra la sesión al descargarse el plugin |

---

## 3. Plugin para Claude Code

### Instalación

```bash
./mem setup claude-code
```

Esto:

1. Copia el skill de memoria a `.claude/plugins/gomemory/skills/memory/SKILL.md`.
2. Escribe `.mcp.json` en la raíz con el server MCP `gomemory` (referencia
   **portable**: `command: "mem"` por PATH, no una ruta absoluta de máquina).
3. Registra **hooks portables** en `.claude/settings.json`: cada evento de Claude
   Code apunta a un subcomando del binario (`mem hook <evento>`), no a scripts
   `.sh`. No dependen de `bash`/`curl` ni del servidor HTTP y corren igual en
   Windows.

### Para qué sirve cada hook

| Evento | Subcomando | Función |
|--------|-----------|---------|
| `SessionStart` (`startup\|resume\|clear`) | `session-start` | Abre sesión si no hay activa **e inyecta el contexto de sesiones previas**. El agente arranca recordando el proyecto. |
| `SessionStart` (`compact`) | `post-compact` | Después de compactar, reinyecta recuperación + memoria de sesión + contexto indexado y reactiva las tools MCP diferidas. |
| `PostCompact` | `compact-summary` | Persiste el resumen producido por la compactación sin cerrar la sesión. |
| `SessionEnd` | `session-end` | Cierra la sesión activa como **red de seguridad** (acepta `summary` por stdin), aunque el modelo no llame `end_session`. |
| `UserPromptSubmit` | `user-prompt-submit` | En el **primer** prompt activa las tools MCP de memoria e inyecta el recordatorio del protocolo; luego es pasivo. En **cada** prompt persiste el texto del turno como provenance (`origin_prompt`) de lo que se guarde. En OpenCode el equivalente es `chat.message` → `mem hook prompt`. |
| `SubagentStart` | `subagent-start` | Registra el inicio de una delegación para preservar su trazabilidad. |
| `SubagentStop` | `subagent-stop` | Registra la actividad del subagente y captura los ítems válidos de su sección de aprendizajes. |
| `Stop` | `turn-end` | Al terminar cada turno, registra automáticamente (sin gastar tokens del agente) qué archivos se editaron y qué comandos corrieron, como memoria tipo `checkpoint`. Turnos de puro chat no generan nada. En OpenCode el equivalente es el evento `session.idle`. |
| `PreToolUse` (`ExitPlanMode`) | `plan-guard` | Valida la forma del plan antes de presentarlo. |
| `PostToolUse` (`ExitPlanMode`) | `plan-approved` | Al **aprobar un plan**, guarda el plan como memoria `decision` de forma determinista, sin depender de que el modelo lo recuerde. Cubre el hueco de `turn-end` (un turno de plan mode no toca archivos ni corre comandos). Cada aprobación (incluidos planes revisados) se acumula. En OpenCode el plugin lo activa al detectar un turno en modo `plan`. |
| `PostToolUse` (`EnterPlanMode`) | `plan-entered` | Inyecta el método de descomposición y el historial antes de redactar el plan. |

> Regla de oro: un hook nunca aborta el arranque del agente — ante error sale con
> código 0. Los hooks son lo que hace que la memoria "tome todo bien": sin ellos,
> las tools MCP existen pero nadie abre/cierra sesiones ni recupera contexto solo.

### Verificación

```bash
# Skill instalado
ls -la .claude/plugins/gomemory/skills/memory/

# Hooks portables registrados (deben referenciar `mem hook ...`)
cat .claude/settings.json

# MCP configurado (command: "mem")
cat .mcp.json
```

---

## 4. Servidor MCP por stdio

`mem mcp` ejecuta el servidor MCP mediante JSON-RPC sobre entrada y salida
estándar. El agente lo inicia como subproceso; no hay servicio residente, URL,
puerto TCP ni API key. Los plugins y hooks llaman a `mem hook <evento>` y
acceden al mismo almacén local.

---

## 5. Verificación

### Test Rápido

```bash
# 1. Crear/gestionar sesión por CLI
./mem session start

# 2. Guardar y buscar una memoria
./mem save -t "Prueba" -y learning "Verificación manual del manual"
./mem search "Prueba"

# 3. Obtener el contexto del proyecto (markdown)
./mem context

# 4. Cerrar sesión con resumen
./mem session end -s "Prueba manual completada"
```

### Tests Automatizados

```bash
go test ./... -v
```

### Verificar Compilación

```bash
go build -o mem ./infrastructure/ && echo "OK"
go vet ./... && echo "OK"
```

---

## 6. Solución de Problemas

### Error: "plugin directory not found"

```bash
# Asegúrate de compilar desde la raíz del proyecto
ls infrastructure/plugin/opencode/gomemory.ts   # debe existir
ls infrastructure/plugin/claude-code/         # debe existir
```

### El agente no muestra el servidor MCP

```bash
# El MCP va por stdio. Verifica que `mem` esté en el PATH y que la
# configuración del agente ejecute `mem mcp`.
which mem
mem doctor
```

### El Plugin OpenCode No se Activa

```bash
# Verificar que el plugin está instalado (archivo suelto, no subcarpeta)
ls ~/.config/opencode/plugins/gomemory.ts

# Verificar la config resuelta (mergea global + proyecto)
opencode debug config

# Reinstalar si es necesario
./mem setup opencode
```

### El Plugin Claude Code No se Activa

```bash
# Verificar archivos instalados
ls .claude/plugins/gomemory/

# Verificar que .mcp.json apunta al binario correcto en esta máquina
cat .mcp.json

# Reinstalar
./mem setup claude-code
```

### Otros problemas comunes

| Problema | Solución |
|----------|----------|
| `/mcp` no muestra el servidor | Ejecutar `mem doctor`, comprobar que `mem` esté en el PATH y volver a registrar con `mem setup-mcp`. Reiniciar el agente. |
| `mem install` falla | Verificar que `mem` esté en el PATH: `which mem` (Linux/macOS) o `where mem` (Windows). |
| Memoria no se persiste entre sesiones | Ejecutar `mem project` y comprobar permisos sobre el directorio de datos global mostrado. Usar `mem doctor` para revisar los hooks. |
| Binario no encontrado después de instalar | Agregar al PATH: `export PATH="$HOME/.local/bin:$PATH"` (Linux/macOS). |
| `mem update` en Windows | El binario en ejecución no se sobrescribe. Cerrar el proceso y ejecutar el comando que `mem update` sugiere. |
| Contexto muy grande | Ejecutar `mem gc` para limpiar memorias antiguas (retención de 90 días por defecto). |
| Base de datos corrupta | Restaurar desde un bundle creado con `mem export`; `mem compact` solo recupera espacio y no repara corrupción. |

---

## 7. Memory Protocol

El Memory Protocol es un conjunto de reglas que le dicen al agente cuándo
guardar, buscar y cerrar sesión de memoria.

- **Con integración de ciclo de vida (OpenCode, Claude Code, Codex)**: se
  inyecta automáticamente y los hooks registran la actividad compatible.
- **Solo MCP (Cursor, Windsurf, Cline)**: el servidor MCP entrega el
  protocolo en la respuesta `initialize`, así que cualquier cliente MCP lo
  recibe sin archivos en el repositorio. Para el ámbito de usuario,
  `mem setup-mcp --scope global` sigue escribiendo el bloque en el archivo de
  instrucciones de cada agente.

### Save Triggers

El agente guarda memoria automáticamente después de:
- Decisiones de arquitectura
- Corrección de bugs (incluye causa raíz)
- Convenciones o patrones establecidos
- Descubrimientos no obvios sobre el código
- Preferencias o restricciones del usuario

### Search Triggers

El agente busca memoria:
- **Reactivo**: cuando preguntas "recuerdas...?" o similar
- **Proactivo**: al iniciar tareas que podrían solaparse con trabajo previo

### Progressive Disclosure (3 Capas)

Aplicado por el propio servidor sobre el texto emitido (no depende del agente):

1. `search_memories()` / `list_memories()` — extractos compactos (~160 chars) con id
2. `get_memory(id)` — contenido completo solo cuando es necesario (capa 3)
3. `get_context()` acotado por presupuesto: entradas largas truncadas con puntero
   `get_memory <id>`; protocolo y conflictos nunca se recortan

### Session Close

Al terminar, el agente registra un resumen estructurado: Objetivo, Hallazgos,
Logrado, Próximos pasos, Archivos.

### Compaction Recovery

Después de compactación, el agente persiste resumen y recupera estado antes
de continuar. Además, al cerrar cada turno, si la huella emitida por gomemory
supera el umbral, el hook sugiere de forma **neutral** compactar el contexto
(nunca ejecuta comandos: solo señala).

### Compactación sin pérdida de memoria

La compactación de la conversación la dispara siempre el cliente (automática
o manual); gomemory nunca la ejecuta. Lo que hace es que ninguna compactación
pierda lo que la memoria registró en la sesión:

- **Antes de compactar** (en los clientes que lo permiten): el compresor
  recibe un bloque breve con las memorias de **esta sesión** — no el proyecto
  entero —, cada una con un puntero `get_memory <id>` para el detalle íntegro,
  más la orden de guardar el resumen compactado.
- **Después de compactar**: el agente recibe, en orden, los pasos de
  recuperación, esa misma memoria de la sesión, y el contexto de proyecto en
  modo índice (solo títulos y punteros, nunca contenido íntegro).
- **El resumen compactado queda guardado**: llama a `save_session_summary`
  (herramienta nueva, complementa a `end_session`) SIN cerrar la sesión — a
  diferencia de `end_session`, que sí la cierra. En los clientes que entregan
  el resumen directamente al hook, gomemory lo guarda por su cuenta, sin
  esperar a que el agente lo haga.
- **Captura pasiva de aprendizajes**: al terminar un subagente, si su mensaje
  final trae una sección `## Aprendizajes clave` (o `## Key Learnings`, en
  español o inglés, con viñetas o numerada), cada ítem se guarda solo, sin
  duplicados y sin que el agente gaste tokens en pedirlo.
- **Aviso opcional al agente** (`compact_agent_notice`, opt-in): al superar el
  umbral de compactación, además del aviso que ya ves tú, el agente recibe un
  recordatorio breve de guardar lo pendiente. Nunca bloquea ni prolonga el
  turno.

Qué capacidad usa cada cliente instalado se ve con `mem doctor`. Que un
cliente no ofrezca una capacidad no rompe nada: el respaldo (los pasos de
recuperación, el aviso a la persona) sigue funcionando igual.

### Huella de contexto (tunables)

Para bajar el costo de tokens de la sesión, gomemory emite lo mínimo desde el
inicio. Ajustable en `.memory/settings.json`, o directamente desde la TUI
(pantalla de Configuración): los tres valores son editables ahí, sin salir a
editar el JSON a mano.

| Clave | Efecto | Default |
|-------|--------|---------|
| `budget` | Techo de `get_context` en caracteres (`< 0` = sin límite) | `24000` (~6k tokens) |
| `compact_threshold` | Huella emitida/sesión que dispara el recordatorio (`<= 0` = off) | `48000` |
| `compact_agent_notice` | Aviso de preparación al agente al superar `compact_threshold`, además del aviso a la persona | `false` |
| `dedup_window_days` | Ventana del dedup por identidad (`<= 0` = off; `topic_key` sigue activo) | `7` |

La deduplicación en la fuente evita filas casi idénticas: guardar una memoria con
un `topic_key` ya usado (o el mismo tipo+título dentro de la ventana) **actualiza**
la existente en vez de crear otra.

### Refuerzo periódico de preferencias

En sesiones largas que no llegan a compactar, el protocolo y las preferencias
del usuario (`type=preference`) solo se inyectaban en `SessionStart` y
`post-compact` — sin nada que las recuerde en el medio, se diluyen del
contexto. El hook de fin de turno (`turn-end`) ahora reutiliza el mismo
contador de huella que gobierna el recordatorio de compactación
(`compact_threshold`): al superar **un tercio** de ese umbral, reinyecta el
**título y contenido real** de las preferencias guardadas más recientes (no un
recordatorio genérico), con un debounce de 20 minutos. Si en el mismo turno
también corresponde sugerir compactar, ese recordatorio tiene prioridad — la
compactación reinyecta el contexto completo de todos modos.

### Memoria conectada a código activo

`get_context` ahora cruza el `Filepath` de cada memoria contra el grafo de
código externo (si hay un proveedor configurado) **en cada llamada**, no solo
al guardar: si el archivo asociado es un hotspot vigente, aparece en una
sección `🔥 Memoria conectada a código activo` con su fan-in actual. A
diferencia de la anotación estática que se pega al `content` al guardar
(ver "Anotación de impacto al guardar" en el README), esta relación se
recalcula contra el snapshot vigente del grafo — si el código se reindexa y
cambian los hotspots, la relevancia se actualiza sola.

---

## 7bis. Documentos fijados: reglas y constitución

gomemory siembra dos memorias la primera vez que se usa en un proyecto:

| Alias | Documento | Tipo | Cómo llega al agente |
|---|---|---|---|
| `rules` | Reglas de trabajo | `preference` | **Íntegra** en cada `get_context()`, en su propia sección |
| `constitution` | Constitución técnica | `architecture` | Bajo demanda: `mem constitution` o `/constitution` |

La diferencia es deliberada. Las reglas de trabajo dicen *cuándo* planificar,
*cómo* verificar y *cómo* tratar un bug: hacen falta en cada sesión, así que son
la única excepción declarada al recorte por presupuesto de `get_context()`. La
constitución dice *cómo* escribir código: son cientos de líneas que no tiene
sentido pagar en cada arranque.

### El contenido es del equipo, no de la herramienta

Lo que gomemory trae es un **punto de partida**, no doctrina. Sin una vía cómoda
de reemplazo, sembrar reglas convertiría a la herramienta en autora de las normas
del equipo — y la memoria dejaría de ser un contenedor neutral.

```bash
mem docs list                                  # qué hay y en qué estado
mem docs export rules -o reglas.md             # exportar el contenido vigente
$EDITOR reglas.md
mem docs import rules reglas.md                # aplicar el del equipo
mem docs reset rules                           # volver al de por defecto
```

`mem docs list` distingue tres estados, derivados comparando con la plantilla
embebida: `sin sembrar`, `por defecto` y `personalizado` (con su fecha).

Lo mismo desde la TUI: `mem` → pantalla de configuración → `Actualizar Reglas IA`
o `Actualizar Constitución`, con **ver, exportar, importar y restaurar**. Las
mismas cuatro operaciones por las dos superficies.

### Garantías

- **Una semilla existente nunca se sobrescribe.** Reinstalar o actualizar no
  pisa lo que el equipo puso, ni con una plantilla más nueva del binario.
- **Importar no publica nada fuera.** Ni sinapsis automáticas ni exportación al
  ADR externo, aunque `adr_sync_enabled` esté activo.
- **La depuración de secretos sigue activa.** Un token pegado por error en el
  archivo importado no se persiste.
- **Un import fallido no destruye nada.** Contenido vacío o ilegible se rechaza
  con su motivo y el documento anterior queda intacto.

### Más allá del catálogo

El catálogo es una comodidad, no un límite. Para cualquier otro documento:

```bash
mem docs import --topic "equipo:runbook" runbook.md
```

### Relación con `mem export` / `mem import`

No se solapan y ambos siguen disponibles: `mem export` vuelca **toda** la memoria
y sus relaciones en un JSON para moverla entre proyectos o máquinas; `mem docs
export` saca **un** documento en texto plano para editarlo a mano.

---

## 8. Mantenimiento de Memoria

Cuando el almacén global de memoria crece demasiado, gomemory ofrece
cuatro acciones de mantenimiento — disponibles por CLI y, salvo desinstalar,
también desde la TUI (tecla `m`). Ninguna se expone vía MCP: son operaciones
destructivas que exigen confirmación humana explícita.

### Purgar memorias

```bash
./mem purge                                  # Purga el proyecto actual (pide confirmación)
./mem purge --type bugfix                    # Solo memorias de un tipo
./mem purge --older-than-days 90             # Solo memorias más viejas que N días
./mem purge --all --yes                      # TODOS los proyectos, sin prompt (scripts)
```

Por defecto el alcance es el proyecto actual (FR-003); `--all` requiere pasarse
explícitamente para afectar todos los proyectos del archivo. Al borrar una
memoria también se limpian las relaciones (`mem compare`) que la referencian.

### Compactar el almacenamiento

```bash
./mem compact
```

Ejecuta `VACUUM` para recuperar el espacio en disco liberado por borrados
previos. Nunca elimina memorias — reporta el tamaño antes/después.

### Garbage collection a demanda

```bash
./mem gc                                     # 90 días de retención por defecto
./mem gc --older-than-days 180 --all --yes
```

Limpieza por antigüedad, reutilizando la misma lógica de `purge`. Solo se
ejecuta cuando el usuario lo pide explícitamente — nunca en segundo plano.

### Desinstalación completa

```bash
./mem uninstall                              # reverso exacto de `mem install`
./mem uninstall ~/proyectos/mi-app --yes
```

Remueve los datos y auxiliares del proyecto, el binario local, los hooks y las
configuraciones MCP creadas por el flujo de proyecto. También retira artefactos
gestionados por versiones antiguas. La configuración global compartida por
otros proyectos se conserva y se informa por separado.

Ver también [contracts/cli-tui-contracts.md](../specs/003-memory-maintenance/contracts/cli-tui-contracts.md)
para el detalle completo de flags y comportamiento.

## 9. Grafo de Código (`mem index`)

```bash
./mem index                 # Indexa el código Go propio (símbolos: archivos, paquetes, funciones, métodos, tipos, llamadas)
./mem index --force          # Reindexado completo, ignora el cache incremental
./mem index --skip-graph     # Solo el grafo propio — no dispara el reindexado del proveedor externo
```

`mem index` construye el grafo de símbolos **propio** de gomemory (Go puro, vía
`go/parser`, sin dependencias externas), que alimentan las tools MCP
`search_code`/`get_symbol`/`list_dependencies`/`graph_status`. Tras el indexado
nativo, si hay un proveedor **externo** de grafo de código configurado
(`codebase-memory-mcp` u otro, multi-lenguaje — ver abajo), también dispara su
reindexado, salvo que se pase `--skip-graph`. Nunca hace fallar el comando si
el proveedor externo no está instalado o el reindexado externo falla: solo
informa o advierte, y el exit code permanece `0` (el indexado nativo, que sí
importa para el resto de gomemory, ya tuvo éxito).

Misma acción disponible en la TUI: pantalla de Configuración → "Reindexar
grafo externo" — corre en segundo plano (no bloquea la interfaz) y tiene una
guardia contra disparos concurrentes.

**Proveedor externo (opcional, "brazo extensor"):** si hay un binario CLI de
grafo de código externo instalado, gomemory lo usa para enriquecer `mem
context` (sección "Grafo de código externo" + "🔥 Memoria conectada a código
activo") y `mem pack build` (§10) con clusters, hotspots y anotación de impacto
al guardar — todo opcional, con degradación silenciosa si no hay proveedor.
Ajustes relevantes (`mem settings` o `.memory/settings.json`):

| Ajuste | Default | Qué hace |
| :--- | :--- | :--- |
| `code_graph_disabled` | `false` | Desactiva el proveedor externo por completo |
| `code_graph_providers` | *(ninguno)* | Lista ordenada de comandos de proveedor (fallback por prioridad) |
| `code_impact_annotation_disabled` | `false` | Desactiva la anotación de impacto (`[impacto: X es hotspot...]`) al guardar una memoria con `--filepath` |
| `adr_sync_enabled` | `false` | Sincronización bidireccional (opt-in) de memorias de arquitectura con el documento ADR del proveedor — ver `mem adr-sync status` |

## 10. Optimización de Contexto (mem pack)

`mem context` te da todo el historial del proyecto. `mem pack` te da solo lo
que hace falta para una tarea concreta, sin pasarte de un presupuesto de
tokens explícito. Son dos herramientas distintas para dos preguntas
distintas: "¿qué pasó en este proyecto?" vs. "¿qué necesito saber para hacer
X, en no más de N tokens?".

```bash
# Armar un paquete de contexto para una tarea, con presupuesto de 4000 tokens
./mem pack build --task "arreglar el bug de login" --max-tokens 4000

# Guardarlo como JSON para reusarlo (pack show/stats lo leen de ahí)
./mem pack build --task "arreglar el bug de login" --max-tokens 4000 --json > paquete.json
./mem pack show  < paquete.json     # re-renderiza el paquete en Markdown
./mem pack stats < paquete.json     # solo el resumen de reducción (tokens antes/después)

# Comprimir un texto suelto (sin buscar memorias ni aplicar presupuesto)
./mem pack compress < notas.txt
```

Qué hace `mem pack build`: busca memorias relevantes a la tarea, descarta las
que son casi duplicadas de otra ya incluida, comprime lo que no es crítico
(sin tocar código, URLs, rutas ni mensajes de error) y arma el paquete final
sin exceder `--max-tokens`. Si lo que es realmente crítico para la tarea ya
excede ese presupuesto por sí solo, el comando falla con un error explícito
en vez de devolverte un paquete incompleto sin avisar — así nunca crees que
tienes todo el contexto crítico cuando en realidad falta parte.

Flags de `pack build`:

| Flag | Qué hace | Obligatorio |
|---|---|---|
| `--task` | Descripción de la tarea | Sí |
| `--max-tokens` | Presupuesto total de tokens | Sí |
| `--project` | Proyecto objetivo (default: el actual) | No |
| `--min-relevance` | Relevancia mínima 0–1 para incluir un candidato | No |
| `--max-items` | Tope de candidatos antes de rankear | No |
| `--no-compress` | Desactiva la compresión del contenido no crítico | No |
| `--no-speckit` | No incluye artefactos de Spec Kit de la feature activa | No |
| `--no-code-graph` | Desactiva la señal del grafo de código externo | No |
| `--json` | Emite el paquete completo en JSON, en vez de Markdown | No |

Disponible también vía MCP (`pack_build`, `pack_show`, `pack_stats`,
`pack_compress`) para que el agente lo pida él mismo dentro de la
conversación, sin pasar por la terminal. La tool `pack_build` expone el
mismo apagador como `no_code_graph` (booleano, no `include_code_graph` — el
default siempre es "activado" salvo que el cliente lo desactive explícito).

Si hay un proveedor de grafo de código externo configurado (`codebase-memory-mcp`
u otro), `mem pack build` también lo consulta, con el mismo criterio
de solo-snapshot-cacheado y degradación silenciosa que ya usa `mem context`:
una memoria candidata cuyo archivo es un hotspot vigente sube de prioridad
Optional a Relevant, y si sobra presupuesto puede aparecer un ítem compacto
de arquitectura (mismos totales/clusters/hotspots que `mem context` ya
muestra). Sin proveedor configurado, o con `--no-code-graph`/`no_code_graph`,
el comportamiento es exactamente el de antes de esta señal.

> `.memory/settings.json` ya reserva claves para esta feature
> (`context_default_budget`, `context_min_relevance`, `context_max_items`,
> `context_compression_disabled`, `context_dedup_disabled`), pero hoy la CLI y
> el MCP no las leen — `--task`/`--max-tokens` son siempre explícitos por
> invocación. Editarlas a mano en el JSON no cambia el comportamiento
> todavía.

## Referencia Rápida

```bash
# Instalación
./mem setup opencode              # Plugin para OpenCode
./mem setup claude-code           # Plugin para Claude Code

# Configuración
./mem settings --show                       # Ver settings (auto-approve, grafo externo, ADR, etc.)
./mem settings --code-graph=false           # Apagar el grafo de código externo
./mem settings --code-graph-providers=a,b   # Proveedores candidatos, en orden de prioridad
./mem settings --code-impact-annotation=false  # Apagar la anotación de impacto al guardar
./mem settings --adr-sync=true              # Activar la sincronización bidireccional de ADR
./mem adr-sync status                       # Ver el estado de la sincronización de ADR

# Portabilidad de memorias
./mem export                          # Volcar memorias + relaciones a un JSON portable
./mem import backup.json              # Importarlas en otro proyecto/máquina (dedup)

# Optimización de contexto
./mem pack build --task "..." --max-tokens 4000   # Paquete de contexto acotado a una tarea
./mem pack compress < texto.txt                   # Comprimir un texto suelto

# Mantenimiento de memoria
./mem purge --older-than-days 90  # Purgar memorias viejas del proyecto actual
./mem compact                     # Recuperar espacio en disco
./mem gc                          # Garbage collection a demanda (90 días default)
./mem uninstall --yes             # Desinstalación completa (reverso de install)

# Verificación
./mem --help                      # Listar comandos
./mem context                     # Contexto del proyecto (markdown)
go test ./...                     # Tests
```

Para más detalles técnicos, ver:
- `docs/MEMORY-PROTOCOL.md` — Referencia técnica del protocolo

---

## 11. Registro Multi-Agente (detalle completo)

Dos formas de configurar agentes, según si soportan registro MCP a nivel de usuario:

**Global (una vez por máquina)** — `mem setup-mcp --scope global --agents claude,codex,opencode`:

| Agente | Config MCP | Notas |
|--------|-----------|-------|
| **Claude Code** | `~/.claude.json` → `mcpServers.gomemory` (scope `user`) | Registrado vía `claude mcp add`, con detección de colisión de nombre |
| **Codex** | `~/.codex/config.toml` → `[mcp_servers.gomemory]` | Tabla única, sin `cwd` por proyecto |
| **OpenCode** | `~/.config/opencode/opencode.json` → `mcp.gomemory` | OpenCode mergea la config de usuario con la del proyecto activo (confirmado con `opencode debug config`); el plugin se instala en el mismo paso |

**Por proyecto (`mem install`, o `mem setup-mcp --scope project`)** — necesario para Cursor/Windsurf/Cline (sin registro global conocido), opcional para Claude/Codex/OpenCode:

| Agente | Config MCP | ¿Lo configura `mem install`? | Hooks |
|--------|-----------|------------------------------|-------|
| **Claude Code** | `.mcp.json` | Sí | Ciclo de sesión, compactación, turnos, subagentes y modo plan |
| **OpenCode** | `opencode.json` | Sí | `plugin/opencode/gomemory.ts` (auto-inicio, ya es global) |
| **Cursor** | `.cursor/mcp.json` | Sí | — |
| **Codex** | `~/.codex/config.toml` | Sí | Ciclo de sesión y turnos; el registro efectivo es de usuario |
| **Windsurf** | `.windsurf/mcp_config.json` | No — solo con `mem setup-mcp --agents windsurf` | — |
| **Cline** | `.cline/mcp_settings.json` | No — solo con `mem setup-mcp --agents cline` | — |

> Windsurf y Cline se configuran por la vía explícita para evitar crear carpetas
> de configuración en proyectos que no usan esos agentes.

> Los hooks son subcomandos del binario (`mem hook <evento>`), no scripts shell:
> no dependen de `bash`/`curl` ni de un servidor HTTP, y corren igual en Windows.
>
> Regla de oro: un hook nunca aborta el arranque del agente — ante cualquier
> error sale silencioso con código 0.
>
> El protocolo de memoria (cuándo guardar, buscar, cerrar sesión) no depende de
> ningún archivo del repositorio: el servidor `mem mcp` lo declara en
> `initialize.instructions`, en la descripción de cada tool, y embebido en la
> respuesta de `get_context`. `mem install` ya **no** escribe el
> bloque en `AGENTS.md`/`CLAUDE.md` — era una segunda copia del mismo texto.

### Configuración Manual MCP

Si prefieres no usar el instalador: con `mem` en el PATH, `claude mcp add -s user gomemory mem mcp` registra el
mismo resultado que `mem setup-mcp --scope global --agents claude` (delegar
en el CLI de Claude Code es más seguro que editar `~/.claude.json` a mano:
es un archivo grande con formato propio). Si prefieres editarlo directamente,
la entrada global vive en `~/.claude.json` → `mcpServers.gomemory`:

```json
{
  "mcpServers": {
    "gomemory": {
      "command": "/ruta/a/mem",
      "args": ["mcp"]
    }
  }
}
```

Para scope de proyecto en vez de global, la misma entrada va en `.mcp.json`
en la raíz del repo. Reiniciar el agente. Verificar con `/mcp`: gomemory expone
28 tools base y cuatro adicionales cuando Octopus AAR está activado.

> Nota: si existen **ambos** (un `.mcp.json` de proyecto y una entrada global
> con la misma clave `gomemory`), el de proyecto tiene precedencia — confirmado
> empíricamente. Si registraste el scope global y no ves el cambio, revisa que
> el repo actual no tenga su propio `.mcp.json` residual.

---

## 12. Seguridad

- **Sin telemetría externa** — los registros de uso y enrutamiento permanecen en el almacén local.
- **Binario autocontenido** — sin dependencias compartidas que puedan ser comprometidas.
- **Redacción automática** — contenido envuelto en `<private>...</private>` se elimina antes de llegar a la base de datos.
- **SQLite WAL** — integridad ACID con Write-Ahead Logging. Los datos sobreviven cortes de energía.
- **Permisos MCP granulares** — `forget_memory` queda fuera de auto-approve por ser destructivo/irreversible.

---

## 13. Stack Técnico

| Componente | Tecnología |
|------------|------------|
| Lenguaje | Go 1.27+ |
| Base de datos | SQLite embebido (`modernc.org/sqlite`, sin CGO) |
| TUI | `charmbracelet/bubbletea` + `bubbles` + `lipgloss` |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` |
| Timestamps | UTC-5 (Bogotá/Colombia, sin DST) |
| Dependencias runtime | 0 — binario autocontenido (~16MB) |
| Portabilidad | Linux, macOS, Windows (cross-compile nativo) |

---

## 14. Portabilidad

```bash
# Cross-compile sin toolchain adicional
GOOS=darwin  GOARCH=arm64 go build -o mem-darwin-arm64 ./infrastructure/
GOOS=darwin  GOARCH=amd64 go build -o mem-darwin-amd64 ./infrastructure/
GOOS=linux   GOARCH=amd64 go build -o mem-linux-amd64 ./infrastructure/
GOOS=windows GOARCH=amd64 go build -o mem-windows-amd64.exe ./infrastructure/
```

- El almacén SQLite global usa WAL; para mover memoria entre máquinas usa
  `mem export` y `mem import`, no copies una base activa.
- Timestamps UTC-5 independientes de la zona horaria local
- Las configuraciones MCP invocan `mem` desde el PATH; vuelve a ejecutar
  `mem setup-mcp` si cambia su ubicación o la integración queda desactualizada.

---

## 15. Modo Plan Determinista (`mem doctor`)

Entrar en modo plan tiene una garantía **determinista**, no solo un texto de
protocolo que el agente puede o no seguir: si el agente presenta un plan para una solicitud no
trivial y ese plan no tiene forma de árbol de tareas atómicas, el sistema lo **devuelve** con el
motivo antes de que llegue a la persona.

### El guard de forma del plan

```bash
mem hook plan-guard        # invocado por el agente antes de presentar el plan
```

- Una sola devolución por episodio de plan — nunca bloquea dos veces.
- Nunca se activa sobre solicitudes triviales de un solo paso.
- Apagable desde la configuración: `plan_guard_disabled` en `.memory/settings.json`, o el
  interruptor "Exigencia de forma del plan" en la TUI (`mem` → Configuración).
- Sesgado a permitir: ante cualquier duda, deja pasar el plan.

Ver [`docs/AGENT-INTEGRATION.md`](./AGENT-INTEGRATION.md) para el contrato completo (los tres
niveles de garantía y los cuatro dialectos de salida — `neutral`, `json`, `claude`, `text`), pensado
para que cualquier agente, incluso uno que gomemory no conozca, pueda implementarlo.

### `mem doctor` — reporte de cobertura

```bash
mem doctor                 # reporte legible
mem doctor --json          # salida estable, para scripts
mem doctor --strict        # exit != 0 si hay canales rotos (uso en CI)
```

Recorre, por agente y por ámbito (proyecto/usuario), los canales del modo plan atómico —guard de
forma, contexto al entrar, recordatorio por turno, instrucciones— y los del brazo extensor de grafo
de código (de solo lectura: `mem doctor` nunca lo escribe ni lo corrige). Reporta cada canal como
`ok`, `outdated`, `duplicated`, `missing` o `not_applicable` (agente no instalado, o no soporta ese
tipo de canal — nunca se usa para ocultar un canal roto). Un canal `not_applicable` con motivo es una
**degradación declarada**, no un problema; `outdated`, `duplicated` y `missing` sí cuentan para
`--strict`.

### Habilitar una sola vez para todos los proyectos

```bash
mem setup-mcp --scope global --agents claude,codex,opencode
```

El ámbito global también registra los hooks del modo plan. Un proyecto nuevo,
sin instalación propia, queda cubierto por la configuración del usuario.

### Export / Import de memorias (portable, cross-OS)

Cuando no quieres copiar la base entera sino **mover el conocimiento de un
proyecto a otro** (o entre máquinas con distinto S.O.), usa el bundle JSON:

```bash
# En el proyecto origen: vuelca memorias + relaciones a un JSON autocontenido
./mem export                                   # gomemory-export-<proyecto>-<YYYYMMDD>.json
./mem export --out backup.json                 # ruta explícita

# En el proyecto destino: impórtalo (append con dedup, no duplica)
./mem import backup.json
```

- **Contenido:** todas las memorias **+ sus relaciones** (sinapsis y veredictos
  del juez), preservando el grafo. El formato es JSON UTF-8, sin ids acoplados a
  la base (se remapean por `ref_id`) ni rutas absolutas de máquina.
- **Import idempotente:** dedup por hash de `tipo+título+contenido` — reimportar
  el mismo archivo no crea duplicados. Preserva los `created_at/updated_at`
  originales, remapea el proyecto y los ids de relación, y **no** genera
  sinapsis automáticas espurias.
- **Privacidad:** el import mantiene la redacción de `<private>` (el export solo
  vuelca lo ya persistido, que en origen ya viene redactado).
- **También desde la TUI:** tecla `c` → *Configuración* → *Exportar memorias* /
  *Importar memorias*. Esa misma pantalla muestra el estado del grafo de código
  externo y permite alternar el toggle sin salir a la línea de comandos.

## 16. Benchmark de tokens (`mem usage`)

`mem usage` responde, con datos medidos y no con intuición, cuánto ahorra `gomemory` al emitir
contexto en la sesión actual.

```bash
mem usage                    # sesión activa (o la más reciente con registros, o vacío)
mem usage --session <id>     # una sesión concreta
mem usage --all              # acumulado de todas las sesiones del proyecto
mem usage --json             # forma legible por máquina — el contrato que manda
```

Cada llamada que emite contexto (`mem context`, `search_memories`, `list_memories`, `mem pack
build/compress`, y cualquier otra operación por cualquier canal) queda registrada con la línea base
(lo que habría costado sin optimizar) y lo efectivamente emitido. El reporte muestra llamadas,
línea base, emitido, ahorro absoluto y porcentaje de reducción, con desglose por operación y por
canal (`mcp`, `cli`, `tui`).

**Honestidad de la medición:** la cabecera del reporte declara que el conteo es una aproximación
neutral (~4 caracteres por token), no el tokenizador de ningún proveedor — las cifras son
comparables contra sí mismas, no contra la facturación de nadie. Por defecto, todo lo que se
muestra está **medido**. Si se configura una ventana de referencia (`usage_window_tokens` en
`.memory/settings.json`, `0` = sin ventana por defecto), aparece una línea adicional rotulada
explícitamente `(estimado)` con el ahorro como porcentaje de esa ventana.

Contrato completo de la salida `--json`: [`docs/USAGE-REPORT-CONTRACT.md`](./USAGE-REPORT-CONTRACT.md).

**También desde la TUI:** tecla `u` desde la lista principal. La pantalla tiene dos secciones: la
[1] muestra el mismo reporte que `mem usage` para la sesión activa; la [2] deja escribir una tarea
y un presupuesto y calcular un snapshot puntual de optimización de contexto (motor compartido con
`mem pack build`), que no se conserva entre visitas a la pantalla.

### Consolidar memorias redundantes (`mem consolidate`)

```bash
mem consolidate            # previsualiza qué se fundiría (nada se modifica)
mem consolidate --apply    # aplica de verdad
```

Funde en una sola fila los grupos de memorias redundantes de un proyecto — por clave de tópico
compartida y por registros automáticos de actividad con contenido idéntico — sin perder ningún
contenido (los textos distintos de un grupo se conservan fusionados en la fila que queda). Es
irreversible: por eso previsualiza por defecto. También disponible en la TUI: pantalla de
*Mantenimiento* → *Consolidar*.

### Detalle de una memoria por ID (`mem get`)

```bash
mem get <id>
```

Recupera el detalle completo de una memoria por su identificador — el mismo mecanismo de
drill-down que la tool MCP `get_memory`, disponible también desde la línea de comandos.

## 17. Revisión adversarial por consenso (`mem review`)

Dos revisores independientes analizan el mismo target congelado, y un defecto
solo se considera real cuando ambos lo encuentran por separado. Sirve para
validación de alta confianza sobre algo concreto: un diff, un commit, una
especificación, una migración, un contrato.

La diferencia con pedirle a un agente que "revise bien" es que aquí las reglas
no se piden, se imponen. gomemory rechaza corregir un hallazgo sin corroborar,
exceder el presupuesto de rondas, o declarar un veredicto por parámetro. El
agente propone; gomemory valida y persiste.

### Abrir una revisión

```bash
mem review --pending          # TODO el trabajo pendiente (recomendado)
mem review --diff             # solo cambios que git diff ve
mem review --commit HEAD      # un commit concreto
mem review --file specs/042/spec.md

mem review --pending --read-only   # revisión que valida pero no corrige
```

Imprime el `review_id`, el digest congelado del target, el nivel de
independencia alcanzado y la **política efectiva** con la que quedó congelada la
revisión. Si ambos revisores usan el mismo proveedor y modelo, el nivel es
`degraded`: se declara así en vez de presentarse como una revisión plenamente
independiente que no lo fue.

**`--pending` frente a `--diff`.** `--diff` usa `git diff`, que no ve los archivos
sin seguimiento: una revisión de trabajo en curso con archivos recién creados
congelaba un target que no los contenía, y los revisores inspeccionaban menos de lo
que se creía. `--pending` incluye cambios preparados, sin preparar y archivos nuevos
no ignorados, con una identidad reproducible. Sobre un árbol limpio falla con
diagnóstico en vez de congelar un target vacío.

**`--read-only`.** Declara una revisión que valida pero no corrige. Con un defecto
grave confirmado termina `ESCALATED` en una sola llamada, en vez de quedarse
esperando una corrección que su alcance prohíbe. Sin esta distinción, una revisión de
solo validación se quedaba bloqueada para siempre.

### Política del proyecto

```jsonc
// .memory/settings.json
{
  "review_max_fix_rounds": 2,
  "review_auto_fix_severities": ["CRITICAL", "HIGH"],
  "review_fix_authorized": true
}
```

La política se congela en la revisión al iniciarla: cambiarla después no altera
revisiones ya abiertas. Un valor explícito en `review_start` gana a la del proyecto,
y esta a los defectos.

### Consultar

```bash
mem review status                 # la revisión abierta, si la hay
mem review status <review-id>
mem review history [--limit N]
mem review show <review-id>       # linaje completo
```

`status` de una revisión en curso muestra su **etapa**, no un veredicto:
confundir "va por la mitad" con "terminó sin defectos" es el error que esta
funcionalidad existe para impedir. Muestra además el alcance (solo lectura o
autorizada a corregir) y los recuentos por clasificación, severidad y estado de
re-juicio.

`show` reconstruye el linaje completo sin abrir la base: para cada hallazgo, sus
hallazgos fuente, la ronda de corrección que lo abordó, lo que declaró **cada**
revisor al revalidarlo y el estado agregado resultante.

### Qué garantiza el protocolo

- La clasificación de consenso debe cubrir **todos** los hallazgos de la ronda,
  exactamente una vez. Omitir uno rechaza la clasificación entera.
- La severidad de una clasificación se **deriva** de sus fuentes: no hay forma de
  degradar un `HIGH` corroborado.
- Un hallazgo queda `RESOLVED` solo si la corrección vigente lo incluye y **los dos**
  revisores lo dan por resuelto por separado. Un `REGRESSED` de cualquiera manda.
- Cada corrección parte del target que dejó la anterior. La cadena no admite saltos.
- `APPROVED`, `ESCALATED` e `INCOMPLETE` son inmutables: sobre ellos no se admite
  ningún resultado, consenso, corrección ni promoción.
- El aprendizaje solo se promueve desde una revisión aprobada.

### El ciclo

```text
target congelado → revisor A + revisor B (aislados) → consenso
   → CONFIRMED severo → corrección acotada → re-revisión
   → veredicto: APPROVED · ESCALATED · INCOMPLETE
```

- **CONFIRMED**: lo encontraron los dos por separado. Es lo único corregible
  automáticamente, y solo en severidad CRITICAL o HIGH por defecto.
- **SUSPECT**: lo vio uno solo. No dispara corrección.
- **ESCALATED**: hay algo severo sin resolver tras agotar las rondas, o una
  contradicción entre revisores. Lo decide una persona.
- **INCOMPLETE**: un revisor falló. Nunca se convierte en aprobado.

El presupuesto por defecto es de 2 rondas de corrección; configurable con
`review_max_fix_rounds` en `.memory/settings.json`, junto a
`review_auto_fix_severities`.

### Qué se conserva

Un defecto confirmado **y resuelto** puede promoverse a memoria del proyecto:
problema, causa raíz, resolución y verificación. No hay dónde poner un
transcript ni una cadena de razonamiento — la estructura no tiene ese campo.
Dos revisiones del mismo patrón refuerzan una memoria en vez de crear dos, y el
aprendizaje reaparece solo en `mem context` de sesiones futuras.

Una revisión aprobada **no** autoriza commit, push, merge, PR ni despliegue.

---

## 18. Octopus AAR

Octopus decide si una unidad de trabajo se mantiene inline o se delega. No inicia agentes: devuelve una ruta, un presupuesto y un contrato para que el runtime del agente ejecute el trabajo.

Está apagado por defecto. Actívalo desde Configuración en la TUI o con `octopus_enabled: true` en `.memory/settings.json`. Mientras está apagado, sus tools MCP no se registran y no se guarda telemetría.

```bash
# Decide la ruta de una tarea
mem octopus route "Investigar una carrera de expiración" --class investigation --read-only --files a.go,b.go

# Simula un plan JSON; no inicia subagentes
mem octopus plan --file plan.json

# Consulta límites, uso e historial
mem octopus status
mem octopus usage
mem octopus history -n 20
```

Las tools MCP disponibles al activar el módulo son `octopus_route_task`, `octopus_route_plan`, `octopus_report` y `octopus_status`.
