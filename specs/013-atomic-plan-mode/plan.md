# Implementation Plan: Modo Plan Atómico con Memoria

**Branch**: `main` (sin rama dedicada) | **Date**: 2026-08-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/013-atomic-plan-mode/spec.md`

## Summary

Al entrar en modo plan, el agente invoca gomemory por su propia iniciativa, recibe el
método de descomposición atómica junto con el historial del proyecto, y entrega un plan
cuyas hojas son tareas verificables una por una.

**Enfoque técnico**: una sola superficie nueva —`get_plan_context()` como herramienta MCP y
`mem plan-context` como comando— devuelve método y contexto en una única llamada. El
disparador viaja en el bloque de protocolo que `mem install` ya escribe en los archivos de
agente, subiendo su marcador de `gomemory-protocol-v5` a `v6`. Esa es la decisión que hace
universal la cobertura: cualquier agente que lea el protocolo y pueda alcanzar gomemory
queda cubierto sin escribir integración específica para él.

La investigación de Fase 0 confirmó que **la mayor parte de la infraestructura ya existe**:
el ámbito global está construido y verificado empíricamente (feature 005), el presupuesto
de contexto se aplica dentro de `ContextBuilder.Build()` y se hereda solo, la captura de
planes aprobados ya cumple FR-035 y FR-036, y el reemplazo de versiones del bloque de
protocolo funciona por expresión regular sin necesidad de migración. El trabajo real es
más pequeño de lo que la spec sugiere.

## Technical Context

**Language/Version**: Go >= 1.22 (stack congelado por la constitución)

**Primary Dependencies**: `modelcontextprotocol/go-sdk` (herramienta MCP nueva),
`flag` stdlib (comando nuevo), `charmbracelet/bubbletea` (interruptor en la interfaz de
texto). Ninguna dependencia nueva.

**Storage**: Ninguno nuevo. Sin migraciones de SQLite. El único dato persistido es un campo
booleano en `.memory/settings.json`.

**Testing**: `testing` stdlib + `testify`, con `tests/unit/` y `tests/integration/`.
Cobertura ≥ 80 %. Además, verificación contra el binario construido según
[quickstart.md](./quickstart.md).

**Target Platform**: Linux, macOS y Windows — binario autocontenido sin CGO.

**Project Type**: Herramienta de línea de comandos con servidor MCP e interfaz de texto,
en arquitectura hexagonal.

**Performance Goals**: Sin objetivos nuevos. `get_plan_context()` reutiliza
`ContextBuilder.Build()`, cuyo coste ya está caracterizado. La única adición es concatenar
una plantilla embebida: coste despreciable.

**Constraints**:
- Código de salida **siempre 0**; ninguna rama puede interrumpir el modo plan (FR-034).
- La sección nueva del bloque de protocolo debe ser breve (~8 líneas): vive en el prompt
  de sistema de todos los turnos, y la feature 008 se hizo para reducir esa huella.
- Sin efectos secundarios en la ruta de planificación: solo lectura.

**Scale/Scope**: 36 requisitos funcionales, 5 historias de usuario. Superficie de código
estimada: una herramienta MCP, un comando, un caso de uso, un campo de configuración, un
interruptor en la interfaz de texto y una plantilla embebida.

## Constitution Check

*GATE: debe pasar antes de la Fase 0. Re-evaluado tras la Fase 1.*

| Principio | Evaluación | Resultado |
|-----------|------------|-----------|
| **I. Arquitectura Hexagonal** | El caso de uso vive en `application/usecases/`; los adaptadores en `adapters/primary/cli/` y `adapters/primary/setup/`; la plantilla en `infrastructure/templates/`. El dominio no se toca. El wiring sigue en el composition root, sin framework de inyección | ✅ PASA |
| **II. SQLite con SQL Directo** | Sin tablas, columnas, índices ni migraciones nuevas | ✅ PASA (no aplica) |
| **III. Testing First** | TDD obligatorio: tests primero para el caso de uso, la composición del documento, las tres ramas de degradación y la subida de versión del bloque de protocolo. Ningún test existente se modifica | ✅ PASA |
| **IV. Configuración y Entorno** | El campo nuevo va en la struct de configuración única (`SettingsData`), sin lógica y con default declarado. Nada hardcodeado | ✅ PASA |
| **V. Principios Operativos** | Idempotencia (la instalación ya la garantiza), fallar rápido no aplica —aquí manda la degradación silenciosa—, fire-and-forget en la misma línea que el resto de la integración, y simplicidad: una superficie nueva en vez de dos | ✅ PASA |
| **Documentación en español** | Toda la documentación de `specs/013-atomic-plan-mode/` está en español latino | ✅ PASA |
| **Prohibiciones absolutas** | Ninguna se roza: no se importan adaptadores desde el dominio, no se expone el driver de base de datos, no hay SQL, no se modifican tests existentes | ✅ PASA |

**Resultado del gate**: pasa sin violaciones. `Complexity Tracking` queda vacío.

### Re-evaluación tras la Fase 1

El diseño no introdujo desviaciones. Dos decisiones concretas **refuerzan** principios de
la constitución en vez de tensionarlos:

- **D5** (una llamada que devuelve método y contexto, en lugar de dos comandos separados)
  aplica el principio de simplicidad: menos superficie pública y menos pasos que el agente
  pueda ejecutar a medias.
- **D3** (reutilizar `ContextBuilder.Build()` en vez de reconstruir el contexto) evita
  duplicar la lógica de presupuesto. Duplicarla habría roto FR-007 en silencio, que es
  justo la clase de fallo que el principio "sin parches temporales" busca prevenir.

Un punto de atención para la implementación, no una violación: la plantilla del método
existirá en el binario **y** en los envoltorios nativos distribuidos. Se mitiga generando
siempre los envoltorios desde la plantilla embebida, nunca editándolos a mano (D6).

## Project Structure

### Documentation (this feature)

```text
specs/013-atomic-plan-mode/
├── spec.md                      # Especificación (36 FR, 13 SC, 5 historias)
├── plan.md                      # Este archivo
├── research.md                  # Fase 0 — 10 decisiones, sin incógnitas abiertas
├── data-model.md                # Fase 1 — entidades y estados
├── quickstart.md                # Fase 1 — 9 escenarios de validación
├── reference-ads-baseline.md    # Línea base del método, aportada por el usuario
├── contracts/
│   └── get-plan-context.md      # Fase 1 — contrato de la superficie nueva
├── checklists/
│   └── requirements.md          # Checklist de calidad de la spec
└── tasks.md                     # Fase 2 — lo genera /speckit-tasks, NO este comando
```

### Source Code (repository root)

Las rutas marcadas con **[nuevo]** se crean; el resto se modifica.

```text
application/
├── ports/
│   └── settings_repository.go        # + campo AtomicPlanDisabled
└── usecases/
    ├── build_plan_context.go         # [nuevo] compone método + contexto
    └── build_plan_context_test.go    # [nuevo] tests de las tres ramas

