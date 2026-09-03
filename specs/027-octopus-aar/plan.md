# Plan de implementación: Octopus AAR — Enrutador Adaptativo de Agentes

**Rama**: `main` (sin rama dedicada; el hook `before_specify` de este proyecto es `gomemory-context`, no la extensión git) | **Fecha**: 2026-09-02 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de funcionalidad en `/specs/027-octopus-aar/spec.md`

## Resumen

Octopus AAR añade a gomemory una capa de **política de enrutamiento**: dada una unidad de trabajo (o un grafo de tareas), su presupuesto y las capacidades declaradas por el runtime, decide `INLINE`, `DELEGATE`, `PARALLEL`, `WAIT` o `REJECT`, con una razón explicable, y arma el contrato de ejecución y el contexto mínimo de lo que se delega. **No ejecuta agentes**: el runtime lo hace y le devuelve el consumo real.

Enfoque técnico: la política vive completa en `domain/` como funciones puras y deterministas (sin I/O, sin modelo), lo que la hace verificable con tests de tabla y cumple la regla de campo "verde en tests no es funciona" — aquí lo verde sí es la especificación, porque la política *es* una función. Alrededor de ese núcleo se reutiliza lo que el proyecto ya tiene, en vez de construir de nuevo: `BuildContextPack` (feature 015) para el contexto mínimo y su presupuesto, `ports.TokenCounter` para las estimaciones, `ports.UsageRecorder` y la tabla de uso para la telemetría, `SpecKitReader` para leer un grafo de tareas ya existente. Lo único nuevo en persistencia es una tabla aditiva de ejecuciones.

El módulo nace apagado. Apagado significa apagado de verdad: ni tools MCP registradas, ni filas en el bootstrap de ToolSearch, ni texto en el bloque de protocolo, ni escrituras en base de datos.

## Contexto técnico

**Lenguaje/Versión**: Go 1.25 (módulo `mem`, toolchain go1.25.11)

**Dependencias principales**: ninguna nueva. `modelcontextprotocol/go-sdk` (tools MCP ya presentes), `charmbracelet/bubbletea` (fila de configuración en la TUI), `modernc.org/sqlite` sin CGO (tabla nueva), `testify` (pruebas)

**Almacenamiento**: SQLite del store global de gomemory. Una tabla nueva `octopus_executions`, creada de forma aditiva e idempotente en `migrate()` (`adapters/secondary/persistence/db.go`). Ajuste de módulo en `.memory/settings.json`

**Pruebas**: `testing` + `testify`. Unitarias sobre el dominio puro (tablas de casos), integración con base de datos real para la tabla nueva, contrato para la superficie MCP y para el apagado del módulo

**Plataforma objetivo**: binario autocontenido para Linux, macOS y Windows; sin dependencias de runtime

**Tipo de proyecto**: CLI + TUI + servidor MCP sobre stdio, arquitectura hexagonal

**Objetivos de rendimiento**: enrutar un plan de 50 tareas en menos de 1 s (SC-004); la decisión unitaria es aritmética sobre estructuras en memoria, sin I/O ni llamadas de red

**Restricciones**: la política es determinista y reproducible (SC-006) — prohibido cualquier no-determinismo (mapas recorridos sin orden, reloj, aleatoriedad) dentro de la decisión. Ninguna ruta puede requerir la ejecución de un modelo. Con el módulo apagado, huella observable cero (SC-001). Longitud de línea 120, `gofumpt`, documentación en español latino

**Escala/Alcance**: planes de hasta ~50 tareas por invocación; ~8 archivos de dominio nuevos, 3 casos de uso, 1 repositorio, 1 comando CLI con 5 subcomandos, 4 tools MCP y 1 fila de configuración en la TUI

## Verificación contra la constitución

*PUERTA: debe pasar antes de la Fase 0 y volver a verificarse tras la Fase 1.*

| Principio | Cómo lo cumple este diseño | Estado |
|---|---|---|
| **I. Arquitectura hexagonal** | La política completa vive en `domain/` sin imports de infraestructura ni I/O. Los casos de uso en `application/usecases/` solo conocen puertos. La persistencia, el CLI, la TUI y MCP son adaptadores. El cableado ocurre solo en `infrastructure/container.go` | ✅ |
| **II. SQLite con SQL directo** | Una tabla nueva vía `CREATE TABLE IF NOT EXISTS` dentro de `migrate()`, solo aditiva. SQL directo con parámetros bind, sin ORM. El repositorio no expone `*sql.DB` al caller | ✅ |
| **III. Testing first** | Cada hoja del plan de tareas nace con su prueba. El dominio puro es tabla de casos; el repositorio nuevo lleva prueba con base de datos real desde su creación; el puerto nuevo lleva su mock. Ningún test existente se modifica salvo el de contrato de tools MCP, que **sí** debe cambiar y se declara explícitamente abajo | ⚠️ ver nota |
| **IV. Configuración y entorno** | Los topes (profundidad, fan-out, concurrencia, reintentos, reparto del presupuesto) no se codifican en los sitios de uso: se declaran una sola vez como constantes de dominio con nombre y se sobrescriben desde `settings.json`, igual que `Budget`, `CompactThreshold` y `DedupWindowDays`. No se introducen variables de entorno porque estos valores cambian por proyecto, no por entorno | ✅ |
| **V. Principios operativos** | Simplicidad: se reutilizan `BuildContextPack`, `TokenCounter`, `UsageRecorder` y `SpecKitReader` en vez de duplicarlos. Idempotencia: enrutar es una función pura, repetirla no tiene efectos. Fallar rápido: el grafo de tareas se valida en el borde (ciclos, dependencias inexistentes). Fire-and-forget: registrar telemetría nunca bloquea ni invalida una decisión. MCP como integración primaria: la superficie de agente es la vía principal | ✅ |
| **Documentación en español latino** | Toda la documentación de `specs/027-octopus-aar/` y los comentarios del código en español | ✅ |
| **Estilo** | Imports agrupados stdlib → terceros → proyecto, `snake_case` en archivos, línea de 120, `gofumpt` | ✅ |

