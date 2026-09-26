package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"mem/adapters/primary/console"
)

func choicesFor(detected ...string) []agentChoice {
	var out []agentChoice
	for _, a := range installAgents {
		c := agentChoice{installAgent: a}
		for _, d := range detected {
			if d == a.name {
				c.detected = true
			}
		}
		out = append(out, c)
	}
	return out
}

// FR-020: agentes (detectados ya marcados) → binario (solo sin global) →
// alcance → resumen con confirmación, en ese orden.
func TestAskInstallSelection_OrdenYDefectos(t *testing.T) {
	var out bytes.Buffer
	// Enter en agentes (acepta los marcados), Enter en binario (global,
	// recomendado), Enter en alcance (proyecto), s en la confirmación.
	ui := console.NewPlain(strings.NewReader("\n\n\ns\n"), &out)
	sel, err := askInstallSelection(ui, choicesFor("claude", "codex"), false, installSelection{})
	if err != nil {
		t.Fatalf("error: %v\n%s", err, out.String())
	}
	if strings.Join(sel.agents, ",") != "claude,codex" || sel.scope != "project" || sel.binary != "global" {
		t.Errorf("selección = %+v", sel)
	}
	s := out.String()
	iAg := strings.Index(s, "¿Qué agentes quieres conectar?")
	iBin := strings.Index(s, "¿Dónde instalar el binario?")
	iSc := strings.Index(s, "¿Alcance de la configuración?")
	iRes := strings.Index(s, "Se configurará")
	if iAg < 0 || iBin < iAg || iSc < iBin || iRes < iSc {
		t.Errorf("orden de preguntas incorrecto:\n%s", s)
	}
	if !strings.Contains(s, "[x] Claude Code") || !strings.Contains(s, "[ ] OpenCode") {
		t.Errorf("los detectados deben ir marcados:\n%s", s)
	}
}

// Con un global no se pregunta por el binario.
func TestAskInstallSelection_ConGlobalSinPreguntaDeBinario(t *testing.T) {
	var out bytes.Buffer
	ui := console.NewPlain(strings.NewReader("\n\ns\n"), &out)
	if _, err := askInstallSelection(ui, choicesFor("claude"), true, installSelection{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "¿Dónde instalar el binario?") {
		t.Errorf("con global no se pregunta por el binario:\n%s", out.String())
	}
}

// FR-023: una selección guardada aparece ya marcada.
func TestAskInstallSelection_SeleccionGuardadaMarcada(t *testing.T) {
	var out bytes.Buffer
	ui := console.NewPlain(strings.NewReader("\n\ns\n"), &out)
	sel, err := askInstallSelection(ui, choicesFor("claude", "codex"), true, installSelection{agents: []string{"opencode"}, scope: "global"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(sel.agents, ",") != "opencode" || sel.scope != "global" {
		t.Errorf("la selección guardada manda sobre la detección: %+v", sel)
	}
}

// FR-023: una selección vacía guardada significa que la persona eligió no
// conectar agentes; no se debe sustituir por los detectados al reinstalar.
func TestInstallSelection_SeleccionVaciaGuardada(t *testing.T) {
	choices := choicesFor("claude", "codex")
	saved := installSelection{scope: "project"}
	if got := resolveInstallSelection(installOptions{}, saved, choices); len(got.agents) != 0 {
		t.Fatalf("reinstalación activó agentes detectados: %v", got.agents)
	}

	var out bytes.Buffer
	ui := console.NewPlain(strings.NewReader("\n\ns\n"), &out)
	got, err := askInstallSelection(ui, choices, true, saved)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.agents) != 0 || strings.Contains(out.String(), "[x] Claude Code") || strings.Contains(out.String(), "[x] Codex") {
		t.Fatalf("la selección vacía debía seguir sin agentes: %+v\n%s", got, out.String())
	}
}

// Cancelar en la confirmación no escribe nada.
func TestAskInstallSelection_Cancelar(t *testing.T) {
	ui := console.NewPlain(strings.NewReader("\n\nn\n"), &bytes.Buffer{})
	if _, err := askInstallSelection(ui, choicesFor("claude"), true, installSelection{}); !errors.Is(err, console.ErrCanceled) {
		t.Fatalf("responder n en la confirmación cancela: %v", err)
	}
}
