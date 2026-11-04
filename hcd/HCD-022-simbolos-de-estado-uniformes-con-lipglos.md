# Símbolos de estado uniformes con lipgloss en las validaciones de terminal

ID: HCD-022
Tipo: evolution
Carril: full
Motivo: Cambia salidas públicas de varios comandos, hay tests que comparan texto y queda abierta la decisión del símbolo de advertencia

## Intención
Sustituir los emojis de estado (✅ ❌ ⚠️ ℹ️) de las validaciones en terminal por el juego ✓ ✗ ○ ● ▸ ⚑ pintado con lipgloss desde la paleta de console, con degradación sin color fuera de TTY o con NO_COLOR, para dar consistencia a toda la CLI y la TUI

Situación actual (medida en 0ce9260):
- `adapters/primary/console` ya tiene un juego de glifos con color: `stateGlyph` (`components.go:31`: `○ ● ✓ ⚠ ✗ −`) y `Palette.StateColor`, y `Layout.styleText` traduce emojis a glifos con el reemplazador `glyphs` (`layout.go:224`). Solo se aplica a lo que pasa por `Layout` en una terminal estilizada.
- La mayoría de los comandos imprimen los emojis directamente con `fmt.Printf`/`humanf` y se saltan ese juego: en producción hay 90 `⚠️`, 83 `✅`, 35 `ℹ️` y 5 `❌`, además de emojis decorativos (`🔌 📦 🧹 🔐 🔄 🎯 🔒 🎉 🗑️ 🔗 📁`). Los que más tienen son `cmd_mcp_setup.go` (70), `cmd_uninstall.go` (43), `cmd_install.go` (37), `cmd_update.go` (20) y `cmd_install_cleanup.go` (13), seguidos de `setup/*_setup.go`, `cmd_doctor*.go` y `tui/tui.go`.
- La TUI marca los errores con `✕` (`tui/styles.go:99`, `tui.go`), que no es el `✗` de console.

## Alcance
Incluido: una única fuente de símbolos de estado en `adapters/primary/console`, que amplía `stateGlyph`/`StateColor` con `▸` (seleccionado) y `⚑` (marcado) y añade una función pública que devuelve el símbolo pintado con lipgloss según la paleta activa, o sin estilo fuera de TTY, con `NO_COLOR` o con `TERM=dumb`; sustituir los emojis de estado y los decorativos en la salida de terminal de `adapters/primary/cli`, `adapters/primary/setup`, `adapters/primary/console` y `adapters/primary/tui` por esa función (incluido `✕`→`✗` en la TUI); actualizar el reemplazador `glyphs` y los tests que comparan texto; un test de guarda que falla si aparece un emoji en código de producción de `adapters/primary`. Símbolo de advertencia: `⚠` (U+26A0 sin U+FE0F, el `StateWarn` existente), pintado con el color de advertencia (Q1). Símbolo informativo: `○` (el glifo neutro por defecto de `stateGlyph`), pintado con el color Muted (Q2); el reemplazador `glyphs` deja de traducir `ℹ️` a `·`. Estructura visual (Q3): los comandos del ciclo de vida (`setup-mcp`, `install`, `uninstall`, `update`, `cmd_install_cleanup.go`) y los mensajes de `setup/*_setup.go` dejan de escribir con `fmt.Print*` y pasan por `console.Printf`/`humanf`, igual que `mem doctor`, así que `Layout` les da títulos `▸`, clave/valor alineado y glifos coloreados con `Mark`. `setup-mcp` usa además `BeginPage`/`EndPage` porque no pregunta nada; `install`, `uninstall` y `update` se estilizan línea a línea para no retener la salida antes de sus preguntas. `statusGlyph` reconoce los ocho estados del juego, `○` incluido.
Excluido: el texto que gomemory inyecta a los agentes (`application/usecases/build_context.go`: `## 🔥`, `## 🧭`, `## 🔗`, `## ⚠`) y los marcadores de método de `domain/plan_shape.go`, porque son contrato de prompt, no salida de terminal; el corpus de `tests/testdata`; el contenido de las memorias, los documentos y las skills; cambiar la paleta o los temas existentes.