**Nota sobre el principio III (tests intocables).** Un test existente debe cambiar y se declara aquí antes de tocarlo: el test de contrato que levanta `mem mcp` y compara `domain.MCPAllTools()` contra el `tools/list` real. Al registrar las tools de Octopus de forma condicional, ese test necesita distinguir el caso apagado (base, sin Octopus) del encendido (base + Octopus). El cambio es una **extensión** del contrato, no una relajación: la aserción existente se conserva íntegra para el caso apagado y se le añade una segunda para el caso encendido. Queda como tarea explícita y con justificación en `tasks.md`.

**Sin violaciones que justificar.** La tabla de Complejidad queda vacía.

## Estructura del proyecto

### Documentación (esta funcionalidad)

```text
specs/027-octopus-aar/
├── plan.md                       # Este archivo
├── research.md                   # Fase 0: decisiones de diseño resueltas
├── data-model.md                 # Fase 1: entidades, invariantes, esquema
├── quickstart.md                 # Fase 1: guía de validación ejecutable
├── contracts/
│   ├── routing-policy.md         # Contrato de la función de política (dominio)
│   ├── mcp-octopus-tools.md      # Contrato de las 4 tools MCP
│   ├── cli-octopus.md            # Contrato del comando `mem octopus`
│   └── module-off.md             # Contrato del apagado (huella cero)
├── checklists/
│   └── requirements.md           # Validación de calidad de la especificación
└── tasks.md                      # Fase 2: NO lo genera /speckit-plan
```

### Código fuente (raíz del repositorio)

```text
domain/                                  # Política pura: sin I/O, sin modelo, determinista
├── octopus_workunit.go                  # WorkUnit, clasificación, alcance, complejidad, riesgo
├── octopus_capability.go                # RuntimeCapabilities + normalización conservadora
├── octopus_budget.go                    # Presupuesto jerárquico, reparto, reserva de validación
├── octopus_route.go                     # Route, RouteDecision, RoutingPlan, grupos paralelos
├── octopus_policy.go                    # RouteTask / RoutePlan: las reglas deterministas
├── octopus_contract.go                  # ExecutionContract, permisos, forma del resultado
├── octopus_result.go                    # Resultado delegado, compactación, contexto insuficiente
├── octopus_telemetry.go                 # Reporte de ejecución, agregados, evidencia histórica
├── octopus_*_test.go                    # Una suite por archivo
└── mcp_tools.go                         # MODIFICADO: nombres de tools + MCPToolsFor(enabled)

application/ports/
├── octopus_repository.go                # NUEVO: persistencia de decisiones, reportes y evidencia
└── settings_repository.go               # MODIFICADO: campo OctopusEnabled

application/usecases/
├── octopus_route_task.go                # Enruta una unidad: política + presupuesto + evidencia
├── octopus_route_plan.go                # Enruta un grafo: dependencias, paralelismo, fan-out
├── octopus_pack_contract.go             # Arma ContextPack + ExecutionContract de lo delegado
├── octopus_report.go                    # Ingesta reportes del runtime y calcula agregados
└── octopus_*_test.go

adapters/secondary/persistence/
├── octopus.go                           # NUEVO: repositorio SQL de octopus_executions
├── octopus_test.go                      # Prueba con base de datos real
├── db.go                                # MODIFICADO: CREATE TABLE IF NOT EXISTS octopus_executions
└── settings.go                          # MODIFICADO: campo OctopusEnabled

adapters/primary/cli/
├── cmd_octopus.go                       # NUEVO: mem octopus plan|route|status|usage|history
├── cmd_mcp_octopus_tools.go             # NUEVO: registerOctopusTools (registro condicional)
├── cmd_mcp.go                           # MODIFICADO: llamada condicional al registro
├── dispatcher.go                        # MODIFICADO: case "octopus"
└── cli.go                               # MODIFICADO: ayuda de uso

adapters/primary/tui/
└── tui.go                               # MODIFICADO: fila configRowOctopus + toggle

infrastructure/
└── container.go                         # MODIFICADO: cableado del repositorio nuevo
```

