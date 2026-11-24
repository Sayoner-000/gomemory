package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/primary/setup"
	"mem/domain"
)

func fixCommands(steps []fixStep) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.accion.Comando)
	}
	return out
}

func TestBuildFixPlan_CanalesDeGomemoryPorAmbito(t *testing.T) {
	st := doctorState{report: domain.CoverageReport{Channels: []domain.ActivationChannel{
		{Arm: domain.ArmGomemory, Agent: "codex", Scope: domain.ScopeUser, Kind: domain.KindInstructions, State: domain.StateMissing},
		{Arm: domain.ArmGomemory, Agent: "claude", Scope: domain.ScopeProject, Kind: domain.KindPlanEntry, State: domain.StateOutdated},
		{Arm: domain.ArmGomemory, Agent: "codex", Scope: domain.ScopeProject, Kind: domain.KindPermissions, State: domain.StateDuplicated},
		{Arm: domain.ArmGomemory, Agent: "opencode", Scope: domain.ScopeProject, Kind: domain.KindPlanEntry, State: domain.StateOK},
		{Arm: domain.ArmGomemory, Agent: "agy", Scope: domain.ScopeProject, Kind: domain.KindPlanEntry, State: domain.StateNotApplicable},
	}}}
	steps := buildFixPlan(st)
	got := fixCommands(steps)
	want := []string{"mem install . --yes", "mem setup-mcp --scope global --agents codex"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("plan = %q, want %q", got, want)
	}
	if len(steps[0].accion.Motivos) != 2 {
		t.Errorf("los dos canales de proyecto comparten la acción: %+v", steps[0].accion)
	}
	if steps[1].accion.Ambito != domain.ScopeUser || steps[1].accion.Advertencia == "" {
		t.Errorf("la acción de usuario advierte su alcance: %+v", steps[1].accion)
	}
	if got := strings.Join(steps[0].argv, " "); got != "mem install . --yes" {
		t.Errorf("argv = %q", got)
	}
}

func TestBuildFixPlan_NuncaTocaElBrazoCodegraph(t *testing.T) {
	st := doctorState{report: domain.CoverageReport{Channels: []domain.ActivationChannel{
		{Arm: domain.ArmCodegraph, Agent: "claude", Scope: domain.ScopeProject, Kind: domain.KindInstructions, State: domain.StateMissing},
		{Arm: domain.ArmCodegraph, Agent: "codex", Scope: domain.ScopeUser, Kind: domain.KindInstructions, State: domain.StateOutdated},
	}}}
	if steps := buildFixPlan(st); len(steps) != 0 {
		t.Fatalf("los canales del brazo codegraph son de solo lectura (INV-1): %q", fixCommands(steps))
	}
}

func TestBuildFixPlan_OpenCodeSeAgrupaConSuCanal(t *testing.T) {
	st := doctorState{
		report: domain.CoverageReport{Channels: []domain.ActivationChannel{
			{Arm: domain.ArmGomemory, Agent: "opencode", Scope: domain.ScopeUser, Kind: domain.KindPlanEntry, State: domain.StateOutdated},
		}},
		openCode: setup.OpenCodeInstallStatus{Version: "2.0.16", Major: 2, PluginShape: setup.PluginShapeV1Only},
	}
	got := fixCommands(buildFixPlan(st))
	if len(got) != 1 || got[0] != openCodeRemedy {
		t.Fatalf("plan = %q, want una sola acción %q", got, openCodeRemedy)
	}
}

func TestBuildFixPlan_OpenCodeArtefactoDePruebas(t *testing.T) {
	st := doctorState{openCode: setup.OpenCodeInstallStatus{Version: "2.0.16", Major: 2, PluginShape: setup.PluginShapeDual, StaleTestArtifact: true}}
	got := fixCommands(buildFixPlan(st))
	if len(got) != 1 || got[0] != openCodeRemedy {
		t.Fatalf("plan = %q", got)
	}
	// Un plugin ajeno no es de gomemory: no produce acción.
	st = doctorState{openCode: setup.OpenCodeInstallStatus{Version: "2.0.16", Major: 2, PluginShape: setup.PluginShapeDual, ForeignV1Plugins: []string{"otro.ts"}}}
	if got := fixCommands(buildFixPlan(st)); len(got) != 0 {
		t.Fatalf("plugins ajenos no se tocan: %q", got)
	}
}

