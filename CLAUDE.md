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
- Pregúntate: “¿Un ingeniero senior aprobaría esto?”
- Ejecuta pruebas, revisa logs, demuestra la corrección

### 5. Exige Elegancia (Balanceada)
- Para cambios no triviales: pausa y pregunta “¿hay una forma más elegante?”
- Si una solución se siente improvisada: “Sabiendo todo lo que sé ahora, implementa la solución elegante”
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

### Cierre de sesión (`end_session`)
Antes de terminar o decir "listo", llamar a `end_session(summary)` con Goal/Discoveries/Accomplished/Next Steps/Relevant Files. El hook `SessionEnd` cierra la sesión como red de seguridad si no se hace, pero sin resumen rico — ese resumen lo aporta el modelo llamando a `end_session` antes de cerrar.

## Principios Fundamentales

- *Simplicidad Primero*: Haz cada cambio lo más simple posible. Impacta el mínimo código.
- *Sin Pereza*: Encuentra la causa raíz. Nada de soluciones temporales. Estándares de desarrollador senior.
- *Impacto Mínimo*: Los cambios deben tocar solo lo necesario. Evita introducir errores.

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
at `specs/003-memory-maintenance/plan.md`.
<!-- SPECKIT END -->

<!-- gomemory-protocol-v2 -->

<!-- gomemory-workrules-v1 -->
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

<!-- gomemory-protocol-v3 -->
## Memoria Persistente (`mem`) — Protocolo Activo

Este proyecto tiene el servidor MCP `gomemory` conectado. Este protocolo es OBLIGATORIO
y SIEMPRE ACTIVO — no esperes a que el usuario lo pida explícitamente.

### Herramientas MCP disponibles
- `save_memory(title, type, content, filepath?)` — guarda una memoria
- `search_memories(query, limit?)` — busca en memorias del proyecto
- `list_memories(limit?)` — lista memorias recientes
- `get_memory(id)` — obtiene una memoria específica
- `forget_memory(id)` — borra una memoria puntual (irreversible)
- `judge_memories(id_a, id_b, verdict, confidence, reasoning)` — veredicto imparcial entre dos memorias en conflicto
- `start_session()` / `end_session(summary?)` — gestiona la sesión de trabajo
- `get_context()` — contexto completo del proyecto en markdown

Si el MCP no está disponible en el agente actual, usa el CLI equivalente:
`./mem save -t "título" -y tipo "contenido"`, `./mem search "tema"`, `./mem context`, `./mem session start|end`, `./mem forget <id>`, `./mem judge -r <veredicto> -m "razón" <id1> <id2>`.

### GUARDAR PROACTIVAMENTE — no esperes a que el usuario lo pida
Llama a `save_memory` (o `./mem save`) INMEDIATAMENTE después de:
- Una decisión técnica o de arquitectura
- Un bug corregido (incluye causa raíz)
- Un patrón o convención establecida
- Un descubrimiento no obvio sobre el código
- El usuario confirma o rechaza un enfoque propuesto

Autochequeo después de CADA tarea: "¿Tomé una decisión, corregí un bug, descubrí algo
o establecí una convención? Si sí → `save_memory` AHORA."

### Juez imparcial (memorias en conflicto)
Si el contexto muestra `## Conflictos sin resolver`, o notas dos memorias que se
contradicen al buscar, no asumas que la más reciente tiene razón: relee el código/archivo
fuente actual para verificar cuál refleja los hechos reales, y registra el veredicto con
`judge_memories` (o `./mem judge`), explicando en el razonamiento qué verificaste.

### Privacidad
Si vas a guardar un secreto, token o credencial, envuelve esa parte en
`<private>...</private>` — nunca se persiste.

### Al inicio de cada sesión:
1. Llama `get_context()` (o `./mem context`) para cargar el contexto histórico
2. Si no hay sesión activa, llama `start_session()` (o `./mem session start`)

### Al cerrar la sesión (antes de decir "listo"):
Llama `end_session(summary)` (o `./mem session end -s "..."`) con un resumen de lo realizado.

### Consultar memoria:
- `search_memories(query)` (o `./mem search "tema"`) cuando el usuario pregunte por trabajo previo
- `./mem` abre la TUI interactiva
