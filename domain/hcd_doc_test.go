package domain

import (
	"os"
	"strings"
	"testing"
)

func unitTemplate(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../infrastructure/templates/hcd/unit/unit.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func render(t *testing.T, kind Kind, lane Lane) string {
	t.Helper()
	return RenderUnitTemplate(unitTemplate(t), Unit{ID: "HCD-001", Title: "Guardar falla", Kind: kind, Lane: lane, Why: "motivo"}, "El guardado pierde datos")
}

// fill completa las secciones comunes; full añade una historia, un slice y
// tareas trazables.
func fill(doc string, lane Lane) string {
	r := []string{
		"## Reproducción\npendiente", "## Reproducción\ngo test ./x -run TestSave",
		"Incluido: pendiente", "Incluido: guardado",
		"Excluido: pendiente", "Excluido: exportación",
		"## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nse pierden datos",
		"## Alternativa más barata\npendiente", "## Alternativa más barata\nninguna",
	}
	if lane == LaneFull {
		r = append(r,
			"### US1. pendiente\npendiente\n- C-01: pendiente", "### US1. Guardar\nComo usuario, quiero guardar, para no perder datos\n- C-01: Dado un cambio, cuando guardo, entonces persiste",
		)
	}
	return strings.NewReplacer(r...).Replace(doc)
}

func withSlice(doc string) string {
	return string(ReplaceSection([]byte(doc), SectionSlices, "### VS-01. Guardado\nTareas: 1\nCriterios: C-01", ""))
}

func withTask(doc string) string {
	return strings.Replace(doc, "## Checklist\n", "## Checklist\n- [ ] 1. Persistir el cambio → go test ./x\n", 1)
}

func TestRenderUnitTemplateBlocksByKindAndLane(t *testing.T) {
	fix := render(t, KindFix, LaneFast)
	if !strings.Contains(fix, "## Reproducción") || !strings.Contains(fix, "1. La reproducción ya no falla") || strings.Contains(fix, "## Historias") || strings.Contains(fix, "## Slices") || strings.Contains(fix, "<!-- fix -->") {
		t.Fatalf("fix fast:\n%s", fix)
	}
	full := render(t, KindFeature, LaneFull)
	if strings.Contains(full, "Reproducción") || !strings.Contains(full, "## Historias") || !strings.Contains(full, "## Slices") || strings.Contains(full, "{") || strings.Contains(full, "<!-- full -->") {
		t.Fatalf("feature full:\n%s", full)
	}
}

func TestParseUnitDocListsMissingByLane(t *testing.T) {
	_, pending := ParseUnitDoc([]byte(render(t, KindFix, LaneFast)), KindFix, LaneFast)
	got := map[string]bool{}
	for _, p := range pending {
		got[p.Field] = true
	}
	for _, f := range []string{SectionReproduction, SectionInaction, SectionCheapest, "Alcance.Incluido", "Alcance.Excluido"} {
		if !got[f] {
			t.Errorf("fast fix lacks pending %s: %+v", f, pending)
		}
	}
	if got["US1"] {
		t.Error("fast lane must not require stories")
	}
	_, pending = ParseUnitDoc([]byte(render(t, KindFeature, LaneFull)), KindFeature, LaneFull)
	got = map[string]bool{}
	for _, p := range pending {
		got[p.Code] = true
	}
	if !got["STORY-TEXT"] || !got["CRITERION-TEXT"] {
		t.Fatalf("full lane must require story text and criteria: %+v", pending)
	}
}

func TestParseUnitDocCompleteDocuments(t *testing.T) {
	doc, pending := ParseUnitDoc([]byte(fill(render(t, KindFix, LaneFast), LaneFast)), KindFix, LaneFast)
	if len(pending) != 0 || len(doc.Tasks) != 1 || doc.Tasks[0].Check != "re-ejecutar la reproducción" {
		t.Fatalf("fast fix: %+v %+v", pending, doc.Tasks)
	}
	doc, pending = ParseUnitDoc([]byte(withTask(withSlice(fill(render(t, KindFeature, LaneFull), LaneFull)))), KindFeature, LaneFull)
	if len(pending) != 0 || len(doc.Stories) != 1 || doc.Stories[0].Criteria[0].ID != "C-01" || len(doc.Slices) != 1 || doc.Slices[0].Tasks[0] != "1" || doc.Slices[0].Criteria[0] != "C-01" {
		t.Fatalf("full feature: %+v %+v %+v", pending, doc.Stories, doc.Slices)
	}
}

func TestParseUnitDocRejectsTaskWithoutCheck(t *testing.T) {
	doc := strings.Replace(fill(render(t, KindFix, LaneFast), LaneFast), "\n## Evidencia", "- [ ] 2. Añadir test\n\n## Evidencia", 1)
	_, pending := ParseUnitDoc([]byte(doc), KindFix, LaneFast)
	if len(pending) != 1 || pending[0].Code != "CHECK-MISSING" {
		t.Fatalf("pending = %+v", pending)
	}
}

func TestClarificationsAndMarkers(t *testing.T) {
	base := fill(render(t, KindFix, LaneFast), LaneFast)
	doc, _ := ParseUnitDoc([]byte(base), KindFix, LaneFast)
	data, id := AddClarification([]byte(base), doc, "¿Se guarda también el borrador?")
	if id != "Q1" {
		t.Fatalf("id = %s", id)
	}
	doc, pending := ParseUnitDoc(data, KindFix, LaneFast)
	if len(pending) != 0 || len(doc.OpenClarifications()) != 1 {
		t.Fatalf("open = %v pending = %+v", doc.OpenClarifications(), pending)
	}
	data, err := AnswerClarification(data, "Q1", "sí, también")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ = ParseUnitDoc(data, KindFix, LaneFast)
	if len(doc.OpenClarifications()) != 0 || doc.Clarifications[0].Answer != "sí, también" {
		t.Fatalf("answered = %+v", doc.Clarifications)
	}
	if _, err := AnswerClarification(data, "Q1", "otra"); err == nil {
		t.Fatal("answering twice accepted")
	}
	if _, err := AnswerClarification(data, "Q9", "x"); err == nil {
		t.Fatal("unknown clarification accepted")
	}
	marked := strings.Replace(base, "## Alternativa más barata\nninguna", "## Alternativa más barata\n[NEEDS CLARIFICATION: ¿hay otra?]", 1)
	doc, pending = ParseUnitDoc([]byte(marked), KindFix, LaneFast)
	if len(pending) != 0 || len(doc.OpenClarifications()) != 1 || doc.OpenClarifications()[0] != SectionCheapest {
		t.Fatalf("marker: open = %v pending = %+v", doc.OpenClarifications(), pending)
	}
}

func TestRevisionsSeparateDefinitionFromWork(t *testing.T) {
	base := withTask(withSlice(fill(render(t, KindFeature, LaneFull), LaneFull)))
	parse := func(s string) UnitDoc { d, _ := ParseUnitDoc([]byte(s), KindFeature, LaneFull); return d }
	b := parse(base)
	// Marcar la tarea, reescribir Evidencia y Siguiente no cambia nada.
	same := string(ReplaceSection(ReplaceSection(MarkTasksDone([]byte(base), []string{"1"}), SectionEvidence, "- 1: ok", ""), SectionNext, "cerrar", ""))
	if s := parse(same); DefinitionRevision(s) != DefinitionRevision(b) || WorkRevision(s) != WorkRevision(b) || !s.Tasks[0].Done {
		t.Fatal("checkbox, evidence or next changed a revision")
	}
	// Añadir una tarea cambia el trabajo, no la definición aprobada.
	more := parse(strings.Replace(base, "## Checklist\n", "## Checklist\n- [ ] 2. Otra → go vet\n", 1))
	if DefinitionRevision(more) != DefinitionRevision(b) || WorkRevision(more) == WorkRevision(b) {
		t.Fatal("a new task must change only the work revision")
	}
	// Cambiar el alcance cambia ambas.
	scope := parse(strings.Replace(base, "Incluido: guardado", "Incluido: guardado y carga", 1))
	if DefinitionRevision(scope) == DefinitionRevision(b) || WorkRevision(scope) == WorkRevision(b) {
		t.Fatal("a scope change must change both revisions")
	}
}

func TestReplaceSectionInsertsBefore(t *testing.T) {
	doc := fill(render(t, KindFix, LaneFast), LaneFast)
	out := string(ReplaceSection([]byte(doc), SectionStories, FullSectionsTemplate, SectionClarifications))
	if strings.Index(out, "## Historias") > strings.Index(out, "## Aclaraciones") || !strings.Contains(out, "### US1.") {
		t.Fatalf("out:\n%s", out)
	}
	out = string(ReplaceSection([]byte(doc), SectionEvidence, "- 1: ok", ""))
	if !strings.Contains(out, "## Evidencia\n- 1: ok\n\n## Siguiente") {
		t.Fatalf("out:\n%s", out)
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{"Corregir guardado ñandú": "corregir-guardado-nandu", "  ¿Qué?  ": "que", "***": "unidad"} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
