# Fase 1 — Modelo de datos: Octopus AAR

Entidades del dominio, reglas de validación y esquema persistido. Los nombres de campo se escriben en inglés porque son identificadores de código; la narrativa va en español, según la constitución.

---

## 1. Entidades de dominio (en memoria, sin persistencia)

### WorkUnit — unidad de trabajo

| Campo | Tipo | Reglas |
|---|---|---|
| `ID` | cadena | Obligatorio, único dentro del plan. Sin espacios |
| `Objective` | cadena | Obligatorio. Un objetivo vacío hace la unidad inenrutable (INV-AAR-003) |
| `Class` | `TaskClass` | Opcional. Catálogo extensible; ausente equivale a `ClassUnknown` |
| `Dependencies` | `[]string` | Identificadores de otras unidades del mismo plan. Una dependencia inexistente es entrada inválida |
| `Scope` | `Scope` | Archivos o artefactos afectados, y si el trabajo es de solo lectura |
| `Complexity` | `Level` | `LevelTrivial`, `LevelLow`, `LevelMedium`, `LevelHigh` |
| `Risk` | `Level` | Misma escala |
| `ContextNeed` | `ContextNeed` | Tokens estimados de contexto requerido y si necesita casi todo el contexto del padre |
| `ExpectedOutput` | `OutputSpec` | Tamaño esperado y campos requeridos del resultado |
| `CriticalPath` | booleano | Si la unidad está en la ruta crítica del plan |
| `Optional` | booleano | Si el plan puede completarse sin ella (afecta la protección de la reserva) |

`TaskClass` es un tipo cadena con constantes conocidas (`trivial`, `local-change`, `implementation`, `investigation`, `repository-exploration`, `research`, `testing`, `documentation`, `architecture`, `validation`, `review`, `migration`, `integration`). Es extensible: un valor desconocido es válido y se trata como `ClassUnknown`, nunca como error (FR-012).

### RuntimeCapabilities — capacidades declaradas por el runtime

| Campo | Tipo | Default conservador |
|---|---|---|
| `Subagents` | booleano | `false` |
| `Parallel` | booleano | `false` |
| `IsolatedContext` | booleano | `false` |
| `ModelSelection` | booleano | `false` |
| `ContinuableAgents` | booleano | `false` |
| `MaxParallel` | entero | `1` |

**Regla de normalización**: una estructura ausente o vacía equivale al conjunto más conservador, lo que fuerza `INLINE` (FR-035). `MaxParallel <= 0` con `Parallel = true` se normaliza a `1`. `IsolatedContext = false` incrementa el costo estimado de delegar (FR-036), no lo prohíbe.

### Budget — presupuesto jerárquico

| Campo | Tipo | Reglas |
|---|---|---|
| `TotalTokens` | entero | `<= 0` significa sin presupuesto declarado: la política funciona igual y omite la validación de presupuesto |
| `MainAgentMax` | entero | Derivado del reparto configurado |
| `DelegationPoolMax` | entero | Techo del fondo de delegación |
| `ValidationReserve` | entero | Reserva protegida (INV-AAR-006) |
| `DelegationSpent` | entero | Consumo acumulado del fondo |

**Invariantes**: `MainAgentMax + DelegationPoolMax + ValidationReserve <= TotalTokens`. `DelegationSpent <= DelegationPoolMax` siempre; una decisión que lo excedería no se emite como `DELEGATE` (INV-AAR-005). La reserva solo se consume con autorización explícita del llamador.

**Reparto por defecto**: se declara una sola vez como constantes de dominio con nombre y es configurable; el protocolo no impone porcentajes fijos (FR-029).

### RouteDecision — decisión de enrutamiento

| Campo | Tipo | Reglas |
|---|---|---|
| `WorkUnitID` | cadena | Obligatorio |
| `Route` | `Route` | `INLINE`, `DELEGATE`, `PARALLEL`, `WAIT`, `REJECT` |
| `Reason` | `Reason` | Código de un catálogo cerrado, con su texto en español |
| `ContextBudget` | entero | `> 0` obligatoriamente cuando la ruta es delegada (FR-025) |
| `OutputBudget` | entero | `>= 0`; `0` significa que el runtime no admite tope de salida |
| `ParallelGroup` | cadena | Vacía salvo que la unidad pertenezca a un grupo |
| `EstimatedCost` | `CostEstimate` | Contexto, contrato, salida, coordinación e integración, cada uno estimado |
| `Estimated` | booleano | `true` mientras ninguna cifra provenga de un reporte real (FR-033) |
| `BlockedBy` | `[]string` | Solo con ruta `WAIT`: dependencias sin resolver |

**Catálogo cerrado de razones.** Esta es la pieza que hace explicable el enrutamiento sin exponer razonamiento privado (FR-007). Cada razón es una constante con texto fijo:

