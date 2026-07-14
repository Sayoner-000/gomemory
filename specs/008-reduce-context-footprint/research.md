# Research — Reducir la huella de contexto de gomemory (Phase 0)

Resuelve las incógnitas del plan con decisiones concretas y justificadas. No hay
`NEEDS CLARIFICATION` bloqueantes: cada punto abajo cierra con una decisión.

## D1 — Valor del presupuesto de contexto por defecto

**Decisión**: presupuesto por defecto **24.000 caracteres** (≈ 6.000 tokens a
~4 chars/token) para la salida de `get_context`. **La unidad almacenada y
comparada es CARACTERES** (`len(output)`), no tokens, para evitar conversiones y
comparar directo contra el `strings.Builder`. `Budget ≤ 0` = **sin límite**
(comportamiento actual, opt-in). (Resuelve A1: unidad fijada en caracteres.)

**Racional**: la salida actual observada es ~82KB (~20k tokens) que persisten
toda la sesión. Bajar a ~24KB (SC-001: ≤ ~20KB) recorta ~70% manteniendo lo
esencial. 6k tokens es holgado para protocolo + conflictos + preferencias +
decisiones/patrones/bugfixes recientes resumidos, sin ahogar la ventana.

**Alternativas consideradas**: (a) 4k tokens — demasiado agresivo, arriesga
perder decisiones recientes útiles; (b) 12k — mejora poco frente al problema;
(c) sin default (opt-in puro) — no ataca el problema para quien no configura.
Se elige un default útil y ajustable.

## D2 — Estrategia de truncado por entrada (progressive disclosure capa 1)

**Decisión**: al construir el contexto, cada entrada larga se recorta a un
**extracto de ~200 caracteres** por su primer párrafo/oración, y se adjunta el
puntero `→ get_memory <id>` para el detalle íntegro. Orden de prioridad al
aplicar el techo: (1) protocolo activo y **conflictos** — nunca se recortan;
(2) preferencias; (3) decisiones/arquitectura/patrones/bugfixes recientes;
(4) aprendizajes y actividad. Si al llegar al techo quedan secciones, se cierran
con una línea «(+N memorias; usa search_memories/get_memory)».

**Racional**: los conflictos son accionables (FR-003) y las preferencias guían
el comportamiento del agente; ambos justifican prioridad. El extracto por
oración conserva el «qué es» sin el cuerpo completo. El puntero preserva acceso
total bajo demanda (FR-002, FR-006).

**Alternativas**: truncado ciego a N chars sin respetar límites de oración
(peor legibilidad); resumen por LLM (viola «sin tokens del agente» y agnosticismo,
añade latencia/costo). Se elige truncado determinista por oración.

## D3 — Compactar `search_memories` (progressive disclosure capa 1 en tools)

**Decisión**: `search_memories` deja de volcar `m.Content` íntegro (hoy
cmd_mcp.go:113) y pasa a `[id] tipo | título` + **extracto ~160 chars**. Se
reutiliza el mismo helper de extracto que D2. `list_memories` ya trunca a 77 →
se unifica al helper. `get_memory` **no cambia** (capa 3: detalle íntegro).

**Racional**: es la fuga por-llamada que reinfla la sesión (FR-005). Referencia
engram: ~100 tokens por resultado. Unificar el helper evita duplicar lógica (V.1).

**Alternativas**: campo `verbose` opt-in para volcar todo — innecesario, `get_memory`
ya cubre el detalle. Rechazado por complejidad.

## D4 — Proxy de «huella» para el recordatorio de compactación (P3)

**Decisión**: gomemory mantiene, por sesión activa, un **contador de bytes que
él mismo ha emitido** en respuestas de tools (se incrementa en el choke point de
respuesta MCP). El `hookTurnEnd` compara ese acumulado contra
`CompactThreshold` (default ~**48.000 caracteres** ≈ 12k tokens) y, si lo supera,
emite un recordatorio **neutral** una sola vez por umbral (debounce como
`computeSaveNudge`): «La memoria persistente ya aportó bastante contexto a esta
sesión; considera compactar el contexto para abaratar los próximos turnos.»

**Racional**: un servidor MCP no puede leer la ventana del cliente; su propia
salida acumulada es el único proxy honesto y es **exactamente** la métrica que el
usuario quiere bajar (el 22%). El mensaje es agnóstico: no nombra `/compact` ni
comando de ningún agente (FR-011); el cliente decide. Reutiliza el patrón de
umbral+debounce ya probado en el nudge de guardado.

