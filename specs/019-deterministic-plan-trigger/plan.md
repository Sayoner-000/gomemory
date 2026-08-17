# Implementation Plan: Activación determinista del modo plan atómico

**Branch**: `main` (sin rama dedicada: el hook de rama de spec-kit no está registrado) | **Date**: 2026-08-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/019-deterministic-plan-trigger/spec.md`

## Summary

El modo plan atómico de la feature 013 depende hoy de que el agente recuerde una instrucción que
solo viaja en el primer prompt de la sesión. Esta feature traslada el determinismo al **borde de
salida**, definido como **contrato neutral de agente**: *antes de presentar un plan, el agente invoca
un comando con el texto del plan y respeta la decisión que recibe*. Si el plan no tiene forma de árbol
y la solicitud no era trivial, se devuelve con el motivo antes de que llegue a la persona — una sola
vez por episodio, apagable y sesgado a permitir.

Ese contrato se publica para integradores y se traduce a los dialectos de cada agente: Claude Code lo
recibe como `PreToolUse(ExitPlanMode)` con `permissionDecision: "deny"`; cualquier otro agente lo
recibe en el dialecto neutral (código de salida y motivo por stderr), que es el que manda por defecto.
Ningún agente es el de referencia: conectar uno nuevo no debe requerir cambios en gomemory.

La inyección al **entrar** en modo plan pasa a ser mejor esfuerzo y aporta el historial; donde no
exista esa señal, una línea por turno cubre el hueco.

En paralelo se cierra la convivencia con el brazo extensor: el texto propio de gomemory deja de
enunciar el grafo de código y el árbol atómico como mandatos rivales y los emite **secuenciados**
(el grafo es el instrumento de exploración; el árbol es la forma de la salida), sin tocar ningún
canal ni mensaje del proveedor externo. Y se hace segura la actualización del bloque de protocolo
(marcador de fin + regla de límite para bloques legados) para poder por fin escribir el canal de
nivel usuario sin destruir contenido de la persona.

## Technical Context

**Language/Version**: Go 1.25.0 (toolchain go1.25.11) — stack congelado por la constitución

**Primary Dependencies**: stdlib + `modernc.org/sqlite` (sin CGO) + `testify` en pruebas. La
heurística de forma del plan NO añade dependencias: es análisis de texto puro.

**Storage**: SQLite ya existente (sin cambios de esquema). El estado de episodio de plan es un
marcador de archivo bajo `.memory/`, igual que `.session-tools-injected` y los marcadores de
debounce de los nudges existentes.

**Testing**: `testing` stdlib + `testify`; `tests/unit/` (mocks), `tests/integration/` (BD real),
`tests/contract/` (contratos hook/MCP). Cobertura ≥ 80%.

**Target Platform**: macOS y Linux, binario único `mem`; se integra por hooks de Claude Code
(`~/.claude/settings.json` y `<proyecto>/.claude/settings.json`) y por el plugin de OpenCode.

**Project Type**: CLI + servidor MCP, arquitectura hexagonal (`domain` / `application` / `adapters`
/ `infrastructure`).

**Performance Goals**: el hook `plan-guard` corre en el camino crítico de cada presentación de plan
→ < 50 ms, **sin tocar la base de datos** (solo texto y un marcador de archivo). `plan-entered`
puede consultar la BD: corre una vez por episodio.

**Constraints**:
- **Tope duro del canal: 10 000 caracteres** por salida de hook (documentado). `Budget` por defecto
  de `get_context` es **24 000**, y el método atómico ocupa ~4,2 KB: el documento de planificación
  **excede el canal por diseño**, así que el recorte con prioridad método > historial (FR-007) es
  obligatorio, no defensivo.
- **Código de salida**: 0 en los dialectos que transportan la decisión en la salida (`claude`, `json`,
  `text`), conservando la regla de la feature 013 (FR-034). Distinto de 0 **solo** en el dialecto
  `neutral`, donde el código *es* el vehículo de la decisión por contrato. Ningún fallo ambiental
  produce nunca un bloqueo: error interno → permitir.
- **Neutralidad de agente** (INV-6): el motor de decisión es único; los formatos por agente son
  traducciones. Un agente desconocido se atiende en `neutral`, no se rechaza.
- **INV-1..INV-5**: no se toca ningún canal, mensaje ni restricción del brazo extensor.

**Scale/Scope**: 2 subcomandos de hook nuevos, 1 subcomando de diagnóstico nuevo, 1 función pura de
dominio, 1 corrección en el gestor de bloques de protocolo, subida de protocolo v7 → v8, paridad de
texto en el plugin de OpenCode, y extensión del script de regresión a los dos brazos.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Cumplimiento | Cómo |
|---|---|---|
| **I. Arquitectura Hexagonal** | ✅ | La heurística de forma del plan es una función pura en `domain/` (sin I/O, sin imports de infraestructura). El estado de episodio y la lectura del payload viven en el adaptador `adapters/primary/cli`. El reporte de cobertura se compone en `application/usecases` sobre un puerto nuevo que consulta los canales instalados; los detalles de rutas quedan en el adaptador de setup. |
| **II. SQLite con SQL Directo** | ✅ | Sin cambios de esquema, sin consultas nuevas. El estado por episodio es un marcador de archivo efímero, coherente con los marcadores ya existentes (`.session-tools-injected`, debounce de nudges); meterlo en la BD añadiría escrituras en el camino crítico del hook sin ganar nada. |
| **III. Testing First (NO NEGOCIABLE)** | ✅ | Cada tarea de implementación va precedida por su test. La heurística se prueba con tabla de casos (planes en árbol, en prosa, triviales, mixtos, en otros idiomas). Los contratos de hook se prueban con payloads reales en `tests/contract/`. Ningún test existente se modifica. |
| **IV. Configuración y Entorno** | ✅ | Un solo interruptor nuevo (`plan_guard_disabled`) en la struct única de settings, con default seguro (activo) y sin lógica en config. Se suma al `atomic_plan_disabled` ya existente, que sigue apagando la feature completa. |
| **V. Principios Operativos** | ✅ | Simplicidad (una función pura + dos subcomandos, sin motor nuevo); sin parches (la causa raíz es que el disparador caduca y que el texto pone a competir dos directivas, y se atacan ambas); idempotencia (FR-012, verificada con dos instalaciones consecutivas); fallar rápido pero **degradando a permitir**; fire-and-forget donde aplica; exit 0 propagado. |

**Tensión resuelta explícitamente**: la feature 013 fijó "ninguna condición puede interrumpir el
modo plan del agente". El `plan-guard` **sí** interrumpe una presentación de plan — pero lo hace con
una decisión declarada, reversible, de una sola vez por episodio y apagable, y sigue terminando con
código 0. Interrumpir por decisión explícita del protocolo no es lo mismo que romper el modo plan
por un fallo ambiental, que es lo que aquella regla prohibía. Queda registrado en
[research.md](./research.md) §6.

**Gate**: PASA. Sin violaciones que justificar → sección *Complexity Tracking* omitida.

## Project Structure

### Documentation (this feature)

```text
specs/019-deterministic-plan-trigger/
├── plan.md              # Este archivo
├── research.md          # Fase 0: decisiones y verificaciones en vivo
├── data-model.md        # Fase 1: entidades y transiciones de estado
├── quickstart.md        # Fase 1: guía de validación ejecutable
├── contracts/
│   ├── agent-integration.md   # EL contrato: neutral, para cualquier agente
│   ├── hook-plan-guard.md     # Traducción a Claude Code del borde de salida
│   ├── hook-plan-entered.md   # Traducción a Claude Code del borde de entrada
│   └── doctor-report.md       # Contrato del reporte de cobertura
├── checklists/
│   └── requirements.md  # Validación de la especificación (ya generado)
└── tasks.md             # Fase 2 (/speckit-tasks — NO lo crea este comando)
```

### Source Code (repository root)

```text
domain/
├── plan_shape.go              # NUEVO — heurística pura: ¿el plan tiene forma de árbol?
├── plan_shape_test.go         # NUEVO — tabla de casos (incluye idiomas y formatos raros)
├── plan_budget.go             # NUEVO — ajuste puro al presupuesto del canal
├── activation.go              # NUEVO — catálogo de canales de activación y sus estados
├── agents.go                  # NUEVO — registro ÚNICO de capacidades por agente (niveles, dialecto, ámbitos)
└── mcp_tools.go               # sin cambios (ya expone MCPAllTools para el bootstrap)

