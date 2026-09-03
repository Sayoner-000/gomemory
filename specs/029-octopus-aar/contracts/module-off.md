# Contrato: apagado del módulo (huella cero)

Este contrato existe porque "apagado" admite una lectura barata —el código no hace nada— y una honesta —el sistema es indistinguible de uno sin la funcionalidad—. Octopus se compromete con la segunda.

## Definición

Con `OctopusEnabled` ausente o `false`, para todo canal (TUI, línea de comandos, MCP) y toda operación:

| Superficie | Comportamiento exigido |
|---|---|
| `tools/list` del servidor MCP | No aparece ninguna tool de Octopus. La lista es **exactamente** `domain.MCPAllTools()` |
| Bloque de protocolo (`buildIntegrationBlock`) | No menciona Octopus. El texto es byte a byte el que se emite hoy |
| Bootstrap de ToolSearch | No materializa ningún nombre de Octopus |
| Listas de auto-aprobación | No contienen nombres de Octopus |
| `get_context()` / `mem context` | Ninguna sección, campo ni línea nueva |
| `get_plan_context()` | Ningún cambio |
| Tabla `octopus_executions` | Existe (migración aditiva) y permanece vacía: cero escrituras |
| `mem octopus <cualquier subcomando>` | Responde que el módulo está desactivado, indica cómo activarlo y termina con código de salida distinto de cero. No emite decisión, contrato, presupuesto ni telemetría |
| Pantalla de configuración de la TUI | Muestra la fila `Octopus AAR: off`. Es la **única** presencia visible de la funcionalidad con el módulo apagado, y es intencional: es el interruptor |

## Cómo se verifica

- Prueba de contrato que levanta el servidor MCP sobre transporte en memoria con el módulo apagado y compara `tools/list` contra `domain.MCPAllTools()`, sin tolerancia.
- Prueba que compara el bloque de protocolo generado con el módulo apagado contra el generado antes de esta funcionalidad.
- Prueba de integración que ejecuta un flujo completo con el módulo apagado y verifica que `SELECT COUNT(*) FROM octopus_executions` devuelve cero.
- Prueba de la TUI que verifica que la fila existe, muestra `off` por defecto y que alternarla persiste.

## Transición encendido → apagado a mitad de un plan

Las decisiones ya emitidas y el trabajo ya completado se conservan; no se borra nada. Lo que deja de ocurrir es la emisión de decisiones nuevas. Apagar el módulo no es una operación destructiva y no requiere confirmación.
