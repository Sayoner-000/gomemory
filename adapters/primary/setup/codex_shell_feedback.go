package setup

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// CodexShellFeedbackHook usa el borde observado en HCD-014, separado del
// antiguo dialecto MCP. El ajuste del proyecto se comprueba en cada ejecución.
var CodexShellFeedbackHook = CodexHook{Event: "PostToolUse", Matcher: "^Bash$", Sub: "tool-output codex-shell"}

// CodexShellFeedbackVersionSupported no extrapola la sonda de una versión a
// otras. Ampliar esta lista requiere comprobar el contrato del host nuevo.
func CodexShellFeedbackVersionSupported(version string) bool {
	return strings.TrimSpace(version) == "codex-cli 0.166.0"
}

// CodexShellFeedbackSupported se usa tanto al registrar como al ejecutar el
// hook: una actualización del host no conserva una afirmación de compatibilidad
// que solo se comprobó en la versión anterior. No invoca al modelo ni la red.
func CodexShellFeedbackSupported() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "codex", "--version").Output()
	return err == nil && CodexShellFeedbackVersionSupported(string(out))
}
