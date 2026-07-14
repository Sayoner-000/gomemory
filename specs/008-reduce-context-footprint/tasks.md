---
description: "Task list — Reducir la huella de contexto de gomemory"
---

# Tasks: Reducir la huella de contexto de gomemory en la sesión

**Input**: Design documents from `specs/008-reduce-context-footprint/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/mcp-tools.md, quickstart.md

**Tests**: INCLUIDOS y obligatorios — la constitución exige TDD (Principio III, NO NEGOCIABLE): test rojo antes de implementar. No se modifican tests existentes (Prohibición absoluta).

**Organization**: agrupado por historia de usuario (P1→P4), cada una entregable e independientemente testeable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: puede correr en paralelo (archivo distinto, sin dependencias pendientes)
- **[Story]**: US1..US4 según spec.md
- Rutas exactas incluidas

## Path Conventions

Proyecto único hexagonal. Tests co-locados como `{paquete}_test.go` (convención vigente: `memory_test.go`, `build_context_test.go`). BD real en los tests de `persistence` (patrón existente).

---

## Phase 1: Setup

**Purpose**: baseline verificable antes de tocar código.

- [x] T001 Registrar baseline de tamaño del contexto: ejecutar `go build -o mem .` y `./mem context | wc -c`, anotar el valor en `specs/008-reduce-context-footprint/quickstart.md` (Escenario 1) como referencia de SC-001.
- [x] T002 Confirmar suite verde de partida: `go test ./...` en verde (garantiza que las regresiones posteriores son atribuibles a esta feature).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: piezas compartidas por varias historias. **⚠️ Bloquean US1 y US2.**

- [x] T003 [P] Escribir test rojo del helper de extracto puro en `domain/extract_test.go`: trunca a N chars sin cortar palabra, respeta la primera oración cuando cabe, no altera textos cortos, entrada vacía → vacío.
- [x] T004 Implementar el helper de extracto puro en `domain/extract.go` (función pura, sin I/O) hasta poner en verde T003.
- [x] T005 Añadir tunables a `application/ports/settings_repository.go` en `SettingsData`: `Budget int`, `CompactThreshold int`, `DedupWindowDays int` (con comentarios de semántica y `≤0`).
- [x] T006 Reflejar los tres campos en `adapters/secondary/persistence/settings.go` (`Settings`) con `json:",omitempty"` y fijar defaults en `DefaultSettings()`: **Budget 24000 (CARACTERES ≈ 6k tokens)**, CompactThreshold 48000 (caracteres), DedupWindowDays 7. Todas las unidades son caracteres emitidos para comparar directamente con `len(...)` (research D1/A1).

**Checkpoint**: helper de extracto disponible y Settings con presupuesto/umbral/ventana listos.

---

## Phase 3: User Story 1 — Contexto de arranque acotado por presupuesto (P1) 🎯 MVP

**Goal**: `get_context`/`mem context`/hook de arranque acotados al presupuesto, con protocolo y conflictos intactos y punteros `get_memory <id>`.

**Independent Test**: con ≥100 memorias largas, `./mem context | wc -c` ≤ techo, `grep get_memory` > 0, y los conflictos siguen presentes (quickstart Escenario 1).

### Tests for User Story 1 ⚠️ (escribir primero, deben FALLAR)

- [x] T007 [P] [US1] Test rojo en `application/usecases/build_context_test.go`: con 100 memorias largas (mocks de lister/relations/session), la salida de `Builder.Build()` respeta el techo **en caracteres** (`len(out) ≤ Budget`), trunca entradas con puntero `get_memory <id>`, y **NO** recorta protocolo ni conflictos.
- [x] T008 [P] [US1] Test rojo en el mismo archivo de dos casos «sin truncar»: (a) opt-in `Budget ≤ 0` ⇒ salida sin truncar (equivalente a la actual); (b) **proyecto pequeño** cuyo contenido total < `Budget` (>0) ⇒ ninguna entrada truncada y sin punteros `get_memory` inyectados (cierra G1 / SC-003 / US1-AS3).

### Implementation for User Story 1

- [x] T009 [US1] Añadir campo `Budget int` a `Builder` en `application/usecases/build_context.go` y aplicar el helper de extracto (T004) por entrada larga (decisiones/arquitectura/patrones/bugfixes/aprendizajes/sesiones) con puntero `→ get_memory <id>`.
- [x] T010 [US1] Implementar contabilidad de presupuesto y prioridad de secciones en `Build()`: el presupuesto se mide en **caracteres emitidos** (`len` del `strings.Builder`); protocolo y conflictos nunca se recortan; al alcanzar el techo, cerrar secciones con «(+N memorias; usa search_memories/get_memory)». `Budget ≤ 0` = sin límite. Si el total ya cabe bajo el techo, no truncar nada ni inyectar punteros.
- [x] T011 [US1] Cablear `Settings.Budget` al `Builder` en `infrastructure/container.go` (leer con `ReadSettings(root)` como ya se hace para CodeGraph).

**Checkpoint**: US1 funcional — MVP demostrable con quickstart Escenario 1.

---

## Phase 4: User Story 2 — Progressive disclosure en tools de consulta (P2)

**Goal**: `search_memories` deja de volcar el contenido íntegro; `list_memories` usa el mismo helper; `get_memory` intacto.

**Independent Test**: quickstart Escenarios 2 y 3 — search devuelve extractos; `get_memory` devuelve el íntegro.

### Tests for User Story 2 ⚠️ (escribir primero, deben FALLAR)

- [x] T012 [P] [US2] Test rojo en `adapters/primary/cli/cmd_mcp_test.go` (crear si no existe): el handler de `search_memories` produce por resultado `[id] tipo | título` + extracto acotado (no el `content` íntegro), respetando un límite total.
- [x] T013 [P] [US2] Test rojo en el mismo archivo: `get_memory` devuelve el `content` completo sin truncar (protege la capa 3).

### Implementation for User Story 2

- [x] T014 [US2] Reemplazar el volcado de `m.Content` íntegro en `search_memories` por el helper de extracto (~160 chars) en `adapters/primary/cli/cmd_mcp.go` (línea del `sb.WriteString` del handler).
- [x] T015 [US2] Unificar `list_memories` al mismo helper de extracto en `adapters/primary/cli/cmd_mcp.go` (reemplaza el truncado ad-hoc a 77 chars), sin cambiar la forma de salida.

**Checkpoint**: US1 + US2 reducen arranque y por-llamada; `get_memory` sigue dando el detalle.

---

## Phase 5: User Story 3 — Compactación al cierre de turno/sesión (P3)

**Goal**: `hookTurnEnd` emite un recordatorio neutral de compactación al superar la huella emitida el umbral; `end_session` guía resumen estructurado.

**Independent Test**: quickstart Escenario 5 — huella < umbral ⇒ sin mensaje; ≥ umbral ⇒ mensaje neutral una vez, sin nombrar comandos de cliente.

### Tests for User Story 3 ⚠️ (escribir primero, deben FALLAR)

- [x] T016 [P] [US3] Test rojo del contador de huella por sesión en `adapters/secondary/persistence/footprint_test.go` (BD real): incrementar acumula por sesión activa; reset al iniciar sesión deja el acumulado en cero.
- [x] T017 [P] [US3] Test rojo del umbral/debounce en `adapters/primary/cli/cmd_hook_test.go`: huella < `CompactThreshold` ⇒ sin salida; ≥ umbral ⇒ mensaje neutral **una sola vez** (segundo turno sin nuevo umbral ⇒ silencio); el mensaje NO contiene `/compact` ni comando de agente.

### Implementation for User Story 3

- [x] T018 [US3] Implementar el contador de huella por sesión activa en `adapters/secondary/persistence/` (columna/campo en la sesión o marcador en `.memory/`, según el patrón de sesión vigente) con `Add(bytes)` y `Reset(project)`; exponer vía el puerto de sesión.
- [x] T019 [US3] Incrementar el contador en el choke point de respuesta MCP (envoltura de `CallToolResult`) en `adapters/primary/cli/cmd_mcp.go`, sumando `len(text)` emitido; best-effort, nunca bloquea.
- [x] T020 [US3] Resetear el contador en `hookPostCompact` y al iniciar sesión (`adapters/primary/cli/cmd_hook.go` / start_session), para que el umbral refleje la huella desde la última compactación.
- [x] T021 [US3] Implementar en `hookTurnEnd` (`adapters/primary/cli/cmd_hook.go`) la comparación huella ≥ `CompactThreshold` con debounce «una vez por umbral» (patrón `computeSaveNudge`) y emitir el recordatorio **neutral** (sin nombrar `/compact` ni comando de agente). `CompactThreshold ≤ 0` = desactivado.
- [x] T022 [P] [US3] Actualizar la descripción de la tool `end_session` en `adapters/primary/cli/cmd_mcp.go` para guiar el formato estructurado (Objetivo / Hallazgos / Logrado / Próximos pasos / Archivos), sin cambiar la firma.

**Checkpoint**: el ciclo se cierra — gomemory señala compactación de forma agnóstica y consolida la sesión.

---

## Phase 6: User Story 4 — Reducir la redundancia en la fuente (dedup + upsert) (P4)

**Goal**: `InsertMemory` consolida memorias equivalentes (identidad o `topic_key`) en vez de crear filas nuevas.

**Independent Test**: quickstart Escenario 4 — guardar 3 equivalentes ⇒ +1 fila; `topic_key` repetido ⇒ actualiza.

### Tests for User Story 4 ⚠️ (escribir primero, deben FALLAR)

- [x] T023 [P] [US4] Test rojo en `adapters/secondary/persistence/memory_dedup_test.go` (BD real): insertar 3 memorias con mismo `project`+`type`+`title` dentro de la ventana ⇒ 1 sola fila (consolida `content`/`updated_at`); un `checkpoint` repetido NO se deduplica.
- [x] T024 [P] [US4] Test rojo en el mismo archivo: insertar con `topic_key` ya existente ⇒ actualiza la fila existente en lugar de crear otra (idempotencia SC-007).

### Implementation for User Story 4

- [x] T025 [US4] Añadir la columna en `migrate(db)` de `adapters/secondary/persistence/db.go` con el mecanismo EXISTENTE (no crear subsistema de migraciones): `addColumnIfMissing(db, "memories", "topic_key", "TEXT")` junto a los `addColumnIfMissing` actuales, y agregar `CREATE INDEX IF NOT EXISTS idx_memories_topic ON memories(project, topic_key) WHERE topic_key IS NOT NULL;` al `schema` embebido.
- [x] T026 [P] [US4] Añadir el campo `TopicKey string` a `domain/memory.go` (opcional; vacío = sin agrupación).
- [x] T027 [US4] Implementar dedup/upsert en `InsertMemory` (`adapters/secondary/persistence/memory.go`) antes del `INSERT`: identidad `project`+`type`+`title` dentro de `DedupWindowDays` (excluyendo `checkpoint`) ⇒ `UPDATE`; `topic_key` existente ⇒ `UPDATE`; con parámetros bind, best-effort respecto a provenance/sinapsis.
- [x] T028 [US4] Añadir el parámetro opcional `topic_key` al handler de `save_memory` en `adapters/primary/cli/cmd_mcp.go` y propagarlo a `domain.Memory.TopicKey` (y al CLI `mem save` si aplica).

**Checkpoint**: menos memorias redundantes en la fuente ⇒ contexto más denso a largo plazo.

---

## Phase 7: Polish & Cross-Cutting

- [x] T029 [P] Exponer `Budget`, `CompactThreshold`, `DedupWindowDays` en la pantalla de settings de la TUI (`adapters/primary/tui/`) para editarlos como los campos de CodeGraph.
- [x] T030 [P] Documentar los tunables y el comportamiento de acotado en `docs/` (español latino): sección en `README.md`/`docs/ARQUITECTURA.md` y, si aplica, `.env.example` no cambia (config vive en settings.json).
- [x] T031 Ejecutar la validación end-to-end de `quickstart.md` (Escenarios 1–6) contra el binario real, incluida la comprobación de no-regresión (ninguna memoria borrada/mutada por generar contexto).
- [x] T032 `go test ./...` verde + `govulncheck ./...` sin CRITICAL/HIGH; verificar cobertura ≥ 80% en los paquetes tocados.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (P1)**: sin dependencias.
- **Foundational (P2)**: tras Setup. **Bloquea US1 y US2** (helper de extracto + Settings).
- **US1 (P3)**: tras Foundational. MVP.
- **US2 (P4)**: tras Foundational (usa el helper de extracto). Independiente de US1.
- **US3 (P5)**: tras Foundational (usa `CompactThreshold`). Independiente de US1/US2.
- **US4 (P6)**: tras Foundational (usa `DedupWindowDays`); su migración es autocontenida. Independiente del resto.
- **Polish (P7)**: tras las historias deseadas.

### Within Each User Story

- Tests primero y en rojo, luego implementación.
- Dominio/migración → repositorio/caso de uso → adaptador/handler → wiring.
- `container.go` y `cmd_mcp.go` son archivos compartidos: sus tareas NO llevan [P] entre sí (evitar conflictos de mismo archivo).

### Parallel Opportunities

- T003 (helper) es [P] respecto de T005/T006 (Settings): archivos distintos.
- Dentro de cada historia, los tests marcados [P] corren juntos.
- Con las bases (P2) listas, **US1, US2, US3 y US4 pueden desarrollarse en paralelo** por distintas personas (tocan archivos mayormente disjuntos; coordinar solo los edits a `cmd_mcp.go`).

---

## Parallel Example: User Story 1

```bash
# Tests de US1 juntos (deben fallar primero):
Task: "T007 build_context_test.go — presupuesto + truncado + secciones intactas"
Task: "T008 build_context_test.go — opt-in Budget<=0"
```

---

## Implementation Strategy

### MVP First (solo US1)

1. Fase 1 Setup → 2. Fase 2 Foundational → 3. Fase 3 US1 → 4. **PARAR y VALIDAR** con quickstart Escenario 1 (≤ techo, punteros, conflictos intactos) → 5. Demo. Ya reduce la mayor fuente (los ~82KB de arranque).

### Incremental Delivery

Setup+Foundational → US1 (MVP, arranque acotado) → US2 (por-llamada acotado) → US3 (señal de compactación) → US4 (dedup en la fuente). Cada historia agrega valor sin romper las anteriores.

---

## Notes

- [P] = archivos distintos, sin dependencias pendientes.
- Verificar que cada test falla antes de implementar (regla de campo #2: verde en tests ≠ funciona; validar además contra el binario real en T031).
- No modificar tests existentes (Prohibición absoluta de la constitución).
- Config en `settings.json` (no env): desviación justificada del Principio IV, ya documentada en plan.md.
- El servidor **señala** compactación; no la ejecuta (evicción = del cliente).