func TestBuildFixPlan_HooksDuplicadosOriginalesYCodeGraph(t *testing.T) {
	st := doctorState{
		dupHooks:         []string{"session-start"},
		compression:      doctorCompressionJSON{NearLimit: true},
		codeGraphMissing: []string{"codex", "agy"},
	}
	steps := buildFixPlan(st)
	got := fixCommands(steps)
	if len(got) != 3 {
		t.Fatalf("se esperaban 3 acciones, plan = %q", got)
	}
	if steps[0].run == nil || steps[0].accion.Ambito != domain.ScopeProject || !strings.Contains(steps[0].accion.Comando, "hooks") {
		t.Errorf("los hooks duplicados se retiran en proceso y en el proyecto: %+v", steps[0].accion)
	}
	if steps[1].run == nil || steps[1].accion.Comando != "mem pack purge" {
		t.Errorf("la purga de originales se hace en proceso: %+v", steps[1].accion)
	}
	if steps[2].accion.Ambito != domain.ScopeUser || steps[2].accion.Comando != codeGraphManual([]string{"codex", "agy"}) {
		t.Errorf("CodeGraph MCP es de ámbito usuario con el instalador de CodeGraph: %+v", steps[2].accion)
	}
	if steps[2].argv[0] != "codegraph" {
		t.Errorf("argv = %q", steps[2].argv)
	}
}

// fixHarness sustituye el diagnóstico, el ejecutor y la entrada de la
// persona: el primer diagnóstico tiene problemas, el siguiente ya no.
type fixHarness struct {
	ran      []string
	inProc   int
	collects int
}

func newFixHarness(t *testing.T, input string) *fixHarness {
	t.Helper()
	h := &fixHarness{}
	broken := doctorState{
		report: domain.CoverageReport{Channels: []domain.ActivationChannel{
			{Arm: domain.ArmGomemory, Agent: "claude", Scope: domain.ScopeProject, Kind: domain.KindPlanEntry, State: domain.StateMissing},
			{Arm: domain.ArmGomemory, Agent: "codex", Scope: domain.ScopeUser, Kind: domain.KindInstructions, State: domain.StateOutdated},
		}},
		problems: 3,
		dupHooks: []string{"session-start"},
	}
	prevCollect, prevRun, prevIn, prevDedup := collectDoctor, runFixCommand, fixConfirmIn, dedupClaudeHooks
	t.Cleanup(func() {
		collectDoctor, runFixCommand, fixConfirmIn, dedupClaudeHooks = prevCollect, prevRun, prevIn, prevDedup
	})
	collectDoctor = func(*Deps, string) doctorState {
		h.collects++
		if h.collects == 1 {
			return broken
		}
		return doctorState{}
	}
	runFixCommand = func(_ context.Context, _ string, argv []string) error {
		h.ran = append(h.ran, strings.Join(argv, " "))
		return nil
	}
	dedupClaudeHooks = func(string, string) ([]string, error) {
		h.inProc++
		return nil, nil
	}
	fixConfirmIn = strings.NewReader(input)
	return h
}

func TestDoctorFix_DryRunNoEjecutaNada(t *testing.T) {
	h := newFixHarness(t, "")
	out := captureStdout(t, func() {
		CmdDoctor(&Deps{Root: t.TempDir()}, []string{"--fix", "--dry-run"})
	})
	if len(h.ran) != 0 || h.inProc != 0 {
		t.Fatalf("--dry-run no ejecuta: ran=%q inProc=%d", h.ran, h.inProc)
	}
	for _, want := range []string{"mem install . --yes", "mem setup-mcp --scope global --agents codex", domain.AdvertenciaAmbitoUsuario, "simulación"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en la salida:\n%s", want, out)
		}
	}
}

