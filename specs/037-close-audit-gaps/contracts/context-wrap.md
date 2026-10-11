# Contratos de contexto y ejecución

Requisitos FR-009–010 y FR-022–023.

## Contexto

Conservar firmas de `ContextBuilder.Build/WriteFile`, entradas CLI y herramientas MCP. No nuevos campos en los contratos públicos existentes de contexto o métricas.

Cada Build inicia contador local de descartes a cero, sin modificar el estado compartido. La configuración no se muta durante el servicio; un cambio futuro de configuración exige otro diseño de sincronización. Memorias, orden, fechas y proveedor de token equivalentes deben producir métricas idénticas. Recorder puede guardar varias observaciones legítimas; no se confundirá total agregado de uso con la cifra de una emisión.

Pruebas obligatorias:

- Tres emisiones consecutivas: mismo contenido y mismo par baseline/emitted.
- Ocho llamadas MCP solapadas sobre la misma instancia: resultados iguales a sus equivalentes aislados, sin carreras.
- Emisión con descartes seguida de otra sin descartes y su variante concurrente: cero contaminación de contador.
- Puertos reales usados por el composition root, incluido recorder, probados concurrentemente. Un mock que serializa solicitudes no demuestra esta condición.

## mem wrap

Sintaxis existente: `mem wrap [-s=false] <command> [args...]`.

| Resultado | Código | Salida y guardado |
| --- | --- | --- |
| Comando ausente o arranque denegado | 1 | WRAP_START_FAILED en stderr, diagnóstico depurado; sin diálogo |
| Hijo iniciado y terminado 0 | 0 | Preservar stdout/stderr y diálogo actual |
| Hijo iniciado y terminado N no cero | N | Preservar stdout/stderr y diálogo actual |

No imprimir argumentos completos potencialmente sensibles ni el error crudo del sistema; ofrecer causa útil de ruta/permisos. Auto-inicio de sesión conserva su contrato actual: esta feature no revierte una sesión auto-iniciada antes de que falle el arranque.

Las pruebas ejecutan un binario real con stdin cerrado y capturan código y stderr. Deben distinguir fallo de arranque de hijo iniciado con código 1. Si termina por señal, preservar la política ya utilizada para ExitError y comprobar que la conversión no produce éxito falso.
