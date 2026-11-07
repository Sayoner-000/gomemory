# hcd-approve ofrece una revisión splash de la definición antes de aprobar

ID: HCD-051
Tipo: evolution
Carril: full
Motivo: Varias decisiones que se afectan entre sí (opcional u obligatoria, qué hacer con los hallazgos, invocación por agente, agentes sin splash) y cambia el contrato de la ceremonia

## Intención
Que en la ceremonia completa /hcd-approve ofrezca lanzar el subagente splash sobre el documento de la unidad antes de pedir la aprobación, para que los errores de la definición (rutas, criterios no comprobables, supuestos sin evidencia) aparezcan cuando corregirlos todavía no obliga a reaprobar ni a verificar de nuevo


## Alcance
Incluido: un paso nuevo en `infrastructure/templates/hcd/hcd-approve/SKILL.md`, antes de presentar la definición. La skill ofrece la revisión y la persona decide en cada unidad (Q1). Los hallazgos se muestran antes de la definición; con un Blocker la skill propone corregir y no pide la cita hasta que se corrija o la persona lo descarte de forma explícita, y Major y Nit son informativos (Q2). Sin `splash` disponible, la skill da una línea de aviso que remite a `mem setup` y sigue como hoy; nunca bloquea (Q3). `mem setup` instala `splash` en los cuatro agentes y `mem uninstall` lo retira (Q4): además de Claude Code y OpenCode, en Codex como `~/.codex/agents/splash.toml` (`sandbox_mode = "read-only"`, instrucciones de revisor) y en agy como `~/.gemini/config/agents/splash/agent.md` (`subagent: true`, `inheritMcp: false`, prompt de revisor). Ambas rutas salen del sondeo real del 2026-11-07 con Herdr (memoria «Sondeo real 2026-11-07…»). La invocación en `docs/MANUAL.md` y en la skill pasa a ser: Claude Code y OpenCode con el subagente `splash`; agy con `invoke_subagent` y `TypeName: "splash"`; Codex con `spawn_agent(agent_type: "splash", fork_turns: "none")`. Limitación de Codex comprobada en el sondeo: el hijo hereda los servidores MCP globales, gomemory incluido, y el TOML del agente no lo impide (ni `enabled = false` ni `enabled_tools = []`). Las instrucciones de `splash` le prohíben usar herramientas MCP, y el MANUAL lo documenta como aislamiento por instrucción, no impuesto. Sin rutas de configuración: `TestHCDSkillsShareRulesAndCoverMVPCommands` rechaza en una SKILL.md `.claude/`, `.codex/`, `.opencode/` y `.agents/`, y la skill tampoco usará otras. El prompt para `splash` pide la lente de la ceremonia: rutas citadas, criterios no comprobables, supuestos sin evidencia y contradicciones entre secciones. `docs/MANUAL.md` y la guía rápida explican el paso.
Excluido: el motor `mem hcd` (ni comandos ni estado nuevos); el Playbook, porque cambiarlo cambia su huella y obligaría a reaprobar las unidades activas (`NORMS-CHANGED`); la Vía rápida, que no tiene `approve`; las demás skills.

## ¿Qué pasa si no lo hacemos?
La revisión sin contexto existe (HCD-047) pero depende de que alguien se acuerde de pedirla. Los errores de la definición se descubren después de aprobar, cuando corregirlos obliga a reaprobar y a verificar otra vez todas las tareas. En HCD-048, una ruta mal citada en la Reproducción obligó a una segunda aprobación (definición `64e31106597f` y después `8516548ee04c`, registradas en la ficha `hcd/HCD-048`) y a repetir las 5 verificaciones, unos 10 minutos.

## Alternativa más barata
Dejarlo documentado en el MANUAL y que la persona lo pida cuando quiera. No cuesta nada, pero es justo lo que pasa hoy: ninguna de las tres unidades posteriores a HCD-047 (HCD-048, HCD-049 y HCD-050) usó `splash`, y sus documentos no lo mencionan. Ponerlo en la skill que pide la aprobación lo deja en el momento exacto en que sirve.

