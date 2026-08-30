package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestParseHookPayload_PlainTextEquivalesAPlanField cubre
// contracts/agent-integration.md «Entrada»: texto plano por stdin equivale a
// {"plan":"…"} — el motor de decisión no debe distinguir entre ambas formas.
func TestParseHookPayload_PlainTextEquivalesAPlanField(t *testing.T) {
	_, planFromJSON := parseHookPayload([]byte(`{"plan":"Texto del plan de prueba."}`))
	_, planFromText := parseHookPayload([]byte("Texto del plan de prueba."))

	if planFromJSON != planFromText {
		t.Errorf("texto plano y {plan:...} deben producir el mismo texto de plan: %q vs %q", planFromText, planFromJSON)
	}
	if planFromJSON != "Texto del plan de prueba." {
		t.Errorf("plan extraído = %q, se esperaba el texto literal", planFromJSON)
	}
}

// TestParseHookPayload_ClaudeEnvelope cubre la envoltura real de Claude Code
// (verificada en vivo, feature 019 T001-T004): el plan viaja en
// tool_input.plan, no en un campo de nivel superior.
func TestParseHookPayload_ClaudeEnvelope(t *testing.T) {
	raw := `{"hook_event_name":"PreToolUse","tool_name":"ExitPlanMode","tool_input":{"plan":"plan real","planFilePath":"/tmp/x.md"}}`
	payload, plan := parseHookPayload([]byte(raw))

	if plan != "plan real" {
		t.Errorf("plan = %q, se esperaba %q", plan, "plan real")
	}
	if payload == nil {
		t.Fatal("el payload no debe ser nil para JSON válido")
	}
}

// TestParseHookPayload_EmptyInput cubre el caso degenerado: sin nada que
// evaluar, el plan queda vacío y el payload nil, sin pánico.
func TestParseHookPayload_EmptyInput(t *testing.T) {
	payload, plan := parseHookPayload([]byte("   \n"))
	if plan != "" || payload != nil {
		t.Errorf("entrada vacía debe producir plan vacío y payload nil, got plan=%q payload=%v", plan, payload)
	}
}

// TestDetectDialect_SinEnvolturaReconocible cubre el default: sin señales de
// ningún agente concreto, el dialecto es neutral (INV-6 — nunca el de un
// agente concreto por omisión).
func TestDetectDialect_SinEnvolturaReconocible(t *testing.T) {
	got := detectDialect(map[string]any{"plan": "x"}, "")
	if got != dialectNeutral {
		t.Errorf("dialecto = %q, se esperaba %q", got, dialectNeutral)
	}

	got = detectDialect(nil, "")
	if got != dialectNeutral {
		t.Errorf("payload nil debe resolver a neutral, got %q", got)
	}
}

// TestDetectDialect_EnvolturaDeClaudeCode cubre la detección automática por
// la forma del payload: presencia de hook_event_name + tool_name (la
// envoltura real observada en vivo) selecciona el dialecto claude.
func TestDetectDialect_EnvolturaDeClaudeCode(t *testing.T) {
	payload := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "ExitPlanMode",
		"tool_input":      map[string]any{"plan": "x"},
	}
	got := detectDialect(payload, "")
	if got != dialectClaude {
		t.Errorf("dialecto = %q, se esperaba %q", got, dialectClaude)
	}
}

// TestDetectDialect_EmitFuerzaElDialecto cubre --emit: fuerza cualquiera de
// los cuatro dialectos, incluso contra una envoltura que sugeriría otro.
func TestDetectDialect_EmitFuerzaElDialecto(t *testing.T) {
	claudePayload := map[string]any{"hook_event_name": "PreToolUse", "tool_name": "ExitPlanMode"}

	cases := []struct {
		forced string
		want   hookDialect
	}{
		{"neutral", dialectNeutral},
		{"json", dialectJSON},
		{"claude", dialectClaude},
		{"text", dialectText},
	}
	for _, c := range cases {
		if got := detectDialect(claudePayload, c.forced); got != c.want {
			t.Errorf("--emit=%s: dialecto = %q, se esperaba %q", c.forced, got, c.want)
		}
	}
}

// TestDetectDialect_EmitInvalidoSeIgnora cubre robustez: un valor de --emit
// que no es ninguno de los cuatro dialectos no debe colarse; se sigue la
// detección automática (y, en última instancia, neutral).
func TestDetectDialect_EmitInvalidoSeIgnora(t *testing.T) {
	got := detectDialect(nil, "un-dialecto-inventado")
	if got != dialectNeutral {
		t.Errorf("un --emit inválido debe degradar a neutral, got %q", got)
	}
}

