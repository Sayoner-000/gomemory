package cli

import (
	"bytes"
	"context"
	"errors"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

type evolveRepoFake struct{ ports.InstinctRepository }

func (evolveRepoFake) List(context.Context, bool) ([]domain.Instinct, error) {
	return []domain.Instinct{{Slug: "cmd:go-test", Description: "Ejecutar pruebas", Confidence: .7}}, nil
}

type evolveGeneratorFake struct {
	ports.SocraticGenerator
	available bool
	body      string
	fail      bool
}

func (g evolveGeneratorFake) Name() string    { return "claude" }
func (g evolveGeneratorFake) Available() bool { return g.available }
func (g evolveGeneratorFake) Harnesses() []ports.SocraticHarness {
	return []ports.SocraticHarness{{ID: "claude", Label: "Claude", Available: g.available}}
}
func (g evolveGeneratorFake) Rewrite(_ context.Context, r ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	if g.fail {
		return domain.RewriteResult{}, errors.New("generation failed")
	}
	if r.Flow.Client != "claude" || r.Flow.Mode != domain.ModeRefine {
		panic("wrong flow")
	}
	return domain.RewriteResult{Proposed: g.body}, nil
}
func TestExpandEvolveApproval(t *testing.T) {
	valid := "---\nname: instinct-cmd-go-test\ndescription: Ejecutar pruebas\n---\n## Capacidades\nUsa `save_memory`; alternativa mem save y registro manual.\n## Procedimiento\n1. Ejecutar pruebas y guardar el resultado.\nInspirada en continuous-learning-v2 de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.\n"
	for _, tc := range []struct {
		name, input, body    string
		tty, available, fail bool
		code, writes         int
	}{
		{"no-tty", "", valid, false, true, false, 2, 0}, {"unavailable", "", valid, true, false, false, 1, 0}, {"generation-failure", "1\n", valid, true, true, true, 1, 0}, {"invalid", "1\n", "bad", true, true, false, 1, 0}, {"cancel", "1\nn\n", valid, true, true, false, 3, 0}, {"approve", "1\ns\n", valid, true, true, false, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := evolveGeneratorFake{available: tc.available, body: tc.body, fail: tc.fail}
			d := &Deps{Instincts: &usecases.Instincts{Repo: evolveRepoFake{}}, Socratica: &usecases.SocraticaFlow{Generator: g}}
			var out, errb bytes.Buffer
			writes := 0
			code := runExpand(d, []string{"instincts", "evolve", "cmd:go-test"}, expandIO{in: strings.NewReader(tc.input), out: &out, err: &errb, tty: tc.tty, install: func(name, body string) ([]string, error) {
				writes++
				if name != "instinct-cmd-go-test" || body != valid {
					t.Fatal(name, body)
				}
				return []string{"skill-path"}, nil
			}})
			if code != tc.code || writes != tc.writes {
				t.Fatalf("got code=%d writes=%d, want %d/%d; %s %s", code, writes, tc.code, tc.writes, out.String(), errb.String())
			}
		})
	}
}
