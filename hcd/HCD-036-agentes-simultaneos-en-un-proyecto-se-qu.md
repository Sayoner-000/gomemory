# Agentes simultáneos en un proyecto se quitan la sesión de gomemory y se reenvían contexto y bootstrap

ID: HCD-036
Tipo: fix
Carril: full
Motivo: varias decisiones interdependientes (identidad del agente por runtime, qué se separa, efecto en resúmenes y checkpoints) y cambia un contrato del motor

## Intención
Que dos o más agentes trabajando a la vez en el mismo proyecto no se cierren la sesión de gomemory entre sí, de modo que lo ya entregado a cada uno no se le reenvíe (contexto ~4,3k tokens y bootstrap ~1,5k por rotación).

## Reproducción
En un repositorio y un almacén aislados (`GOMEMORY_DATA_HOME` temporal, nivel de compresión `max`, alguna memoria guardada):
1. `echo '{"hook_event_name":"SessionStart","source":"startup","session_id":"agente-A"}' | mem hook session-start` → `mem context` de A muestra 3 marcas «ya entregado».
2. `echo '{"hook_event_name":"SessionEnd","session_id":"agente-B"}' | mem hook session-end` (otro agente termina).
3. `echo '{"hook_event_name":"UserPromptSubmit","session_id":"agente-A","prompt":"sigue"}' | mem hook user-prompt-submit` → A recibe otra vez el bootstrap «PRIMERA ACCIÓN» (1), y `mem context` de A vuelve a 0 marcas: todo se reenvía.
En uso real (2026-11-05): una sola conversación de Claude pasó por 4 sesiones de gomemory (3ff42e82, 42efd862, 59c64a59, a6a59ef5); el `last_prompt` de 42efd862 era de otro agente que trabajaba a la vez en el proyecto.

## Alcance
Incluido: cada conversación de agente tiene su propio payload de sesión, sin compartirlo con otros agentes del mismo proyecto. Ese payload incluye: (1) la sesión de gomemory, identificada por el `session_id` del host o por un id local si el host no lo da; (2) el registro de entregas (`context_deliveries`, `delivered_blocks`) acotado a esa sesión; (3) el estado por turno de `.memory/` (`.conversation`, `.session-tools-injected`, marcas de plan, huella, avisos), que pasa a guardarse por conversación. SessionStart, SessionEnd, `end_session`, la rotación por inactividad y el reinicio tras compactar actúan solo sobre la sesión propia. Las memorias guardadas se asocian a la sesión del agente que las guarda. Superficies: hooks (`adapters/primary/cli/cmd_hook.go`, `adapters/primary/cli/conversation.go`), el servidor MCP (`adapters/primary/cli/cmd_mcp.go`), la persistencia de sesiones y entregas, y la CLI `mem session`/`mem context`.
Excluido: cambiar el contenido del contexto o del bootstrap (otra unidad); el compresor; dar sesión a los subagentes (siguen con el aviso mínimo de HCD-026); migrar las sesiones pasadas, que se quedan como están.

## ¿Qué pasa si no lo hacemos?
Con dos o más agentes en el mismo proyecto (paneles de Herdr, otra terminal), cada arranque o cierre de uno le cuesta al otro unos 4.300 tokens de contexto reenviado y unos 1.500 de bootstrap. Además, sus memorias y checkpoints se mezclan en la sesión del otro y el resumen de cierre de uno cierra el trabajo del otro. En la medición del 2026-11-05 pasó 4 veces en una sola conversación.

## Alternativa más barata
Mitigación mínima: que SessionStart y SessionEnd no cierren una sesión con actividad reciente. Es más barata, pero no basta: las marcas por turno y el registro de entregas siguen siendo del proyecto, así que el arranque de otro agente sigue borrando la marca del bootstrap y reiniciando las entregas. Se descarta porque solo resuelve una de las tres fugas.

## Historias
### US1. Mi sesión es mía aunque otro agente trabaje en el proyecto
Como agente que trabaja en paralelo con otro en el mismo proyecto, quiero que mi sesión de gomemory, mi registro de entregas y mi estado por turno sean solo míos, para no recibir de nuevo el contexto ni el bootstrap por lo que hace el otro.
- C-01: Dado que el agente A ya recibió el contexto, cuando el agente B arranca o termina su sesión, entonces `get_context` de A sigue devolviendo «ya entregado» y la sesión de A sigue activa.
- C-02: Dado que el agente A ya recibió el bootstrap, cuando el agente B arranca, entonces el siguiente prompt de A no vuelve a recibir el bootstrap.
- C-03: Dados A y B activos, cuando A guarda una memoria, entonces queda asociada a la sesión de A y no a la de B.