## Historias
### US1. Revisión sin contexto antes de aprobar
Como persona que aprueba una definición en la ceremonia completa, quiero que /hcd-approve haga revisar el documento por `splash` antes de pedirme la cita, para corregir sus errores cuando todavía no obligan a reaprobar.
- C-01: Dado `infrastructure/templates/hcd/hcd-approve/SKILL.md`, cuando se lee el paso nuevo, entonces va antes de pedir la aprobación y dice que la revisión se ofrece y decide la persona (Q1), la invocación de Claude Code, OpenCode, agy y Codex tal como la da la tabla de `docs/MANUAL.md`, la lente de la revisión, y que un Blocker no deja pedir la cita salvo corrección o descarte explícito mientras Major y Nit son informativos (Q2).
- C-02: Dadas las skills HCD, cuando se ejecuta `go test ./adapters/primary/setup -run TestHCDSkills -count=1`, entonces pasa: siguen las reglas comunes y ninguna contiene `.claude/`, `.codex/`, `.opencode/` ni `.agents/`.
- C-03: Dado un agente sin `splash` disponible, cuando se lee el paso nuevo de `infrastructure/templates/hcd/hcd-approve/SKILL.md`, entonces contiene el aviso que remite a `mem setup` y la frase de que la aprobación sigue sin la revisión.

### US3. splash instalado en los cuatro agentes
Como persona que trabaja con Codex o agy, quiero que `mem setup` instale `splash` también allí, para que la revisión tenga las mismas instrucciones de revisor y la misma restricción de solo lectura que en Claude Code y OpenCode.
- C-05: Dado un HOME con Codex y agy, cuando se ejecuta `go test ./adapters/primary/setup -run TestSplash -count=1`, entonces la instalación escribe `.codex/agents/splash.toml` y `.gemini/config/agents/splash/agent.md` bajo ese HOME, no toca los agentes ausentes, es idempotente y `mem uninstall` los retira.
- C-06: Dado `splash` instalado en una sesión real de agy, cuando se lanza con `invoke_subagent` y `TypeName: "splash"`, entonces responde con el formato de revisor y no tiene las herramientas MCP de gomemory (`inheritMcp: false`).
- C-07: Dado `splash` instalado en una sesión real de Codex, cuando se lanza con `spawn_agent(agent_type: "splash", fork_turns: "none")`, entonces responde con el formato de revisor; la herencia del MCP queda documentada como limitación.

### US2. Saber que el paso existe
Como persona que usa HCD, quiero que el manual y la guía rápida expliquen la revisión `splash` en approve, para saber qué recibiré y cómo responder.
- C-04: Dados `docs/MANUAL.md` y la guía rápida HCD vigente (`docs/Guia-Rapida-HCD_GOMEMORY-v*.html`), cuando se busca `splash`, entonces ambos describen el paso de approve y lo que se hace con los hallazgos.

## Aclaraciones
- [x] Q1. ¿Cuándo se lanza splash en /hcd-approve: se ofrece, se lanza siempre o solo si la persona lo pide? → Se ofrece (Recommended): antes de presentar la definición, la skill pregunta si se lanza splash; la persona decide en cada unidad.
- [x] Q2. ¿Qué se hace con los hallazgos de splash: son informativos o un Blocker impide pedir la aprobación? → Tú decides, Blocker pesa (Recommended): los hallazgos se muestran antes de la definición; con un Blocker la skill propone corregir y no pide la cita hasta que se corrija o la persona lo descarte de forma explícita; Major y Nit son informativos; el motor no cambia.
- [x] Q3. ¿Qué hace /hcd-approve en un agente sin splash disponible? → Aviso y sigue (Recommended): una línea que dice que splash no está disponible y cómo instalarlo (mem setup), y la skill presenta la definición como hoy; nunca bloquea.
- [x] Q4. ¿HCD-051 incluye que mem setup instale splash también en Codex (~/.codex/agents/splash.toml, solo lectura) y en agy (agent.md, con un sondeo previo del directorio que agy lee), o eso va en otra unidad? → Sí, en HCD-051 (Recommended): mem setup instala splash también en Codex (~/.codex/agents/splash.toml, solo lectura) y en agy (agent.md, con el directorio confirmado por un sondeo real), y mem uninstall los retira; el aviso y la invocación de approve se basan en lo instalado.

