---
name: speckit-gomemory-context-update
description: Incorporar el resumen de historial de gomemory al contexto de la especificación
compatibility: Requires spec-kit project structure with .specify/ directory
metadata:
  author: github-spec-kit
  source: gomemory-context:commands/speckit.gomemory-context.update.md
---

# Actualizar contexto de gomemory para spec-kit

Obtiene el resumen de historial del proyecto que gomemory ya construye
(`mem context`) — memorias por tipo (decisiones, patrones, bugfixes) y,
cuando hay un proveedor externo de grafo de código conectado, un resumen
aparte y rotulado de esa estructura — y lo entrega como salida de este
comando para que quien redacta la especificación lo tenga presente.

## Comportamiento

El script lee `.memory/settings.json` del proyecto: si
`speckit_context_disabled` es `true`, termina sin salida (la integración
está apagada — ver `mem`/TUI → pantalla de configuración → "Brazo extensor
spec-kit"). Si el binario `mem` no está disponible (ni `./mem` en la raíz
del proyecto ni `mem` en `PATH`), o si `mem context` falla por cualquier
motivo (proyecto sin memoria inicializada, error interno), también termina
sin salida. En ningún caso interrumpe el flujo de `/speckit-specify`: el
código de salida siempre es `0`.

Cuando hay contexto disponible, la salida es el Markdown completo que
produce `mem context` — no se filtra ni resume más allá de lo que esa
misma herramienta ya acota (presupuesto de caracteres configurado en
gomemory).

## Execution

- **Bash**: `.specify/extensions/gomemory-context/scripts/bash/update-gomemory-context.sh`
- **PowerShell**: `.specify/extensions/gomemory-context/scripts/powershell/update-gomemory-context.ps1`

Ninguno de los dos scripts recibe argumentos.