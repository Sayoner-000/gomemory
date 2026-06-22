## Reglas de trabajo (lecciones de campo — LEER PRIMERO)

> Estas reglas tienen prioridad sobre la ceremonia. Nacieron de un caso real donde el proceso (SDD/constitución) dio falsa sensación de avance mientras los bugs reales seguían vivos.

1. **El proceso se ajusta a la tarea.** Bug o paridad con un sistema que YA corre → reproducir contra el sistema EN EJECUCIÓN primero (logs / `curl` / navegador), arreglar y verificar ahí mismo. NO correr SDD (specify→plan→tasks→implement) para un bug. El SDD es solo para features nuevas no triviales.
2. **"Verde en tests" NO es "funciona".** Un test vale lo que vale su fixture: si el mock no refleja la respuesta real del upstream, miente con cara de éxito. Antes de decir "listo": verificar contra el contenedor en ejecución (`docker exec` / `curl` / logs), no solo unit tests.
3. **"No se ve el cambio" → primero el despliegue, no el código.** Verificar el artefacto realmente servido (bundle/binario DENTRO del contenedor), la URL y la caché del navegador ANTES de tocar código. `docker compose up` reusa la imagen vieja; usar `docker compose up --build`. El `index.html` se sirve con `no-cache`; los assets van hasheados.
4. **La constitución es referencia de CÓMO escribir código** (capas, estilo), no un mandato de ritual por tarea. No aplicar un requisito del spec que rompa el flujo real del usuario sin contrastarlo antes.

---

## Orquestación del Flujo de Trabajo

### 1. Modo Plan por Defecto
- Entra en modo plan para CUALQUIER tarea no trivial (3+ pasos o decisiones arquitectónicas)
- Si algo se desvía, DETENTE y replantea inmediatamente — no sigas avanzando a la fuerza
- Usa el modo plan también para pasos de verificación, no solo para construir
- Escribe especificaciones detalladas desde el inicio para reducir ambigüedad

### 2. Estrategia de Subagentes
- Usa subagentes libremente para mantener limpio el contexto principal
- Delega investigación, exploración y análisis paralelo a subagentes
- Para problemas complejos, utiliza más cómputo mediante subagentes
- Un enfoque por subagente para una ejecución enfocada

### 3. Bucle de Mejora Continua
- Después de CUALQUIER corrección del usuario: actualiza tasks/lessons.md con el patrón
- Escribe reglas para ti mismo que prevengan el mismo error
- Itera agresivamente sobre estas lecciones hasta reducir la tasa de errores
- Revisa las lecciones al inicio de la sesión para el proyecto relevante

### 4. Verificación Antes de Dar por Terminado
- Nunca marques una tarea como completa sin demostrar que funciona
- Compara (diff) el comportamiento entre el estado original y tus cambios cuando aplique
- Pregúntate: "¿Un ingeniero senior aprobaría esto?"
- Ejecuta pruebas, revisa logs, demuestra la corrección

### 5. Exige Elegancia (Balanceada)
- Para cambios no triviales: pausa y pregunta "¿hay una forma más elegante?"
- Si una solución se siente improvisada: "Sabiendo todo lo que sé ahora, implementa la solución elegante"
- Omite esto para arreglos simples y evidentes — no sobre-ingenierizar
- Cuestiona tu propio trabajo antes de presentarlo

### 6. Corrección Autónoma de Bugs
- Cuando recibas un reporte de bug: arréglalo. No pidas guía paso a paso
- Señala logs, errores, pruebas fallidas — luego resuélvelos
- Cero necesidad de cambiar el contexto del usuario
- Arregla fallos en CI sin que te indiquen cómo hacerlo

## Gestión de Tareas

1. *Planifica Primero*: Escribe el plan en tasks/todo.md con ítems verificables
2. *Verifica el Plan*: Confirma antes de comenzar la implementación
3. *Haz Seguimiento*: Marca los ítems como completados a medida que avanzas
4. *Explica los Cambios*: Resume a alto nivel en cada paso
5. *Documenta Resultados*: Añade una sección de revisión en tasks/todo.md
6. *Captura Lecciones*: Actualiza tasks/lessons.md después de correcciones

## Memory Protocol — Memoria Persistente (gomemory)

Hay acceso a gomemory vía MCP. Tools disponibles: `save_memory`, `search_memories`, `list_memories`, `get_memory`, `start_session`, `end_session`, `get_context`. Este protocolo es OBLIGATORIO y SIEMPRE ACTIVO — no es algo que se active a pedido.

### Cuándo guardar (`save_memory`) — inmediatamente después de:
- Decisión de arquitectura o diseño
- Bug corregido (incluir la causa raíz)
- Convención o patrón establecido
- Elección de librería/herramienta con sus tradeoffs
- Hallazgo no obvio sobre el código

Autochequeo después de CADA tarea: "¿se tomó una decisión, se arregló un bug, se descubrió algo o se estableció un patrón? Si sí → `save_memory` ahora, sin esperar a que se pida."

### Cuándo buscar (`search_memories`)
- Reactivo: el usuario dice "recuerdas", "qué hicimos antes", o referencia trabajo previo
- Proactivo: al empezar algo que podría solaparse con sesiones anteriores

Revelación progresiva: `search_memories(query)` para resultados compactos → `get_memory(id)` solo si se necesita el contenido completo. Nunca volcar toda la memoria de una.

### Sesión de trabajo (`start_session` / `end_session`)
Con Claude Code u OpenCode la sesión se crea y cierra sola (hooks del plugin); en cualquier otro agente (Cursor, Windsurf, Cline, Codex) no hay hook — hay que llamar a `start_session()` al empezar a trabajar y a `end_session(summary)` antes de terminar, con Goal/Discoveries/Accomplished/Next Steps/Relevant Files. Sin esto, la sesión nunca se cierra del lado de esos agentes.

## Arquitectura del Proyecto

Ver `docs/architecture.md` para el detalle completo de la arquitectura del proyecto.

Resumen rápido para orientarse:
- **Stack**: Go puro, sin CGO. CLI + TUI (Bubbletea) + servidor MCP (`modelcontextprotocol/go-sdk`) + servidor HTTP, todo sobre el mismo binario (`mem`). Persistencia en SQLite embebido (`modernc.org/sqlite`).
- **Arquitectura hexagonal en 4 capas**: `domain/` (tipos y validación, cero dependencias del proyecto) → `application/` (casos de uso + puertos/interfaces en `application/ports/`) → `adapters/` (`primary/` = CLI, TUI, MCP, setup de plugins; `secondary/` = persistencia SQLite) → `infrastructure/` (composition root: `main.go`, `container.go`, plugins embebidos vía `go:embed`).
- **Regla de dependencias**: nada depende directamente de `adapters/primary` o `adapters/secondary` — todo pasa por interfaces en `application/ports/`.
- **Integración multi-agente**: `mem setup <agente>` instala plugin completo (hooks + inyección automática de protocolo + sesión autónoma) para Claude Code y OpenCode; `mem setup-mcp --agents <lista>` solo registra el servidor MCP (sin automatización de sesión/protocolo) para Cursor, Windsurf, Cline y Codex.

### Principios

- *Simplicidad Primero*: Haz cada cambio lo más simple posible. Impacta el mínimo código.
- *Sin Pereza*: Encuentra la causa raíz. Nada de soluciones temporales.
- *Impacto Mínimo*: Los cambios deben tocar solo lo necesario.
