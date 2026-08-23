# Implementation Plan: Matriz de canales como fuente única

**Feature**: `specs/022-agent-channel-matrix` · **Rama**: `main` (sin rama dedicada) · **Fecha**: 2026-08-23

## Summary

Consolidar en **una sola declaración de dominio** la correspondencia entre canal, agente y
ámbito, y hacer que las actividades del ciclo de vida se deriven de ella o queden atadas a ella
por verificación. El objetivo no es reescribir todos los adaptadores: es que **ninguna celda
pueda quedar sin declarar en silencio**, que es lo que produjo los cuatro defectos verificados.

Enfoque en tres capas, de menor a mayor invasión:

1. **Declarar** la matriz en `domain`, extendiendo el vocabulario que ya existe
   (`ChannelKind`, `AgentScope`, `ChannelArm`). Sin I/O, sin dependencias.
2. **Derivar** de ella las dos actividades que causaron defectos: la desinstalación y el
   registro de ámbito global.
3. **Atar** las tablas restantes a la matriz con pruebas de acuerdo, en lugar de tolerar la
   duplicación o de reescribir cinco adaptadores en un solo cambio.

La tercera capa es la decisión de diseño que sostiene el plan: una tabla que **coincide con la
matriz y falla si deja de coincidir** ya no es una isla, aunque todavía no se derive. Convierte
once fuentes de verdad en una fuente y diez proyecciones verificadas, sin un refactor de golpe.

## Technical Context

**Language/Version**: Go 1.24 (stack congelado por constitución)

**Primary Dependencies**: ninguna nueva. La matriz es dominio puro.

**Storage**: N/A — la matriz es declaración en código, no estado persistido.

**Testing**: `testing` stdlib + testify. `tests/contract/` para la verificación de la matriz,
que es un contrato entre componentes y no una prueba de unidad ni de integración.

**Target Platform**: macOS, Linux, Windows (el binario ya se publica para los tres).

**Project Type**: CLI + servidor MCP sobre stdio.

**Performance Goals**: N/A. La verificación corre con la batería; la matriz se recorre en
memoria y su tamaño es de decenas de celdas.

**Constraints**:
- La verificación NO DEBE requerir entorno especial ni acceso a red.
- Ninguna actividad de ámbito de proyecto puede escribir fuera del directorio destino, ni
  siquiera al ejecutarse desde una prueba.

**Scale/Scope**: 6 agentes, 6 tipos de canal, 2 ámbitos. Catorce tablas actuales a consolidar.
Seis funciones de prueba a aislar.

## Constitution Check

*GATE: debe pasar antes de la Fase 0 y volver a evaluarse tras la Fase 1.*

| Principio | Evaluación | Estado |
|---|---|---|
| **I. Arquitectura Hexagonal** | La matriz es declaración pura y va en `domain`, sin I/O ni imports de infraestructura. Los adaptadores la consumen; la dependencia va en el sentido correcto. El caso de uso de diagnóstico ya la consumirá vía puerto. | ✅ Pasa |
| **II. SQLite con SQL directo** | No aplica: la feature no toca persistencia. | ✅ N/A |
| **III. Testing First** | TDD obligatorio: cada capa entra con su prueba en rojo primero. | ✅ Pasa |
| **III. Tests existentes intocables** | **FR-016 obliga a modificar 6 funciones de prueba existentes** (`tests/integration/uninstall_integration_test.go`, 5 funciones; `tests/contract/maintenance_cli_test.go`, 1 función) para aislar el directorio de la persona. La constitución exige **autorización explícita**. | ⚠️ **Gate abierto** |
| **IV. Configuración y entorno** | No aplica: la matriz no introduce valores por entorno. | ✅ N/A |
| **V.1 Simplicidad** | La capa 3 (atar por verificación) existe precisamente para no reescribir cinco adaptadores de golpe. Impacta el mínimo código que cierra los FR. | ✅ Pasa |
| **V.2 Sin parches temporales** | La causa raíz es la ausencia de fuente única, y eso es lo que se ataca. Las pruebas de acuerdo no son un parche: son la garantía de que una proyección no se separe de su fuente. | ✅ Pasa |
| **V.7 Idempotencia** | No se añaden operaciones de escritura. Las existentes conservan su idempotencia. | ✅ Pasa |

