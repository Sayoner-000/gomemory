---
name: strategic-compact
description: Aplica este flujo para resumir y compactar tras un hito.
---

## Capacidades
Prefiere las herramientas MCP de gomemory cuando estén disponibles. Como alternativa usa la CLI `mem`; sin ambas, registra los mismos datos en un documento local y realiza las comprobaciones manualmente.

## Procedimiento
1. Evalúa el contexto tras investigar, completar un hito o descartar un enfoque fallido. No compactes a mitad de una implementación ni mientras haya resultados que aún no se hayan registrado.
2. Guarda objetivo, decisiones, evidencia, cambios, validaciones y próximos pasos mediante `save_session_summary`; alternativa `mem session` para consultar el flujo soportado y `mem save` para conservar el resumen. Sin ambas, escribe una nota de continuidad.
3. Solo después de guardar el resumen usa el mecanismo nativo de la tabla. Si no conviene compactar, registra expresamente la decisión de no hacerlo. Salida: resumen recuperable antes de compactar, o decisión explícita de continuar.

## Por harness
| Acción | Claude Code | Codex | OpenCode |
|---|---|---|---|
| Medir contexto | /context | /status | No disponible: compactación automática |
| Compactar | /compact | /compact | No disponible: compactación automática |

Inspirada en strategic-compact de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.
