# Fase 1 — Modelo de datos: registro y reporte de uso

**Feature**: 020-token-usage-benchmark · **Fecha**: 2026-08-20

Dos entidades nuevas. Una se persiste (`UsageRecord`); la otra se calcula al vuelo y nunca toca el
disco (`UsageReport`). Ambas viven en `domain/usage.go`, sin imports de infraestructura
(Principio I).

---

## 1. `UsageRecord` — la unidad medida

Una emisión de contexto, medida. Es la única entidad nueva que se persiste.

| Campo | Tipo | Obligatorio | Significado |
|---|---|---|---|
| `ID` | `int64` | sí (autogenerado) | Identidad de la fila |
| `Project` | `string` | sí | Proyecto al que pertenece la emisión |
| `SessionID` | `string` | no | Sesión activa al emitir. Vacío si no había ninguna |
| `Operation` | `string` | sí | Operación de dominio que emitió (ver §1.1) |
| `Channel` | `string` | sí | Etiqueta descriptiva del medio. **Dato abierto** (ver §1.2) |
| `BaselineTokens` | `int` | sí | Lo que la emisión habría costado sin optimizar. ≥ `EmittedTokens` |
| `EmittedTokens` | `int` | sí | Lo que efectivamente se emitió. ≥ 0 |
| `CreatedAt` | `string` | sí | Instante, UTC-5, valor por defecto de la base |

### 1.1 `Operation` es de dominio, no de protocolo

`Operation` nombra **qué se hizo**, no cómo se llamó la función en un canal concreto. Los valores
son constantes del dominio:

```
build_context · search_memories · list_memories · get_memory
build_pack · compress_pack · plan_context · save_memory · other
```

Un canal que exponga la misma operación bajo otro nombre la registra igual: la traducción entre el
nombre del canal y la operación de dominio ocurre en el adaptador, nunca en el dominio. Esto es lo
que impide que la capacidad quede definida en el formato de un canal.

### 1.2 `Channel` es una etiqueta, no una autorización

`Channel` es un `string` libre. **No se valida contra ninguna lista** (FR-004). Los valores que hoy
produce el proyecto son `mcp`, `cli` y `tui`, pero un canal desconocido se registra con su etiqueta
y aparece en el reporte igual que los demás. Añadir uno nuevo no toca esta entidad.

### 1.3 Invariantes

| # | Invariante | Dónde se hace cumplir |
|---|---|---|
| I1 | `BaselineTokens ≥ EmittedTokens ≥ 0` | Constructor de dominio; el repositorio no arregla datos malos |
| I2 | `Saved() = BaselineTokens − EmittedTokens` | Método de dominio, nunca columna almacenada |
| I3 | Una operación que no optimiza tiene `BaselineTokens == EmittedTokens` y por tanto `Saved() == 0` | FR-005, vía el registro de respaldo del canal |
| I4 | Un fallo al persistir un registro no propaga error a quien emitía | FR-006; el grabador traga el error |

`Saved()` es un método, no una columna: almacenar un valor derivado abre la puerta a que la fila se
contradiga a sí misma.

---

## 2. `UsageReport` — la agregación

Se calcula a partir de los registros y **nunca se persiste**, igual que el paquete de contexto
(FR-036).

```
UsageReport
├── Scope          ámbito: sesión concreta | todas las sesiones
├── SessionID      vacío cuando el ámbito es «todas»
├── Project
├── Calls          número de emisiones registradas
├── BaselineTokens suma
├── EmittedTokens  suma
├── WindowTokens   ventana de referencia vigente (0 = sin ventana)
├── SchemaTokens   costo de los descriptores publicados (0 si no se midió)
├── ByOperation    []UsageBucket  ordenado descendente por BaselineTokens
└── ByChannel      []UsageBucket  ordenado descendente por BaselineTokens

UsageBucket
├── Key            nombre de la operación, o etiqueta del canal
├── Calls
├── BaselineTokens
└── EmittedTokens
```

### 2.1 Valores derivados

| Método | Definición | Caso borde |
|---|---|---|
| `Saved()` | `BaselineTokens − EmittedTokens` | Nunca negativo si se cumple I1 |
| `ReductionRatio()` | `Saved() / BaselineTokens` | **`0` cuando `BaselineTokens == 0`**, no división por cero |
| `WindowRatio()` | `Saved() / WindowTokens` | **Solo válido si `WindowTokens > 0`**; con 0, quien formatea omite la línea entera (FR-015) |

`WindowRatio()` es el **único** valor estimado del modelo. Puede superar 1 si el ahorro excede la
ventana declarada; se muestra tal cual, rotulado, sin recortarlo (caso borde de la spec).

### 2.2 Consistencia exigible

Para todo reporte, y en particular en la salida legible por máquina:

```
BaselineTokens − Saved() == EmittedTokens          (SC-002, diferencia exacta cero)
Σ ByOperation[].BaselineTokens == BaselineTokens
Σ ByChannel[].BaselineTokens   == BaselineTokens
Σ ByOperation[].Calls == Σ ByChannel[].Calls == Calls
```

---

## 3. Esquema de persistencia

Aditivo, dentro del `migrate()` existente (`adapters/secondary/persistence/db.go:105`), entre el
bloque de esquema y los `addColumnIfMissing`. Se usa la constante `Now` ya definida en `db.go:15`
(`datetime('now', '-5 hours')`, UTC-5, Principio II).

```sql
CREATE TABLE IF NOT EXISTS usage_records (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    project         TEXT NOT NULL,
    session_id      TEXT,
    operation       TEXT NOT NULL,
    channel         TEXT NOT NULL,
    baseline_tokens INTEGER NOT NULL DEFAULT 0,
    emitted_tokens  INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (<Now>)
);

CREATE INDEX IF NOT EXISTS idx_usage_project_session
    ON usage_records(project, session_id);

CREATE INDEX IF NOT EXISTS idx_usage_created
    ON usage_records(created_at DESC);
```

**Notas de esquema**:

- `session_id` admite nulo a propósito: una emisión sin sesión activa se mide igual, y perder el
  dato es preferible a perder la medición.
- `channel` es `TEXT NOT NULL` **sin `CHECK`**: cualquier restricción de valores permitidos violaría
  FR-004.
- No hay clave foránea hacia `sessions`: purgar una sesión no debe hacer fallar una inserción de
  uso, y el registro es fire-and-forget (I4).
- Solo `CREATE ... IF NOT EXISTS`, ningún `ALTER` destructivo: correr la migración dos veces
  termina sin error y sin tocar las tablas previas (SC-011, Principio V.7).

---

## 4. `ports.UsageRepository`

```
Record(rec domain.UsageRecord) error
    Inserta una emisión. Parámetros bind. No lee antes de escribir.

BySession(project, sessionID string) ([]domain.UsageRecord, error)
    Registros de una sesión. Devuelve slice vacío, no error, si no hay ninguno.

Sessions(project string, limit int) ([]string, error)
    Identificadores de sesión con registros, del más reciente al más antiguo.

Totals(project string) ([]domain.UsageRecord, error)
    Todos los registros del proyecto, para el ámbito «todas las sesiones».
```

Siguiendo el manejo de errores del proyecto: **ausencia de filas devuelve vacío, nunca un error de
«no encontrado»**.

---

## 5. Ajustes nuevos en `SettingsData`

Dos campos en la struct única `ports.SettingsData`, declarativos y sin lógica (Principio IV).

| Campo | JSON | Por defecto | Semántica |
|---|---|---|---|
| `UsageWindowTokens` | `usage_window_tokens` | **`0`** | Ventana de referencia. `0` = sin ventana: la línea de porcentaje no se muestra (FR-014, FR-015). Ningún valor por defecto corresponde a la ventana de un agente concreto |
| `ContextIndexMode` | `context_index_mode` | **`false`** | `false` = emisión completa, el comportamiento actual. `true` = modo índice (FR-034) |

Ambos siguen el patrón opt-in/opt-out ya establecido por `SpeckitContextDisabled`,
`AtomicPlanDisabled` y `PlanGuardDisabled`: ausente equivale al comportamiento histórico.

---

## 6. Entidades de la fase B

No introducen tablas. `GrupoConsolidable` es una vista calculada sobre `memories`:

| Criterio | Cómo agrupa | Requisito |
|---|---|---|
| Por clave de tópico | `project + topic_key`, con `topic_key` no vacío | FR-026 |
| Por contenido idéntico | `project + type + hash(content)`, restringido a registros automáticos de actividad | Necesario para que FR-030 y SC-008 tengan un Δ medible — ver [research.md §5](./research.md) |

**Regla de fusión**: se conserva la fila **más reciente** del grupo y se funde en ella el contenido
de las demás sin perder nada; el resto se elimina. Las memorias sin criterio de agrupación no se
tocan (FR-029). La operación previsualiza por defecto y solo aplica cuando se lo piden
explícitamente (FR-027), porque eliminar filas es irreversible.

---

## 7. Lo que este modelo NO cambia

| Se conserva | Por qué |
|---|---|
| `ContextPack` sigue sin persistirse | Decisión previa intacta; lo persistido es una entidad nueva y separada (FR-036) |
| `.memory/.footprint` en caracteres | Lo consumen el aviso de compactación y el enganche de fin de turno (FR-037) |
| Las columnas de `sessions` | No se añade ninguna métrica ahí; el uso vive en su propia tabla |
| Los tests existentes | Principio III: intocables sin autorización explícita |
