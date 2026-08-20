# Plan: migración bubbletea v1.3.10 → v2.0.9 (+ bubbles v2.1.1, lipgloss v2.0.6)

Ver plan completo: /Users/josegomezj/.claude/plans/generic-growing-valley.md

## Estado

- [x] Comparación de patrones de TUI gomemory vs engram (Parte 1 del plan) — documentada, sin cambios de código requeridos
- [x] Separar estilos de tui.go en styles.go propio (mejora opcional, pedida explícitamente por el usuario a mitad de turno)
- [x] Tarea 1.1: go.mod/go.sum a bubbletea v2.0.9 + bubbles v2.1.1 + lipgloss v2.0.6 (+ compat v2.0.6) — `go mod tidy` limpio, cero rastro de v1
- [x] Tarea 1.2/3.1: helper `keyMsg` unificado en tui_usage_test.go, reescrito al modelo tea.Key de v2, aplicado a los 37 literales `tea.KeyMsg{Type: tea.KeyXxx}` que había en tui_test.go
- [x] Tarea 2.1: imports migrados a las 4 rutas /v2 (tui.go, tui_usage.go, tui_test.go, tui_usage_test.go, styles.go)
- [x] Tarea 2.2: Run() — tea.WithAltScreen() movido al campo AltScreen del tea.View
- [x] Tarea 2.3: View() reescrito a `func (m model) View() tea.View`; toda la lógica de switch por pantalla se movió intacta a un nuevo `renderView() string` interno
- [x] Tarea 3.2: bug real corregido — `case " ":` (tui.go, toggle de exclusión en duplicados) → `case "space":`, confirmado con TestOptimizeDetail_SpaceExcludesFromDeletion en verde
- [x] Tarea 3.3: switches de update* verificados vía suite completa en verde (msg.String() sigue siendo estable en v2)
- [x] Tarea 4.1: 11 lipgloss.AdaptiveColor → compat.AdaptiveColor (charm.land/lipgloss/v2/compat), colores envueltos con lipgloss.Color(...)
- [x] Tarea 4.2: tests de layout (TestListFitsTerminalHeight, TestBodyBudgetCalculations, etc.) — pasan sin tocar constantes. Se encontró y corrigió un problema real de test: lipgloss v2 Style.Render() ya NO degrada color por perfil de terminal en la propia cadena (eso ahora ocurre en el renderer real de tea.Program) — dos asserts en TestConfigScreen_MuestraInterruptorDePlanificacionAtomica comparaban substrings coloreados; se corrigieron con `ansi.Strip()` antes del Contains, no tocando el código de producción
- [x] Tarea 5.1: 11 instancias de textinput.Model — `.Width = N` (campo público en v1) → `.SetWidth(N)` (método en v2), 12 sitios entre tui.go y tui_test.go
- [x] Tarea 6.1: `go build ./...`, `go vet ./...`, `go test ./...` en verde (todo el módulo, no solo tui)
- [~] Tarea 6.2: recorrido manual de la TUI real en pty — confirmado en vivo: pantalla list (bordes redondeados, ayuda de teclas), pantalla usage (`u`) con placeholders de textinput visibles, esc de vuelta a list, quit limpio (exit 0, sin panic). **Pendiente de confirmar**: la pantalla de guardado (`s`) no se capturó de forma concluyente en el harness de pty (puede ser timing/buffering del harness, no necesariamente un bug real — "u" con el mismo mecanismo de despacho sí funcionó) — repetir con más margen de tiempo entre keystrokes antes de dar la migración por 100% verificada en vivo

## Cambios realizados (archivos)

- `adapters/primary/tui/styles.go` (nuevo): paleta de colores + typeColor/typeIcon/typeLabel + estilos lipgloss, extraídos de tui.go
- `adapters/primary/tui/tui.go`: imports /v2, Run() sin WithAltScreen, View()/renderView() separados, fix `case "space":`, 11× `.SetWidth()`
- `adapters/primary/tui/tui_usage.go`: import /v2
- `adapters/primary/tui/tui_test.go`: imports /v2 (sin `tea` directo, todo vía `keyMsg`), 37 literales migrados, 1× `.SetWidth()`, 2 asserts con `ansi.Strip()`
- `adapters/primary/tui/tui_usage_test.go`: imports /v2, helper `keyMsg` reescrito a `tea.KeyPressMsg{Code:...}`
- `go.mod`/`go.sum`: bubbletea/bubbles/lipgloss v1 → v2 (vía `go mod tidy`)

## Notas

- Blast radius confirmado acotado a `adapters/primary/tui/` — ningún otro paquete del repo importa bubbletea/bubbles/lipgloss.
- No se ha hecho commit todavía — pendiente de confirmación explícita del usuario antes de `git commit`/`git push` (regla de CLAUDE.md).
- Bump también corrigió un hallazgo lateral: separación de estilos en archivo propio (`styles.go`), siguiendo el patrón que usa engram — mejora de legibilidad sin riesgo funcional.