## Slices
<!-- ### VS-01. comportamiento entregable
Tareas: 1, 2
Criterios: C-01 -->
### VS-01. splash instalado en Codex y agy
Tareas: 1, 2, 3, 4, 8
Criterios: C-05, C-06, C-07

### VS-02. /hcd-approve ofrece la revisión splash
Tareas: 5
Criterios: C-01, C-02, C-03

### VS-03. Manual y guía rápida explican el paso
Tareas: 6, 7
Criterios: C-04

## Checklist
- [x] 1. Instalar splash en Codex y agy desde `adapters/primary/setup/splash_agent.go` con plantillas propias en `infrastructure/templates/agents/` → `go test ./adapters/primary/setup -run TestSplash -count=1`
- [x] 2. Cambiar el aviso de Codex de `domain/splash.go` a `agent_type: "splash"` → `go test ./domain ./adapters/primary/cli -run Splash -count=1`
- [x] 3. Comprobar splash instalado en una sesión real de agy (Herdr) desde `infrastructure/templates/agents/agy/splash.md`: responde como revisor y sin MCP de gomemory → observación manual
- [x] 4. Comprobar splash instalado en una sesión real de Codex (Herdr) desde `infrastructure/templates/agents/codex/splash.toml`: responde como revisor → observación manual
- [x] 5. Añadir el paso splash a `infrastructure/templates/hcd/hcd-approve/SKILL.md` → `go test ./adapters/primary/setup -run TestHCDSkills -count=1`
- [x] 6. Documentar el paso y la instalación en `docs/MANUAL.md` y en la guía rápida → `grep -q "Revisión splash antes de aprobar" docs/MANUAL.md docs/Guia-Rapida-HCD_GOMEMORY-v2.50.html`
- [x] 7. Mantener en verde la suite, `go vet` y la compilación de Windows → `go vet ./... && GOOS=windows go build ./... && go test ./... -count=1`
- [x] 8. Comprobar splash en una sesión real de OpenCode (Herdr) desde `infrastructure/templates/agents/opencode/splash.md`: responde como revisor y sin MCP de gomemory → observación manual

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-07 06:31 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/setup -run TestSplash -count=1` (salida 1)
      --- FAIL: TestSplashCodexYAgy (0.00s)
          splash_agent_test.go:52: solo Codex = [] <nil>, quiero /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSplashCodexYAgy1632689316/001/.codex/agents/splash.toml
      FAIL
      FAIL	mem/adapters/primary/setup	0.217s
      FAIL
- 2026-11-07 06:32 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/setup -run 'TestSplash|SplashAgent|GlobalGeneratedArtifacts' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.243s
- 2026-11-07 06:32 -05:00 · tarea 2 · rojo observado · caducada · `go test ./domain ./adapters/primary/cli -run Splash -count=1` (salida 1)
      --- FAIL: TestSplashCodexForkWarning_RecomiendaAgentType (0.00s)
          splash_test.go:14: el aviso no recomienda agent_type: "splash": gomemory: este spawn_agent lanza un subagente splash sin fork_turns: "none". Heredará tu conversación y la memoria del proyecto, y deja de ser un revisor sin contexto. Lánzalo con spawn_agent(task_name: "splash", fork_turns: "none").
      FAIL
      FAIL	mem/domain	0.166s
      ok  	mem/adapters/primary/cli	0.282s
      FAIL
- 2026-11-07 06:32 -05:00 · tarea 2 · verde · caducada · `go test ./domain ./adapters/primary/cli -run Splash -count=1` (salida 0)
      ok  	mem/domain	0.157s
      ok  	mem/adapters/primary/cli	0.312s
- 2026-11-07 06:33 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/setup ./infrastructure -run 'TestSplash|SplashAgent|GlobalGeneratedArtifacts' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.206s
      ok  	mem/infrastructure	0.290s
- 2026-11-07 06:34 -05:00 · tarea 3 · manual · caducada
      Sesión real de agy 1.5.3 con Herdr, con infrastructure/templates/agents/agy/splash.md instalado en ~/.gemini/config/agents/splash/agent.md: invoke_subagent con TypeName splash sobre un doc.md con dos errores sembrados devolvió Hallazgo 1 Blocker (src/exportar.go no existe, con cita y pregunta) y Hallazgo 2 Major (criterio «rápido» no comprobable), y la línea «MCP: NINGUNA» (inheritMcp: false).
- 2026-11-07 06:36 -05:00 · tarea 4 · manual · caducada
      Sesión real de Codex 0.167.0 con Herdr, con infrastructure/templates/agents/codex/splash.toml instalado en ~/.codex/agents/splash.toml: spawn_agent(agent_type: splash, fork_turns: none) sobre un doc.md con dos errores sembrados devolvió dos hallazgos Major con cita y pregunta (src/exportar.go no existe; criterio «rápido» sin umbral). Limitación documentada: el hijo hereda el MCP global; el sondeo previo mostró que el TOML no lo impide, y la plantilla lo prohíbe por instrucción.
- 2026-11-07 06:36 -05:00 · tarea 8 · manual · caducada
      Sesión real de OpenCode 2.2.5 con Herdr, con infrastructure/templates/agents/opencode/splash.md instalado en ~/.config/opencode/agents/splash.md: la herramienta task con el subagente splash sobre un doc.md con dos errores sembrados devolvió Hallazgo 1 Blocker (src/exportar.go no existe, con cita doc.md:3 y pregunta) y Hallazgo 2 Major (criterio «rápido» no comprobable, doc.md:4), y la línea «MCP: NINGUNA».
- 2026-11-07 06:37 -05:00 · tarea 5 · rojo observado · caducada · `go test ./adapters/primary/setup -run TestHCDSkills -count=1` (salida 1)
      --- FAIL: TestHCDSkillsApproveOfreceSplash (0.00s)
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "Revisión splash antes de aprobar"
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "Ofrece"
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "`invoke_subagent` con `TypeName: \"splash\"`"
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "`spawn_agent(agent_type: \"splash\", fork_turns: \"none\")`"
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "Blocker"
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "descarta de forma explícita"
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "Major y Nit son informativos"
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "`mem setup`"
          hcd_skills_test.go:103: hcd-approve/SKILL.md no contiene "la aprobación sigue sin la revisión"
          hcd_skills_test.go:108: la revisión splash debe ir antes de pedir la aprobación (revisión en -1, pregunta en 2749)
      FAIL
      FAIL	mem/adapters/primary/setup	0.364s
      FAIL
