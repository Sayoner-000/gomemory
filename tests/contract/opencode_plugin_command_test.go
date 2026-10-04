package main

import (
	"strings"
	"testing"
)

// T078 (feature 035, FR-017/FR-027): el plugin de OpenCode pasa el id de su
// sesión como conversación y el comando de bash al hook de salidas, para que
// las exclusiones por lectura exacta (sed, cat…) apliquen también allí.
func TestOpenCodePlugin_ConversacionYComando(t *testing.T) {
	src := gomemoryPluginSource(t)
	if !strings.Contains(src, "`--conversation=${conversationID}`") {
		t.Error("session.created debe pasar --conversation con el id de la sesión de OpenCode")
	}
	if !strings.Contains(src, `JSON.stringify({ tool: input.tool, output: output.output, command })`) {
		t.Error("tool.execute.after debe enviar el comando a mem hook tool-output opencode")
	}
}
