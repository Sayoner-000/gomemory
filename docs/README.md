# Documentación de gomemory

Bienvenido al centro de documentación técnica y operativa de **gomemory** (`v2.51.1`).

gomemory proporciona memoria persistente, local y portable para agentes de codificación de inteligencia artificial (Claude Code, OpenCode, Codex, Antigravity CLI / agy y harnesses personalizados), gestionada mediante una base de datos SQLite independiente por proyecto fuera del árbol de trabajo de git y servida a través del estándar [Model Context Protocol (MCP)](https://modelcontextprotocol.io/).

---

## 🗺️ Mapa de la Documentación

| Documento | Descripción |
|---|---|
| [**Manual de Usuario (MANUAL.md)**](MANUAL.md) | Guía completa de uso, comandos de CLI, configuración de agentes, flags y ejemplos prácticos. |
| [**Arquitectura General (ARQUITECTURA.md)**](ARQUITECTURA.md) | Resumen técnico de diseño en capas, adaptadores primarios/secundarios, SQLite, hostbind y ciclo de vida de sesiones. |
| [**Especificación Interna de Arquitectura (architecture.md)**](architecture.md) | Detalle profundo de estructuras internas, eventos de hooks, compresión estructural y árbol de dependencias. |
| [**Configuración y Variables de Entorno (CONFIGURATION.md)**](CONFIGURATION.md) | Variables de entorno (`GOMEMORY_*`), límites de almacenamiento, umbrales y timeouts. |
| [**Esquema de Base de Datos (DATABASE.md)**](DATABASE.md) | Migraciones de esquema SQLite (versiones 1 a 12), tablas (`memory_files`, `sessions`, `code_nodes`, etc.) e índices. |
| [**Integración de Agentes y Runtimes (AGENT-INTEGRATION.md)**](AGENT-INTEGRATION.md) | Contrato de integración para nuevos agentes (Niveles 1, 2 y 3) y soporte específico para Claude Code, OpenCode, agy y Codex. |
| [**Protocolo de Memoria (MEMORY-PROTOCOL.md)**](MEMORY-PROTOCOL.md) | Formatos JSON/MCP, bloques de integración, persistencia de decisiones, aprendizajes y checkpoints. |
| [**Superherramienta Socratica (socratica.md)**](socratica.md) | Flujo socrático para clarificación reflexiva y diseño antes de implementar. |
| [**Superherramienta Expand (expand.md)**](expand.md) | Auditoría de reglas operativas en tiempo real durante pre-tool invocations. |
| [**Lecciones Aprendidas (lessons.md)**](lessons.md) | Registro de aprendizajes técnicos y casos límite resueltos en el desarrollo del proyecto. |
| [**Temas de Consola y TUI (CONSOLE-THEMES.md)**](CONSOLE-THEMES.md) | Configuración visual y paletas para la interfaz de terminal interactiva. |
| [**TDD Visual en CLI (CLI-VISUAL-TDD.md)**](CLI-VISUAL-TDD.md) | Metodología de pruebas para salidas visuales en consola. |
| [**Notas y Guía de Release (release_gomemory_notes.md)**](release_gomemory_notes.md) | Proceso de empaquetado y publicación automatizada con GoReleaser y GitHub Actions. |
| [**Auditorías Técnicas (audits/)**](audits/) | Informes de auditorías ACR/ADM, veredictos de calidad y registros de cierre de hallazgos. |

---

## 📘 Guías Vigentes (HTML imprimibles e interactivas)

El proyecto mantiene tres guías ejecutivas visuales de diseño editorial en formato HTML (con soporte de impresión a PDF en 1040px y barra fija de herramientas):

1. **[Guía Rápida HCD (Guia-Rapida-HCD_GOMEMORY-v2.51.html)](Guia-Rapida-HCD_GOMEMORY-v2.51.html)**: Resumen operativo de la metodología HCD (Hipótesis, Comprobación y Demostración), carril rápido (`fast`) vs ceremonia completa (`full`), comandos esenciales y buenas prácticas.
2. **[Guía SDD_HCD (Guia-SDD_HCD_GOMEMORY-v2.51.html)](Guia-SDD_HCD_GOMEMORY-v2.51.html)**: Manual de convivencia entre Spec Kit y HCD, con ejemplos de descomposición atómica y trazabilidad.
3. **[Guía Técnica de Arquitectura (Guia-Tecnica_GOMEMORY-v2.51.html)](Guia-Tecnica_GOMEMORY-v2.51.html)**: Arquitectura completa de gomemory en 14 capítulos, cubriendo desde el almacenamiento SQLite hasta la compresión con preservación sintáctica.

---

## 🐟 Subagente `splash`: Revisor sin Contexto (Modelo Elephant-Goldfish)

Una de las innovaciones centrales de gomemory (incorporada en `HCD-047` y consolidada en `HCD-051`) es el subagente **`splash`**.

### ¿Qué es y por qué existe?
El agente principal que trabaja en un proyecto opera bajo el modelo del **elefante**: acumula contexto, historial de decisiones, convenciones y memoria continua. Sin embargo, al momento de revisar una propuesta, un diseño o una definición de requerimientos, este exceso de contexto produce un **sesgo de confirmación**: el agente asume como evidentes cosas que nunca se especificaron o ignora incoherencias porque "sabe" lo que se intentaba hacer.

Para resolver esto, gomemory implementa el modelo **Elephant-Goldfish (EGM)** con el subagente `splash` (la gota que salpica sin arrastrar memoria):
- **Contexto Cero:** Arranca sin memoria previa del proyecto, sin volcado de memorias ni avisos de contexto.
- **Sin Herramientas MCP de Memoria:** No tiene acceso a `save_memory`, `search_memories`, ni a instrucciones globales.
- **Solo Código Vivo y Documento:** Únicamente dispone de lectura de archivos locales (`read`, `grep`, `glob`) para auditar la realidad física del repositorio frente al texto planteado.

### Despliegue en los 4 Runtimes Soportados

`mem setup` instala automáticamente la plantilla de `splash` en todos los agentes que se encuentren configurados en el entorno:

| Runtime | Archivo de Configuración | Modo de Lanzamiento | Mecanismo de Aislamiento en gomemory |
|---|---|---|---|
| **Claude Code** | `~/.claude/agents/splash.md` | Subagente tipo `splash` (`/splash`) | `SubagentStart` detecta `agent_type: "splash"` y omite la inyección de contexto. Configurado con `omitClaudeMd: true` y tools de solo lectura (`Read`, `Grep`, `Glob`). |
| **OpenCode** | `~/.config/opencode/agents/splash.md` | Subagente `splash` (tool `task`) | El plugin de OpenCode intercepta la creación de la sesión hija y no le inyecta el aviso de subagente ni el protocolo. |
| **Antigravity (agy)** | `~/.gemini/config/agents/splash/agent.md` | `invoke_subagent` con `TypeName: "splash"` | El hook `classifyAgySubagent` marca el subagente y `SessionStart` suprime el bloque de memoria; `inheritMcp: false` lo aísla del MCP. |
| **Codex** | `~/.codex/agents/splash.toml` | `spawn_agent(agent_type: "splash", fork_turns: "none")` | Instrucciones estrictas de no invocar herramientas MCP; `sandbox_mode = "read-only"`. Si el padre omite `fork_turns: "none"`, el hook pre-tool avisa de la herencia indeseada. |

### Integración en la Ceremonia HCD (`/hcd-approve`)

En la ceremonia completa de HCD, el paso `/hcd-approve` ofrece invocar al revisor `splash` sobre el documento de la unidad (`hcd/HCD-NNN-*.md`) antes de pedir la aprobación formal:
1. **Lentes de Inspección:**
   - **Rutas inexistentes:** Rutas de archivo citadas en el documento que no existen en el árbol de trabajo.
   - **Criterios no comprobables:** Condiciones de aceptación ambiguas o sin método de prueba reproducible.
   - **Supuestos sin evidencia:** Declaraciones sobre el comportamiento del sistema no respaldadas por pruebas.
   - **Contradicciones internas:** Incoherencias entre alcance, intención y tareas.
2. **Severidad de Hallazgos:**
   - **Blocker:** Detiene la solicitud de aprobación hasta que el error sea corregido o descartado explícitamente por el usuario.
   - **Major / Nit:** Informativos. Se propone la corrección y el usuario decide si aplicarla.
3. **Economía del Cambio:** Corregir la definición en la fase de aprobación evita re-aprobaciones costosas y repetición de baterías de pruebas durante la verificación.

---

## 🛠️ Comandos Esenciales de gomemory

```bash
# Diagnóstico integral del sistema y agentes
mem doctor

# Inspección y estado del ciclo HCD
mem hcd
mem hcd status
mem hcd analyze

# Historial de decisiones y unidades HCD que citan un archivo
mem hcd history [texto] [--file ruta/al/archivo]

# Gestión y exportación de documentos fijados
mem docs list
mem docs show rules
mem docs show constitution

# Instalación y configuración de agentes y subagentes
mem setup
mem update
```
