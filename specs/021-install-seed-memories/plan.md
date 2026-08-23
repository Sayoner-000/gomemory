# Implementation Plan: Instalación sin artefactos — reglas y constitución como memorias semilla

**Branch**: `main` (sin rama dedicada) | **Date**: 2026-08-23 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/021-install-seed-memories/spec.md`

## Summary

`mem install` deja hoy cinco artefactos en la raíz del proyecto destino
(`AGENTS.md`, `CLAUDE.md`, `speckit-constitution-gen.md`, `.windsurf/`, `.cline/`).
El bloque de protocolo que escribe en los archivos de instrucciones es una **segunda
copia** del texto que el agente ya recibe en la respuesta `initialize` del MCP
(`cmd_mcp.go:58`), y la constitución copiada son 635 líneas congeladas que divergen
de la fuente en cuanto alguien edita una de las dos.

La feature invierte el modelo: la instalación deja de generar archivos y siembra dos
memorias identificadas por `topic_key` — las reglas de trabajo como `preference`
(que `get_context()` emite **íntegra**, como excepción declarada al recorte por
presupuesto) y la constitución como `architecture` (consultable con
`mem constitution` y con el envoltorio `/constitution`). Los artefactos de
instalaciones previas se retiran, con respaldo previo de los archivos de
instrucciones en `.memory/backups/agent-files/`.

**Enfoque técnico**: sin tablas ni columnas nuevas —`memories.topic_key` existe desde
la feature 008 con su índice parcial. El trabajo es un puerto de lectura nuevo
(`MemoryTopicQuerier`), un caso de uso de siembra `create-if-missing`, una excepción
de emisión en el constructor de contexto, y cirugía de sustracción en `CmdInstall`.

**Dos arreglos que la feature absorbe porque los activa** (no son andamiaje, son
deuda que se paga en el mismo cambio):

1. **Defecto latente de `topic_key`** (research R2). `ListMemories` no proyecta la
   columna que su hermana `ListAllMemories` sí proyecta, así que `TopicKey` llega
   vacío a todo consumidor de esa vía. Hoy nadie depende de ella —la consolidación
   lee con `ListAll`—, pero FR-009 la usaría para excluir la semilla de la sección de
   preferencias: sin el arreglo, la comparación daría siempre falso y las reglas se
   emitirían **dos veces**. Se corrige de fondo (FR-030), y la presencia de la
   semilla deja de depender de la ventana de 100 memorias recientes (FR-031), que
   con los checkpoints al 69% del corpus la enterraría en ~100 turnos.

2. **Siembra inerte** (research R4). De los cuatro efectos laterales de
   `InsertMemory`, `exportToADR` **publicaría las 635 líneas de la constitución en el
   ADR externo** de quien tenga `adr_sync_enabled=true` —`architecture` es un tipo
   exportable— y otros dos son inocuos solo por accidente. Los tres se cierran con un
   único seam: `InsertSeedMemory`, que comparte núcleo con `InsertMemory` y omite los
   canales laterales (FR-032, FR-033, FR-034).

**Y una capacidad que legitima todo el modelo** (Historia 5): sembrar reglas y
constitución sin dar una vía cómoda de reemplazo convertiría a la herramienta en
autora de las normas del equipo. Las plantillas que se envían son un **default, no
doctrina**. Por eso la feature incluye export/import/restauración de documentos
fijados, por consola **y** por TUI, sobre un catálogo table-driven en el que añadir un
documento nuevo es una entrada, nunca un comando ni una pantalla nueva
(FR-035..FR-046).

## Technical Context

**Language/Version**: Go 1.22+ (`go.mod`: `go 1.24.0`)

**Primary Dependencies**: `modernc.org/sqlite` (sin CGO), `modelcontextprotocol/go-sdk`,
`charmbracelet/bubbletea` v2 · Ninguna dependencia nueva.

**Storage**: SQLite, store global con clave por proyecto. Sin migración: se reutiliza
`memories.topic_key` y su índice `idx_memories_topic` (`persistence/db.go:224-225`).

**Testing**: `testing` stdlib + `testify`. Suites en `tests/unit/`, `tests/integration/`,
`tests/contract/` y tests de paquete junto al código.

**Target Platform**: CLI autocontenido — Linux, macOS y Windows.

**Project Type**: CLI + servidor MCP sobre stdio, arquitectura hexagonal.

**Performance Goals**: la siembra añade 2 `SELECT` por clave sobre índice parcial en
el arranque del MCP, y 0 escrituras a partir del segundo arranque. La sección de
reglas suma ~2.6 KB fijos a `get_context()`, compensados por retirar la copia
duplicada del bloque de protocolo del archivo de instrucciones.

**Constraints**: la siembra y la limpieza son capas oportunistas — **ningún** fallo
puede interrumpir la instalación ni impedir que arranque el servidor MCP.
Idempotencia estricta: repetir `install` no debe producir cambio alguno.

**Scale/Scope**: 8 archivos nuevos, ~11 modificados, 46 requisitos funcionales.
Un solo cambio destructivo, explícitamente autorizado y con red de seguridad; más dos
correcciones de defecto que la feature activaría si no se pagaran aquí.

## Constitution Check

*GATE: verificado antes de la Fase 0 y revisado tras la Fase 1.*

| Principio | Cumplimiento | Cómo |
|---|---|---|
| **I. Arquitectura hexagonal** | ✅ | `domain/seed.go` es puro (dos constantes). El puerto `MemoryTopicQuerier` se declara en `application/ports/`, lo implementa `persistence.MemoryRepository`, y el wiring ocurre **solo** en `infrastructure/container.go:69-70`, junto a `Graph`/`Counter`/`Recorder`. `SeedDefaults` vive en `application/usecases/` y no importa ningún adaptador. |
| **II. SQLite con SQL directo** | ✅ | Sin ORM, sin migración. `GetMemoryByTopicKey` usa la misma consulta con parámetros bind que `findDuplicate` (`memory.go:317`). Se añade `COALESCE(topic_key,'')` al `SELECT` de `ListMemories`, alineándola con `ListAllMemories`. Cero concatenación de strings SQL. |
| **III. Testing first (NO NEGOCIABLE)** | ✅ | Cada tarea de código va precedida de su test en rojo. Los tests existentes de `buildIntegrationBlock()` y `composeAgentFile()` **no se tocan** (esas funciones sobreviven). Los dos que sí cambian —`protocol_block_test.go` y `lazy_init_test.go`— cambian porque el comportamiento que verificaban se retiró a propósito, y se declara en tasks. |
| **IV. Configuración y entorno** | ✅ | Sin variables de entorno nuevas. Las claves de tópico son constantes de dominio, no configuración: cambiarlas huerfanaría las semillas existentes (ver data-model §1). |
| **V.1 Simplicidad** | ✅ | Predomina la sustracción: se borran ~60 líneas de `CmdInstall`. Lo que se añade reutiliza cuatro patrones ya probados (`InstallAtomicPlanWrappers`, `removeMCPEntries`, campos opcionales del `Builder`, y el par `screenConfig`→`screenImport` con entrada de ruta). El catálogo `PinnedDocs` es table-driven: un documento nuevo es una entrada, no una rama nueva en CLI ni TUI. |
| **V.2 Sin parches temporales** | ✅ | Tres causas raíz atacadas, no rodeadas: la duplicación del protocolo entre archivo e `Instructions`; la columna ausente en `ListMemories` (se corrige la consulta, no se copia el valor a mano); y los canales laterales sobre la siembra (se añade una vía inerte, no se confía en que una opción siga apagada). Descartado explícitamente falsear `created_at` para ganarle al `ORDER BY` (research R2). |
| **V.7 Idempotencia** | ✅ | Siembra `create-if-missing` (C2/C6 del contrato), limpieza silenciosa en la segunda pasada, envoltorios que solo se reescriben si difieren. |
| **V.6 Fire-and-forget** | ✅ | Siembra, limpieza y envoltorios: avisan y siguen. Mismo criterio que `InstallSpeckitExtension` e `InstallAtomicPlanWrappers`. |
| **V.5 Fallar rápido / seguridad** | ✅ | La redacción de secretos **no** se desactiva para las semillas: la vía inerte omite sinapsis y export, nunca la defensa contra secretos. La igualdad texto-origen se verifica con un test guardián en vez de apagar la protección (data-model §3bis). |
| **Manejo de errores** | ✅ | `ByTopicKey` devuelve `(nil, nil)` para "no encontrado" — nunca un error de *not found*, según la regla explícita de la constitución. |
| **Documentación en español** | ✅ | `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md` en español latino. |

**Violaciones**: ninguna. La sección **Complexity Tracking** se omite por vacía.

**Punto de atención declarado (no es violación)**: FR-016 elimina archivos que pueden
contener texto propio de la persona. Es una decisión explícita y confirmada del
usuario, registrada en Supuestos de la spec. FR-017/FR-018 la acotan: respaldo previo
en `.memory/backups/agent-files/` y negativa a borrar si el respaldo falla. La
constitución no prohíbe operaciones destructivas autorizadas; sí exige informarlas,
y el plan lo hace en pantalla.

## Project Structure

### Documentation (this feature)

```text
specs/021-install-seed-memories/
├── spec.md                         # Especificación (/speckit-specify)
├── plan.md                         # Este archivo
├── research.md                     # Fase 0 — 11 hallazgos verificados contra el código
├── data-model.md                   # Fase 1 — entidades, ciclo de vida, reglas de limpieza
├── quickstart.md                   # Fase 1 — 10 escenarios de validación end-to-end
├── contracts/
│   ├── seed-defaults.md            # Puerto + caso de uso de siembra
│   ├── pinned-docs.md              # Catálogo + CLI `mem docs` + pantalla TUI
│   ├── cli-constitution.md         # Atajo `mem constitution`
│   └── install-artifacts.md        # Qué toca y qué deja de tocar la instalación
├── checklists/
│   └── requirements.md             # Checklist de calidad de la spec (16/16)
└── tasks.md                        # Fase 2 (/speckit-tasks — NO lo crea este comando)
```

### Source Code (repository root)

```text
domain/
└── seed.go                                    # NUEVO — claves canónicas + catálogo PinnedDocs

