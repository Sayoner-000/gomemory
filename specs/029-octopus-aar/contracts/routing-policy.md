# Contrato: función de política de enrutamiento (dominio)

La política es el corazón de Octopus y vive en `domain/octopus_policy.go`. Este contrato define su firma, su orden de evaluación y sus garantías. Es lo único que hay que respetar para que la funcionalidad sea correcta; todo lo demás es transporte.

## Firma conceptual

```go
// RouteTask decide la ruta de UNA unidad de trabajo. Función pura: mismas
// entradas ⇒ misma salida, sin I/O, sin reloj, sin aleatoriedad.
func RouteTask(in RouteInput) RouteDecision

// RoutePlan decide las rutas de un grafo completo, formando grupos paralelos
// y aplicando los topes de fan-out. Función pura.
func RoutePlan(in PlanInput) (RoutingPlan, error)

type RouteInput struct {
    Unit         WorkUnit
    Resolved     map[string]bool      // dependencias ya completadas
    Capabilities RuntimeCapabilities
    Budget       Budget
    Policy       PolicyOverrides
    Evidence     *ClassEvidence       // nil = sin historial (cold start)
    Depth        int                  // profundidad actual de delegación
}
```

**El dominio no mide texto.** `RouteInput` llega con las cifras ya calculadas: quien invoca (el caso de uso) usa `ports.TokenCounter` para medir contexto, contrato y salida esperada, y las deposita en `WorkUnit`. La política solo hace aritmética sobre esas cifras y sobre constantes con nombre. Invertir esto crearía un ciclo de imports (`domain` → `ports` → `domain`) y destruiría la pureza que hace verificable a esta función.

`RoutePlan` devuelve error **solo** por entrada inválida (ciclo en el grafo, dependencia a una tarea inexistente, identificador duplicado o vacío). Nunca devuelve error por falta de presupuesto, de capacidades o de historial: eso son decisiones, no errores.

## Orden de evaluación (determinista y verificable)

Las reglas se evalúan en este orden. La primera que aplica gana y fija la razón. El orden es parte del contrato porque hace la decisión predecible y auditable.

| # | Condición | Ruta | Razón |
|---|---|---|---|
| 1 | `Policy.DelegationDisabled` | `INLINE` | `ReasonPolicyForcedInline` |
| 2 | Alguna dependencia directa sin resolver | `WAIT` | `ReasonUnresolvedDependency` |
| 3 | `!Capabilities.Subagents` | `INLINE` | `ReasonNoSubagents` |
| 4 | `Depth >= MaxDepth` efectiva | `INLINE` | `ReasonDepthLimit` |
| 5 | Trabajo equivalente ya cubierto | `INLINE` | `ReasonDuplicateWork` |
| 6 | `Unit.Complexity == LevelTrivial` | `INLINE` | `ReasonTrivial` |
| 7 | `Unit.ContextNeed.NearlyFullParent` | `INLINE` | `ReasonContextNearlyFull` |
| 8 | Costo estimado de delegar ≥ costo estimado inline | `INLINE` | `ReasonOverheadExceedsBenefit` |
| 9 | Presupuesto de delegación restante < costo estimado | `INLINE` o `REJECT` | `ReasonBudgetExhausted` |
| 10 | El remanente solo existe en la reserva de validación y la unidad es opcional | `INLINE` | `ReasonValidationReserveProtected` |
| 11 | Tope de agentes del plan alcanzado (solo en `RoutePlan`) | `INLINE` | `ReasonFanOutLimit` |
| 12 | Independiente, contexto aislable y beneficio > costo | `DELEGATE` | `ReasonIsolatableInvestigation` o `ReasonBoundedInterface` |
| 13 | En otro caso | `INLINE` | `ReasonOverheadExceedsBenefit` |

**Regla 9 — cuándo `REJECT` y cuándo `INLINE`**: `REJECT` cuando la unidad *exige* delegación para poder ejecutarse (el llamador la marcó como forzada, o el trabajo excede lo que el agente principal puede asumir); `INLINE` en cualquier otro caso. `REJECT` significa "esta delegación no debe ocurrir", nunca "esta tarea no puede completarse".

**Regla 12 — el desempate.** El beneficio esperado suma aislamiento de contexto, especialización, alivio de presión de contexto y paralelización; el costo suma transferencia de contexto, arranque del agente, coordinación, generación de resultado e integración. `Evidence`, cuando existe, mueve el desempate; nunca salta ninguna de las reglas 1 a 11. `Policy.DelegationForced` también mueve el desempate y también queda sujeto a las reglas 2 y 3 y a la validación de permisos.

**Regla 4 — `MaxDepth` efectiva** es `min(Policy.MaxDepth, configuración, tope de dominio)`, con el mismo criterio de "el más restrictivo gana" que se aplica a fan-out y concurrencia.

## Formación de grupos paralelos (`RoutePlan`)

Después de decidir cada unidad, las que quedaron `DELEGATE` se agrupan:

1. Se recorren las unidades **ordenadas por identificador**, nunca por recorrido de mapa.
2. Una unidad entra a un grupo existente solo si no tiene dependencia directa ni transitiva sin resolver con ningún miembro, y su alcance de escritura no interseca el de ningún miembro.
3. Un grupo nunca supera `min(Capabilities.MaxParallel, Policy.MaxParallel, configuración)`.
4. Las unidades de un grupo pasan a ruta `PARALLEL` con su identificador de grupo asignado; una unidad delegada que queda sola conserva `DELEGATE`.
5. Si `!Capabilities.Parallel`, no se forma ningún grupo y las unidades conservan `DELEGATE`.

## Garantías verificables

Cada una es una prueba, no una intención:

- **Pureza**: `RouteTask` y `RoutePlan` no abren archivos, no consultan la base de datos, no leen el reloj y no usan aleatoriedad.
- **Reproducibilidad**: enrutar el mismo plan cien veces produce serializaciones idénticas byte a byte.
- **Terminación**: `RoutePlan` termina en tiempo lineal sobre el número de aristas del grafo. Un ciclo se detecta antes de decidir, no durante.
- **Cold start**: con `Evidence == nil`, toda unidad recibe una decisión válida.
- **Razón siempre presente**: ninguna decisión sale con razón vacía.
- **Topes**: `DelegatedCount <= MaxSubagentsPerPlan` y ningún grupo excede la concurrencia efectiva, para cualquier entrada.
