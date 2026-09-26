package console

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// FR-022, FR-027: sin TTY o con --yes nunca se pregunta; sin color, terminal
// tonta o estrecha, se pregunta en texto plano.
func TestDetectMode(t *testing.T) {
	cases := []struct {
		name string
		e    Env
		yes  bool
		want Mode
	}{
		{"yes manda", Env{StdinTTY: true, StdoutTTY: true, Width: 120, Getenv: env(nil)}, true, ModeNonInteractive},
		{"stdin no es TTY", Env{StdinTTY: false, StdoutTTY: true, Width: 120, Getenv: env(nil)}, false, ModeNonInteractive},
		{"stdout no es TTY", Env{StdinTTY: true, StdoutTTY: false, Width: 120, Getenv: env(nil)}, false, ModeNonInteractive},
		{"terminal completa", Env{StdinTTY: true, StdoutTTY: true, Width: 120, Getenv: env(nil)}, false, ModeRich},
		{"NO_COLOR", Env{StdinTTY: true, StdoutTTY: true, Width: 120, Getenv: env(map[string]string{"NO_COLOR": "1"})}, false, ModePlain},
		{"TERM=dumb", Env{StdinTTY: true, StdoutTTY: true, Width: 120, Getenv: env(map[string]string{"TERM": "dumb"})}, false, ModePlain},
		{"estrecha", Env{StdinTTY: true, StdoutTTY: true, Width: 59, Getenv: env(nil)}, false, ModePlain},
		{"ancho desconocido", Env{StdinTTY: true, StdoutTTY: true, Width: 0, Getenv: env(nil)}, false, ModePlain},
	}
	for _, c := range cases {
		if got := DetectMode(c.e, c.yes); got != c.want {
			t.Errorf("%s: DetectMode = %v; quiero %v", c.name, got, c.want)
		}
	}
}

func agentOpts() []Option {
	return []Option{
		{Value: "claude", Label: "Claude Code", Checked: true},
		{Value: "codex", Label: "Codex", Checked: true},
		{Value: "opencode", Label: "OpenCode"},
	}
}

func TestPlainMultiSelect(t *testing.T) {
	cases := []struct {
		name, input string
		want        []string
	}{
		{"Enter conserva los marcados", "\n", []string{"claude", "codex"}},
		{"números con coma", "1,3\n", []string{"claude", "opencode"}},
		{"números con espacio", "3 2\n", []string{"codex", "opencode"}},
		{"número inválido y reintento", "9\n2\n", []string{"codex"}},
	}
	for _, c := range cases {
		var out bytes.Buffer
		got, err := NewPlain(strings.NewReader(c.input), &out).MultiSelect("Agentes", agentOpts())
		if err != nil {
			t.Fatalf("%s: error %v", c.name, err)
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: = %v; quiero %v (salida: %q)", c.name, got, c.want, out.String())
		}
	}
}

func TestPlainMultiSelect_MuestraMarcasYTitulo(t *testing.T) {
	var out bytes.Buffer
	_, _ = NewPlain(strings.NewReader("\n"), &out).MultiSelect("¿Qué agentes?", agentOpts())
	for _, want := range []string{"¿Qué agentes?", "1) [x] Claude Code", "3) [ ] OpenCode"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("falta %q en %q", want, out.String())
		}
	}
}

func TestPlain_FinDeEntradaCancela(t *testing.T) {
	ui := NewPlain(strings.NewReader(""), &bytes.Buffer{})
	if _, err := ui.MultiSelect("x", agentOpts()); !errors.Is(err, ErrCanceled) {
		t.Errorf("MultiSelect sin entrada: err=%v", err)
	}
	if _, err := ui.Confirm("x", true); !errors.Is(err, ErrCanceled) {
		t.Errorf("Confirm sin entrada: err=%v", err)
	}
}

func TestPlainSelect(t *testing.T) {
	opts := []Option{
		{Value: "global", Label: "Global en ~/.local/bin", Recommended: true},
		{Value: "project", Label: "Copia en el proyecto"},
	}
	var out bytes.Buffer
	got, err := NewPlain(strings.NewReader("\n"), &out).Select("¿Dónde?", opts)
	if err != nil || got != "global" {
		t.Fatalf("Enter debe elegir la recomendada: %q %v", got, err)
	}
	if !strings.Contains(out.String(), "(Recomendado)") {
		t.Errorf("la recomendada debe marcarse: %q", out.String())
	}
	got, _ = NewPlain(strings.NewReader("2\n"), &bytes.Buffer{}).Select("¿Dónde?", opts)
	if got != "project" {
		t.Errorf("2 debe elegir la segunda: %q", got)
	}
}

