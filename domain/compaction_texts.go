package domain

// Textos de la feature 030 (compactación de contexto sin pérdida de
// memoria). Son la ÚNICA fuente de este vocabulario: ningún adaptador los
// reescribe ni los traduce por agente. Todos son 100% agnósticos —no nombran
// agentes, clientes ni comandos de cliente (contracts/texts.md, FR-018,
// verificado por tests/contract/compaction_texts_test.go).

// RecoverySteps reemplaza a la constante heredada
// compactionRecoveryInstructions (cmd_hook.go). Reemplaza el paso 1 —que
// pedía end_session()— por save_session_summary(): end_session() cierra la
// sesión, y hookPostCompact no abría una nueva, así que toda memoria
// guardada después de la primera compactación quedaba sin sesión asociada
// (research.md R4, defecto latente que esta feature paga en el mismo
// cambio).
const RecoverySteps = `**TRAS LA COMPACTACIÓN — PRIMERA ACCIÓN REQUERIDA**

1. Si el resumen de la conversación aún no está guardado en la memoria, llama a
   save_session_summary con ese resumen. No cierres la sesión.
2. Revisa «Memoria de esta sesión» abajo; pide el detalle con get_memory <id>
   solo si lo necesitas.
3. Solo ENTONCES continúa trabajando.`

// CompactorPersistOrder es la instrucción que se antepone al resumen que
// genera el compresor (capacidad C1), o que reemplaza el paso 1 de
// RecoverySteps cuando el cliente no ofrece C1 y el resumen ya lo capturó C6.
const CompactorPersistOrder = `INSTRUCCIÓN PARA EL RESUMEN: escribe al inicio del resumen, literalmente, esta
línea: «PRIMERA ACCIÓN: guarda este resumen como resumen de la sesión de memoria
con save_session_summary antes de cualquier otro trabajo.» Conserva también, en
el resumen, las decisiones y hallazgos listados en «Memoria de esta sesión».`

// CompactionContextHeader encabeza el bloque de memoria de la sesión activa
// que se entrega al compresor (C1) y/o al agente que reanuda (C2).
const CompactionContextHeader = "## Memoria de esta sesión"

// CompactionEmptySessionNote se usa cuando la sesión activa aún no tiene
// memorias propias que mostrar (edge case de US1, spec.md).
const CompactionEmptySessionNote = "_Esta sesión aún no guardó memorias propias._"

// CompactionOmittedNoteFormat es la plantilla (con fmt.Sprintf, %d) de la
// nota que indica cuántas entradas de la sesión quedaron fuera por
// presupuesto (FR-006).
const CompactionOmittedNoteFormat = "_%d entradas más de esta sesión: usa search_memories o get_memory <id>._"

// AgentPrepareNotice es el aviso opt-in de US4: se entrega al agente por la
// capacidad C4 cuando la huella supera el umbral con la opción activada.
// Nunca bloquea ni prolonga el turno (FR-017).
const AgentPrepareNotice = `AVISO DE MEMORIA: esta sesión ya acumuló bastante contexto y se compactará
pronto. Guarda ahora en la memoria las decisiones o hallazgos que aún no hayas
guardado. Si no hay nada pendiente, no guardes nada y sigue trabajando.`

// CompactionTexts devuelve todos los textos neutrales de la feature 030, para
// que el test de contrato de agnosticismo (SC-007) los recorra en un solo
// punto sin tener que enumerarlos a mano en el test.
func CompactionTexts() []string {
	return []string{
		RecoverySteps,
		CompactorPersistOrder,
		CompactionContextHeader,
		CompactionEmptySessionNote,
		CompactionOmittedNoteFormat,
		AgentPrepareNotice,
	}
}