### US2. El cierre de un agente cierra solo lo suyo
Como persona que usa varios agentes a la vez, quiero que el cierre de uno no cierre la sesión de los otros, para que cada resumen describa solo el trabajo de su agente.
- C-04: Dados A y B activos, cuando B termina (SessionEnd o `end_session`), entonces solo la sesión de B queda cerrada y con su resumen.
- C-05: Dado un único agente en el proyecto, cuando arranca, trabaja y cierra, entonces el comportamiento visible es el de hoy (una sesión, contexto completo al inicio y «ya entregado» después).

## Aclaraciones
- [x] Q1. Identidad: el servidor MCP no recibe el session_id del host. ¿Cómo sabe a qué sesión de agente pertenece una llamada MCP? → Enlace por proceso del host (Recommended): SessionStart publica pid del host + hora de arranque del proceso → session_id (patrón hostbind de engram); el MCP lo lee por su proceso padre; enlace ausente o caducado → sesión local propia del MCP, nunca la de otro agente
- [x] Q2. CLI desde el agente: cuando el agente ejecuta mem context o mem save en Bash, ¿qué sesión usa? → La del agente (Recommended): la CLI sube por sus procesos ancestros hasta un host con enlace y usa esa sesión; sin enlace (persona en su terminal) usa una sesión propia de terminal
- [x] Q3. Contexto con sesiones en paralelo: ¿qué muestra get_context en «Sesión Activa» y «Sesiones Recientes» cuando hay varios agentes activos? → Solo la propia (Recommended): «Sesión Activa» muestra solo la sesión del agente que pide el contexto; «Sesiones Recientes» sigue mostrando las cerradas de todos

## Slices
### VS-01. Cada proceso sabe a qué conversación de agente pertenece
Tareas: 1, 2, 3
Criterios: C-05

### VS-02. Una sesión de gomemory por conversación
Tareas: 4, 5
Criterios: C-03, C-04

### VS-03. Entregas y estado por turno propios de cada conversación
Tareas: 6, 7
Criterios: C-01, C-02

### VS-04. Superficies MCP y CLI con la sesión propia, comprobadas en real
Tareas: 8, 9
Criterios: C-05

## Checklist
- [x] 1. La reproducción ya no falla: dos agentes en `tests/integration/hcd036_sesion_por_agente_test.go` (A arranca, B cierra, A conserva «ya entregado» y no recibe el bootstrap) → go test ./tests/integration/ -run TestHCD036
- [x] 2. Definir la identidad de la conversación en `domain/conversation.go` (precedencia: id del hook > enlace del host > id local) y su port en `application/ports/conversation.go` → go test ./domain/ -run TestConversationIdentity
- [x] 3. Crear el enlace por proceso del host en `adapters/secondary/hostbind/hostbind.go` (publicar y buscar con pid y hora de arranque, recorrer ancestros, podar los muertos) → go test ./adapters/secondary/hostbind/
- [x] 4. Asociar las sesiones a su conversación en `adapters/secondary/persistence/session.go` y `adapters/secondary/persistence/db.go` (columna aditiva `conversation_id`; Active, Start y SetLastPrompt filtran por ella) → go test ./adapters/secondary/persistence/ -run TestSesionPorConversacion
- [x] 5. Hacer que SessionStart publique el enlace y que SessionEnd, `end_session` y la rotación cierren solo la sesión propia en `adapters/primary/cli/cmd_hook.go` y `adapters/primary/cli/conversation.go` → go test ./adapters/primary/cli/ -run TestSesionPropia
- [x] 6. Acotar el registro de entregas y de bloques a la sesión de la conversación en `adapters/secondary/persistence/context_delivery.go` y `adapters/secondary/persistence/delivered_blocks.go` → go test ./adapters/secondary/persistence/ -run TestEntregasPorConversacion
- [x] 7. Guardar el estado por turno de `.memory/` por conversación en `adapters/primary/cli/conversation.go` → go test ./adapters/primary/cli/ -run TestEstadoPorTurnoPorConversacion
- [x] 8. Resolver la conversación del proceso en `adapters/secondary/hostbind/resolver.go` (id del hook, enlace con caché, id local según el canal) y cablearla en `infrastructure/container.go` para sesión, entregas y bloques, de modo que «Sesión Activa» muestre solo la propia (Q3) → go test ./adapters/secondary/hostbind/ ./infrastructure/ -run "TestResolver|TestContenedorSesionPorConversacion"
- [x] 9. Comprobar con dos sesiones reales de Claude en paralelo sobre `hcd/` de este repositorio que ninguna rota la sesión de la otra → observar `mem session list` y el registro de entregas

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 20:36 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./tests/integration/ -run "^TestHCD036$"` (salida 1)
      --- FAIL: TestHCD036 (1.37s)
          hcd036_sesion_por_agente_test.go:147: C-02: A recibió otra vez el bootstrap tras la actividad de B
      FAIL
      FAIL	mem/tests/integration	1.827s
      FAIL
- 2026-11-05 20:38 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./tests/integration/ -run "^TestHCD036$"` (salida 1)
      --- FAIL: TestHCD036 (1.49s)
          hcd036_sesion_por_agente_test.go:153: sql: no rows in result set
      FAIL
      FAIL	mem/tests/integration	1.769s
      FAIL