| Código | Texto |
|---|---|
| `ReasonTrivial` | el sobrecosto de delegar supera el beneficio esperado |
| `ReasonNoSubagents` | el runtime no declara soporte de subagentes |
| `ReasonUnresolvedDependency` | quedan dependencias directas sin resolver |
| `ReasonContextNearlyFull` | la tarea requiere casi todo el contexto del agente principal |
| `ReasonOverheadExceedsBenefit` | el costo estimado de delegar iguala o supera el de ejecutar inline |
| `ReasonIsolatableInvestigation` | investigación independiente con contexto fuertemente aislable |
| `ReasonBoundedInterface` | trabajo acotado sobre un contrato estable |
| `ReasonParallelEligible` | independiente de las demás y elegible para ejecución concurrente |
| `ReasonBudgetExhausted` | el presupuesto de delegación restante no cubre el costo estimado |
| `ReasonValidationReserveProtected` | los tokens restantes pertenecen a la reserva de validación |
| `ReasonFanOutLimit` | se alcanzó el tope de agentes delegados del plan |
| `ReasonDepthLimit` | se alcanzó la profundidad máxima de delegación |
| `ReasonDuplicateWork` | trabajo equivalente ya completado, en curso o cubierto por el contexto del padre |
| `ReasonPolicyForcedInline` | la política del llamador exige ejecución inline |
| `ReasonHistoricalEvidence` | la evidencia histórica del patrón favorece la delegación |

Un código nuevo se añade aquí y en ningún otro sitio. La razón nunca se compone concatenando texto libre.

### RoutingPlan — plan de enrutamiento

| Campo | Tipo | Reglas |
|---|---|---|
| `PlanID` | cadena | Obligatorio |
| `Decisions` | `[]RouteDecision` | **Ordenado por `WorkUnitID`**, nunca por orden de recorrido de mapa (SC-006) |
| `ParallelGroups` | `[]ParallelGroup` | Ordenado por identificador de grupo; miembros ordenados por identificador de tarea |
| `Budget` | `Budget` | Estado del presupuesto tras aplicar el plan |
| `DelegatedCount` | entero | `<= MaxSubagentsPerPlan` (INV-AAR-010) |

**Invariantes del plan**:

- Ningún grupo paralelo contiene dos unidades con dependencia directa entre sí, ni con dependencia transitiva sin resolver (INV-AAR-007).
- Ningún grupo excede `min(runtime.MaxParallel, policy.MaxParallel)` (INV-AAR-008).
- Dos unidades cuyos `Scope` de escritura se intersecan no comparten grupo, aunque no exista dependencia declarada.
- Un grafo con ciclos es entrada inválida y se rechaza señalando el ciclo, no se enruta como espera perpetua.

### ExecutionContract — contrato de ejecución

| Campo | Tipo | Reglas |
|---|---|---|
| `TaskID` | cadena | Obligatorio |
| `Objective` | cadena | Obligatorio y acotado (INV-AAR-003) |
| `Scope` | `Scope` | Archivos y artefactos del alcance |
| `Permissions` | `Permissions` | `Filesystem` (`none`/`read-only`/`read-write`), `Network` booleano |
| `ContextBudget` | entero | `> 0` |
| `Output` | `OutputSpec` | Tope de tokens y campos requeridos |
| `MaxDepth` | entero | Profundidad restante autorizada; `0` prohíbe al hijo delegar (INV-AAR-009) |

**Invariante de seguridad**: `Permissions` del contrato nunca es un superconjunto de los permisos del flujo principal (INV-AAR-014, FR-028). La comprobación se hace al construir el contrato, no al ejecutarlo, porque Octopus no ejecuta.

### DelegatedResult — resultado de una unidad delegada

| Campo | Tipo | Reglas |
|---|---|---|
| `TaskID` | cadena | Obligatorio |
| `Status` | `ResultStatus` | `completed`, `failed`, `insufficient_context` |
| `Summary` | cadena | Acotado por el presupuesto de integración |
| `Evidence`, `AffectedSymbols`, `Artifacts`, `Unresolved` | `[]string` | Listas acotadas |
| `Missing` | `[]string` | Solo con estado `insufficient_context` |

**Regla de compactación**: cuando el resultado excede el presupuesto de integración se reduce preservando conclusiones, evidencia, artefactos, decisiones, fallos, pendientes y referencias que necesiten tareas posteriores; se descarta explicación repetida, relleno conversacional, contexto duplicado y razonamiento intermedio (FR-040). La transcripción completa nunca se integra (INV-AAR-012).

### ExecutionReport — reporte del runtime

| Campo | Tipo | Reglas |
|---|---|---|
| `TaskID` | cadena | Debe corresponder a una decisión ya emitida |
| `Route` | `Route` | La ruta realmente ejecutada |
| `Status` | `ResultStatus` | Resultado |
| `ContextTokens`, `OutputTokens` | entero | Consumo real; `>= 0` |
| `DurationMS` | entero | `>= 0` |
| `Quality` | `Quality` | `accepted`, `partial`, `rejected` |

Un reporte para una tarea sin decisión previa se ignora sin error: registrar telemetría es fire-and-forget y nunca puede romper el flujo del runtime.

### PolicyOverrides — política del llamador

