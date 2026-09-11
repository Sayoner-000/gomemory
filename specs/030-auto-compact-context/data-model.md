# Modelo de datos: Compactación de contexto sin pérdida de memoria

**Feature**: `030-auto-compact-context` · **Fecha**: 2026-09-10

No hay migraciones: todo el estado persistente usa columnas que ya existen.

## 1. CompactionCapability *(dominio, nuevo)*

Vocabulario del contrato (spec, «Capacidades del cliente»).

| Valor | Significado |
|-------|-------------|
| `pre_compact_input` | C1: aporte previo a la compactación |
| `post_compact_channel` | C2: canal al agente tras compactar |
| `subagent_final_text` | C3: fin de subagente con su texto final |
| `agent_turn_channel` | C4: canal al agente en el fin de turno o el turno siguiente |
| `human_channel` | C5: canal a la persona |
| `compact_summary_input` | C6: la integración recibe el resumen compactado |

## 2. AgentCapability *(dominio, ampliada)*

Campos nuevos:

| Campo | Tipo | Regla |
|-------|------|-------|
| `Compaction` | `map[CompactionCapability]bool` | Capacidades que la integración de gomemory aprovecha en ese cliente. |
| `CompactionUnavailable` | `map[CompactionCapability]string` | Por qué no se aprovecha: el cliente no la ofrece, o no se usa y por qué (research.md R12). |

**INV-C1**: para cada agente de `KnownAgents` y cada una de las 6 capacidades,
se cumple exactamente una de dos: `Compaction[c] == true`, o
`CompactionUnavailable[c] != ""`. Lo verifica un test de contrato que recorre
todos los agentes, igual que INV-1 en `ChannelMatrix`.

Valores iniciales: tabla R1 de research.md.

## 3. Session *(existente, uso ampliado)*

| Campo | Cambio |
|-------|--------|
| `summary` | Además del resumen de cierre, guarda el **último resumen compactado** mientras la sesión sigue activa. `end_session` lo sobrescribe con el resumen final. |
| `ended_at` | Sin cambio. `UpdateSummary` NUNCA lo toca (FR-023). |
| `last_prompt` | Sin cambio; lo lee el contexto de compactación. |

**Transiciones**:

```text
activa ──save_session_summary / compact-summary──▶ activa (summary actualizado)
activa ──end_session──▶ cerrada
(sin activa) ──post-compact──▶ activa (nueva)
```

Puerto nuevo y estrecho, `ports.SessionSummaryUpdater`:
- `UpdateSummary(id, summary string) error`: solo afecta a una sesión con
  `ended_at IS NULL`. Si no hay fila afectada, devuelve un error (misma
  semántica que `End`).

No se amplía `ports.SessionRepository`: tiene dobles de prueba en
`tui_usage_test.go` y `cmd_save_test.go`, que dejarían de compilar, y la
constitución prohíbe modificar tests existentes sin autorización. El
repositorio concreto implementa ambas interfaces.

## 4. Memory *(existente, consulta nueva)*

- Puerto nuevo y estrecho, `ports.SessionMemoryLister`:
  `ListBySession(project, sessionID string, limit int) ([]domain.Memory,
  error)`. Devuelve las memorias no borradas de esa sesión, de la más reciente
  a la más antigua. Si `sessionID` está vacío, devuelve una lista vacía y `nil`.
  No se amplía `ports.MemoryRepository`, que tiene dobles en `tui_test.go`,
  `cmd_save_test.go`, `cmd_footprint_test.go` e `import_adrs_test.go`.

**Aprendizaje capturado** (memoria `learning` creada por US3):

| Campo | Valor |
|-------|-------|
| `Type` | `learning` |
| `Title` | primeros 80 caracteres del ítem |
| `Content` | ítem completo + `\n\nProcedencia: subagente` |
| `TopicKey` | `passive:` + los 16 primeros caracteres hex de `sha256(ítem normalizado)` |
| `SessionID` | sesión activa |

Normalizar = pasar a minúsculas, colapsar espacios y quitar la puntuación
final. Así el mismo aprendizaje con distinto formato cae en el mismo tópico.

## 5. CompactionContext *(aplicación, valor calculado, no persistido)*

| Parte | Origen | Tope |
|-------|--------|------|
| Encabezado | constante neutral | fijo |
| Resumen previo | `Session.summary` | 600 caracteres |
| Último prompt | `Session.last_prompt` | 200 caracteres |
| Entradas | `ListBySession`, ordenadas: decisiones y bugfixes primero, luego por recencia | extracto de 300 caracteres cada una |
| Nota de omitidas | recuento de las que no cupieron | fija |

Tope total: 40 % del presupuesto de arranque (`Settings.Budget`). Si el
presupuesto es ≤ 0, no hay tope (spec 008, FR-004).

## 6. Settings *(existente, campo nuevo)*

| Campo | Tipo | Por defecto | Uso |
|-------|------|-------------|-----|
| `CompactAgentNotice` | `bool` | `false` | Activa US4. Solo tiene efecto con `CompactThreshold > 0`. |

## 7. Estado efímero en `.memory/` *(archivos, existente y nuevo)*

| Archivo | Estado | Ciclo |
|---------|--------|-------|
| `.last-compact-nudge` | existente | Pausa de 30 min, compartida con US4. |
| `.pending-agent-notice` | nuevo | Aviso de US4 pendiente para C4 en el turno siguiente. Se borra al consumirlo y en `post-compact`. |

La recuperación diferida de las integraciones que entregan C2 en el turno
siguiente se guarda en la memoria del proceso de la propia integración, no en
disco: su vida es la de la sesión del cliente.
