# Contrato de prioridad de recuperación — 038

## Superficies y versión

La directiva `domain.CodeRetrievalPolicy` DEBE estar presente sin divergencias semánticas en:

1. `buildIntegrationBlock`: protocolo global gestionado y `ServerOptions.Instructions` de MCP initialize.
2. `memoryProtocolReminder`: recordatorios de usuario/subagente y respuestas que lo entregan.

El marcador DEBE pasar de v8 a v9. `composeAgentFile` DEBE reemplazar únicamente el bloque gestionado, conservar contenido ajeno antes/después, baseline universal y marcador final; repetir instalación no duplica ni cambia el resultado. Los protocolos legados siguen siendo reconocidos. No crear AGENTS por proyecto.

## Regla canónica prevista

> Prioriza herramientas de grafo frente a lecturas manuales cuando cubran el proyecto y la consulta. Si el agente no admite compresión de resultados externos, como Codex en las versiones verificadas, prioriza recuperación comprimida en origen disponible y suficiente; usa search_code para búsquedas Go cubiertas y get_symbol para firma, ubicación y relaciones, sin asumir que entrega cuerpos o compresión. Si la cobertura o la evidencia no basta, usa un proveedor externo disponible para el proyecto y, si tampoco basta, una lectura focalizada, indicando la limitación. En agentes con compresión externa compatible conserva su ruta compatible, sin asumir que todas las herramientas o lecturas exactas se comprimen. Si la capacidad es desconocida, no prometas compresión automática. No infieras ahorro de una herramienta por su nombre; pack_build recupera memoria/Spec Kit y pack_compress no elimina el coste del texto ya recibido.

La implementación puede ajustar redacción sin cambiar esta precedencia. Ninguna otra frase gestionada puede volver a exigir proveedor externo incondicionalmente.

## Matriz verificable

| Condición | Resultado exigido |
|-----------|-------------------|
| Codex incompatible, consulta Go cubierta, search_code suficiente | preferir recuperación en origen; ahorro se mide, no se presupone |
| Se necesita firma/relación | get_symbol permitido como evidencia estructural |
| Se necesita cuerpo, otro lenguaje o índice ausente | alternativa externa disponible; lectura focalizada si es insuficiente |
| Compresión en origen deshabilitada/sin ganancia | no presentar resultado como comprimido |
| Rewrite compatible | conservar ruta compatible y respetar exclusiones existentes |
| Capacidad desconocida | alternativa que no dependa de rewrite desconocido; sin promesa automática |
| Instrucciones externas ajenas contradictorias | preservar bloques y advertir conflicto; no editar silenciosamente |

## Límites públicos

No cambian nombres/esquemas MCP, política de permisos, hooks compatibles, niveles de compresión o motor. No reinstalar `CodexToolOutputHook`. Mantener bootstrap de descubrimiento externo disponible para alternativas y análisis que el grafo propio no cubre.

## Pruebas obligatorias

Contrato de fragmento común, ausencia de prioridad contradictoria, v8→v9, protocolos legados, preservación de bloques CodeGraph/externos, instalación repetida, MCP initialize, recordatorios y perfiles compatible/desconocido. Pruebas de instalación aisladas no modifican el HOME real ni usan binarios ajenos del PATH.
