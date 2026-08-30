package cli

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// hookDialect es la traducción de salida del contrato neutral
// (contracts/agent-integration.md) para un agente concreto. Vive en el
// adaptador, NUNCA en domain/: si un dialecto concreto se filtrara al motor
// de decisión, el siguiente agente heredaría la forma del anterior — la
// asimetría exacta que la feature 019 corrige (plan.md, Structure Decision).
type hookDialect string

const (
	// dialectNeutral es el default y el que manda ante cualquier duda
	// (INV-6): nunca se responde en el dialecto de un agente concreto sin
	// una señal explícita de que corresponde.
	dialectNeutral hookDialect = "neutral"
	dialectJSON    hookDialect = "json"
	dialectClaude  hookDialect = "claude"
	dialectText    hookDialect = "text"
)

// isKnownDialect reporta si s nombra uno de los cuatro dialectos del
// contrato, para distinguir un --emit válido de uno inventado.
func isKnownDialect(s string) bool {
	switch hookDialect(s) {
	case dialectNeutral, dialectJSON, dialectClaude, dialectText:
		return true
	}
	return false
}

// parseHookPayload interpreta el stdin crudo de un hook según
// contracts/agent-integration.md «Entrada»: JSON con tool_input.plan
// (envoltura de Claude Code, verificada en vivo en T001-T004), JSON con
// `plan` de nivel superior (forma mínima neutral), o texto plano — si el
// contenido no es JSON, se toma íntegro como el plan. Nunca falla: ante
// cualquier forma inesperada, plan queda vacío y payload nil.
func parseHookPayload(raw []byte) (payload map[string]any, plan string) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, ""
	}

	var p map[string]any
	if err := json.Unmarshal(trimmed, &p); err != nil {
		// No es JSON: se toma como texto plano del plan (equivalente a
		// {"plan": "..."} por contrato).
		return nil, string(trimmed)
	}

	if ti, ok := p["tool_input"].(map[string]any); ok {
		if s, ok := ti["plan"].(string); ok {
			return p, s
		}
	}
	if s, ok := p["plan"].(string); ok {
		return p, s
	}
	return p, ""
}

// detectDialect determina el dialecto de un payload de hook. forced (de
// --emit) tiene prioridad absoluta si nombra uno de los cuatro dialectos
// conocidos; en cualquier otro caso se detecta por la forma del payload:
// la presencia conjunta de hook_event_name + tool_name es la envoltura real
// de Claude Code (confirmada en vivo en T001-T004). Sin esa señal, neutral.
func detectDialect(payload map[string]any, forced string) hookDialect {
	if isKnownDialect(forced) {
		return hookDialect(forced)
	}
	if payload != nil {
		_, hasEvent := payload["hook_event_name"]
		_, hasTool := payload["tool_name"]
		if hasEvent && hasTool {
			return dialectClaude
		}
	}
	return dialectNeutral
}

// planGuardDecision es el veredicto del motor de decisión de plan-guard
// (domain.EvaluatePlanShape + estado de episodio), independiente de
// cualquier agente o dialecto.
type planGuardDecision struct {
	deny   bool
	reason string
}

// hookRenderedOutput es el resultado ya traducido a un dialecto concreto:
// qué escribir en stdout, qué escribir en stderr, y con qué código salir.
type hookRenderedOutput struct {
	stdout   string
	stderr   string
	exitCode int
}

// renderGuardDecision traduce un veredicto de plan-guard al dialecto d, según
// las cuatro formas documentadas en contracts/hook-plan-guard.md. El código
// de salida es 0 en todos los dialectos que transportan la decisión en la
// salida (claude, json, text); solo en neutral el código ES el vehículo de
// la decisión por contrato (contracts/agent-integration.md, "Nivel 1").
func renderGuardDecision(d hookDialect, decision planGuardDecision) hookRenderedOutput {
	switch d {
	case dialectClaude:
		if !decision.deny {
			return hookRenderedOutput{stdout: "{}"}
		}
		out, _ := json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "deny",
				"permissionDecisionReason": decision.reason,
			},
		})
		return hookRenderedOutput{stdout: string(out)}

	case dialectJSON:
		if !decision.deny {
			out, _ := json.Marshal(map[string]any{"decision": "allow"})
			return hookRenderedOutput{stdout: string(out)}
		}
		out, _ := json.Marshal(map[string]any{"decision": "deny", "reason": decision.reason})
		return hookRenderedOutput{stdout: string(out)}

	case dialectText:
		if !decision.deny {
			return hookRenderedOutput{}
		}
		return hookRenderedOutput{stdout: decision.reason}

	default: // dialectNeutral
		if !decision.deny {
			return hookRenderedOutput{}
		}
		return hookRenderedOutput{stderr: decision.reason, exitCode: 1}
	}
}