application/
├── ports/
│   ├── memory_topic.go                        # NUEVO — MemoryTopicQuerier
│   └── memory_seeder.go                       # NUEVO — MemorySeeder (vía inerte)
└── usecases/
    ├── seed_defaults.go                       # NUEVO — SeedDefaults (create-if-missing)
    ├── seed_defaults_test.go                  # NUEVO
    ├── build_context.go                       # MOD — campo Topics + sección fijada
    └── build_context_test.go                  # MOD — íntegra bajo Budget, sin duplicar

adapters/
├── primary/
│   ├── cli/
│   │   ├── cmd_install.go                     # MOD — quita pasos 4/4b, siembra, mensaje
│   │   ├── cmd_install_cleanup.go             # NUEVO — cleanupLegacyArtifacts
│   │   ├── cmd_install_cleanup_test.go        # NUEVO
│   │   ├── cmd_constitution.go                # NUEVO — atajo: mem constitution [--sync]
│   │   ├── cmd_docs.go                        # NUEVO — mem docs list|show|export|import|reset
│   │   ├── cmd_docs_test.go                   # NUEVO
│   │   ├── cmd_mcp.go                         # MOD — siembra al arrancar
│   │   ├── dispatcher.go                      # MOD — case "constitution"
│   │   └── cli.go                             # MOD — línea de ayuda
│   ├── tui/
│   │   ├── tui_docs.go                        # NUEVO — screenDocs (ver/exportar/importar/restaurar)
│   │   ├── tui_docs_test.go                   # NUEVO
│   │   └── tui.go                             # MOD — filas de catálogo al final del menú config
│   └── setup/
│       ├── constitution_setup.go              # NUEVO — envoltorios /constitution
│       ├── constitution_setup_test.go         # NUEVO
│       └── activation_inspect.go              # MOD — proyecto -> not_applicable
└── secondary/
    └── persistence/
        ├── memory.go                          # MOD — topic_key en ListMemories (defecto);
        │                                       #       ByTopicKey; InsertSeedMemory (vía inerte)
        ├── memory_test.go                     # MOD
        └── repositories.go                    # MOD — método del repositorio

