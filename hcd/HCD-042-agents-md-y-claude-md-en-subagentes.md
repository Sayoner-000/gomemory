# AGENTS.md y CLAUDE.md en subagentes

ID: HCD-042
Tipo: evolution
Carril: fast
Motivo: decisión cerrada por la persona: registrar y no hacer; no hay trabajo de código

## Intención
Decidir si los subagentes deben dejar de heredar los archivos de instrucciones (AGENTS.md, CLAUDE.md) sin quitárselos al agente raíz (HCD-026, Q3).


Inventario (2026-11-06, máquina de la persona):
- `~/.claude/CLAUDE.md` 9.825 caracteres, `~/.config/opencode/AGENTS.md` 5.192 y `~/.codex/AGENTS.md` 951. Ninguno contiene el bloque de gomemory. El repositorio no tiene `AGENTS.md`, `CLAUDE.md` ni `GEMINI.md`.
- gomemory ya no escribe archivos de instrucciones (`mem doctor`: «protocolo y baseline llegan por MCP, hooks y contexto»).
- Lo que gomemory controla en el arranque de un subagente ya está cerrado en HCD-026: el aviso mínimo de solo lectura (~700 caracteres) en Claude, agy y OpenCode. Codex no dispara hooks en el hijo (memoria 1719).

## Alcance
Incluido: dejar constancia de que la herencia de los archivos de instrucciones de la persona en los subagentes la decide cada runtime, no gomemory.
Excluido: cambiar la configuración de los runtimes o los archivos de instrucciones de la persona; las sondas reales por runtime.

## ¿Qué pasa si no lo hacemos?
Nada que gomemory pueda corregir: los archivos son de la persona y su carga en subagentes depende de cada runtime. El coste de lo que sí inyecta gomemory ya se redujo en HCD-026.

## Alternativa más barata
Esta misma: registrar la decisión y no hacer nada. Investigar por runtime, con o sin sondas reales, cuesta cupo de 4 agentes y, en Codex, modifica `~/.codex/config.toml`, sin una palanca de gomemory a la vista. Se reabre si algún runtime ofrece una opción para excluir instrucciones en subagentes.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->

## Siguiente
Cancelada: Registrar y no hacer (Recommended)
