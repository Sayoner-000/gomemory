# Documentación de gomemory

Índice y referencias técnicas de gomemory (versión 2.53.0).

gomemory mantiene memoria persistente para agentes de desarrollo (Claude Code, OpenCode, Codex y Antigravity CLI). Cada proyecto almacena sus datos en una base SQLite local fuera del árbol de Git y se comunica con el modelo mediante el protocolo MCP (Model Context Protocol).

## Índice de documentos

| Archivo | Contenido |
|---|---|
| [MANUAL.md](MANUAL.md) | Manual de usuario: comandos, opciones de terminal, configuración de agentes y ejemplos de uso. |
| [ARQUITECTURA.md](ARQUITECTURA.md) | Resumen de diseño: separación por capas, SQLite, resolución de identidad de procesos con hostbind y ciclo de vida. |
| [architecture.md](architecture.md) | Especificación técnica interna: ciclo de eventos de hooks, compresión de contexto y estructura del grafo. |
| [CONFIGURATION.md](CONFIGURATION.md) | Variables de entorno del sistema (`GOMEMORY_*`), límites de almacenamiento y tiempos de espera. |
| [DATABASE.md](DATABASE.md) | Estructura de tablas e índices en SQLite (migraciones del esquema 1 al 12). |
| [AGENT-INTEGRATION.md](AGENT-INTEGRATION.md) | Guía de integración para arneses y agentes nuevos (niveles 1, 2 y 3). |
| [MEMORY-PROTOCOL.md](MEMORY-PROTOCOL.md) | Formato de mensajes MCP, bloques de contexto y guardado de decisiones o aprendizajes. |
| [socratica.md](socratica.md) | Herramienta de clarificación socrática para refinar tareas antes de codificar. |
| [expand.md](expand.md) | Verificación de reglas operativas en pre-tool para evitar ediciones directas de código con scripts. |
| [lessons.md](lessons.md) | Registro de incidentes técnicos y decisiones tomadas ante problemas de concurrencia o límites de entorno. |
| [CONSOLE-THEMES.md](CONSOLE-THEMES.md) | Configuración de color y temas visuales para la interfaz de consola. |
| [CLI-VISUAL-TDD.md](CLI-VISUAL-TDD.md) | Pruebas de contrato visual para interfaces de terminal. |
| [release_gomemory_notes.md](release_gomemory_notes.md) | Procedimiento de publicación y empaquetado multiplataforma con GoReleaser. |
| [audits/](audits/) | Informes de auditorías ACR y ADM con seguimiento de cierres verificados. |

## Guías de referencia

Documentos en formato HTML preparados para lectura en pantalla o impresión directa a PDF:

- [Guía rápida HCD (Guia-Rapida-HCD_GOMEMORY-v2.53.html)](Guia-Rapida-HCD_GOMEMORY-v2.53.html): resumen del flujo de trabajo con unidades HCD, diferencias entre la vía rápida y la ceremonia completa, y comandos de verificación.
- [Guía SDD y HCD (Guia-SDD_HCD_GOMEMORY-v2.53.html)](Guia-SDD_HCD_GOMEMORY-v2.53.html): coordinación entre especificaciones formales con Spec Kit y el ciclo de comprobación de HCD.
- [Guía técnica (Guia-Tecnica_GOMEMORY-v2.53.html)](Guia-Tecnica_GOMEMORY-v2.53.html): desglose de arquitectura del proyecto en 14 secciones, desde persistencia hasta compresión de respuestas.
- [Guía de explicación para desarrolladores (Guia-Explicacion-Developers-GOMEMORY.html)](Guia-Explicacion-Developers-GOMEMORY.html): guía visual e introductoria para equipos de ingeniería sobre el funcionamiento, arquitectura y adopción de gomemory.
- [Manual de referencia para desarrolladores (Manual-Referencia-Developers-GOMEMORY.html)](Manual-Referencia-Developers-GOMEMORY.html): referencia detallada de protocolos, flujos de hooks/MCP, glosario y Anexo A1 con la comparativa de los tres grafos de código (`gomemory`, `CodeGraph` y `codebase-memory-mcp`) y el guardarraíl `graph-before-grep`.

## Idioma del proyecto (`es` / `en`)

Desde la v2.52.0 (HCD-056), gomemory incluye plantillas y avisos en español (`es`, por defecto) e inglés (`en`), empaquetados en `infrastructure/templates/locales/<código>/`.