func TestDoctorFix_YesEjecutaTodoYVuelveADiagnosticar(t *testing.T) {
	h := newFixHarness(t, "")
	out := captureStdout(t, func() {
		CmdDoctor(&Deps{Root: t.TempDir()}, []string{"--fix", "--yes"})
	})
	want := []string{"mem install . --yes", "mem setup-mcp --scope global --agents codex"}
	if strings.Join(h.ran, "|") != strings.Join(want, "|") || h.inProc != 1 {
		t.Fatalf("ran=%q inProc=%d", h.ran, h.inProc)
	}
	if h.collects != 2 {
		t.Errorf("tras reparar se vuelve a diagnosticar: collects=%d", h.collects)
	}
	if !strings.Contains(out, "3 → 0") {
		t.Errorf("falta el resumen antes/después:\n%s", out)
	}
}

func TestDoctorFix_ConfirmaProyectoUnaVezYUsuarioPorAccion(t *testing.T) {
	// "si" a las acciones de proyecto, "no" a la de usuario.
	h := newFixHarness(t, "si\nno\n")
	out := captureStdout(t, func() {
		CmdDoctor(&Deps{Root: t.TempDir()}, []string{"--fix"})
	})
	if strings.Join(h.ran, "|") != "mem install . --yes" || h.inProc != 1 {
		t.Fatalf("solo se aplican las de proyecto: ran=%q inProc=%d", h.ran, h.inProc)
	}
	if !strings.Contains(out, "omitida") {
		t.Errorf("la acción rechazada se informa como omitida:\n%s", out)
	}
}

func TestDoctorFix_SinRespuestaNoEjecuta(t *testing.T) {
	h := newFixHarness(t, "")
	captureStdout(t, func() {
		CmdDoctor(&Deps{Root: t.TempDir()}, []string{"--fix"})
	})
	if len(h.ran) != 0 || h.inProc != 0 {
		t.Fatalf("sin confirmación no se toca nada: ran=%q inProc=%d", h.ran, h.inProc)
	}
}

func TestDoctorFix_JSON(t *testing.T) {
	h := newFixHarness(t, "")
	out := captureStdout(t, func() {
		CmdDoctor(&Deps{Root: t.TempDir()}, []string{"--fix", "--yes", "--json"})
	})
	var rep struct {
		Problems int `json:"problems"`
		Fixes    *struct {
			Acciones []struct {
				Comando   string `json:"comando"`
				Ambito    string `json:"ambito"`
				Resultado string `json:"resultado"`
			} `json:"acciones"`
			ProblemasAntes   int `json:"problemas_antes"`
			ProblemasDespues int `json:"problemas_despues"`
		} `json:"fixes"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, out)
	}
	if rep.Fixes == nil || len(rep.Fixes.Acciones) != 3 || rep.Fixes.ProblemasAntes != 3 || rep.Fixes.ProblemasDespues != 0 || rep.Problems != 0 {
		t.Fatalf("fixes = %+v, problems = %d", rep.Fixes, rep.Problems)
	}
	for _, a := range rep.Fixes.Acciones {
		if a.Resultado != "aplicada" {
			t.Errorf("acción %q: resultado %q", a.Comando, a.Resultado)
		}
	}
	if len(h.ran) != 2 {
		t.Errorf("ran=%q", h.ran)
	}
}

func TestDoctorFix_JSONSinYesSoloSimula(t *testing.T) {
	h := newFixHarness(t, "si\nsi\n")
	out := captureStdout(t, func() {
		CmdDoctor(&Deps{Root: t.TempDir()}, []string{"--fix", "--json"})
	})
	if len(h.ran) != 0 || h.inProc != 0 {
		t.Fatalf("--json sin --yes no pregunta ni ejecuta: ran=%q", h.ran)
	}
	if !strings.Contains(out, `"resultado": "simulada"`) {
		t.Errorf("las acciones se informan simuladas:\n%s", out)
	}
}

func TestDoctorSinFix_NoCambiaElJSON(t *testing.T) {
	newFixHarness(t, "")
	out := captureStdout(t, func() {
		CmdDoctor(&Deps{Root: t.TempDir()}, []string{"--json"})
	})
	if strings.Contains(out, `"fixes"`) {
		t.Fatalf("sin --fix no aparece la clave fixes:\n%s", out)
	}
}

func TestBuildFixPlan_SinProblemasNoHayPlan(t *testing.T) {
	if steps := buildFixPlan(doctorState{}); len(steps) != 0 {
		t.Fatalf("plan = %q", fixCommands(steps))
	}
}
