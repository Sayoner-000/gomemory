package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/application/ports"
)

// En Claude Code, entrar en modo plan con Shift+Tab no llama a la herramienta
// EnterPlanMode, así que el hook PostToolUse:EnterPlanMode nunca se dispara. El
// payload de UserPromptSubmit sí trae permission_mode (verificado en Claude
// Code 2.1.270): estos tests fijan que ese prompt entregue el método de plan.

type planPromptActivity struct{ fired []string }

func (a *planPromptActivity) RecordFired(agent, scope, kind string) error {
	a.fired = append(a.fired, agent+"/"+scope+"/"+kind)
	return nil
}
func (a *planPromptActivity) RecordError(string, string, string, string) error { return nil }
func (a *planPromptActivity) Last(string, string, string) (time.Time, string, bool) {
	return time.Time{}, "", false
}
func (a *planPromptActivity) SessionsSince(time.Time) int { return 0 }

var _ ports.ChannelActivityLog = (*planPromptActivity)(nil)

type planPromptSettings struct{ disabled bool }

func (s planPromptSettings) Read(string) ports.SettingsData {
	return ports.SettingsData{AtomicPlanDisabled: s.disabled}
}
func (s planPromptSettings) Write(string, ports.SettingsData) error      { return nil }
func (s planPromptSettings) ApplyAutoApprove(string, ports.SettingsData) {}

type planPromptContext struct{ doc string }

func (c planPromptContext) Build() (string, error) { return c.doc, nil }
func (c planPromptContext) WriteFile() error       { return nil }

func newPlanPromptDeps(t *testing.T, disabled bool) (*Deps, *planPromptActivity, string) {
	t.Helper()
	previous := planMethod
	SetPlanMethod("MÉTODO DE DESCOMPOSICIÓN")
	t.Cleanup(func() { SetPlanMethod(previous) })
	root := t.TempDir()
	act := &planPromptActivity{}
	deps := &Deps{
		ProjectRepo:     &fakeProjectRepo{root: root},
		SettingsRepo:    planPromptSettings{disabled: disabled},
		ContextBuilder:  planPromptContext{doc: "HISTORIAL DEL PROYECTO"},
		ChannelActivity: act,
	}
	return deps, act, root
}

func TestPlanEntryFromPrompt_ModoPlanEntregaElMetodoYRegistraActividad(t *testing.T) {
	deps, act, root := newPlanPromptDeps(t, false)

	doc, ok := planEntryFromPrompt(deps, root, map[string]any{"permission_mode": "plan", "prompt": "planifica"})
	if !ok || !strings.Contains(doc, "MÉTODO DE DESCOMPOSICIÓN") || !strings.Contains(doc, "HISTORIAL DEL PROYECTO") {
		t.Fatalf("en modo plan debe entregar método e historial: ok=%t doc=%q", ok, doc)
	}
	if len(act.fired) != 1 || act.fired[0] != "claude/user/plan_entry" {
		t.Fatalf("debe registrar la actividad del canal plan_entry: %v", act.fired)
	}
	if _, err := os.Stat(planEnteredMarkerPath(deps, root)); err != nil {
		t.Fatalf("debe marcar la entrega en la sesión: %v", err)
	}

	if _, again := planEntryFromPrompt(deps, root, map[string]any{"permission_mode": "plan"}); again {
		t.Fatal("en la misma sesión el documento completo no se repite")
	}
}

func TestPlanEntryFromPrompt_OtroModoNoHaceNada(t *testing.T) {
	deps, act, root := newPlanPromptDeps(t, false)
	for _, payload := range []map[string]any{nil, {"permission_mode": "default"}, {"prompt": "sin modo"}} {
		if _, ok := planEntryFromPrompt(deps, root, payload); ok {
			t.Fatalf("fuera de modo plan no debe entregar nada: %v", payload)
		}
	}
	if len(act.fired) != 0 {
		t.Fatalf("fuera de modo plan no se ejerce el canal: %v", act.fired)
	}
	if _, err := os.Stat(filepath.Join(root, ".plan-entered-emitted")); err == nil {
		t.Fatal("fuera de modo plan no debe marcar la entrega")
	}
}

func TestPlanEntryFromPrompt_GateDeshabilitadoRegistraPeroNoEntrega(t *testing.T) {
	deps, act, root := newPlanPromptDeps(t, true)
	if _, ok := planEntryFromPrompt(deps, root, map[string]any{"permission_mode": "plan"}); ok {
		t.Fatal("con el plan atómico deshabilitado no debe entregar el documento")
	}
	if len(act.fired) != 1 {
		t.Fatalf("el canal sí se ejerció aunque el gate esté deshabilitado: %v", act.fired)
	}
}