- 2026-11-05 20:39 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./tests/integration/ -run "^TestHCD036$"` (salida 1)
      --- FAIL: TestHCD036 (1.37s)
          hcd036_sesion_por_agente_test.go:157: sql: no rows in result set
      FAIL
      FAIL	mem/tests/integration	1.664s
      FAIL
- 2026-11-05 20:40 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./tests/integration/ -run "^TestHCD036$"` (salida 1)
              ## Decisiones Técnicas
              - **Decisión sobre persistencia**: Se eligió una regla clara para persistencia porque reduce el acoplamiento y… → `get_memory 3`
              - **Decisión sobre pruebas**: Se eligió una regla clara para pruebas porque reduce el acoplamiento y… → `get_memory 2`
              - **Decisión sobre capas**: Se eligió una regla clara para capas porque reduce el acoplamiento y simplifica… → `get_memory 1`
              ## Aprendizajes Recientes
              <!-- mem:volatile -->
              ## Sesiones Recientes
              - 2026-11-05 20:40:44 → 2026-11-05 20:40:45: (sin resumen)
          hcd036_sesion_por_agente_test.go:173: C-02: A recibió otra vez el bootstrap tras la actividad de B
      FAIL
      FAIL	mem/tests/integration	1.626s
      FAIL
- 2026-11-05 20:41 -05:00 · tarea 2 · rojo observado · caducada · `go test ./domain/ -run TestConversationIdentity` (salida 1)
      # mem/domain [mem/domain.test]
      domain/conversation_test.go:69:13: undefined: ResolveConversationID
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 20:41 -05:00 · tarea 2 · verde · caducada · `go test ./domain/ -run TestConversationIdentity` (salida 0)
      ok  	mem/domain	0.170s
- 2026-11-05 20:41 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/secondary/hostbind/` (salida 1)
      # mem/adapters/secondary/hostbind [mem/adapters/secondary/hostbind.test]
      adapters/secondary/hostbind/hostbind_test.go:36:69: undefined: Store
      adapters/secondary/hostbind/hostbind_test.go:38:7: undefined: New
      FAIL	mem/adapters/secondary/hostbind [build failed]
      FAIL
- 2026-11-05 20:42 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/secondary/hostbind/` (salida 0)
      ok  	mem/adapters/secondary/hostbind	0.144s
- 2026-11-05 20:42 -05:00 · tarea 4 · rojo observado · caducada · `go test ./adapters/secondary/persistence/ -run TestSesionPorConversacion` (salida 1)
      # mem/adapters/secondary/persistence [mem/adapters/secondary/persistence.test]
      adapters/secondary/persistence/session_conversation_test.go:18:53: NewSessionRepository(db).(*SessionRepository).WithConversation undefined (type *SessionRepository has no field or method WithConversation)
      adapters/secondary/persistence/session_conversation_test.go:19:53: NewSessionRepository(db).(*SessionRepository).WithConversation undefined (type *SessionRepository has no field or method WithConversation)
      FAIL	mem/adapters/secondary/persistence [build failed]
      FAIL