// emitFlagValue extrae el valor de --emit=<dialecto> de args, sin depender
// del paquete flag (los subcomandos de hook reciben args crudos desde
// CmdHook). Devuelve "" si no está presente.
func emitFlagValue(args []string) string {
	const prefix = "--emit="
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix)
		}
	}
	return ""
}

// budgetFlagValue extrae el valor de --budget=<n> de args (contracts/agent-integration.md,
// «Nivel 2»). Devuelve 0 si no está presente o no es un entero positivo, para
// que el llamador aplique su propio default.
func budgetFlagValue(args []string) int {
	const prefix = "--budget="
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			if n, err := strconv.Atoi(strings.TrimPrefix(a, prefix)); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

// renderPromptContext traduce el contexto que se inyecta en cada turno
// (recordatorio de guardado y/o de modo plan) al dialecto d.
//
// Existe porque `user-prompt-submit` era el único hook que emitía su salida sin
// pasar por el traductor: hablaba JSON de Claude Code y punto. Eso bastaba
// mientras fuera el único agente con inyección por turno, y dejó de bastar al
// verificarse en sesión real que Codex también dispara UserPromptSubmit y toma
// el stdout del hook COMO CONTEXTO TAL CUAL. Entregarle el JSON de Claude le
// inyectaría la envoltura como si fuera texto para el modelo.
//
// texto == "" representa el silencio: no hay nada que recordar este turno.
func renderPromptContext(d hookDialect, texto string) hookRenderedOutput {
	switch d {
	case dialectClaude:
		if texto == "" {
			return hookRenderedOutput{stdout: "{}"}
		}
		out, _ := json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "UserPromptSubmit",
				"additionalContext": texto,
			},
		})
		return hookRenderedOutput{stdout: string(out)}

	case dialectJSON:
		out, _ := json.Marshal(map[string]any{"context": texto})
		return hookRenderedOutput{stdout: string(out)}

	default:
		// dialectNeutral y dialectText: el texto va desnudo a stdout. El
		// silencio es la cadena vacía y no `{}` — quien lee stdout como
		// contexto se tragaría esas dos llaves como si fueran una instrucción.
		return hookRenderedOutput{stdout: texto}
	}
}

// renderTurnEnd traduce la salida del hook de fin de turno al dialecto d.
//
// Existe por el mismo motivo que renderPromptContext, y por el mismo descuido:
// `turn-end` emitía el sobre de Claude Code a cualquier agente. Codex toma el
// stdout del hook COMO CONTEXTO TAL CUAL y no reconoce ese sobre, así que
// rechazaba la salida entera —«invalid stop hook JSON output»— y el refuerzo de
// preferencias no llegaba nunca. El campo Emit de la tabla de hooks de Codex ya
// existía para esto; turn-end era el único que inyecta texto al modelo y se
// había registrado sin pedirlo.
//
// paraElHumano distingue los dos mensajes que este hook puede emitir: el aviso
// de compactación va dirigido a quien mira la terminal —systemMessage en Claude
// Code, que el modelo no ve— y el refuerzo de preferencias al modelo, vía
// additionalContext. En los dialectos planos esa distinción no existe: hay un
// solo canal, stdout, y ambos salen como texto desnudo.
//
// texto == "" representa el silencio: este turno no tiene nada que decir.
func renderTurnEnd(d hookDialect, texto string, paraElHumano bool) string {
	if texto == "" {
		return ""
	}
	switch d {
	case dialectClaude:
		if paraElHumano {
			out, _ := json.Marshal(map[string]any{"systemMessage": texto})
			return string(out)
		}
		out, _ := json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "Stop",
				"additionalContext": texto,
			},
		})
		return string(out)

	case dialectJSON:
		out, _ := json.Marshal(map[string]any{"context": texto})
		return string(out)

	default:
		// dialectNeutral y dialectText: texto desnudo. El silencio ya salió
		// arriba como cadena vacía, nunca como `{}`.
		return texto
	}
}

// renderEnteredDocument traduce el documento de plan-entered (ya ajustado al
// presupuesto por domain.AdjustPlanDocumentToBudget) al dialecto d, según las
// tres formas documentadas en contracts/hook-plan-entered.md. doc == ""
// representa el Caso B (silencio): apagado, sin proyecto resoluble, o ya
// emitido esta sesión sin nada más que ofrecer.
func renderEnteredDocument(d hookDialect, doc string) hookRenderedOutput {
	switch d {
	case dialectClaude:
		if doc == "" {
			return hookRenderedOutput{stdout: "{}"}
		}
		out, _ := json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "PostToolUse",
				"additionalContext": doc,
			},
		})
		return hookRenderedOutput{stdout: string(out)}

	case dialectJSON:
		out, _ := json.Marshal(map[string]any{"context": doc})
		return hookRenderedOutput{stdout: string(out)}

	default: // dialectNeutral y dialectText: el documento va directo a stdout.
		return hookRenderedOutput{stdout: doc}
	}
}