application/
├── ports/
│   └── activation.go          # NUEVO — puerto: inspector de canales instalados
└── usecases/
    ├── build_plan_context.go  # + recorte por presupuesto de canal (método > historial)
    └── activation_report.go   # NUEVO — compone el reporte de cobertura de los dos brazos

adapters/primary/cli/
├── cmd_hook.go                # + case "plan-entered", + case "plan-guard"; línea por turno
├── hook_dialect.go            # NUEVO — detección y traducción de dialectos (neutral por defecto)
├── cmd_doctor.go              # NUEVO — `mem doctor [--json] [--strict]`
├── cmd_install.go             # marcador de fin de bloque + límite de bloques legados; v7 → v8
├── dispatcher.go              # + case "doctor"
└── nudge.go                   # + recordatorio de modo plan de una línea (barato, cada turno)

adapters/primary/setup/
├── claude_code_setup.go       # + PostToolUse(EnterPlanMode), + PreToolUse(ExitPlanMode)
└── activation_inspect.go      # NUEVO — implementación del puerto: lee settings/instrucciones

adapters/secondary/persistence/
└── settings.go                # + PlanGuardDisabled

adapters/primary/tui/          # + interruptor del guard en la pantalla de configuración

infrastructure/
├── plugin/opencode/gomemory.ts  # texto compuesto (grafo → árbol), paridad de la instrucción
└── templates/atomic-plan-method.md  # sin cambios de método; solo si el formato de salida se cita