adapters/primary/
├── cli/
│   ├── dispatcher.go                 # + case "plan-context"
│   ├── cmd_plan_context.go           # [nuevo] comando
│   ├── cmd_plan_context_test.go      # [nuevo]
│   ├── cmd_mcp.go                    # + herramienta get_plan_context
│   └── cmd_install.go                # bloque de protocolo v5 → v6
├── setup/
│   ├── claude_code_setup.go          # + get_plan_context en ClaudeAutoAllowTools
│   ├── opencode_setup.go             # [nuevo] writeOpenCodePermissions, ambos ámbitos
│   ├── opencode_permissions_test.go  # [nuevo]
│   ├── atomic_plan_setup.go          # [nuevo] envoltorios nativos, ambos ámbitos
│   └── atomic_plan_setup_test.go     # [nuevo]
└── tui/
    └── tui.go                        # + interruptor en la pantalla de configuración

infrastructure/
└── templates/
    └── atomic-plan-method.md         # [nuevo] el método, fuente única de verdad
```

**Structure Decision**: se mantiene la arquitectura hexagonal ya establecida, sin capas ni
directorios nuevos. La feature encaja en la estructura existente:

- **`application/usecases/`** — la composición del documento de planificación es lógica de
  aplicación pura: recibe el método y el constructor de contexto, decide entre las tres
  ramas de salida y no toca infraestructura. Es la capa correcta y la que hace el caso de
  uso comprobable con dobles de prueba.
- **`application/ports/`** — el campo de configuración va en la struct existente, siguiendo
  la convención de los cuatro interruptores que ya viven ahí.
- **`adapters/primary/cli/`** — comando y herramienta MCP son adaptadores primarios: dos
  transportes distintos hacia el mismo caso de uso.
- **`adapters/primary/setup/`** — la distribución de los envoltorios nativos reutiliza
  `InstallPlugin`, igual que `speckit_extension.go` (feature 012).
- **`infrastructure/templates/`** — la plantilla del método queda cubierta por la directiva
  `go:embed all:templates` que ya existe. **No hace falta una directiva nueva**, igual que
  ocurrió con las plantillas del brazo extensor.

## Fases de implementación sugeridas

Ordenadas por dependencia y alineadas con las prioridades de la spec. `/speckit-tasks` las
convertirá en tareas.

| Fase | Entrega | Historias | Independientemente verificable |
|------|---------|-----------|-------------------------------|
| **A** | Plantilla del método + caso de uso + comando `mem plan-context` | 1, 2 | Escenarios 1 y 2 de `quickstart.md` |
| **B** | Herramienta MCP `get_plan_context` + pre-aprobación en **Claude Code y OpenCode** | 1, 2 | Escenarios 6, 6b y 7 |
| **C** | Bloque de protocolo v6 con el disparador | 1, 2, 3 | Escenarios 4 y 5 |
| **D** | Interruptor `atomic_plan_disabled` + pantalla de configuración | 4 | Escenario 3 |
| **E** | Envoltorios nativos y ámbito global | 3 | Escenario 5 |
| **F** | Verificación de la captura del plan aprobado | 5 | Escenario 8 |

**La fase A ya entrega valor por sí sola**: con el comando disponible, una persona puede
invocarlo a mano y obtener método más contexto. La fase C es la que activa la parte
autónoma.

**La fase F es de verificación, no de construcción**: D9 confirmó que la ruta ya cumple
FR-035 y FR-036; solo hay que comprobar que el árbol con caracteres de dibujo sobrevive
intacto.

## Riesgos y mitigaciones

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| El agente ignora la instrucción del protocolo | Se pierde la carga de contexto | Aceptado explícitamente en la spec (Assumptions). Se mitiga con una instrucción imperativa y breve, y con la pre-aprobación de la herramienta (D10, D11), que elimina la causa más común de que el protocolo no se aplique |
| **OpenCode no tiene hoy ninguna pre-aprobación** | La activación autónoma queda pidiendo permiso en cada planificación — la feature no funcionaría en OpenCode | Construir `writeOpenCodePermissions` con el esquema `permission` real de OpenCode, en ambos ámbitos (D11). **Es trabajo de la fase B, no opcional** |
| Escribir los permisos de OpenCode con la forma de Claude (`mcpServers[].autoApprove`) | Cero errores visibles y cero efecto: OpenCode ignora ese esquema | Documentado como prohibición explícita en el contrato. El proyecto ya tropezó con esto una vez (comentario de `WriteOpenCodeMCP`) |
| Abrir `gomemory_*` con un comodín plano en OpenCode | Se pre-aprobaría `forget_memory`, que es irreversible | Comodín más excepción: `gomemory_forget_memory` queda en `ask`, replicando la exclusión ya decidida del lado de Claude Code |
| La plantilla embebida y los envoltorios distribuidos divergen | Dos versiones del método en circulación | Generar siempre los envoltorios desde la plantilla embebida en cada instalación (D6) |
| El bloque de protocolo crece y engorda el prompt de sistema | Contradice la feature 008 | Restricción de tamaño en el contrato: la sección nueva es de ~8 líneas; el método completo llega por la llamada, no por el bloque (D5) |
| Cursor, Windsurf y Cline no tienen ámbito de usuario | Sin cobertura global para ellos | Documentado en D1. Quedan cubiertos por ámbito de proyecto, que `mem install` ya escribe |
| El árbol con caracteres de dibujo se mutila al persistirse | Se pierde la descomposición (FR-035) | Escenario 8 de `quickstart.md` lo verifica explícitamente |

## Complexity Tracking

> Se rellena solo si el Constitution Check tiene violaciones que justificar.

**Sin violaciones.** No hay complejidad que justificar.
