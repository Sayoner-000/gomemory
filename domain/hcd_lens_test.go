package domain

import (
	"slices"
	"strings"
	"testing"
)

// HCD-052: una ruta ausente bajo un directorio de configuración de agente
// (.codex/…, .gemini/…) es del HOME, no del repositorio: no da PATH-MISSING.
// Las demás ausentes, incluidas las que solo empiezan por punto, sí.
func TestHCDLens_RepoMissingPaths(t *testing.T) {
	got := RepoMissingPaths([]string{
		".codex", ".codex/agents/splash.toml", ".gemini/config/agents/splash/agent.md",
		".claude/agents/splash.md", ".config/opencode/agents/splash.md", ".cursor/mcp.json",
		".windsurf/mcp_config.json", ".cline/mcp_settings.json",
		"src/exportar.go", ".github/workflows/nuevo.yml", ".codexx/x.md",
	})
	want := []string{"src/exportar.go", ".github/workflows/nuevo.yml", ".codexx/x.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("RepoMissingPaths = %q, want %q", got, want)
	}
}

// HCD-050: el sufijo de líneas se quita en todas sus formas; con «:52/55»
// la ruta quedaba con el sufijo y analyze daba un PATH-MISSING falso.
func TestHCDLens_CitedPathsSufijosDeLinea(t *testing.T) {
	doc := UnitDoc{Sections: map[string]string{SectionScope: "`a/uno.go:12`, `a/dos.go:12:3`, `a/tres.go:152-159`, " +
		"`a/cuatro.go:160,190`, `tests/integration/session_delta_test.go:52/55` y `a/cinco.go:1-4/9`."}}
	got := CitedPaths(doc)
	want := []string{"a/cinco.go", "a/cuatro.go", "a/dos.go", "a/tres.go", "a/uno.go", "tests/integration/session_delta_test.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("CitedPaths = %q, want %q", got, want)
	}
	if !TaskNamesFile(Task{Text: "Ajustar session_delta_test.go:52/55"}) {
		t.Error("TaskNamesFile no reconoce una ruta con sufijo «:52/55»")
	}
}

// HCD-047: una ruta del HOME («~/.claude/agents/splash.md») no es del
// repositorio; citarla daba un falso PATH-MISSING en analyze.
func TestHCDLens_CitedPathsIgnoraHome(t *testing.T) {
	doc := UnitDoc{Sections: map[string]string{SectionScope: "Instala `~/.claude/agents/splash.md` y toca `adapters/primary/setup/splash_agent.go`."}}
	got := CitedPaths(doc)
	if len(got) != 1 || got[0] != "adapters/primary/setup/splash_agent.go" {
		t.Fatalf("CitedPaths = %v, quiero solo la ruta del repositorio", got)
	}
}

// HCD-034: las lentes son avisos MEDIUM; PATH-MISSING solo desde verify (Q1).
func TestAnalyzeLenses(t *testing.T) {
	doc := fill(render(t, KindFeature, LaneFast), LaneFast)
	doc = strings.Replace(doc, "Incluido: guardado", "Incluido: guardado en `domain/falta.go`", 1)
	doc = strings.Replace(doc, "## Checklist\n", "## Checklist\n- [ ] 1. Persistir el cambio → go test ./x\n- [ ] 2. Medir la salida → observar a mano\n", 1)
	s := stateOf(t, doc, KindFeature, LaneFast)
	s.MissingPaths = []string{"domain/falta.go"}

	got := codes(s.Analyze())
	if got["TASK-NO-FILE:2"] != SeverityMedium || got["TASK-NO-FILE:1"] != "" {
		t.Errorf("TASK-NO-FILE: %v", got)
	}
	if got["CHEAPEST-FILLER:"+SectionCheapest] != SeverityMedium {
		t.Errorf("CHEAPEST-FILLER: %v", got)
	}
	if got["PATH-MISSING:domain/falta.go"] != "" {
		t.Errorf("PATH-MISSING antes de verify (Q1): %v", got)
	}

	s = stateOf(t, string(MarkTasksDone([]byte(doc), []string{"1", "2"})), KindFeature, LaneFast)
	s.MissingPaths = []string{"domain/falta.go"}
	s.Evidence = []Evidence{green(s, "f1", t0, "1", "2")}
	findings := s.Analyze()
	if s.Stage() != StageClose || codes(findings)["PATH-MISSING:domain/falta.go"] != SeverityMedium {
		t.Fatalf("PATH-MISSING al cerrar: stage %s %v", s.Stage(), codes(findings))
	}
	if HasCritical(findings) {
		t.Fatalf("las lentes no bloquean el cierre: %+v", findings)
	}
}

// HCD-034: lentes deterministas de analyze, sin coste en tokens.
func TestHCDLens_CitedPaths(t *testing.T) {
	doc := UnitDoc{Sections: map[string]string{
		SectionScope: "Incluido: `domain/hcd_stage.go`, `HCDState.Analyze`, `./adapters/x/` y `README.md:12`.\n" +
			"Excluido: `https://example.com/a.go`, `hcd/HCD-*.md`, `<id>.md`, `go test ./...` y `mem hcd analyze`.",
		SectionChecklist: "- [ ] 1. Crear `domain/hcd_lens.go` → go test ./domain/",
		SectionEvidence:  "`hcd/no-cuenta.md`",
		SectionIntent:    "<!-- `comentario/oculto.go` --> nada citado `domain/hcd_stage.go`",
	}}
	got := CitedPaths(doc)
	want := []string{"README.md", "adapters/x", "domain/hcd_lens.go", "domain/hcd_stage.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("CitedPaths = %q, want %q", got, want)
	}
}

func TestHCDLens_TaskNamesFile(t *testing.T) {
	cases := []struct {
		task Task
		want bool
	}{
		{Task{Text: "Crear `domain/hcd_lens.go`", Check: "go test"}, true},
		{Task{Text: "Implementar la detección", Check: "go test ./domain/ -run X"}, true},
		{Task{Text: "Actualizar README.md", Check: "observar"}, true},
		{Task{Text: "Implementar la detección", Check: "observar la salida"}, false},
		{Task{Text: "Cambiar HCDState.Analyze", Check: "revisar CLI/MCP a mano"}, false},
	}
	for _, c := range cases {
		if got := TaskNamesFile(c.task); got != c.want {
			t.Errorf("TaskNamesFile(%q → %q) = %v, want %v", c.task.Text, c.task.Check, got, c.want)
		}
	}
}

func TestHCDLens_CheapestFiller(t *testing.T) {
	for _, s := range []string{"ninguna", "Ninguna.", "No hay", "n/a", "N/A.", "no aplica", "nada", "-", "no", "  No.  "} {
		if !IsCheapestFiller(s) {
			t.Errorf("IsCheapestFiller(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"Ninguna: la skill ya lo intentó y el modelo lo omite", "Documentarlo en la skill", "pendiente", ""} {
		if IsCheapestFiller(s) {
			t.Errorf("IsCheapestFiller(%q) = true, want false", s)
		}
	}
}