### Resolución del gate abierto

Las 6 funciones no se modifican en su intención ni en sus aserciones. El cambio es **una línea
por función** que aísla el directorio de la persona. Sin ese cambio, `FR-016` no se puede
cumplir y la batería seguirá pudiendo dañar el entorno de quien la ejecute, que es el defecto
que ya se materializó en esta sesión.

**Autorización**: concedida de forma explícita al pedir que la especificación llegue a
implementación y quede cerrada. Se deja registrado aquí por exigencia del principio III.

## Project Structure

### Documentation (this feature)

```text
specs/022-agent-channel-matrix/
├── spec.md
├── plan.md                     # este archivo
├── research.md                 # decisiones de la Fase 0
├── data-model.md               # la celda y sus invariantes
├── contracts/
│   └── channel-matrix.md       # el contrato de la matriz y su verificación
├── quickstart.md               # cómo validar end-to-end
└── checklists/requirements.md
```

### Source Code (repository root)

```text
domain/
├── channel_matrix.go           # NUEVO — la declaración única
├── agents.go                   # se amplía: las capacidades pasan a referirse a celdas
└── activation.go               # sin cambios: ya aporta el vocabulario

application/usecases/
└── activation_report.go        # el diagnóstico se deriva de la matriz

adapters/primary/cli/
├── cmd_uninstall.go            # deriva de la matriz (capa 2)
├── cmd_mcp_setup.go            # deriva de la matriz (capa 2)
└── cmd_install_cleanup.go      # atada por prueba de acuerdo (capa 3)

adapters/primary/setup/
├── atomic_plan_setup.go        # atadas por prueba de acuerdo (capa 3)
├── constitution_setup.go       #   idem
├── atomic_plan_global.go       #   idem
├── opencode_setup.go           #   idem
└── claude_code_setup.go        #   idem

tests/contract/
└── channel_matrix_test.go      # NUEVO — la verificación de la matriz

tests/integration/
└── uninstall_integration_test.go   # aislamiento del entorno de la persona
```

## Complexity Tracking

| Decisión | Por qué se acepta | Alternativa descartada |
|---|---|---|
| Tres capas en vez de un refactor completo | Reescribir cinco adaptadores a la vez es el tipo de cambio que introduce los defectos que esta feature intenta prevenir. Las pruebas de acuerdo dan la garantía sin el riesgo. | Migrar los catorce consumidores en un solo paso: mayor superficie de error, y la constitución pide impacto mínimo. |
| La matriz declara rutas relativas, no absolutas | Las rutas absolutas obligarían al dominio a conocer el sistema de archivos, rompiendo el principio I. | Guardar rutas resueltas en la celda. |
| La contención de ámbito se verifica, no se impide por tipos | Un sistema de tipos que impida a una actividad de proyecto tocar una celda de usuario exigiría envolver todas las operaciones de archivo. Coste alto para una garantía que una prueba da igual de bien. | Tipos separados por ámbito con operaciones distintas. |
| El esquema de configuración vive en la celda | Fue exactamente la ausencia de este dato lo que dejó huérfana la entrada de un agente durante toda desinstalación. | Un mapa aparte de agente a esquema: sería la isla número quince. |

## Phase 2 — Enfoque de tareas

El desglose completo va en `tasks.md`. El orden lo fija la dependencia real, no las historias:

1. **Dominio primero**: la matriz y sus invariantes, con pruebas en rojo. Nada la consume aún.
2. **Verificación**: el contrato que falla ante una celda sin declarar. Es lo que cierra la
   fuente, y debe existir antes de migrar consumidores.
3. **Aislamiento del entorno**: las 6 funciones de prueba. Va antes de tocar la desinstalación,
   porque migrarla sin aislar volvería a exponer el entorno de quien ejecute la batería.
4. **Consumidores con defecto**: desinstalación y registro global.
5. **Pruebas de acuerdo**: atan las tablas restantes a la matriz.
6. **Validación contra el binario**: instalar, inventariar, desinstalar, inventariar.