## ¿Qué pasa si no lo hacemos?
La misma validación aparece con emojis o con glifos según el comando y según pase o no por `Layout`: `mem setup-mcp`, `install`, `uninstall` y `update` muestran `✅/⚠️/ℹ️` sin el color del tema, mientras `doctor` y los informes muestran `✓/⚠/✗` coloreados. Los emojis cambian de ancho entre terminales (variante FE0F), descuadran columnas y no respetan `NO_COLOR`.

## Alternativa más barata
Aplicar el reemplazador `glyphs` existente a toda la salida (envolver stdout). Cambia el texto, pero no el color, y la guarda contra emojis nuevos tampoco existe. Además, con `glyphs` fuera de la TTY los emojis desaparecen solo a veces. La ventaja es menos diff. El inconveniente es que deja dos fuentes de verdad.

## Historias
### US1. Validaciones de terminal con un solo juego de símbolos
Como persona que instala y diagnostica gomemory, quiero que cada validación de la CLI use el mismo juego de símbolos con el color del tema, para leer el estado de un vistazo sin importar el comando.
- C-01: Dado un HOME temporal, cuando ejecuto `mem setup-mcp --scope global` en una TTY simulada, entonces las líneas de resultado empiezan por `✓`, `✗`, `⚠` (sin U+FE0F) o `○`, pintados con los colores `StateColor` de la paleta, y ninguna contiene un emoji.
- C-02: Dado `NO_COLOR=1` o una salida que no es TTY, cuando ejecuto el mismo comando, entonces las líneas llevan los mismos símbolos sin secuencias ANSI.

### US2. Guarda contra emojis nuevos
Como persona que mantiene gomemory, quiero que un test falle si alguien vuelve a escribir un emoji en la salida de terminal, para que la consistencia no dependa de la memoria.
- C-03: Dado el código de producción de `adapters/primary`, cuando ejecuto el test de guarda, entonces falla si encuentra un emoji: cualquier carácter de U+1F300–U+1FAFF, el selector de variante U+FE0F o los emojis del plano básico usados hoy (`✅` U+2705, `❌` U+274C, `➖` U+2796, `ℹ` U+2139, `✕` U+2715, `♥` U+2665); los símbolos del juego (`✓ ✗ ⚠ ○ ● ▸ ⚑ −`) están permitidos y solo se definen en `console`.

### US3. La TUI usa los mismos símbolos
Como persona que usa la TUI, quiero que los errores y estados de la TUI usen los mismos símbolos que la CLI, para no aprender dos lenguajes visuales.
- C-04: Dado un error en la TUI (duplicados, importación, ajustes), cuando se renderiza, entonces se marca con `✗` de la fuente única de `console` y no con `✕`.

### US4. Los comandos del ciclo de vida se ven como mem doctor
Como persona que instala y registra gomemory, quiero que `setup-mcp`, `install`, `uninstall` y `update` tengan la misma estructura visual que `mem doctor` y `mem --help`, para que la consola sea coherente en todos los comandos de validación.
- C-05: Dado el código de producción de los comandos del ciclo de vida y de `setup/*_setup.go`, cuando se inspecciona, entonces ninguno escribe en stdout con `fmt.Print*`; y dada una terminal estilizada, cuando `Layout` recibe una línea `  ○ agy: …`, entonces el `○` sale pintado con `Mark(StatePending)`.