// TestRenderGuardDecision_LasCuatroFormasParaElMismoVeredicto cubre el
// requisito central del contrato: el mismo veredicto (permitir o devolver)
// produce las cuatro formas documentadas en contracts/hook-plan-guard.md,
// sin que el motor de decisión sepa nada de agentes.
func TestRenderGuardDecision_LasCuatroFormasParaElMismoVeredicto(t *testing.T) {
	deny := planGuardDecision{deny: true, reason: "falta el árbol de tareas"}
	permit := planGuardDecision{deny: false}

	t.Run("neutral deny: código != 0, motivo por stderr, stdout vacío", func(t *testing.T) {
		out := renderGuardDecision(dialectNeutral, deny)
		if out.exitCode == 0 {
			t.Error("neutral+deny debe salir con código != 0")
		}
		if out.stdout != "" {
			t.Errorf("neutral+deny no debe escribir stdout, got %q", out.stdout)
		}
		if !strings.Contains(out.stderr, "falta el árbol de tareas") {
			t.Errorf("neutral+deny debe llevar el motivo por stderr, got %q", out.stderr)
		}
	})

	t.Run("neutral permit: código 0, sin salida", func(t *testing.T) {
		out := renderGuardDecision(dialectNeutral, permit)
		if out.exitCode != 0 || out.stdout != "" || out.stderr != "" {
			t.Errorf("neutral+permit debe ser silencioso y exit 0, got %+v", out)
		}
	})

	t.Run("claude deny: JSON con permissionDecision deny, exit 0", func(t *testing.T) {
		out := renderGuardDecision(dialectClaude, deny)
		if out.exitCode != 0 {
			t.Error("el dialecto claude transporta la decisión en la salida, no en el código")
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(out.stdout), &parsed); err != nil {
			t.Fatalf("stdout no es JSON válido: %v (%q)", err, out.stdout)
		}
		hso, _ := parsed["hookSpecificOutput"].(map[string]any)
		if hso == nil {
			t.Fatal("falta hookSpecificOutput")
		}
		if hso["permissionDecision"] != "deny" {
			t.Errorf("permissionDecision = %v, se esperaba deny", hso["permissionDecision"])
		}
		if hso["permissionDecisionReason"] != deny.reason {
			t.Errorf("permissionDecisionReason = %v, se esperaba %q", hso["permissionDecisionReason"], deny.reason)
		}
	})

	t.Run("claude permit: objeto vacío, exit 0", func(t *testing.T) {
		out := renderGuardDecision(dialectClaude, permit)
		if out.exitCode != 0 {
			t.Error("claude+permit debe salir con código 0")
		}
		if strings.TrimSpace(out.stdout) != "{}" {
			t.Errorf("claude+permit debe emitir {}, got %q", out.stdout)
		}
	})

	t.Run("json deny: decision deny + reason, exit 0", func(t *testing.T) {
		out := renderGuardDecision(dialectJSON, deny)
		if out.exitCode != 0 {
			t.Error("el dialecto json transporta la decisión en la salida, no en el código")
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(out.stdout), &parsed); err != nil {
			t.Fatalf("stdout no es JSON válido: %v", err)
		}
		if parsed["decision"] != "deny" {
			t.Errorf("decision = %v, se esperaba deny", parsed["decision"])
		}
		if parsed["reason"] != deny.reason {
			t.Errorf("reason = %v, se esperaba %q", parsed["reason"], deny.reason)
		}
	})

	t.Run("json permit: decision allow, exit 0", func(t *testing.T) {
		out := renderGuardDecision(dialectJSON, permit)
		var parsed map[string]any
		if err := json.Unmarshal([]byte(out.stdout), &parsed); err != nil {
			t.Fatalf("stdout no es JSON válido: %v", err)
		}
		if parsed["decision"] != "allow" {
			t.Errorf("decision = %v, se esperaba allow", parsed["decision"])
		}
	})

	t.Run("text deny: motivo por stdout, exit 0", func(t *testing.T) {
		out := renderGuardDecision(dialectText, deny)
		if out.exitCode != 0 {
			t.Error("el dialecto text transporta la decisión en la salida, no en el código")
		}
		if out.stdout != deny.reason {
			t.Errorf("stdout = %q, se esperaba el motivo literal %q", out.stdout, deny.reason)
		}
	})

	t.Run("text permit: sin salida, exit 0", func(t *testing.T) {
		out := renderGuardDecision(dialectText, permit)
		if out.exitCode != 0 || out.stdout != "" {
			t.Errorf("text+permit debe ser silencioso y exit 0, got %+v", out)
		}
	})
}

// TestRenderTurnEndPorDialecto fija las dos propiedades que el fallo de campo
// destapó: en dialectos planos NO puede salir el sobre de Claude Code, y el
// silencio es la cadena vacía y no `{}` —quien lee stdout como contexto se
// tragaría esas dos llaves como si fueran una instrucción—.
func TestRenderTurnEndPorDialecto(t *testing.T) {
	const aviso = "considera compactar"
	const refuerzo = "recuerda las preferencias"

	t.Run("claude envuelve segun el destinatario", func(t *testing.T) {
		humano := renderTurnEnd(dialectClaude, aviso, true)
		if !strings.Contains(humano, `"systemMessage"`) {
			t.Fatalf("el aviso al humano debe ir en systemMessage: %s", humano)
		}
		modelo := renderTurnEnd(dialectClaude, refuerzo, false)
		if !strings.Contains(modelo, `"hookEventName":"Stop"`) ||
			!strings.Contains(modelo, `"additionalContext"`) {
			t.Fatalf("el refuerzo al modelo debe ir en additionalContext de Stop: %s", modelo)
		}
	})

	t.Run("los dialectos planos no emiten envoltura", func(t *testing.T) {
		for _, d := range []hookDialect{dialectText, dialectNeutral} {
			for _, paraElHumano := range []bool{true, false} {
				got := renderTurnEnd(d, refuerzo, paraElHumano)
				if got != refuerzo {
					t.Fatalf("dialecto %q: se esperaba texto desnudo, salió %q", d, got)
				}
			}
		}
	})

	t.Run("el silencio nunca imprime llaves", func(t *testing.T) {
		for _, d := range []hookDialect{dialectClaude, dialectJSON, dialectText, dialectNeutral} {
			if got := renderTurnEnd(d, "", false); got != "" {
				t.Fatalf("dialecto %q: el silencio salió como %q", d, got)
			}
		}
	})
}