- 2026-11-05 20:44 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/persistence/ -run "TestSesionPorConversacion|TestSchemaGolden"` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.237s
- 2026-11-05 20:46 -05:00 · tarea 5 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run TestSesionPropia` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/session_propia_test.go:40:2: undefined: adoptHookConversation
      adapters/primary/cli/session_propia_test.go:48:2: undefined: adoptHookConversation
      adapters/primary/cli/session_propia_test.go:53:2: undefined: adoptHookConversation
      adapters/primary/cli/session_propia_test.go:58:2: undefined: endOwnSession
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-05 20:46 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli/ -run TestSesionPropia` (salida 0)
      ok  	mem/adapters/primary/cli	0.313s
- 2026-11-05 20:46 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli/ -run TestSesionPropia` (salida 0)
      ok  	mem/adapters/primary/cli	(cached)
- 2026-11-05 20:47 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/secondary/persistence/ -run TestEntregasPorConversacion` (salida 1)
      # mem/adapters/secondary/persistence [mem/adapters/secondary/persistence.test]
      adapters/secondary/persistence/delivery_conversation_test.go:9:6: relojFijo redeclared in this block
      	adapters/secondary/persistence/borrado_memorias_test.go:19:6: other declaration of relojFijo
      adapters/secondary/persistence/delivery_conversation_test.go:11:18: method relojFijo.Now already declared at adapters/secondary/persistence/borrado_memorias_test.go:21:18
      adapters/secondary/persistence/delivery_conversation_test.go:28:44: NewDeliveryLogRepository(db, "p").WithConversation undefined (type *DeliveryLogRepository has no field or method WithConversation)
      adapters/secondary/persistence/delivery_conversation_test.go:29:44: NewDeliveryLogRepository(db, "p").WithConversation undefined (type *DeliveryLogRepository has no field or method WithConversation)
      adapters/secondary/persistence/delivery_conversation_test.go:30:65: NewDeliveredBlocksRepository(db, "p", relojFijo{}).WithConversation undefined (type *DeliveredBlocksRepository has no field or method WithConversation)
      adapters/secondary/persistence/delivery_conversation_test.go:31:65: NewDeliveredBlocksRepository(db, "p", relojFijo{}).WithConversation undefined (type *DeliveredBlocksRepository has no field or method WithConversation)
      FAIL	mem/adapters/secondary/persistence [build failed]
      FAIL
- 2026-11-05 20:47 -05:00 · tarea 6 · falló · caducada · `go test ./adapters/secondary/persistence/ -run TestEntregasPorConversacion` (salida 1)
      # mem/adapters/secondary/persistence [mem/adapters/secondary/persistence.test]
      adapters/secondary/persistence/delivery_conversation_test.go:9:6: relojFijo redeclared in this block
      	adapters/secondary/persistence/borrado_memorias_test.go:19:6: other declaration of relojFijo
      adapters/secondary/persistence/delivery_conversation_test.go:11:18: method relojFijo.Now already declared at adapters/secondary/persistence/borrado_memorias_test.go:21:18
      FAIL	mem/adapters/secondary/persistence [build failed]
      FAIL
- 2026-11-05 20:48 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/secondary/persistence/ -run TestEntregasPorConversacion` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.228s
- 2026-11-05 20:49 -05:00 · tarea 7 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run TestEstadoPorTurnoPorConversacion` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/estado_por_conversacion_test.go:16:41: undefined: stateConversation
      adapters/primary/cli/estado_por_conversacion_test.go:17:2: undefined: stateConversation
      adapters/primary/cli/estado_por_conversacion_test.go:46:2: undefined: stateConversation
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-05 20:51 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli/ -run TestEstadoPorTurnoPorConversacion` (salida 0)
      ok  	mem/adapters/primary/cli	0.297s
- 2026-11-05 20:52 -05:00 · tarea 8 · rojo observado · caducada · `go test ./adapters/secondary/hostbind/ ./infrastructure/ -run "TestResolver|TestContenedorSesionPorConversacion"` (salida 1)
      # mem/adapters/secondary/hostbind [mem/adapters/secondary/hostbind.test]
      adapters/secondary/hostbind/resolver_test.go:25:7: undefined: NewResolver
      adapters/secondary/hostbind/resolver_test.go:46:12: undefined: NewResolver
      FAIL	mem/adapters/secondary/hostbind [build failed]
      ok  	mem/infrastructure	0.274s [no tests to run]
      FAIL