## Aclaraciones
- [x] Q1. ¿Qué símbolo sustituye a ⚠️/⚠ (advertencia) en las validaciones de terminal? → ⚠ sin emoji (Recommended): se conserva el glifo StateWarn ⚠ de console sin la variante U+FE0F, pintado con el color de advertencia de la paleta
- [x] Q2. ¿Qué símbolo sustituye a ℹ️ (información) en las validaciones de terminal? → ○ gris (Recommended): ℹ️ pasa a ○, el glifo neutro por defecto de stateGlyph, pintado con el color Muted de la paleta
- [x] Q3. ¿Qué comandos pasan a la estructura visual de Layout (BeginPage/humanf/EndPage, como mem doctor y mem --help) además de cambiar los símbolos? → Ciclo de vida (Recommended): setup-mcp, install, uninstall y update, más los mensajes que imprimen setup/*_setup.go, pasan a la estructura de Layout como mem doctor; el resto de la CLI solo cambia de símbolos

## Slices
### VS-01. Las validaciones de la CLI salen con el juego de símbolos de console, con color en TTY y sin él fuera
Tareas: 1, 2, 4
Criterios: C-01, C-02

### VS-02. La TUI marca estados y errores con los mismos símbolos
Tareas: 5
Criterios: C-04

### VS-03. Una guarda impide que vuelvan los emojis
Tareas: 3, 8
Criterios: C-03

### VS-04. Los comandos del ciclo de vida usan la estructura de Layout
Tareas: 6, 7
Criterios: C-05

## Checklist
- [x] 1. Exponer `console.Mark(StepState)`/`Layout.Mark` con `▸` (seleccionado) y `⚑` (marcado) añadidos al juego, pintado con `StateColor` en terminal estilizada y sin ANSI fuera de TTY, con `NO_COLOR` o `TERM=dumb` → `go test ./adapters/primary/console -run TestMark -count=1`
- [x] 2. Sustituir los emojis de `mem setup-mcp` (`cmd_mcp_setup.go`, `agent_detect.go`) por `console.Mark` → `go test ./adapters/primary/cli -run TestSetupMCPGlobal_SimbolosSinEmoji -count=1`
- [x] 3. Crear la guarda que detecta emojis (U+1F300–U+1FAFF, U+FE0F, ✅ ❌ ➖ ℹ ✕ ♥) y permite el juego `✓ ✗ ⚠ ○ ● ▸ ⚑ −` → `go test ./adapters/primary/console -run TestEmojiGuard_Detecta -count=1`
- [x] 4. Sustituir los emojis restantes de `adapters/primary/cli`, `adapters/primary/setup` y `adapters/primary/console` por `console.Mark` y retirar `ℹ️`→`·` del reemplazador `glyphs` → `go test ./adapters/primary/console -run 'TestSinEmojis/(cli|setup|console)' -count=1`
- [x] 5. Sustituir `✕`, `♥` y los emojis de `adapters/primary/tui` por los símbolos de console → `go test ./adapters/primary/console ./adapters/primary/tui -run 'TestSinEmojis/tui|TestTUI_ErrorUsaMarcaDeConsole' -count=1`
- [x] 6. Hacer que `Layout` coloree con `Mark` las líneas que empiezan por cualquiera de los ocho glifos (`statusGlyph` con `○ ● ▸ ⚑`) → `go test ./adapters/primary/console -run TestLayout_ColoreaTodosLosEstados -count=1`
- [x] 7. Pasar `setup-mcp` (con `BeginPage`/`EndPage`), `install`, `uninstall`, `update`, `cmd_install_cleanup.go` y `setup/*_setup.go` de `fmt.Print*` a `humanf`/`humanln`/`console.Printf` con glifos planos → `go test ./adapters/primary/cli -run 'TestCicloDeVida_SinFmtPrint|TestSetupMCPGlobal_SimbolosSinEmoji' -count=1`
- [x] 8. Verificación completa → `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-04 04:41 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/console -run TestMark -count=1` (salida 1)
      # mem/adapters/primary/console [mem/adapters/primary/console.test]
      adapters/primary/console/mark_test.go:15:39: undefined: StateSelected
      adapters/primary/console/mark_test.go:15:61: undefined: StateFlagged
      adapters/primary/console/mark_test.go:24:28: NewLayout(styled).Mark undefined (type Layout has no field or method Mark)
      adapters/primary/console/mark_test.go:32:27: NewLayout(e).Mark undefined (type Layout has no field or method Mark)
      adapters/primary/console/mark_test.go:37:23: NewLayout(styled).Mark undefined (type Layout has no field or method Mark)
      FAIL	mem/adapters/primary/console [build failed]
      FAIL
- 2026-11-04 04:41 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/console -run TestMark -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.178s
- 2026-11-04 04:41 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestSetupMCPGlobal_SimbolosSinEmoji -count=1` (salida 1)
      mem/adapters/primary/cli.setupOpenCodeGlobal({{0x102b7b1f8, 0x3}, {0x4add2eabb120, 0x1, 0x1}, {0x102b7b1f8, 0x3}, 0x1})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_setup.go:364 +0x70
      mem/adapters/primary/cli.runGlobalScopeAgent({0x102ba0b97?, 0x4add2ef65ba8?}, {{0x102b7b1f8, 0x3}, {0x4add2eabb120, 0x1, 0x1}, {0x102b7b1f8, 0x3}, 0x1})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_setup.go:348 +0x168
      mem/adapters/primary/cli.runGlobalScopeSetup({0x4add2eaac820, 0x5, 0x102b7aec1?})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_setup.go:174 +0x220
      mem/adapters/primary/cli.CmdMCPSetup(0x0, {0x4add2e9f9380, 0x4, 0x4})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_setup.go:75 +0x200
      mem/adapters/primary/cli.TestSetupMCPGlobal_SimbolosSinEmoji.func1()
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/hcd022_setup_mcp_symbols_test.go:22 +0x88
      mem/adapters/primary/cli.captureStdout(0x4add2eacf688, 0x4add2ebacf48)
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_plan_context_test.go:59 +0x110
      mem/adapters/primary/cli.TestSetupMCPGlobal_SimbolosSinEmoji(0x4add2eacf688)
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/hcd022_setup_mcp_symbols_test.go:21 +0xc4
      testing.tRunner(0x4add2eacf688, 0x103785b58)
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2195 +0xc4
      created by testing.(*T).Run in goroutine 1
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2260 +0x3b8
      FAIL	mem/adapters/primary/cli	0.358s
      FAIL
- 2026-11-04 04:42 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestSetupMCPGlobal_SimbolosSinEmoji -count=1` (salida 1)
                ✅ codex: 10 hook(s) del ciclo de gomemory registrado(s) en config.toml
                ℹ️  codex: autoriza los hooks nuevos en tu próxima sesión de Codex para que queden activos
                ✅ codex: ~/.codex/config.toml actualizado con registro global (gomemory)
                ℹ️  agy: no instalado (ni `agy` en el PATH ni ~/.gemini/config); se omite
                ⚠️  cursor no soporta --scope global (solo por proyecto): usa 'mem setup-mcp --scope project --agents cursor --target <dir>'
                ✅ hooks de modo plan: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupMCPGlobal_SimbolosSinEmoji1320282914/001/.claude/settings.json
                ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupMCPGlobal_SimbolosSinEmoji1320282914/001/.claude/skills/constitution/SKILL.md
                ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupMCPGlobal_SimbolosSinEmoji1320282914/001/.codex/skills/constitution/SKILL.md
              ✅ 1 registro(s) global(es) completados. Reinicia el agente para que los detecte.
              🔌 Configurando MCP para gomemory en /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupMCPGlobal_SimbolosSinEmoji1320282914/003
                ℹ️  codex: el registro MCP es global; se reutiliza para todos los proyectos
                ✅ codex: ~/.codex/config.toml ya tiene el registro global de gomemory
                ⚠️  Agente desconocido: desconocido (opciones: opencode, claude, cursor, codex, all)
              ✅ 1 configuraciones MCP generadas. Reinicia el agente para que las detecte.
      FAIL
      FAIL	mem/adapters/primary/cli	0.291s
      FAIL
- 2026-11-04 04:43 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestSetupMCPGlobal_SimbolosSinEmoji -count=1` (salida 1)
                ✅ codex: 10 hook(s) del ciclo de gomemory registrado(s) en config.toml
                ℹ️  codex: autoriza los hooks nuevos en tu próxima sesión de Codex para que queden activos
                ✅ codex: ~/.codex/config.toml actualizado con registro global (gomemory)
                ℹ️  agy: no instalado (ni `agy` en el PATH ni ~/.gemini/config); se omite
                ⚠️  cursor no soporta --scope global (solo por proyecto): usa 'mem setup-mcp --scope project --agents cursor --target <dir>'
                ✅ hooks de modo plan: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupMCPGlobal_SimbolosSinEmoji2024143160/001/.claude/settings.json
                ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupMCPGlobal_SimbolosSinEmoji2024143160/001/.claude/skills/constitution/SKILL.md
                ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupMCPGlobal_SimbolosSinEmoji2024143160/001/.codex/skills/constitution/SKILL.md
              ✅ 1 registro(s) global(es) completados. Reinicia el agente para que los detecte.
              🔌 Configurando MCP para gomemory en /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupMCPGlobal_SimbolosSinEmoji2024143160/003
                ℹ️  codex: el registro MCP es global; se reutiliza para todos los proyectos
                ✅ codex: ~/.codex/config.toml ya tiene el registro global de gomemory
                ⚠️  Agente desconocido: desconocido (opciones: opencode, claude, cursor, codex, all)
              ✅ 1 configuraciones MCP generadas. Reinicia el agente para que las detecte.
      FAIL
      FAIL	mem/adapters/primary/cli	0.311s
      FAIL
- 2026-11-04 04:45 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestSetupMCPGlobal_SimbolosSinEmoji -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.388s
- 2026-11-04 04:46 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/console -run TestEmojiGuard_Detecta -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.221s
- 2026-11-04 04:46 -05:00 · tarea 4 · rojo observado · caducada · `go test ./adapters/primary/console -run 'TestSinEmojis/(cli|setup|console)' -count=1` (salida 1)
      --- FAIL: TestSinEmojis (0.01s)
          --- FAIL: TestSinEmojis/cli (0.01s)
              emoji_guard_test.go:45: ../cli/cmd_update.go:161: emoji "️"; usa console.Mark o el glifo de stateGlyph
          --- FAIL: TestSinEmojis/console (0.00s)
              emoji_guard_test.go:45: ../console/layout.go:224: emoji "✅"; usa console.Mark o el glifo de stateGlyph
      FAIL
      FAIL	mem/adapters/primary/console	0.168s
      FAIL
- 2026-11-04 04:49 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/primary/console -run 'TestSinEmojis/(cli|setup|console)' -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.196s
- 2026-11-04 04:49 -05:00 · tarea 5 · rojo observado · caducada · `go test ./adapters/primary/console ./adapters/primary/tui -run 'TestSinEmojis/tui|TestTUI_ErrorUsaMarcaDeConsole' -count=1` (salida 1)
                 ─────────────────────────────────────────────────────                                                                                                                                                                                                                                                                            
                  tab cambiar campo  ·  enter calcular  ·  esc volver                                                                                                                                                                                                                                                                            
          hcd022_symbols_test.go:35: preferencia: "♥", se esperaba ⚑
          hcd022_symbols_test.go:38: bugfix: "✕", se esperaba ✗
      FAIL
      FAIL	mem/adapters/primary/tui	0.325s
      FAIL
- 2026-11-04 04:50 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/console ./adapters/primary/tui -run 'TestSinEmojis/tui|TestTUI_ErrorUsaMarcaDeConsole' -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.144s
      ok  	mem/adapters/primary/tui	0.209s
- 2026-11-04 04:50 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/console -run TestLayout_ColoreaTodosLosEstados -count=1` (salida 1)
      --- FAIL: TestLayout_ColoreaTodosLosEstados (0.00s)
          mark_test.go:49: estado 0 (○): Layout no lo pinta con Mark: "\x1b[38;2;34;227;247m  ○ agy: texto de prueba\x1b[0m\n"
          mark_test.go:49: estado 1 (●): Layout no lo pinta con Mark: "\x1b[38;2;34;227;247m  ● agy: texto de prueba\x1b[0m\n"
          mark_test.go:49: estado 2 (✓): Layout no lo pinta con Mark: "  \x1b[38;2;6;193;238m✓\x1b[0m agy: texto de prueba\n"
          mark_test.go:49: estado 3 (⚠): Layout no lo pinta con Mark: "  \x1b[38;2;244;198;107m⚠\x1b[0m agy: texto de prueba\n"
          mark_test.go:49: estado 4 (✗): Layout no lo pinta con Mark: "  \x1b[38;2;255;119;127m✗\x1b[0m agy: texto de prueba\n"
          mark_test.go:49: estado 5 (−): Layout no lo pinta con Mark: "  \x1b[38;2;145;169;197m−\x1b[0m \x1b[38;2;145;169;197magy: texto de prueba\x1b[0m\n"
          mark_test.go:49: estado 6 (▸): Layout no lo pinta con Mark: "\x1b[38;2;34;227;247m  ▸ agy: texto de prueba\x1b[0m\n"
          mark_test.go:49: estado 7 (⚑): Layout no lo pinta con Mark: "\x1b[38;2;34;227;247m  ⚑ agy: texto de prueba\x1b[0m\n"
      FAIL
      FAIL	mem/adapters/primary/console	0.212s
      FAIL
- 2026-11-04 04:51 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/primary/console -run TestLayout_ColoreaTodosLosEstados -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.156s
- 2026-11-04 04:54 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli -run 'TestCicloDeVida_SinFmtPrint|TestSetupMCPGlobal_SimbolosSinEmoji' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.381s
- 2026-11-04 04:54 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/console -run TestMark -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.171s
- 2026-11-04 04:54 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestSetupMCPGlobal_SimbolosSinEmoji -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.304s
- 2026-11-04 04:54 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/console -run TestEmojiGuard_Detecta -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.121s
- 2026-11-04 04:54 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/primary/console -run 'TestSinEmojis/(cli|setup|console)' -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.148s
- 2026-11-04 04:54 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/console ./adapters/primary/tui -run 'TestSinEmojis/tui|TestTUI_ErrorUsaMarcaDeConsole' -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.141s
      ok  	mem/adapters/primary/tui	0.213s
- 2026-11-04 04:54 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/primary/console -run TestLayout_ColoreaTodosLosEstados -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.128s
- 2026-11-04 04:54 -05:00 · tarea 8 · verde · caducada · `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh` (salida 0)
      ok  	mem/adapters/secondary/expandstate	0.293s
      ok  	mem/adapters/secondary/generator	1.275s
      ok  	mem/adapters/secondary/hcdfiles	0.652s
      ok  	mem/adapters/secondary/launcher	0.741s
      ok  	mem/adapters/secondary/persistence	32.344s
      ok  	mem/adapters/secondary/release	0.442s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.212s
      ok  	mem/adapters/secondary/speckit	0.170s
      ok  	mem/adapters/secondary/tokens	0.154s
      ok  	mem/adapters/secondary/usage	0.173s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	13.365s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.217s
      ok  	mem/infrastructure	0.333s
      ok  	mem/tests/contract	88.546s
      ok  	mem/tests/integration	38.649s
      ?   	mem/version	[no test files]
      OK: --uninstall solo retira binarios propios
- 2026-11-04 05:06 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/console -run TestMark -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.420s
- 2026-11-04 05:06 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/primary/cli -run TestSetupMCPGlobal_SimbolosSinEmoji -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.369s
- 2026-11-04 05:06 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/console -run TestEmojiGuard_Detecta -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.135s
- 2026-11-04 05:06 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/primary/console -run 'TestSinEmojis/(cli|setup|console)' -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.148s
- 2026-11-04 05:06 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/console ./adapters/primary/tui -run 'TestSinEmojis/tui|TestTUI_ErrorUsaMarcaDeConsole' -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.134s
      ok  	mem/adapters/primary/tui	0.295s
- 2026-11-04 05:06 -05:00 · tarea 6 · verde · vigente · `go test ./adapters/primary/console -run TestLayout_ColoreaTodosLosEstados -count=1` (salida 0)
      ok  	mem/adapters/primary/console	0.127s
- 2026-11-04 05:06 -05:00 · tarea 7 · verde · vigente · `go test ./adapters/primary/cli -run 'TestCicloDeVida_SinFmtPrint|TestSetupMCPGlobal_SimbolosSinEmoji' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.309s
- 2026-11-04 05:06 -05:00 · tarea 8 · verde · vigente · `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh` (salida 0)
      ok  	mem/adapters/secondary/expandstate	0.340s
      ok  	mem/adapters/secondary/generator	1.251s
      ok  	mem/adapters/secondary/hcdfiles	0.671s
      ok  	mem/adapters/secondary/launcher	0.790s
      ok  	mem/adapters/secondary/persistence	31.913s
      ok  	mem/adapters/secondary/release	0.406s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.236s
      ok  	mem/adapters/secondary/speckit	0.194s
      ok  	mem/adapters/secondary/tokens	0.162s
      ok  	mem/adapters/secondary/usage	0.164s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	12.217s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.226s
      ok  	mem/infrastructure	0.372s
      ok  	mem/tests/contract	86.197s
      ok  	mem/tests/integration	37.123s
      ?   	mem/version	[no test files]
      OK: --uninstall solo retira binarios propios

## Siguiente
Cerrada con aprobación sobre la revisión 92070b1b3cd4.