**Alternativas**: (a) estimar la ventana del cliente — imposible desde el
servidor; (b) contar turnos — no correlaciona con tokens; (c) pedir al agente su
tamaño de contexto — rompe agnosticismo. Se elige el contador de huella propia.

**Nota de alcance**: el servidor **señala**, no compacta. La evicción la ejecuta
el cliente. Si el cliente no muestra el recordatorio, no se rompe nada.

## D5 — Resumen de sesión estructurado (P3)

**Decisión**: `end_session(summary)` conserva su firma (string libre) pero su
descripción MCP guía el formato estructurado **Objetivo / Hallazgos / Logrado /
Próximos pasos / Archivos**. En «Sesiones Recientes» del contexto, el resumen se
muestra **acotado al presupuesto** como cualquier otra entrada.

**Racional**: patrón validado por engram (bloque estructurado auto-inyectado).
Guiar por descripción evita romper el contrato de la tool y mantiene el
agnosticismo (cualquier agente puede rellenar las secciones). Mínimo impacto (V.1).

**Alternativas**: campos estructurados separados en el esquema de la tool —
mayor cambio de contrato sin beneficio claro; el markdown estructurado en el
string ya es consultable. Rechazado por complejidad.

## D6 — Estrategia de deduplicación/upsert en la fuente (P4)

**Decisión**: en `InsertMemory` (choke point único), antes de insertar:
- **Identidad exacta**: si existe una memoria del mismo `project`+`type`+`title`
  dentro de una **ventana temporal** (default 7 días), no se crea fila nueva:
  se **actualiza** `content`/`updated_at` de la existente (consolidación).
- **Upsert por tópico**: nueva columna opcional `topic_key`; si el guardado trae
  `topic_key` y ya existe una memoria con ese `topic_key` en el proyecto, se
  actualiza la existente (revisión) en vez de crear otra.
- Best-effort y no bloqueante respecto de la sinapsis/provenance ya presentes.

**Racional**: menos memorias redundantes ⇒ menos que inyectar (ataca la causa
raíz del tamaño). El choke point único garantiza que aplica a MCP, CLI, TUI y
checkpoints sin tocar cada call site. Idempotente (V.7). Patrón validado por
engram (dedup por identidad + upsert por `topic_key`).

**Alternativas**: (a) dedup por hash de contenido — frágil ante cambios triviales
de redacción; identidad por título es más estable y explicable; (b) dedup en
lectura (al construir contexto) — no reduce el almacenamiento ni el ruido en
búsquedas. Se elige dedup en escritura.

**Migración**: `ALTER TABLE memories ADD COLUMN topic_key TEXT` idempotente
(numerada, `IF NOT EXISTS` según el patrón del proyecto); índice parcial sobre
`(project, topic_key)` donde `topic_key IS NOT NULL`. Los `checkpoint`
automáticos quedan **excluidos** del dedup por identidad (su contenido varía por
turno; su acumulación se controla por el cap existente de 5 en el contexto).

## D7 — Configuración: dónde viven los tunables

**Decisión**: `Budget` y `CompactThreshold` (y `DedupWindowDays` si se expone)
se añaden a `Settings`/`SettingsData` (JSON en `.memory/settings.json`), con
defaults en `DefaultSettings()`, y se cablean al `Builder` y al hook desde
`container.go`. Ajustables desde la pantalla de settings de la TUI.

**Racional**: son preferencias de usuario por proyecto y en caliente, coherentes
con `CodeGraphDisabled`/`CodeGraphCommand`. Documentado como desviación
justificada del Principio IV en el plan (Complexity Tracking).

**Alternativas**: variables de entorno — reservadas a diferencias de entorno de
despliegue, no ajustables desde la TUI ni por proyecto. Rechazado.

## Resumen de defaults elegidos

| Tunable | Default | Semántica |
|---------|---------|-----------|
| `Budget` (contexto) | 24.000 caracteres (~6k tokens) | Techo blando de `get_context` medido en `len`; ≤ 0 = sin límite |
| Extracto por entrada (contexto) | ~200 chars | Capa 1 de progressive disclosure |
| Extracto `search_memories` | ~160 chars | Reemplaza el volcado íntegro actual |
| `CompactThreshold` | ~48.000 chars (~12k tokens) emitidos/sesión | Dispara el recordatorio neutral (una vez por umbral) |
| `DedupWindowDays` | 7 días | Ventana de identidad proyecto+tipo+título |