- 2026-11-07 06:37 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/setup -run TestHCDSkills -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.191s
- 2026-11-07 06:38 -05:00 · tarea 6 · verde · vigente · `grep -q 'Revisión splash antes de aprobar' docs/MANUAL.md && grep -q 'Revisión splash antes de aprobar' docs/Guia-Rapida-HCD_GOMEMORY-v2.50.html` (salida 0)
- 2026-11-07 06:38 -05:00 · tarea 7 · falló · vigente · `go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 </dev/null` (salida 1)
      ok  	mem/adapters/secondary/launcher	0.563s
      --- FAIL: TestCompressKeepsEveryRefWithSmallConfiguredCap (1.14s)
          compression_originals_cap_test.go:44: se esperaban dos originales guardados: <nil> []
      FAIL
      FAIL	mem/adapters/secondary/persistence	32.739s
      ok  	mem/adapters/secondary/release	0.392s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.198s
      ok  	mem/adapters/secondary/speckit	0.143s
      ok  	mem/adapters/secondary/tokens	0.152s
      ok  	mem/adapters/secondary/usage	0.174s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	15.212s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.264s
      ok  	mem/infrastructure	0.971s
      ok  	mem/tests/contract	96.070s
      ok  	mem/tests/integration	47.880s
      ?   	mem/version	[no test files]
      FAIL
