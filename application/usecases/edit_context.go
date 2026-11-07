package usecases

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// EditContext reúne, para un archivo que se va a editar, lo que el proyecto
// ya sabe de él (HCD-028): memorias ancladas, pruebas que lo cubren e impacto.
// La sección de la Constitución la añade domain.RenderEditContext por la ruta.
// Cada fuente es opcional: la que falla o llega tras el plazo se omite y las
// demás siguen (C-04).
type EditContext struct {
	Source ports.EditContextSource
	// Impact consulta el snapshot cacheado del proveedor externo; nil = sin él.
	Impact  func(path string) (domain.CodeImpactAnnotation, bool)
	Root    string
	Project string
}

// editContextFetch es cuántas memorias se piden para poder priorizar las
// decisiones sobre los checkpoints automáticos antes de recortar a 3.
const editContextFetch = 12

// Build devuelve la entrada del contexto de filePath. Una ruta fuera de la
// raíz del proyecto no tiene contexto (Path vacío).
func (e EditContext) Build(ctx context.Context, filePath string) domain.EditContextInput {
	rel, ok := e.relative(filePath)
	if !ok {
		return domain.EditContextInput{}
	}
	in := domain.EditContextInput{Path: rel}
	if ctx.Err() != nil || e.Source == nil {
		return in
	}
	paths := []string{rel}
	if abs := filepath.Join(e.Root, filepath.FromSlash(rel)); abs != rel {
		paths = append(paths, abs)
	}
	if mems, err := e.Source.MemoriesByFilepath(e.Project, paths, editContextFetch); err == nil {
		in.Memories = prioritize(mems)
	}
	if ctx.Err() != nil {
		return in
	}
	if strings.HasSuffix(rel, ".go") {
		if tests, err := e.Source.TestsCovering(e.Project, rel, 3); err == nil {
			in.Tests = tests
		}
	}
	if ctx.Err() == nil && e.Impact != nil {
		if a, ok := e.Impact(rel); ok {
			in.Impact = &a
		}
	}
	return in
}

// relative normaliza la ruta a relativa y con barras, como la guarda el
// grafo propio. Una ruta que sale de la raíz no pertenece al proyecto.
func (e EditContext) relative(p string) (string, bool) {
	if strings.TrimSpace(p) == "" {
		return "", false
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(e.Root, p)
	}
	if rel, ok := relInside(e.Root, p); ok {
		return rel, true
	}
	// El host puede dar la ruta con los enlaces resueltos y la raíz sin
	// resolver, o al revés (/var frente a /private/var en macOS).
	root, p2 := e.Root, p
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if d, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		p2 = filepath.Join(d, filepath.Base(p))
	}
	return relInside(root, p2)
}

func relInside(root, p string) (string, bool) {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// prioritize pone primero las memorias escritas por el agente y las fichas de
// las unidades HCD (decisiones, bugfix, arquitectura…), después las fichas de
// auditoría que solo citan el archivo (HCD-046) y al final los checkpoints
// automáticos. Conserva el orden de recencia dentro de cada grupo y deja solo
// lo que se cita.
func prioritize(mems []domain.Memory) []domain.EditContextMemory {
	rank := func(m domain.Memory) int {
		switch {
		case m.Type == domain.Checkpoint:
			return 2
		case strings.HasPrefix(m.TopicKey, domain.AuditDigestTopicPrefix):
			return 1
		default:
			return 0
		}
	}
	sorted := slices.Clone(mems)
	slices.SortStableFunc(sorted, func(a, b domain.Memory) int { return rank(a) - rank(b) })
	out := make([]domain.EditContextMemory, 0, 3)
	for _, m := range sorted {
		if len(out) == 3 {
			break
		}
		out = append(out, domain.EditContextMemory{ID: m.ID, Type: string(m.Type), Title: m.Title})
	}
	return out
}
