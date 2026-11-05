package usecases

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mem/domain"
)

// HCD-028 VS-02: el caso de uso reúne las fuentes del archivo editado.

type fakeEditSource struct {
	mems     []domain.Memory
	tests    []domain.EditContextTest
	testsErr error
	gotPaths []string
}

func (f *fakeEditSource) MemoriesByFilepath(_ string, paths []string, _ int) ([]domain.Memory, error) {
	f.gotPaths = paths
	return f.mems, nil
}

func (f *fakeEditSource) TestsCovering(string, string, int) ([]domain.EditContextTest, error) {
	return f.tests, f.testsErr
}

func TestEditContext(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "domain", "rules.go")
	src := &fakeEditSource{
		mems: []domain.Memory{
			{ID: 9, Type: domain.Checkpoint, Title: "Editó rules.go"},
			{ID: 8, Type: domain.Decision, Title: "graph-before-grep"},
			{ID: 7, Type: domain.Checkpoint, Title: "Editó otra vez"},
			{ID: 6, Type: domain.Bugfix, Title: "sustituciones"},
		},
		tests: []domain.EditContextTest{{Name: "TestBloquea", Dir: "./domain"}},
	}
	uc := EditContext{Source: src, Root: root, Project: "proj",
		Impact: func(p string) (domain.CodeImpactAnnotation, bool) {
			return domain.CodeImpactAnnotation{Hotspot: true, Symbol: "EvaluateRules", FanIn: 13}, p == "domain/rules.go"
		}}

	in := uc.Build(context.Background(), abs)
	if in.Path != "domain/rules.go" {
		t.Fatalf("la ruta debe quedar relativa a la raíz: %q", in.Path)
	}
	if len(src.gotPaths) != 2 || src.gotPaths[0] != "domain/rules.go" || src.gotPaths[1] != abs {
		t.Fatalf("debe buscar memorias por ruta relativa y absoluta: %v", src.gotPaths)
	}
	var ids []int64
	for _, m := range in.Memories {
		ids = append(ids, m.ID)
	}
	if len(ids) != 3 || ids[0] != 8 || ids[1] != 6 || ids[2] != 9 {
		t.Fatalf("decisiones y bugfix antes que checkpoints, máximo 3: %v", ids)
	}
	if len(in.Tests) != 1 || in.Impact == nil || in.Impact.FanIn != 13 {
		t.Fatalf("faltan pruebas o impacto: %+v", in)
	}

	// Una fuente que falla se omite; las demás siguen.
	src.testsErr = errors.New("grafo no disponible")
	if in := uc.Build(context.Background(), abs); in.Tests != nil || len(in.Memories) == 0 {
		t.Fatalf("la fuente que falla se omite sin perder las demás: %+v", in)
	}

	// Fuera del proyecto no hay contexto.
	if in := uc.Build(context.Background(), filepath.Join(t.TempDir(), "x.go")); in.Path != "" {
		t.Fatalf("una ruta fuera de la raíz no tiene contexto: %+v", in)
	}

	// Con el plazo vencido no se consulta nada más.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if in := uc.Build(ctx, abs); len(in.Memories) != 0 || in.Tests != nil {
		t.Fatalf("con el plazo vencido no se consultan fuentes: %+v", in)
	}
}
