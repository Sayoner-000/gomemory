# Feature Specification: Plugin de Memoria con Contexto Automático

**Feature Branch**: `001-plugin-memory-context`

**Creado**: 2026-06-21

**Estado**: Draft

**Entrada**: El usuario solicita transformar gomemory de un simple servidor MCP a un
sistema de plugins que se invoque en cada inferencia del agente, proporcionando
contexto automático de lo que se ha hecho en el proyecto y evitando el consumo
excesivo de tokens con modelos frontera. Inspirado en el proyecto Engram
(Gentleman-Programming/engram).

## Escenarios de Usuario *(obligatorio)*

### Historia de Usuario 1 — Plugin para OpenCode (Prioridad: P1)

Como desarrollador que usa gomemory con OpenCode, quiero que el contexto de
memoria se inyecte automáticamente en cada inferencia sin tener que llamar
herramientas MCP explícitamente, para que el agente siempre sepa lo que se ha
decidido y aprendido en el proyecto sin gastar tokens en comandos manuales.

**Por qué esta prioridad**: OpenCode es el agente principal del ecosistema. Sin
este plugin, el agente solo accede a memoria cuando el usuario recuerda pedírselo,
lo que anula el valor de la persistencia de contexto. Es la puerta de entrada a
todo el sistema de plugins.

**Prueba Independiente**: Se instala el plugin en un proyecto con OpenCode,
se guardan 3 memorias de arquitectura, se abre una nueva sesión, y el agente
demuestra conocimiento de esas decisiones sin que el usuario las mencione.

**Escenarios de Aceptación**:

1. **Dado** un proyecto con gomemory instalado, **Cuando** se ejecuta
   `mem setup opencode` y se reinicia OpenCode, **Entonces** el plugin se
   registra y el servidor MCP de gomemory se inicia automáticamente al abrir
   OpenCode.
2. **Dado** un agente OpenCode con el plugin activo, **Cuando** el usuario
   inicia una conversación, **Entonces** el plugin inyecta el contexto de
   sesiones previas (memorias recientes, sesión activa) en el system prompt
   del agente sin intervención del usuario.
3. **Dado** un agente que acaba de completar un bugfix, **Cuando** finaliza
   la interacción, **Entonces** el protocolo de memoria (Memory Protocol)
   le indica al agente que debe guardar el aprendizaje mediante `save_memory`
   antes de cerrar la sesión.
4. **Dado** que ocurre una compactación de contexto en OpenCode, **Cuando**
   el agente se recupera, **Entonces** el plugin inyecta instrucciones para
   que el agente llame `get_context()` y recupere el estado previo antes de
   continuar trabajando.
5. **Dado** un agente con el plugin activo, **Cuando** el agente busca
   información relevante, **Entonces** usa el patrón de revelación progresiva
   (primero busca resúmenes, luego profundiza si es necesario) en lugar de
   volcar toda la memoria, minimizando el consumo de tokens.

---

### Historia de Usuario 2 — Plugin para Claude Code (Prioridad: P2)

Como desarrollador que usa Claude Code, quiero tener el mismo nivel de
integración automática de memoria que en OpenCode, mediante hooks nativos
de Claude Code que manejen el ciclo de vida de sesiones y la recuperación
post-compactación.

**Por qué esta prioridad**: Claude Code es el segundo agente más usado del
ecosistema. Sin soporte nativo, los usuarios de Claude Code quedan excluidos
de la automatización de contexto y deben usar comandos manuales.

**Prueba Independiente**: Se instala el plugin via `mem setup claude-code`,
se inicia una sesión, se guardan memorias, se fuerza una compactación, y
el agente recupera el contexto automáticamente sin pérdida de información.

**Escenarios de Aceptación**:

1. **Dado** un proyecto con gomemory instalado, **Cuando** se ejecuta
   `mem setup claude-code`, **Entonces** se configuran los hooks de Claude
   Code (startup, compact, user-prompt-submit, shutdown) y el skill de
   Memory Protocol.
2. **Dado** que se inicia Claude Code, **Cuando** el hook `startup` se
   ejecuta, **Entonces** se crea una sesión automáticamente, se importan
   memorias sincronizadas por git (si existen), y se inyecta el contexto
   de sesiones previas en la conversación inicial.
3. **Dado** que ocurre una compactación de contexto, **Cuando** el hook
   `compact` se ejecuta, **Entonces** se inyecta el resumen de la sesión
   previa y se instruye al agente a persistirlo mediante
   `end_session(summary)` antes de continuar.
