---
name: instincts
description: Aplica este flujo para revisar, confirmar o rechazar patrones aprendidos.
---

## Capacidades
Prefiere las herramientas MCP de gomemory cuando estén disponibles. Como alternativa usa la CLI `mem`; sin ambas, registra los mismos datos en un documento local y realiza las comprobaciones manualmente.

## Procedimiento
1. La CLI es la vía principal para listar propuestas: `mem expand instincts list`, o `mem expand instincts list --all` para incluir rechazadas. No existe una herramienta MCP equivalente para listar o rechazar instincts.
2. Comprueba que cada patrón representa una preferencia estable, tiene observaciones suficientes y no es una excepción local, información sensible ni un fallo transitorio.
3. Confirma con `mem expand instincts confirm <slug>`: persiste como preferencia. Si la CLI no está disponible, usa `save_memory` con type=preference y topic_key=instinct:<slug>, describiendo la regla estable.
4. Rechaza propuestas incorrectas con `mem expand instincts reject <slug>`. Sin CLI registra el rechazo y no lo presentes como una transición ya persistida. Sin MCP ni CLI mantén una lista manual de decisiones y aplícalas en la siguiente revisión disponible.
5. Salida: cada instinct revisado queda confirmado o rechazado; informa por separado las decisiones que aún no pudieron persistirse.

Inspirada en continuous-learning-v2 de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.
