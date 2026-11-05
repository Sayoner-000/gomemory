package domain

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// Contexto de edición (HCD-028): al ir a editar un archivo, el agente recibe
// lo que el proyecto ya sabe de él — decisiones ancladas, pruebas que lo
// cubren, la sección de la Constitución de su capa y su impacto —, solo con
// punteros y con tope propio (Q1).

// EditContextMaxRunes es el tope del contexto de edición (Q1): mayor que el
// de los avisos de expand porque agrupa cuatro fuentes, y acotado para no
// competir con la tarea.
const EditContextMaxRunes = 1200

// Límites por fuente (Q1).
const (
	editContextMaxMemories = 3
	editContextMaxTests    = 3
	editContextTitleRunes  = 90
)

// ConstitutionSection identifica una sección de la Constitución técnica.
type ConstitutionSection struct {
	Number int
	Title  string
}

// ConstitutionSectionFor asigna a una ruta la sección de la Constitución que
// rige su capa. Lo más específico va primero: una prueba de dominio es una
// prueba (§10) antes que dominio (§3). Las rutas sin capa de código
// (documentación, CI) no tienen sección.
func ConstitutionSectionFor(p string) (ConstitutionSection, bool) {
	p = "/" + strings.TrimPrefix(path.Clean(strings.ReplaceAll(p, `\`, "/")), "/")
	has := func(dir string) bool { return strings.Contains(p, "/"+dir+"/") }
	ext := path.Ext(p)
	switch {
	case strings.HasSuffix(p, "_test.go") || has("tests") || has("test") || strings.Contains(p, ".test."):
		return ConstitutionSection{10, "Pruebas"}, true
	case has("migrations"):
		return ConstitutionSection{6, "Migraciones"}, true
	case has("persistence") || has("repository") || has("repositories"):
		return ConstitutionSection{5, "Persistencia y transacciones"}, true
	case has("domain") || has("application") || has("adapters") || has("usecases") || has("ports"):
		return ConstitutionSection{3, "Arquitectura hexagonal obligatoria"}, true
	case has("infrastructure"):
		return ConstitutionSection{4, "Composition root"}, true
	case ext == ".tsx" || ext == ".jsx" || ext == ".vue" || ext == ".svelte" || ext == ".css" ||
		((ext == ".ts" || ext == ".js") && (has("web") || has("frontend") || has("src"))):
		return ConstitutionSection{14, "Frontend"}, true
	}
	return ConstitutionSection{}, false
}

// EditContextMemory es una memoria anclada a la ruta, solo con lo que se cita.
type EditContextMemory struct {
	ID    int64
	Type  string
	Title string
}

// EditContextTest es una prueba que cubre la ruta y el paquete donde corre.
type EditContextTest struct {
	Name string
	Dir  string
}

// EditContextInput reúne lo que las fuentes encontraron para la ruta.
type EditContextInput struct {
	Path     string
	Memories []EditContextMemory
	Tests    []EditContextTest
	Impact   *CodeImpactAnnotation
}

// RenderEditContext compone el contexto de edición en orden fijo — memorias,
// pruebas, Constitución e impacto —, con tope EditContextMaxRunes. Devuelve ""
// si no hay nada que aportar.
func RenderEditContext(in EditContextInput) string {
	var lines []string
	for i, m := range in.Memories {
		if i == editContextMaxMemories {
			break
		}
		lines = append(lines, fmt.Sprintf("- [%d] %s · %s → get_memory %d", m.ID, m.Type, truncRunes(m.Title, editContextTitleRunes), m.ID))
	}
	if len(in.Tests) > 0 {
		var names []string
		dir := in.Tests[0].Dir
		for _, t := range in.Tests {
			if len(names) == editContextMaxTests {
				break
			}
			if t.Dir == dir {
				names = append(names, t.Name)
			}
		}
		lines = append(lines, fmt.Sprintf("- Pruebas que lo cubren (ponlas en rojo antes de cambiarlo): go test %s -run '%s'", dir, strings.Join(names, "|")))
	}
	section, hasSection := ConstitutionSectionFor(in.Path)
	if hasSection {
		lines = append(lines, fmt.Sprintf("- Constitución §%d %s → mem constitution", section.Number, section.Title))
	}
	if in.Impact != nil && in.Impact.Hotspot {
		lines = append(lines, fmt.Sprintf("- Impacto: %s es un hotspot (fan-in %d); revisa sus llamadores", in.Impact.Symbol, in.Impact.FanIn))
	}
	if len(in.Memories) == 0 && len(in.Tests) == 0 && (in.Impact == nil || !in.Impact.Hotspot) && !hasSection {
		return ""
	}
	out := "Contexto de " + in.Path + " (gomemory):\n" + strings.Join(lines, "\n")
	return truncRunes(out, EditContextMaxRunes)
}

// truncRunes recorta s a n caracteres sin partir un carácter UTF-8.
func truncRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
