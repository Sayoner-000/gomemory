---
name: iterative-retrieval
description: Aplica este flujo para recuperar contexto acotado antes de delegar.
---

## Capacidades
Prefiere las herramientas MCP de gomemory cuando estén disponibles. Como alternativa usa la CLI `mem`; sin ambas, registra los mismos datos en un documento local y realiza las comprobaciones manualmente.

## Procedimiento
1. Define una pregunta acotada, archivos afectados, dependencias y presupuesto. Construye un paquete con `pack_build`; alternativa `mem pack build --task "objetivo" --max-tokens 3000`, o extractos manuales con rutas y líneas.
2. Dispatch: recupera el contexto y consulta `search_code` y `get_symbol` para localizar evidencia actual. Usa `pack_retrieve` solo para recuperar originales omitidos; alternativa `mem pack retrieve`.
3. Evaluate: comprueba suficiencia, actualidad, contratos y relaciones necesarias. Refine: ajusta la pregunta y recupera solo lo que falta. Como máximo tres ciclos; detente antes si la evidencia permite ejecutar y validar sin suposiciones materiales.
4. Si Octopus está disponible, consulta `octopus_route_task` antes de delegar y respeta su ruta. Alternativa `mem octopus route "objetivo"`; sin router decide con las reglas del proyecto y documenta el motivo.
5. Entrega al subagente el paquete acotado, objetivo, alcance, restricciones, limitaciones y criterio de validación. Con INLINE realiza aquí el trabajo con ese mismo paquete. Salida: contexto suficiente entregado o usado, con incógnitas explícitas.

Inspirada en iterative-retrieval de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.