4. **Dado** que el usuario finaliza Claude Code, **Cuando** el hook
   `shutdown` se ejecuta, **Entonces** se cierra la sesión activa con
   un resumen de lo realizado.

---

### Historia de Usuario 3 — Protocolo de Memoria con Consumo Eficiente (Prioridad: P3)

Como agente AI, quiero tener reglas claras sobre cuándo guardar, buscar y
cerrar memoria, para no desperdiciar tokens en operaciones innecesarias y
solo persistir información de valor.

**Por qué esta prioridad**: Sin un protocolo explícito, los agentes tienden
a guardar todo (inflando la base de memoria) o no guardar nada (perdiendo
contexto). El protocolo balancea ambos extremos y optimiza el uso de tokens.

**Prueba Independiente**: Se inyecta el protocolo en el system prompt de
cualquier agente MCP. El agente debe demostrar que guarda solo después de
eventos significativos (bugfix, decisión, descubrimiento) y busca solo
cuando es relevante.

**Escenarios de Aceptación**:

1. **Dado** un agente con el Memory Protocol inyectado, **Cuando** completa
   una corrección de bug, **Entonces** guarda automáticamente la memoria
   con tipo "bugfix", causa raíz y solución.
2. **Dado** un agente con el Memory Protocol inyectado, **Cuando** el usuario
   pregunta "¿recuerdas cómo solucionamos X?", **Entonces** el agente busca
   en la memoria (búsqueda reactiva) antes de responder.
3. **Dado** un agente trabajando en una tarea que se parece a trabajo previo,
   **Cuando** inicia la tarea, **Entonces** busca proactivamente en la memoria
   por palabras clave relevantes (búsqueda proactiva) para no repetir errores.
4. **Dado** un agente al final de una sesión, **Cuando** se prepara para cerrar,
   **Entonces** ejecuta el protocolo de cierre: resumen de metas, descubrimientos,
   logros y siguientes pasos, y lo persiste via end_session.
5. **Dado** un agente que sufre compactación de contexto, **Cuando** se
   recupera, **Entonces** su primera acción es llamar `get_context()` para
   restaurar el estado previo antes de continuar.
6. **Dado** un agente que necesita contexto histórico, **Cuando** busca en la
   memoria, **Entonces** usa revelación progresiva de 3 capas: busca resúmenes
   compactos (~100 tokens), luego linea de tiempo si necesita contexto
   cronológico, y solo obtiene contenido completo cuando es necesario.

---

### Casos Borde

- ¿Qué pasa cuando el servidor HTTP de gomemory no puede iniciar (puerto
  ocupado, permiso denegado)?
- ¿Cómo maneja el plugin una sesión que ya está activa cuando se intenta
  crear otra?
- ¿Qué ocurre si el archivo de configuración del agente (`.opencode.json`,
  `.mcp.json`) ya tiene configuraciones MCP existentes?
- ¿Cómo se comporta el plugin cuando el agente no soporta ciertos hooks
  (ej. compact hook en agentes que no compactan)?
- ¿Qué pasa cuando hay múltiples proyectos con gomemory y el agente cambia
  entre ellos?
- ¿Cómo se maneja la inyección de contexto cuando el system prompt del
  agente ya está cerca del límite de tokens?

## Requerimientos *(obligatorio)*

### Requerimientos Funcionales

- **RF-001**: El sistema DEBE proporcionar un comando `mem setup opencode`
  que instale el plugin para OpenCode, configurando el servidor MCP y el
  plugin TypeScript con auto-inicio del servidor HTTP de background.
- **RF-002**: El sistema DEBE proporcionar un comando `mem setup claude-code`
  que instale el plugin para Claude Code, configurando hooks (startup,
  compact, user-prompt-submit, shutdown) y el skill de Memory Protocol.
- **RF-003**: El plugin DEBE inyectar el Memory Protocol en el system prompt
  del agente en cada inferencia, concatenándolo al mensaje de sistema
  existente (no como mensaje separado), para compatibilidad con modelos
  que solo aceptan un bloque de sistema.
- **RF-004**: El sistema DEBE iniciar automáticamente un servidor HTTP de
  background (gomemory serve) cuando el plugin se activa, para manejar
  sesiones y captura pasiva de contexto.
- **RF-005**: El plugin DEBE crear una sesión automáticamente al inicio
  de cada conversación del agente, con detección de sesión activa previa
  para evitar duplicados.