- 2026-11-05 20:53 -05:00 · tarea 8 · verde · caducada · `go test ./adapters/secondary/hostbind/ ./infrastructure/ -run "TestResolver|TestContenedorSesionPorConversacion"` (salida 0)
      ok  	mem/adapters/secondary/hostbind	0.134s
      ok  	mem/infrastructure	0.284s
- 2026-11-05 21:02 -05:00 · tarea 1 · verde · caducada · `go test -count=1 ./tests/integration/ -run "^TestHCD036$"` (salida 0)
      ok  	mem/tests/integration	1.760s
- 2026-11-05 21:02 -05:00 · tarea 2 · verde · caducada · `go test ./domain/ -run TestConversationIdentity` (salida 0)
      ok  	mem/domain	(cached)
- 2026-11-05 21:02 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/secondary/hostbind/` (salida 0)
      ok  	mem/adapters/secondary/hostbind	(cached)
- 2026-11-05 21:02 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/persistence/ -run TestSesionPorConversacion` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.393s
- 2026-11-05 21:02 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli/ -run TestSesionPropia` (salida 0)
      ok  	mem/adapters/primary/cli	0.307s
- 2026-11-05 21:02 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/secondary/persistence/ -run TestEntregasPorConversacion` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.180s
- 2026-11-05 21:02 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli/ -run TestEstadoPorTurnoPorConversacion` (salida 0)
      ok  	mem/adapters/primary/cli	0.284s
- 2026-11-05 21:02 -05:00 · tarea 8 · verde · caducada · `go test ./adapters/secondary/hostbind/ ./infrastructure/ -run "TestResolver|TestContenedorSesionPorConversacion"` (salida 0)
      ok  	mem/adapters/secondary/hostbind	(cached)
      ok  	mem/infrastructure	0.233s
- 2026-11-05 21:11 -05:00 · tarea 9 · manual · vigente
      2026-11-05 21:10, binario de la rama delante en el PATH (sin tocar el instalado), dos claude -p --model haiku solapados en este repositorio: A (5b16b9d9…) abrió la sesión a725f8cb; B (58dae61c…) arrancó a los 10 s, abrió 5dae6545 y la cerró a las 21:10:18 mientras A seguía vivo; la sesión de A siguió abierta hasta su propio SessionEnd (21:10:39). Cada host publicó su enlace (hostbind/81233.json, 81520.json). Sin sesiones huérfanas del MCP (la primera prueba, antes de corregir la carrera, dejó 2a1de275 y 97d4f585 abiertas). Límite: el registro de entregas no se pudo observar porque en este proyecto el contexto de arranque (~17.000 caracteres) supera HookInlineContextMaxChars y el hook no lo anota como entregado; es preexistente y queda propuesto como unidad aparte.
- 2026-11-05 21:11 -05:00 · tarea 1 · verde · vigente · `go test -count=1 ./tests/integration/ -run "^TestHCD036$"` (salida 0)
      ok  	mem/tests/integration	1.938s
- 2026-11-05 21:11 -05:00 · tarea 2 · verde · vigente · `go test ./domain/ -run TestConversationIdentity` (salida 0)
      ok  	mem/domain	(cached)
- 2026-11-05 21:11 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/secondary/hostbind/` (salida 0)
      ok  	mem/adapters/secondary/hostbind	(cached)
- 2026-11-05 21:11 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/secondary/persistence/ -run TestSesionPorConversacion` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.185s
- 2026-11-05 21:11 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/cli/ -run TestSesionPropia` (salida 0)
      ok  	mem/adapters/primary/cli	0.322s
- 2026-11-05 21:11 -05:00 · tarea 6 · verde · vigente · `go test ./adapters/secondary/persistence/ -run TestEntregasPorConversacion` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.335s
- 2026-11-05 21:11 -05:00 · tarea 7 · verde · vigente · `go test ./adapters/primary/cli/ -run TestEstadoPorTurnoPorConversacion` (salida 0)
      ok  	mem/adapters/primary/cli	0.286s
- 2026-11-05 21:11 -05:00 · tarea 8 · verde · vigente · `go test ./adapters/secondary/hostbind/ ./infrastructure/ -run "TestResolver|TestContenedorSesionPorConversacion"` (salida 0)
      ok  	mem/adapters/secondary/hostbind	0.160s
      ok  	mem/infrastructure	0.262s

## Siguiente
Cerrada con aprobación sobre la revisión de6bdd9e255d.
