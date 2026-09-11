package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// gomemoryPluginSource lee el complemento de OpenCode como texto plano. Mismo
// patrón que TestOpenCode_ObtieneLaPoliticaOctopusDelGeneradorComun.
func gomemoryPluginSource(t *testing.T) string {
	t.Helper()
	datos, err := os.ReadFile("../../infrastructure/plugin/opencode/gomemory.ts")
	if err != nil {
		t.Fatalf("leer el complemento: %v", err)
	}
	return string(datos)
}

// blockBetween extrae el bloque de texto entre la primera aparición de start y
// la siguiente clave de nivel superior del objeto (línea que empieza, con 4
// espacios de indentación, por una comilla — el mismo patrón que usa
// hooksQueRegistraElComplemento en opencode_hooks_test.go). Sirve para acotar
// las aserciones a UN handler concreto, no a todo el archivo.
func blockBetween(t *testing.T, texto, start string) string {
	t.Helper()
	i := strings.Index(texto, start)
	if i < 0 {
		t.Fatalf("no se encontró %q en el complemento", start)
	}
	resto := texto[i+len(start):]
	// Siguiente clave de nivel superior (4 espacios + comilla) o cierre del
	// objeto de retorno.
	fin := len(resto)
	for _, marker := range []string{"\n    \"", "\n  };"} {
		if j := strings.Index(resto, marker); j >= 0 && j < fin {
			fin = j
		}
	}
	return start + resto[:fin]
}

// TestOpenCodeCompaction_SessionCompactingPideCompactionContext cubre US1
// (feature 030, capacidad C1): antes de compactar, el complemento debe pedir
// `mem hook compaction-context` (memoria de LA SESIÓN, acotada), no
// `mem hook post-compact` (que resetea huella/marcadores y abre sesión — un
// efecto que no debe ocurrir antes de que la compactación exista).
func TestOpenCodeCompaction_SessionCompactingPideCompactionContext(t *testing.T) {
	texto := gomemoryPluginSource(t)
	bloque := blockBetween(t, texto, `"experimental.session.compacting"`)

	if !strings.Contains(bloque, `["hook", "compaction-context"]`) {
		t.Errorf("experimental.session.compacting debe pedir compaction-context: %s", bloque)
	}
	if strings.Contains(bloque, `"post-compact"`) {
		t.Errorf("experimental.session.compacting NO debe llamar a post-compact (eso ocurre DESPUÉS de compactar, en session.compacted): %s", bloque)
	}
}

// TestOpenCodeCompaction_SessionCompactedLlamaPostCompact cubre la capacidad
// C2: tras compactar, el complemento debe invocar `mem hook post-compact`
// (que sí resetea huella/marcadores y garantiza una sesión activa) desde el
// evento posterior a la compactación, no desde el previo.
func TestOpenCodeCompaction_SessionCompactedLlamaPostCompact(t *testing.T) {
	texto := gomemoryPluginSource(t)
	bloque := blockBetween(t, texto, `event: async`)

	if !strings.Contains(bloque, `"session.compacted"`) {
		t.Fatalf("el manejador de eventos debe reconocer session.compacted: %s", bloque)
	}
	if !strings.Contains(bloque, `["hook", "post-compact"]`) {
		t.Errorf("session.compacted debe invocar post-compact: %s", bloque)
	}
}

// TestOpenCodeCompaction_SessionCompactedEnviaElResumenAlHook cubre US2
// (feature 030, capacidad C6): tras compactar, el complemento debe buscar el
// mensaje de resumen (info.summary === true) en client.session.messages y
// enviar su texto a `mem hook compact-summary` por stdin, para que quede
// persistido sin acción del agente.
func TestOpenCodeCompaction_SessionCompactedEnviaElResumenAlHook(t *testing.T) {
	texto := gomemoryPluginSource(t)
	bloque := blockBetween(t, texto, `event: async`)

	if !strings.Contains(bloque, "summary === true") && !strings.Contains(bloque, "summary: true") {
		t.Errorf("debe buscar el mensaje marcado como resumen (info.summary): %s", bloque)
	}
	if !strings.Contains(bloque, `["hook", "compact-summary"]`) {
		t.Errorf("debe enviar el texto del resumen a compact-summary: %s", bloque)
	}
}

// TestOpenCodeCompaction_SystemTransformPideAgentNotice cubre US4 (feature
// 030, capacidad C4): junto a la llamada vigente a `mem hook nudge`,
// system.transform debe pedir el aviso pendiente de US4 (`mem hook
// agent-notice`) e inyectarlo cuando no esté vacío.
func TestOpenCodeCompaction_SystemTransformPideAgentNotice(t *testing.T) {
	texto := gomemoryPluginSource(t)
	bloque := blockBetween(t, texto, `"experimental.chat.system.transform"`)

	if !strings.Contains(bloque, `["hook", "agent-notice"]`) {
		t.Errorf("system.transform debe pedir el aviso de US4 con agent-notice: %s", bloque)
	}
}

// TestOpenCodeCompaction_ToolExecuteAfterEnviaSubagentStop cubre US3 (feature
// 030, capacidad C3): al terminar una tarea delegada, el complemento debe
// enviar el texto de salida a `mem hook subagent-stop` para la captura pasiva
// de aprendizajes.
//
// ⚠ El identificador exacto de la herramienta de delegación en OpenCode
// ("task") no se confirmó en sesión interactiva — ver quickstart.md Q0 de la
// feature 030.
func TestOpenCodeCompaction_ToolExecuteAfterEnviaSubagentStop(t *testing.T) {
	texto := gomemoryPluginSource(t)
	bloque := blockBetween(t, texto, `"tool.execute.after"`)

	// Exige la comparación sobre input.tool (=== o su guarda !==), no una
	// mención suelta de "task" en un comentario o un nombre de variable.
	if !regexp.MustCompile(`input\.tool\s*[!=]==\s*"task"`).MatchString(bloque) {
		t.Errorf(`tool.execute.after debe comparar input.tool con "task": %s`, bloque)
	}
	if !strings.Contains(bloque, `["hook", "subagent-stop"]`) {
		t.Errorf("debe enviar el resultado a subagent-stop: %s", bloque)
	}
	if !strings.Contains(bloque, "last_assistant_message") {
		t.Errorf("debe enviar el campo last_assistant_message (mismo contrato que Claude Code): %s", bloque)
	}
}

// TestOpenCodeCompaction_SystemTransformInyectaLaRecuperacionPendiente cubre
// que el resultado de post-compact (capturado en session.compacted, que no
// tiene forma de inyectar contexto directamente) se entregue al agente en el
// PRÓXIMO system.transform — el único canal de OpenCode que sí llega al
// modelo antes de su respuesta.
func TestOpenCodeCompaction_SystemTransformInyectaLaRecuperacionPendiente(t *testing.T) {
	texto := gomemoryPluginSource(t)
	bloque := blockBetween(t, texto, `"experimental.chat.system.transform"`)

	if !strings.Contains(bloque, "pendingRecovery") {
		t.Errorf("system.transform debe consumir la recuperación pendiente de session.compacted: %s", bloque)
	}
}
