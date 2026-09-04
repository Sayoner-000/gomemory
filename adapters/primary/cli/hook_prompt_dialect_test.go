package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/primary/setup"
)

// TestRenderPromptContext_TextoPlanoParaAgentesNoClaude es el contrato que hace
// utilizable la inyección por turno fuera de Claude Code.
//
// Codex toma el stdout del hook COMO CONTEXTO TAL CUAL (verificado en sesión
// real: el transcript muestra «hook context: <stdout>» y el modelo lo lee). Si
// se le entrega el JSON de Claude, el agente recibe la envoltura como texto y
// el recordatorio queda enterrado en ruido sintáctico — un canal que se
// instala, no falla, y no sirve.
func TestRenderPromptContext_TextoPlanoParaAgentesNoClaude(t *testing.T) {
	const recordatorio = "Llama a get_plan_context() antes de redactar el plan."

	for _, d := range []hookDialect{dialectText, dialectNeutral} {
		out := renderPromptContext(d, recordatorio)
		if out.stdout != recordatorio {
			t.Errorf("dialecto %q: stdout = %q, se esperaba el texto desnudo", d, out.stdout)
		}
		if strings.Contains(out.stdout, "hookSpecificOutput") {
			t.Errorf("dialecto %q: filtró la envoltura JSON de Claude Code", d)
		}
	}
}

// TestRenderPromptContext_ClaudeConservaSuEnvoltura: el dialecto por defecto no
// cambia. Claude Code solo inyecta lo que venga en additionalContext.
func TestRenderPromptContext_ClaudeConservaSuEnvoltura(t *testing.T) {
	out := renderPromptContext(dialectClaude, "recordatorio")

	var payload struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out.stdout), &payload); err != nil {
		t.Fatalf("la salida de Claude no es JSON válido: %v (%q)", err, out.stdout)
	}
	if payload.HookSpecificOutput.HookEventName != "UserPromptSubmit" {
		t.Errorf("hookEventName = %q, se esperaba UserPromptSubmit", payload.HookSpecificOutput.HookEventName)
	}
	if payload.HookSpecificOutput.AdditionalContext != "recordatorio" {
		t.Errorf("additionalContext = %q", payload.HookSpecificOutput.AdditionalContext)
	}
}

// TestRenderPromptContext_SilencioPorDialecto: sin nada que decir, cada agente
// necesita su forma de "nada". Para Claude es `{}`; para quien lee stdout como
// contexto, la cadena vacía — imprimir `{}` le inyectaría esas dos llaves como
// si fueran una instrucción.
func TestRenderPromptContext_SilencioPorDialecto(t *testing.T) {
	if out := renderPromptContext(dialectClaude, ""); out.stdout != "{}" {
		t.Errorf("silencio en Claude = %q, se esperaba {}", out.stdout)
	}
	for _, d := range []hookDialect{dialectText, dialectNeutral} {
		if out := renderPromptContext(d, ""); out.stdout != "" {
			t.Errorf("silencio en %q = %q, se esperaba cadena vacía", d, out.stdout)
		}
	}
}

// TestRenderPromptContext_ElDefectoEsClaudeNoElNeutral fija el defecto que
// rompí al añadir el dialecto y que atraparon los tests de integración.
//
// `detectDialect` devuelve neutral cuando no se fuerza nada, y usarlo aquí sin
// más dejaba a Claude Code —que invoca este hook SIN bandera— recibiendo texto
// plano donde espera JSON: su inyección por turno moría en silencio, porque un
// hook que imprime algo no parece roto. Codex pide --emit=text explícitamente.
func TestRenderPromptContext_ElDefectoEsClaudeNoElNeutral(t *testing.T) {
	if got := dialectoDePrompt(nil); got != dialectClaude {
		t.Errorf("sin argumentos el dialecto es %q; debe ser claude", got)
	}
	if got := dialectoDePrompt([]string{"--emit=text"}); got != dialectText {
		t.Errorf("con --emit=text el dialecto es %q", got)
	}
	if got := dialectoDePrompt([]string{"--emit=disparate"}); got != dialectClaude {
		t.Errorf("un dialecto inválido debe caer al defecto de Claude, no a %q", got)
	}
}

// dialectoDePrompt replica la resolución de hookUserPromptSubmit. Se mantiene
// junto a su test para que un cambio en aquella que no pase por aquí falle.
func dialectoDePrompt(args []string) hookDialect {
	d := dialectClaude
	if v := emitFlagValue(args); isKnownDialect(v) {
		d = hookDialect(v)
	}
	return d
}

// TestCodexHooks_IncluyenInyeccionPorTurno cubre el hueco que cerró la
// verificación en vivo: Codex SÍ dispara UserPromptSubmit. Su stdout se valida
// como JSON de hook, por lo que limitarlo a SessionStart y Stop lo dejaba con
// el protocolo solo en un archivo estático que se diluye con la conversación.
func TestCodexHooks_IncluyenInyeccionPorTurno(t *testing.T) {
	var encontrado *setup.CodexHook
	for _, h := range setup.CodexGomemoryHooks() {
		if h.Event == "UserPromptSubmit" {
			hook := h
			encontrado = &hook
		}
	}
	if encontrado == nil {
		t.Fatal("Codex no declara ningún hook UserPromptSubmit: sigue sin inyección por turno")
	}
	if encontrado.Sub != "user-prompt-submit" {
		t.Errorf("Sub = %q, se esperaba user-prompt-submit", encontrado.Sub)
	}

	// Codex necesita JSON incluso cuando este hook no tenga contexto que
	// inyectar; el adaptador lo declara explícitamente para no depender de un
	// dialecto por defecto.
	cmd, _ := setup.CodexHookGroup(*encontrado, "mem")["hooks"].([]any)
	if len(cmd) == 0 {
		t.Fatal("el grupo TOML no declara ningún comando")
	}
	primero, _ := cmd[0].(map[string]any)
	linea, _ := primero["command"].(string)
	if !strings.Contains(linea, "--emit=json") {
		t.Errorf("el comando %q debe declarar JSON para Codex", linea)
	}
}

// TestAgentesPorDefecto_IncluyenCodex: `mem setup-mcp --scope global` sin
// argumentos no registraba Codex pese a que globalScopeAgents sí lo declara.
// Quien no supiera pasar `--agents all` se quedaba sin su ciclo de memoria
// entero — sin que nada fallara ni avisara.
func TestAgentesPorDefecto_IncluyenCodex(t *testing.T) {
	for _, esperado := range []string{"opencode", "claude", "codex"} {
		if !strings.Contains(defaultAgentList, esperado) {
			t.Errorf("el valor por defecto de --agents (%q) no incluye %q", defaultAgentList, esperado)
		}
	}
	// Todo agente del defecto debe poder registrarse en ámbito global; si no,
	// el defecto promete algo que el flujo no cumple.
	for _, a := range strings.Split(defaultAgentList, ",") {
		if !globalScopeAgents[strings.TrimSpace(a)] {
			t.Errorf("el defecto incluye %q, que no está en globalScopeAgents", a)
		}
	}
}