func TestPlainConfirm(t *testing.T) {
	cases := []struct {
		input string
		def   bool
		want  bool
	}{
		{"\n", true, true},
		{"\n", false, false},
		{"s\n", false, true},
		{"sí\n", false, true},
		{"y\n", false, true},
		{"n\n", true, false},
		{"no\n", true, false},
	}
	for _, c := range cases {
		got, err := NewPlain(strings.NewReader(c.input), &bytes.Buffer{}).Confirm("¿Continuar?", c.def)
		if err != nil || got != c.want {
			t.Errorf("Confirm(%q, def=%v) = %v,%v; quiero %v", c.input, c.def, got, err, c.want)
		}
	}
}

// FR-016: el alcance de sistema exige escribir la palabra exacta.
func TestPlainTypedConfirm(t *testing.T) {
	ok, err := NewPlain(strings.NewReader("gomemory\n"), &bytes.Buffer{}).TypedConfirm("Escribe gomemory", "gomemory")
	if err != nil || !ok {
		t.Fatalf("la palabra exacta debe confirmar: %v %v", ok, err)
	}
	for _, in := range []string{"GOMEMORY\n", "gomemory \n", "s\n", "\n"} {
		ok, _ := NewPlain(strings.NewReader(in), &bytes.Buffer{}).TypedConfirm("x", "gomemory")
		if ok && strings.TrimSpace(in) != "gomemory" {
			t.Errorf("%q no debe confirmar", in)
		}
	}
}

// Modelo rico: espacio marca, flechas mueven, Enter termina.
func TestRichMultiSelectModel(t *testing.T) {
	m := newMultiSelectModel("Agentes", agentOpts())
	steps := []tea.Msg{
		tea.KeyPressMsg{Code: tea.KeySpace}, // desmarca claude
		tea.KeyPressMsg{Code: tea.KeyDown},  // codex
		tea.KeyPressMsg{Code: tea.KeyDown},  // opencode
		tea.KeyPressMsg{Code: tea.KeySpace}, // marca opencode
		tea.KeyPressMsg{Code: tea.KeyEnter},
	}
	var model tea.Model = m
	for _, s := range steps {
		model, _ = model.Update(s)
	}
	final := model.(multiSelectModel)
	if !final.done || final.canceled {
		t.Fatalf("Enter debe terminar sin cancelar: %+v", final)
	}
	if got := strings.Join(final.values(), ","); got != "codex,opencode" {
		t.Errorf("valores = %q", got)
	}
}

func TestRichMultiSelectModel_EscCancela(t *testing.T) {
	var model tea.Model = newMultiSelectModel("Agentes", agentOpts())
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !model.(multiSelectModel).canceled {
		t.Fatal("Esc debe cancelar")
	}
}

func TestRichSelectModel_EmpiezaEnLaRecomendada(t *testing.T) {
	opts := []Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B", Recommended: true}}
	var model tea.Model = newSelectModel("x", opts)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := model.(selectModel).value(); got != "b" {
		t.Errorf("Enter sin moverse debe elegir la recomendada: %q", got)
	}
}

// FR-015, FR-024: resumen con ✓/⚠/✗; el ⚠ lleva el comando manual.
func TestRenderSummary_Plano(t *testing.T) {
	got := RenderSummary([]StepResult{
		{Name: "Binario global", Status: StepOK},
		{Name: "~/.codex/config.toml", Status: StepWarn, Detail: "no se pudo interpretar", Manual: "edita el archivo y quita [mcp_servers.gomemory]"},
		{Name: "Checksum", Status: StepFail, Detail: "no coincide"},
	}, false)
	want := "✓ Binario global\n" +
		"⚠ ~/.codex/config.toml: no se pudo interpretar → edita el archivo y quita [mcp_servers.gomemory]\n" +
		"✗ Checksum: no coincide\n"
	if got != want {
		t.Errorf("RenderSummary =\n%s\nquiero\n%s", got, want)
	}
}

func TestReporter_AcumulaYCuentaAvisos(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out, false)
	r.Done(StepResult{Name: "a", Status: StepOK})
	r.Done(StepResult{Name: "b", Status: StepWarn, Detail: "x", Manual: "y"})
	if !strings.Contains(out.String(), "✓ a") || !strings.Contains(out.String(), "⚠ b: x → y") {
		t.Errorf("cada paso debe informarse al terminar: %q", out.String())
	}
	if r.Warnings() != 1 || r.Failures() != 0 || len(r.Results()) != 2 {
		t.Errorf("recuento: warn=%d fail=%d n=%d", r.Warnings(), r.Failures(), len(r.Results()))
	}
}