scripts/
└── test-codebase-memory-activation.sh  # extensión: canales de modo plan + no-regresión del extensor

tests/
├── contract/                  # contratos de hook (payload real → JSON esperado), sin duplicados
├── integration/               # composeAgentFile preserva contenido posterior; doble instalación
└── (unit junto al paquete, por convención vigente del repo)
```

**Structure Decision**: se conserva la disposición hexagonal ya vigente del repositorio; no se
introduce ningún directorio nuevo de primer nivel. Dos reglas nuevas de ubicación, y las dos existen
para sostener la neutralidad de agente:

1. **Toda la decisión sobre la forma del plan vive en `domain/plan_shape.go`** (pura, sin I/O). El
   adaptador solo traduce payload → veredicto → dialecto. Eso hace la heurística comprobable con tabla
   de casos, sin montar un agente ni una sesión.
2. **Nada específico de un agente entra en `domain/` salvo como fila del registro**
   (`domain/agents.go`). Los nombres de eventos, matchers y formatos viven en
   `adapters/primary/cli/hook_dialect.go` y en el adaptador de setup. Si un dialecto concreto se
   filtrara al dominio, el siguiente agente heredaría la forma del anterior — que es exactamente la
   asimetría que esta feature corrige.

**Fuera de alcance declarado**: unificar en el registro las tablas por agente que ya existen dispersas
en el instalador (agentes con ámbito de usuario, envoltorios nativos, archivos de instrucciones
reconocidos). El registro nace como fuente única de lo nuevo; migrar el resto aquí convertiría esta
feature en un refactor del instalador y la constitución pide impacto mínimo. Queda anotado en
[research.md](./research.md) §13.3 como trabajo posterior.

## Fases

- **Fase 0 — [research.md](./research.md)**: verificación de las dos capacidades del canal de Claude
  Code (decisión con motivo en el borde de salida; inyección de contexto en el borde de entrada),
  diseño de la heurística, regla de límite para bloques de protocolo legados, ubicación del reporte
  de cobertura y reconciliación con la regla de "no interrumpir" de la 013.
- **Fase 1 — [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)**:
  entidades y transiciones del episodio de plan, contratos JSON de los dos hooks nuevos y del
  reporte, y guía de validación ejecutable extremo a extremo.
- **Fase 2 — `/speckit-tasks`**: desglose en tareas atómicas con TDD estricto por historia.

## Re-evaluación de la Constitución tras la Fase 1

Revisada contra los artefactos de diseño ya escritos:

- **I. Hexagonal**: el diseño confirma la separación — `PlanShapeVerdict` y el catálogo de canales son
  dominio puro; los dos hooks solo traducen payload → veredicto → JSON; el reporte se compone en un
  caso de uso sobre un puerto, con la inspección de rutas en el adaptador de setup. Ningún artefacto
  pide que el dominio lea disco.
- **II. SQLite**: confirmado sin esquema ni consultas nuevas. El único estado nuevo es un contador en
  un marcador de archivo, y el contrato del `plan-guard` lo declara como invariante ("la ruta de este
  hook no abre SQLite").
- **III. Testing First**: los tres contratos enumeran sus pruebas exigidas antes de existir el código,
  y el quickstart añade la validación contra el sistema en ejecución. La heurística queda comprobable
  con tabla de casos sin montar un agente, que era la condición para que la Historia 1 tenga pruebas
  reales.
- **IV. Configuración**: un solo interruptor nuevo (`plan_guard_disabled`) en la struct única, default
  seguro, sin lógica.
- **V. Operativos**: idempotencia (doble instalación en el quickstart §6), degradación silenciosa,
  exit 0 en todos los caminos, y la reconciliación explícita con la regla de no interrumpir
  (research.md §6).

**Gate post-diseño**: PASA. Sin violaciones nuevas; *Complexity Tracking* sigue sin aplicar.

**Riesgo único que puede cambiar el plan**: si V1 o V2 de [quickstart.md](./quickstart.md) §1 salen
negativas, la Historia 1 se queda sin mecanismo y hay que replantearla **antes** de escribir código.
Por eso la verificación en vivo es la primera tarea de la Fase 2, no un paso de validación al final.