- **RF-006**: El plugin DEBE inyectar el contexto de sesiones previas
  (resumen de últimas sesiones, memorias relevantes) en la conversación
  inicial del agente.
- **RF-007**: El sistema DEBE soportar el patrón de revelación progresiva
  de 3 capas: búsqueda de resúmenes (~100 tokens), línea de tiempo
  cronológica, y contenido completo.
- **RF-008**: El Memory Protocol DEBE incluir reglas explícitas para:
  cuándo guardar (solo eventos significativos), cuándo buscar (reactivo
  + proactivo), protocolo de cierre de sesión, y recuperación post-compactación.
- **RF-009**: El plugin DEBE inyectar instrucciones de recuperación
  post-compactación que indiquen al agente llamar `get_context()` como
  primera acción después de la compactación.
- **RF-010**: El sistema DEBE soportar sincronización de memorias vía git
  (archivos comprimidos en `.memory/sync/`) para compartir contexto entre
  máquinas.
- **RF-011**: El plugin DEBE importar automáticamente memorias sincronizadas
  por git al iniciar una sesión.
- **RF-012**: El Memory Protocol DEBE incluir una regla de "sesión cercana"
  obligatoria: antes de terminar, el agente DEBE llamar `end_session` con
  un resumen estructurado (Goal/Discoveries/Accomplished/Next Steps/Files).
- **RF-013**: El sistema DEBE soportar la desactivación por agente: el
  usuario debe poder elegir qué agentes tener plugin activo (opencode,
  claude-code, o ambos).
- **RF-014**: El sistema DEBE manejar graceful degradation: si el servidor
  HTTP de background no puede iniciar, el plugin debe continuar con
  funcionalidad reducida (solo MCP stdio, sin captura pasiva).

### Entidades Clave

- **Plugin**: Módulo de integración para un agente específico (OpenCode,
  Claude Code). Contiene configuración MCP, hooks, scripts y el protocolo
  de memoria.
- **Memory Protocol**: Conjunto de instrucciones inyectadas en el system
  prompt del agente que definen cuándo y cómo interactuar con la memoria.
- **Sesión de Agente**: Ciclo de vida de una conversación del agente con
  el usuario. Tiene inicio, fin, resumen y memorias asociadas.
- **Revelación Progresiva**: Estrategia de 3 capas para recuperar contexto
  de memoria minimizando tokens: resumen → línea de tiempo → contenido
  completo.
- **Hook de Agente**: Punto de extensión en el ciclo de vida del agente
  (startup, compact, submit, shutdown) que el plugin utiliza para
  inyectar comportamiento.

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **CE-001**: Un agente con plugin instalado inicia una sesión y demuestra
  conocimiento de memorias previas sin que el usuario las mencione, en
  menos de 3 interacciones.
- **CE-002**: La inyección de contexto automática agrega menos de 200 tokens
  al system prompt del agente (resumen de sesiones previas comprimido).
- **CE-003**: Un agente que sufre compactación de contexto se recupera
  completamente (recupera conocimiento de sesión previa) en menos de 2
  interacciones después de la compactación.
- **CE-004**: El protocolo de revelación progresiva reduce el consumo de
  tokens de recuperación de contexto en al menos 60% comparado con volcar
  toda la memoria.
- **CE-005**: El 90% de los eventos significativos (bugfix, decisión,
  descubrimiento) se persisten automáticamente en memoria sin intervención
  del usuario.
- **CE-006**: La instalación del plugin para cualquier agente toma menos
  de 5 segundos y no requiere configuración manual adicional.
- **CE-007**: El servidor HTTP de background consume menos de 10MB de RAM
  adicional y menos de 0.5% de CPU en estado idle.

## Suposiciones

- Los agentes destino (OpenCode, Claude Code) soportan el mecanismo de
  plugins/hooks descrito. Si un agente no soporta un hook específico,
  ese feature se desactiva sin afectar el resto.
- El servidor HTTP de background corre en `127.0.0.1:9735` (default) y
  solo es accesible localmente.
- El usuario tiene permisos para escribir archivos de configuración del
  agente (`.opencode.json`, `.mcp.json`, `.claude/settings.json`).
- No se requiere escalabilidad horizontal ni balanceo de carga — el
  sistema es monousuario por proyecto.
- Los modelos frontera (Claude Opus, GPT-4, Gemini Ultra) son los
  principales beneficiarios de la reducción de tokens; modelos pequeños
  también se benefician pero el ahorro es menor.
- La sincronización git de memorias es unidireccional (export → commit →
  pull → import) y no requiere resolución de conflictos.