Durante la instalación, `mem install` pregunta qué idioma usar o recibe `--lang es|en` en modo no interactivo. Para cambiarlo más adelante basta con ejecutar `mem lang en` o `mem lang es` (o `mem lang` sin argumentos para elegirlo en una lista). El ajuste se guarda en `.memory/settings.json` (`language`) y surte efecto en la siguiente llamada a `get_context`, `get_plan_context`, `mem://context`, `mem context` y los hooks de ciclo de vida, sin reiniciar el servidor MCP ni la sesión del agente.

## Subagente splash: revisor sin contexto

El subagente `splash` resuelve un problema recurrente al revisar código o especificaciones: el sesgo de confirmación.

Cuando un agente trabaja durante varios turnos en un proyecto, acumula contexto de las decisiones y discusiones previas (el modelo del elefante). Si ese mismo agente revisa una definición técnica, tiende a dar por sentadas premisas que nunca se escribieron en el documento o pasa por alto rutas que no existen porque recuerda la intención original.

gomemory implementa para este caso el modelo de revisor de contexto cero (el modelo del pez dorado o goldfish, denominado `splash`):
- Inicia la sesión sin memorias históricas del proyecto ni bloques de contexto previo.
- No dispone de herramientas MCP para escribir o buscar recuerdos.
- Solo utiliza herramientas de lectura del sistema de archivos (`read`, `grep`, `glob`) para contrastar el documento con el estado real del repositorio.

### Soporte por agente

El comando `mem setup` configura la plantilla de `splash` en los entornos presentes en la máquina:

| Entorno | Archivo instalado | Invocación | Aislamiento en gomemory |
|---|---|---|---|
| Claude Code | `~/.claude/agents/splash.md` | Subagente `splash` (`/splash`) | `SubagentStart` detecta `agent_type: "splash"` y no inyecta recuerdos. La plantilla usa `omitClaudeMd: true` y solo permite lectura de archivos. |
| OpenCode | `~/.config/opencode/agents/splash.md` | Subagente `splash` mediante la herramienta `task` | El plugin de OpenCode intercepta la sesión hija y no envía el aviso de subagente ni el bloque de protocolo. |
| Antigravity (agy) | `~/.gemini/config/agents/splash/agent.md` | `invoke_subagent` con `TypeName: "splash"` | El hook `classifyAgySubagent` identifica el nombre en pre-tool; `SessionStart` suprime el contexto y la configuración fija `inheritMcp: false`. |
| Codex | `~/.codex/agents/splash.toml` | `spawn_agent(agent_type: "splash", fork_turns: "none")` | La plantilla opera en `sandbox_mode = "read-only"`. Las instrucciones prohíben llamar herramientas MCP. Si el padre no pasa `fork_turns: "none"`, el hook pre-tool emite un aviso. |

### Revisión en la ceremonia de aprobación

En la ceremonia completa de HCD, el paso `/hcd-approve` propone ejecutar `splash` sobre el archivo de la unidad antes de pedir la aprobación formal.

La revisión analiza cuatro puntos: rutas que no existen en el disco, criterios de aceptación que no se pueden medir, afirmaciones sin respaldo técnico y contradicciones entre apartados del documento.

Los problemas detectados se clasifican en dos niveles:
1. Hallazgos bloqueantes (Blocker): impiden pedir la aprobación hasta que la persona decida corregir el documento o descartar el aviso de forma explícita.
2. Hallazgos informativos (Major y Nit): sugerencias de redacción o advertencias menores que no detienen el flujo.

Detectar estos errores antes de la aprobación evita tener que reescribir la definición después de implementar, lo que obligaría a reiniciar las comprobaciones de todas las tareas.

## Comandos frecuentes

```bash
# Diagnóstico de integración y estado de agentes
mem doctor

# Estado de la unidad de trabajo activa
mem hcd
mem hcd status
mem hcd analyze

# Historial de decisiones asociadas a un archivo
mem hcd history [texto] [--file ruta/al/archivo]

# Gestión de documentos fijados del proyecto
mem docs list
mem docs show rules
mem docs show constitution

# Cambio de idioma del proyecto en caliente (es / en)
mem lang
mem lang en
mem lang es

# Configuración y actualización de plantillas de agente
mem setup
mem update
```