infrastructure/
└── container.go                               # MOD — contextBuilder.Topics = memRepo

tests/
├── integration/
│   ├── protocol_block_test.go                 # REESCRITO — verifica eliminación + respaldo
│   └── lazy_init_test.go                      # MOD — amplía TestNoRepoFilesCreated
└── contract/
    └── integration_block_test.go              # SIN CAMBIOS (verificado)

docs/, README.md, INSTALLATION.md              # MOD — modelo de semillas, mem constitution
```

**Structure Decision**: se respeta la estructura hexagonal existente sin introducir
capas ni directorios nuevos. Cada archivo nuevo se coloca junto a su hermano del
mismo rol: `seed_defaults.go` junto a `build_context.go`, `constitution_setup.go`
junto a `atomic_plan_setup.go`, `cmd_constitution.go` junto al resto de comandos.
Los archivos que quedan sin llamador (`defaultAgentFile`) se eliminan; los que
conservan otro llamador (`composeAgentFile`, `protocolStart`, `protocolEnd`,
`setupWindsurf`, `setupCline`) **se conservan** — los usa
`setup-mcp --scope global` y `setup-mcp --agents`.

## Fases de implementación

| Fase | Alcance | Historia | Entregable verificable |
|---|---|---|---|
| **A0 — Defecto `topic_key`** | `topic_key` en el `SELECT`/`Scan` de `ListMemories` + test de regresión | US1 (FR-030) | Listar memorias devuelve la clave real, no vacía |
| **A1 — Vía inerte** | `insertMemory` con opciones, `InsertSeedMemory`, puerto `MemorySeeder` + tests G1/G2/G3 | US1 (FR-032..034) | 0 relaciones, 0 export con ADR activado, texto idéntico |
| **A2 — Semillas** | `domain/seed.go`, `MemoryTopicQuerier`, `ByTopicKey`, `SeedDefaults` | US1 | Test de idempotencia y no-sobrescritura en verde |
| **B — Contexto** | Campo `Topics`, sección fijada, exclusión de preferencias, wiring | US1 | `mem context` muestra las reglas íntegras (quickstart §3) |
| **C — Instalación** | Sustracción en `CmdInstall`, siembra, mensaje, siembra en MCP | US2 | Instalación limpia sin artefactos (quickstart §2, §8) |
| **D — Limpieza** | `cleanupLegacyArtifacts` + respaldo | US3 | Proyecto legado queda limpio con respaldo (quickstart §5, §6) |
| **E — Constitución** | `mem constitution`, dispatcher, envoltorios | US4 | Documento completo en una invocación (quickstart §7) |
| **G — Documentos fijados** | Catálogo `PinnedDocs`, `mem docs`, `screenDocs`, paridad CLI/TUI | US5 | Exportar → editar → importar en 3 pasos (quickstart §14-16) |
| **F — Colaterales** | `activation_inspect`, tests reescritos, documentación | US2 | `mem doctor` sin falsas alarmas (quickstart §9) |

A0 y A1 son prerrequisitos duros de A2: sembrar antes de cerrar la vía inerte
publicaría la constitución fuera en cualquier proyecto con `adr_sync_enabled=true`, y
emitir la sección antes de corregir `ListMemories` duplicaría las reglas en el
contexto. A2 y B completan la base; sin ellas, C sería una pérdida neta de
funcionalidad. D, E, F y G son independientes entre sí y paralelizables una vez
cerrada C. **G depende de A1** (comparte la vía inerte) y **de A2** (comparte el
catálogo de claves), pero no de C ni de D: se puede entregar aunque la limpieza del
instalador se posponga.

**A0 y A1 son entregables por sí solos**: corrigen defectos del producto actual y
tienen valor aunque el resto de la feature se posponga.

## Re-evaluación de la Constitución (post-diseño)

Revisado tras generar `research.md`, `data-model.md`, `contracts/` y `quickstart.md`:
**sin violaciones nuevas**.

Lo que el diseño cambió respecto de la primera lectura, y conviene dejar por escrito:

1. **La auditoría de efectos laterales pasó de nota al pie a decisión de diseño.**
   La primera lectura concluyó «los cuatro efectos son inocuos». Es falso para
   `exportToADR`: `architecture` es un tipo exportable (`adrSectionForType`,
   `memory.go:230`) y la exportación corre **síncrona** dentro de `InsertMemory` con
   4 s de timeout. Quien tenga `adr_sync_enabled=true` vería la constitución entera
   publicada en su ADR externo al instalar. Un efecto que solo es inofensivo mientras
   una opción esté apagada no es inofensivo. De ahí la vía inerte.

2. **Dos inercias eran accidentales, no garantizadas.** `formSynapse` no enlaza
   *porque* `SessionID` va vacío, y `annotateImpact` no anota *porque* no hay
   `Filepath`. Ambas dependían de datos que parecen incidentales y que un cambio
   razonable futuro alteraría sin que nadie lo notara. La vía inerte convierte la
   primera en garantía estructural; la segunda queda cubierta por el test de igualdad
   texto-origen.

3. **La redacción de secretos se mantiene activa sobre las semillas.** Es una defensa
   de seguridad, no un canal lateral: apagarla por comodidad abriría un agujero. La
   brecha real no era que se aplicara, sino que pudiera mutilar el texto en silencio;
   eso lo cierra el test guardián G1, no la desactivación. Verificado hoy: los 6
   patrones contra ambas plantillas dan **0 coincidencias**.

4. **El defecto de `ListMemories` se paga en esta feature.** Es latente solo porque
   `ConsolidateMemories` lee con `ListAll`. FR-009 sería el primer consumidor real de
   esa columna por la vía de `List`; dejarlo sin corregir produciría reglas
   duplicadas en el contexto. Un defecto latente que la feature activa deja de ser
   latente.

## Actualización del contexto del agente

El paso del flujo de spec-kit que pide anclar la referencia del plan entre marcadores
`<!-- SPECKIT START/END -->` en `CLAUDE.md` **no aplica en este repositorio**: esos
marcadores no existen, y esta feature retira precisamente ese archivo del modelo.
Introducirlos sería sembrar el artefacto que el cambio elimina.

**Sustitución, no omisión**: la referencia del plan se ancla en gomemory, que es
donde esta feature traslada el contexto del agente. La memoria con `topic_key`
`feature-021-plan` cumple la misma función —dar al agente un puntero estable al plan
vigente— y llega por `get_context()` sin depender de ningún archivo del repositorio.

Es el mismo principio que gobierna la feature: el contexto del agente vive en la
memoria, no en archivos duplicados en la raíz del proyecto.