| Campo | Tipo | Efecto |
|---|---|---|
| `DelegationDisabled` | booleano | Fuerza `INLINE` |
| `DelegationForced` | booleano | Prefiere `DELEGATE`, **sujeto a** capacidades y seguridad (FR-051) |
| `MaxSubagents`, `MaxParallel`, `MaxDepth`, `MaxRetries` | entero | Topes; se toma siempre el más restrictivo entre el llamador, la configuración y el runtime |
| `PreferInline` | booleano | Inclina el desempate hacia `INLINE` |
| `TokenBudget` | entero | Presupuesto total para esta invocación |

---

## 2. Transiciones de estado de una unidad delegada

```text
        decisión emitida
               │
               ▼
          DELEGATED ──── reporte: completed ────▶ COMPLETED
               │
               ├──────── reporte: insufficient_context ──▶ EXPANDED (una sola vez)
               │                                              │
               │                                    ┌─────────┴─────────┐
               │                                    ▼                   ▼
               │                               COMPLETED         FALLBACK_INLINE
               │
               └──────── reporte: failed ──▶ RETRIED (a lo sumo MaxRetries)
                                                  │
                                        ┌─────────┴─────────┐
                                        ▼                   ▼
                                   COMPLETED         FALLBACK_INLINE
```

Reglas de la máquina:

- `EXPANDED` se alcanza como máximo una vez por tarea (FR-042, AC-012). Una segunda respuesta de contexto insuficiente va directo a `FALLBACK_INLINE`.
- `RETRIED` se alcanza como máximo `MaxRetries` veces, con valor por defecto 1 (INV-AAR-011, AC-011).
- `FALLBACK_INLINE` conserva el resultado parcial útil de la delegación fallida cuando es seguro entregarlo (FR-043).
- Ninguna transición reenruta trabajo ya completado (FR-020).

---

## 3. Esquema persistido

Una tabla nueva, aditiva e idempotente, dentro de `migrate()` en `adapters/secondary/persistence/db.go`:

```sql
CREATE TABLE IF NOT EXISTS octopus_executions (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    project           TEXT    NOT NULL,
    plan_id           TEXT    NOT NULL DEFAULT '',
    task_id           TEXT    NOT NULL,
    task_class        TEXT    NOT NULL DEFAULT '',
    route             TEXT    NOT NULL,
    reason_code       TEXT    NOT NULL,
    parallel_group    TEXT    NOT NULL DEFAULT '',
    context_budget    INTEGER NOT NULL DEFAULT 0,
    output_budget     INTEGER NOT NULL DEFAULT 0,
    estimated_tokens  INTEGER NOT NULL DEFAULT 0,
    decided_at        TEXT    NOT NULL,

    -- Se completan cuando llega el reporte del runtime; nulos hasta entonces.
    status            TEXT,
    context_tokens    INTEGER,
    output_tokens     INTEGER,
    duration_ms       INTEGER,
    quality           TEXT,
    reported_at       TEXT
);

CREATE INDEX IF NOT EXISTS idx_octopus_project_class
    ON octopus_executions(project, task_class);
```

**Por qué esta forma cierra la privacidad.** No hay ninguna columna de texto libre alimentada por contenido: `reason_code` viene del catálogo cerrado, `route`, `status` y `quality` son enums, y el resto son identificadores y cifras. No existe un lugar donde pudiera colarse contenido de contexto, una transcripción, una credencial o razonamiento privado. INV-AAR-013, FR-047 y SC-011 se verifican inspeccionando el esquema, no auditando cada escritura.

**Timestamps** en UTC-5 (Bogotá, sin DST), como el resto del proyecto.

**Con el módulo apagado no se escribe ninguna fila.** La tabla se crea igual, porque la migración es aditiva e incondicional; una tabla vacía no tiene huella observable.

---

## 4. Evidencia histórica (agregado, no tabla)

La recomendación por patrón de tarea se calcula con una consulta sobre `octopus_executions`, agrupando por `task_class`:

- número de ejecuciones,
- consumo medio con ruta inline,
- consumo medio con ruta delegada y su contexto medio,
- tasa de éxito de lo delegado.

Es **asesora**: alimenta el desempate de la política y nunca anula presupuesto, dependencias, seguridad, capacidades, fan-out ni recursión (FR-049, INV-AAR-015). Con la tabla vacía, la política funciona igual y emite decisiones válidas (FR-048, AC-015).

---

## 5. Cambios en configuración

Campo nuevo en `application/ports.SettingsData` y en `adapters/secondary/persistence.Settings`:

```go
// OctopusEnabled activa el módulo Octopus AAR (feature 027). Ausente/false =
// APAGADO. Polaridad en positivo, a diferencia de los ajustes vecinos
// *Disabled: Octopus es un flujo nuevo completo y opcional, no el refinamiento
// de uno existente. Mismo patrón opt-in que AdrSyncEnabled.
OctopusEnabled bool `json:"octopus_enabled,omitempty"`
```

Los topes de política (profundidad, fan-out, concurrencia, reintentos y reparto del presupuesto) se declaran como constantes de dominio con nombre y son sobrescribibles desde la misma configuración, con la semántica ya usada en el proyecto: ausente o `0` significa el valor de fábrica.