- 2026-11-07 06:40 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/setup ./infrastructure -run 'TestSplash|SplashAgent|GlobalGeneratedArtifacts' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.197s
      ok  	mem/infrastructure	0.183s
- 2026-11-07 06:40 -05:00 · tarea 2 · verde · vigente · `go test ./domain ./adapters/primary/cli -run Splash -count=1` (salida 0)
      ok  	mem/domain	0.133s
      ok  	mem/adapters/primary/cli	0.293s
- 2026-11-07 06:40 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/setup -run TestHCDSkills -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.165s
- 2026-11-07 06:40 -05:00 · tarea 6 · verde · vigente · `grep -q 'Revisión splash antes de aprobar' docs/MANUAL.md && grep -q 'Revisión splash antes de aprobar' docs/Guia-Rapida-HCD_GOMEMORY-v2.50.html` (salida 0)
- 2026-11-07 06:40 -05:00 · tarea 3 · manual · vigente
      Las plantillas instaladas son idénticas (cmp) a las del repositorio, sin cambios desde el sondeo de las 06:33-06:37; la evidencia caducó solo por cambios posteriores en documentación y en la skill. Sesión real de agy 1.5.3 (Herdr) con ~/.gemini/config/agents/splash/agent.md: invoke_subagent con TypeName splash sobre doc.md con dos errores sembrados devolvió Blocker (src/exportar.go no existe) y Major (criterio «rápido» no comprobable) con cita y pregunta, y «MCP: NINGUNA».
- 2026-11-07 06:40 -05:00 · tarea 4 · manual · vigente
      Las plantillas instaladas son idénticas (cmp) a las del repositorio, sin cambios desde el sondeo de las 06:33-06:37; la evidencia caducó solo por cambios posteriores en documentación y en la skill. Sesión real de Codex 0.167.0 (Herdr) con ~/.codex/agents/splash.toml: spawn_agent(agent_type: splash, fork_turns: none) devolvió dos Major con cita y pregunta (ruta inexistente y criterio sin umbral). Limitación documentada: hereda el MCP global; prohibido por instrucción.
- 2026-11-07 06:40 -05:00 · tarea 8 · manual · vigente
      Las plantillas instaladas son idénticas (cmp) a las del repositorio, sin cambios desde el sondeo de las 06:33-06:37; la evidencia caducó solo por cambios posteriores en documentación y en la skill. Sesión real de OpenCode 2.2.5 (Herdr) con ~/.config/opencode/agents/splash.md: task con el subagente splash devolvió Blocker (doc.md:3, ruta inexistente) y Major (doc.md:4, criterio no comprobable) con cita y pregunta, y «MCP: NINGUNA».
- 2026-11-07 06:40 -05:00 · tarea 7 · verde · vigente · `go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 </dev/null` (salida 0)
      ok  	mem/adapters/secondary/generator	1.206s
      ok  	mem/adapters/secondary/gitdiff	0.584s
      ok  	mem/adapters/secondary/hcdfiles	3.128s
      ok  	mem/adapters/secondary/hostbind	0.137s
      ok  	mem/adapters/secondary/launcher	0.553s
      ok  	mem/adapters/secondary/persistence	32.887s
      ok  	mem/adapters/secondary/release	0.331s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.167s
      ok  	mem/adapters/secondary/speckit	0.112s
      ok  	mem/adapters/secondary/tokens	0.094s
      ok  	mem/adapters/secondary/usage	0.105s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	15.411s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.227s
      ok  	mem/infrastructure	0.878s
      ok  	mem/tests/contract	95.695s
      ok  	mem/tests/integration	47.924s
      ?   	mem/version	[no test files]

## Siguiente
Cerrada con aprobación sobre la revisión b8edc827a0da.
