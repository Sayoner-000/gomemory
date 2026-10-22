---
name: search-first
description: Aplica este flujo para buscar antes de construir o añadir dependencias.
---

## Capacidades
Prefiere las herramientas MCP de gomemory cuando estén disponibles. Como alternativa usa la CLI `mem`; sin ambas, registra los mismos datos en un documento local y realiza las comprobaciones manualmente.

## Procedimiento
1. Busca decisiones y rechazos previos con `search_memories`; alternativa `mem search`. Sin memoria consulta ADR e historial.
2. Localiza lo existente con `search_code` y verifica ubicación y relaciones con `get_symbol`. Amplía al proveedor externo de grafo si está disponible. Sin grafo usa búsquedas dirigidas y lee las fuentes.
3. Revisa las skills instaladas por su nombre y descripción. Solo después consulta fuentes externas oficiales para dependencias o integraciones MCP.
4. Compara las opciones antes de elegir:

| Opción | Cuándo elegirla | Evidencia |
|---|---|---|
| Adopt | Una capacidad existente resuelve el requisito | Contrato y prueba compatible |
| Extend | Falta una parte acotada en una capacidad existente | Cambio mínimo identificado |
| Build | No existe una opción adecuada | Búsqueda y alternativas documentadas |

5. Guarda la decisión y sus motivos con `save_memory` de tipo decision, alternativa `mem save`, o un ADR manual. Salida: decisión Adopt/Extend/Build respaldada por evidencia.

Inspirada en search-first de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.
