# Contrato: textos emitidos y herramienta MCP

Los textos viven como constantes en una sola fuente (dominio). Son idénticos
para todos los clientes. Un test de contrato los recorre contra una lista de
términos prohibidos: nombres de agentes y clientes, y comandos de cliente que
empiecen por `/` (SC-007).

## Textos

### `CompactorPersistOrder` *(C1)*

```text
INSTRUCCIÓN PARA EL RESUMEN: escribe al inicio del resumen, literalmente, esta
línea: «PRIMERA ACCIÓN: guarda este resumen como resumen de la sesión de memoria
con save_session_summary antes de cualquier otro trabajo.» Conserva también, en
el resumen, las decisiones y hallazgos listados en «Memoria de esta sesión».
```

### `RecoverySteps` *(C2; reemplaza a `compactionRecoveryInstructions`)*

```text
**TRAS LA COMPACTACIÓN — PRIMERA ACCIÓN REQUERIDA**

1. Si el resumen de la conversación aún no está guardado en la memoria, llama a
   save_session_summary con ese resumen. No cierres la sesión.
2. Revisa «Memoria de esta sesión» abajo; pide el detalle con get_memory <id>
   solo si lo necesitas.
3. Solo ENTONCES continúa trabajando.
```

Cambio respecto al texto vigente: el paso 1 deja de pedir `end_session`, que
cerraba la sesión (research.md R4), y el paso 2 deja de pedir `get_context`,
porque el contexto ya viene incluido y acotado.

### `CompactionContext` *(encabezado)*

```text
## Memoria de esta sesión
```

Sin memorias en la sesión:
`_Esta sesión aún no guardó memorias propias._` (US1.3).

Nota de omitidas:
`_N entradas más de esta sesión: usa search_memories o get_memory <id>._`

### `AgentPrepareNotice` *(US4, C4)*

```text
AVISO DE MEMORIA: esta sesión ya acumuló bastante contexto y se compactará
pronto. Guarda ahora en la memoria las decisiones o hallazgos que aún no hayas
guardado. Si no hay nada pendiente, no guardes nada y sigue trabajando.
```

### `CompactNudge` *(C5; vigente, sin cambios)*

Es `compactNudgeMessage` de `footprint.go`. No se modifica.

## Herramienta MCP `save_session_summary` *(nueva)*

- **Descripción**: «Guarda o actualiza el resumen de la sesión de memoria activa
  sin cerrarla. Úsala tras una compactación de la conversación, con el resumen
  de lo trabajado hasta ese punto.»
- **Entrada**: `{"summary": string}` (obligatorio, no vacío).
- **Efecto**: `UpdateSummary(sesiónActiva, summary)`. Sin sesión activa, abre
  una primero.
- **Salida**: `✓ Resumen de la sesión <id8> actualizado`.
- **Auto-aprobable**: sí. Se añade a `domain.MCPAutoApprovableTools` porque no
  es destructiva.
- **Huella**: la descripción no supera los 250 caracteres (spec 008).
