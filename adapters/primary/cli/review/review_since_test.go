package review

import (
	"slices"
	"strings"
	"testing"

	"mem/adapters/primary/cli/clikit"
	"mem/domain"
)

// HCD-032 VS-03: las superficies MCP y CLI exponen el enlace, el delta, los
// heredados y settled_ref.

func TestReviewSince_Modificadores(t *testing.T) {
	rest, ro, since, err := extraerModificadores([]string{"--commit", "abc", "--since", "acr_1", "--read-only"})
	if err != nil || !ro || since != "acr_1" || !slices.Equal(rest, []string{"--commit", "abc"}) {
		t.Fatalf("--since y --read-only son modificadores en cualquier posición: %v %v %q %v", rest, ro, since, err)
	}
	if _, _, since, err := extraerModificadores([]string{"--since=acr_2", "--pending"}); err != nil || since != "acr_2" {
		t.Fatalf("--since=<id>: %q %v", since, err)
	}
	if _, _, _, err := extraerModificadores([]string{"--commit", "abc", "--since"}); err == nil {
		t.Fatal("--since sin identificador es un error")
	}
	if rest, ro, since, err := extraerModificadores([]string{"--diff"}); err != nil || ro || since != "" || len(rest) != 1 {
		t.Fatalf("sin modificadores, todo como antes: %v %v %q %v", rest, ro, since, err)
	}
}

func TestReviewSince_SettledRefEnElEnvio(t *testing.T) {
	fs := findingsDesdeEntrada([]reviewFindingInput{
		{LocalID: "A-1", Location: "x.go:1", Severity: "HIGH", SettledRef: 7},
		{LocalID: "A-2", Location: "y.go:1", Severity: "LOW"},
	})
	if fs[0].SettledRef != 7 || fs[1].SettledRef != 0 || fs[0].Settled {
		t.Fatalf("settled_ref viaja; settled lo decide el motor, nunca la entrada: %+v", fs)
	}
}

func TestReviewSince_Enlace(t *testing.T) {
	review := &domain.Review{SinceReviewID: "acr_prev", DeltaSource: domain.DeltaSourceGit, DeltaScope: []string{"adapters/x.go"}}
	inherited := []domain.SettledFinding{{ConsensusID: 10, Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
		Claim: "resuelto", Files: []string{"domain/rules.go"}}}
	enlace := enlaceDeRevision(review, inherited)
	if enlace["since_review_id"] != "acr_prev" {
		t.Fatalf("falta el enlace: %+v", enlace)
	}
	delta, _ := enlace["delta"].(map[string]any)
	if delta["source"] != domain.DeltaSourceGit || !slices.Equal(delta["files"].([]string), []string{"adapters/x.go"}) {
		t.Fatalf("falta el delta: %+v", enlace)
	}
	lista, _ := enlace["inherited"].([]map[string]any)
	if len(lista) != 1 || lista[0]["consensus_id"] != int64(10) || !slices.Equal(lista[0]["files"].([]string), []string{"domain/rules.go"}) {
		t.Fatalf("faltan los heredados con su id y archivos: %+v", enlace)
	}
	if s, _ := enlace["instructions"].(string); !strings.Contains(s, "settled_ref") || !strings.Contains(s, "delta") {
		t.Fatalf("los revisores deben saber cómo citar lo heredado y limitarse al delta: %q", s)
	}
	if e := enlaceDeRevision(&domain.Review{}, nil); len(e) != 0 {
		t.Fatalf("una revisión sin enlace no añade nada: %+v", e)
	}
}

// HCD-035: review_start informa del ahorro medido; lo que el revisor recibe de
// más es exactamente la lista de heredados y las instrucciones.
type archivosFijos map[string]string

func (a archivosFijos) TextFilesAt(string, []string) (map[string]string, error) { return a, nil }

type contadorRunas struct{}

func (contadorRunas) Count(s string) int { return len([]rune(s)) }

func TestReviewSince_Savings(t *testing.T) {
	review := &domain.Review{SinceReviewID: "acr_prev", DeltaSource: domain.DeltaSourceGit, DeltaScope: []string{"b.go"},
		Target: domain.Target{Revision: "abc"}}
	enlace := enlaceDeRevision(review, nil)
	deps := &clikit.Deps{RevisionFiles: archivosFijos{"a.go": strings.Repeat("a", 5000), "b.go": "bb"}, TokenCounter: contadorRunas{}}
	s := ahorroDeRevision(deps, review, enlace)
	extra := len([]rune(cargaHeredada(enlace)))
	if extra == 0 || s["measurable"] != true || s["scope_tokens"] != 5002 || s["delta_tokens"] != 2 ||
		s["inherited_tokens"] != extra || s["avoided_tokens"] != 5000-extra {
		t.Fatalf("savings = %+v (extra %d)", s, extra)
	}
	if s := ahorroDeRevision(deps, &domain.Review{SinceReviewID: "p", DeltaSource: domain.DeltaSourceScope}, enlace); s["measurable"] != false || s["reason"] == "" {
		t.Fatalf("sin delta de git: %+v", s)
	}
}

// HCD-033: el orquestador lee las descripciones de las herramientas al usarlas.
func TestReviewSince_Descripciones(t *testing.T) {
	if !strings.Contains(reviewStartDescription, "since_review_id") || !strings.Contains(reviewStartDescription, "inherited") {
		t.Errorf("review_start debe explicar la revisión de seguimiento: %q", reviewStartDescription)
	}
	if !strings.Contains(reviewSubmitDescription, "settled_ref") {
		t.Errorf("review_submit debe explicar settled_ref: %q", reviewSubmitDescription)
	}
}
