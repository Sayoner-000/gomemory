# Contratos — Tools MCP afectadas (Phase 1)

gomemory expone su funcionalidad vía MCP (Principio V.9). Estos son los cambios
de **comportamiento** de las tools; las firmas se mantienen salvo donde se indica.
Todo cambio es agnóstico al agente: afecta el texto emitido, no un cliente concreto.

## `get_context` — sin cambio de firma, salida acotada

- **Antes**: devuelve el contexto completo (~82KB con 100 memorias íntegras).
- **Después**: devuelve el contexto **acotado al presupuesto** (`Settings.Budget`,
  default ~6k tokens). Cada entrada larga se muestra como extracto (~200 chars) +
  `→ get_memory <id>`. **Protocolo activo y conflictos nunca se recortan.** Si se
  alcanza el techo, cierra secciones con «(+N memorias; usa search_memories/get_memory)».
- **Invariante**: `Budget ≤ 0` ⇒ salida idéntica a la actual (opt-in).
- **Contrato de test**: con ≥100 memorias largas, `len(salida) ≤ techo` y la
  salida contiene protocolo + todos los conflictos.

## `search_memories` — sin cambio de firma, extracto en vez de contenido íntegro

- **Antes**: `[id] tipo | título\n  {content íntegro}` por resultado (fuga por-llamada).
- **Después**: `[id] tipo | título\n  {extracto ~160 chars}` por resultado.
- **Invariante**: el detalle íntegro sigue disponible en `get_memory <id>`.
- **Contrato de test**: una búsqueda de 10 memorias largas produce una salida
  cuyo tamaño total respeta un límite y cada ítem está truncado (no íntegro).

## `list_memories` — sin cambio de firma, extracto unificado

- **Después**: usa el mismo helper de extracto que `search_memories`/contexto
  (hoy trunca a 77 chars con lógica propia).
- **Contrato de test**: salida equivalente a la actual en forma, generada por el
  helper compartido (sin regresión).

## `get_memory` — SIN CAMBIOS

- Sigue devolviendo el `content` íntegro (capa 3 de progressive disclosure).
- **Contrato de test**: la salida contiene el `content` completo sin truncar.

## `end_session` — misma firma, resumen estructurado guiado

- **Firma**: `end_session(summary?: string)` (sin cambios).
- **Cambio**: la **descripción** de la tool guía el formato **Objetivo /
  Hallazgos / Logrado / Próximos pasos / Archivos**. El resumen persistido se
  muestra acotado en «Sesiones Recientes».
- **Contrato de test**: un `summary` estructurado se persiste y reaparece
  acotado en el contexto posterior (no íntegro si excede el extracto).

## `save_memory` — firma extendida (opcional), dedup en la fuente

- **Firma**: se añade parámetro **opcional** `topic_key?: string`. Sin él, el
  comportamiento por defecto solo cambia por el dedup de identidad.
- **Cambio**: antes de insertar, aplica dedup/upsert (D6): identidad
  proyecto+tipo+título dentro de la ventana, o `topic_key` existente ⇒ consolida
  la memoria existente en vez de crear otra. `checkpoint` excluido.
- **Contrato de test**: guardar 2 memorias equivalentes ⇒ 1 sola fila (SC-007);
  guardar con `topic_key` repetido ⇒ actualiza la existente.

## Hook `turn-end` — recordatorio neutral de compactación

- **No es una tool MCP**: es el `hookTurnEnd` (Stop en Claude Code / session.idle
  en OpenCode). Contrato transversal a agentes.
- **Cambio**: si la huella emitida por gomemory en la sesión supera
  `CompactThreshold`, emite **una vez por umbral** un recordatorio **neutral**
  (no nombra `/compact` ni comando de agente), no bloqueante.
- **Contrato de test**: huella < umbral ⇒ sin mensaje; huella ≥ umbral ⇒ mensaje
  neutral una sola vez (debounce); mensaje no contiene comandos de cliente.

## No-objetivos (frontera del contrato)

- gomemory **no** ejecuta compactación ni evicción: solo señala. La evicción es
  del cliente.
- Ningún cambio elimina o muta `content` persistido al acotar la salida.