**Decisión de estructura**: se conserva la disposición hexagonal ya vigente del repositorio, sin subpaquetes nuevos. Los archivos de dominio de Octopus se distinguen por el prefijo `octopus_`, siguiendo la convención plana que `domain/` ya usa para capacidades completas (`adr_document.go`, `channel_matrix.go`, `plan_shape.go`, `context_pack.go`). La estructura por subpaquetes que sugería el documento de entrada (`internal/octopus/domain/…`) se descarta: el proyecto no tiene precedente de subpaquetes bajo `domain/` y añadirlo obligaría a un ciclo de imports o a duplicar tipos compartidos como `ContextPack`.

## Fases

- **Fase 0 — Investigación**: decisiones de diseño resueltas en [research.md](./research.md). Sin `NEEDS CLARIFICATION` pendientes.
- **Fase 1 — Diseño y contratos**: [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md).
- **Fase 2 — Tareas**: la genera `/speckit-tasks`. El orden de entrega recomendado sigue las prioridades de la especificación: política determinista (P1) → interruptor del módulo (P2) → planes y dependencias (P3) → contexto y contrato (P4) → presupuesto (P5) → simulación y telemetría (P6) → fallos (P7) → evidencia histórica (P8).

## Seguimiento de complejidad

Sin violaciones de la constitución que justificar.

## Riesgos identificados y su cierre

Declarados aquí antes de implementar, con la solución en el mismo paso:

| Riesgo | Por qué importa | Cierre |
|---|---|---|
| Registrar 4 tools MCP nuevas engorda el contexto de arranque de toda sesión, incluso de quien no usa Octopus | Contradice el trabajo de las funcionalidades 008, 015, 020 y 023 y rompería SC-001 | Registro condicional: las tools solo existen con el módulo encendido. `MCPAllTools()` conserva su significado actual (módulo apagado) y se añade `MCPToolsFor(enabled bool)` para el caso encendido |
| El bloque de protocolo, el bootstrap de ToolSearch y las listas de auto-aprobación derivan de `domain/mcp_tools.go` | Si derivan de la lista completa, mencionarían Octopus incluso apagado | Todos consumen `MCPToolsFor(s.OctopusEnabled)`; el valor por defecto reproduce exactamente el texto actual |
| `OctopusEnabled` invierte la polaridad de los ajustes vecinos, casi todos `*Disabled` | Quien lea `settings.json` puede asumir la convención contraria y dejar el módulo encendido sin querer | Nombre explícito en positivo, con precedente real en el proyecto (`AdrSyncEnabled`, también opt-in), documentado en el campo y con prueba que fija "ausente = apagado" |
| La aritmética del presupuesto invita a números mágicos repartidos por el código | Prohibición constitucional de hardcodear configuración | Constantes de dominio con nombre, definidas una sola vez, sobrescribibles desde `settings.json` |
| Un mapa de Go recorrido sin orden dentro de la política rompería la reproducibilidad (SC-006) | Un fallo así es intermitente y caro de diagnosticar | Toda salida ordenada de forma explícita por identificador de tarea; prueba que enruta el mismo plan 100 veces y compara la serialización completa |
| La telemetría podría filtrar contenido de contexto o credenciales | Violaría INV-AAR-013, FR-047 y SC-011 | La tabla solo admite identificadores, enums y cifras: no tiene ninguna columna de texto libre proveniente del contenido. Prueba de contrato sobre el esquema |

## Reverificación de la constitución tras la Fase 1

Repetida sobre el diseño ya concreto (`data-model.md` y `contracts/`), no sobre la intención:

- **Hexagonal**: `contracts/routing-policy.md` fija la política como funciones puras en `domain/`, sin puertos ni I/O en la firma. El puerto nuevo (`ports.OctopusRepository`) solo aparece en los casos de uso. Sin regresión. ✅
- **SQLite directo y aditivo**: una sola tabla, `CREATE TABLE IF NOT EXISTS`, más un índice también condicional. Sin `ALTER` destructivo, sin ORM. ✅
- **Testing first**: cada garantía de `contracts/routing-policy.md` y cada fila de `contracts/module-off.md` está redactada como aserción, lo que hace que `/speckit-tasks` pueda derivar la prueba antes que el código. El único test existente que cambia queda declarado con su justificación. ✅
- **Configuración**: un campo booleano nuevo, más topes con semántica "ausente o 0 = valor de fábrica", idéntica a `Budget`, `CompactThreshold` y `DedupWindowDays`. Sin valores de configuración incrustados en los sitios de uso. ✅
- **Operativos**: idempotencia garantizada por la pureza de la política; fallar rápido en la validación del grafo; fire-and-forget en `octopus_report`, que ignora sin error un reporte huérfano. ✅

**Hallazgo de la Fase 1 que corrige la especificación**: durante el diseño se verificó que el proyecto **sí** tiene precedente de ajuste opt-in en positivo — `AdrSyncEnabled`, apagado por defecto. La sección de Supuestos de `spec.md` se corrigió para citarlo: la polaridad de `OctopusEnabled` deja de ser una excepción sin respaldo y pasa a seguir un patrón ya existente.

Sin violaciones nuevas. La tabla de Complejidad sigue vacía.
