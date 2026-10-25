package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func completeSpecifyContent() SpecifyContent {
	return SpecifyContent{
		Intent: &SpecifyIntent{Objective: "Consultar el historial propio fuera de la aplicación", ExpectedResult: "La persona obtiene un archivo con su historial autorizado",
			Actors: []SpecifyActor{{ID: "A1", Name: "Persona titular del historial"}}},
		Scope: &SpecifyScope{
			Included:    []string{"Exportar el historial propio"},
			Excluded:    DeclaredList{Declared: true, Items: []string{"Historiales de otras personas"}},
			Constraints: DeclaredList{Declared: true, Items: []string{}},
			Assumptions: DeclaredList{Declared: true, Items: []string{"La aplicación ya permite consultar el historial"}},
			Pending: PendingList{Declared: true, Items: []SpecifyPending{
				{ID: "P1", Description: "Formato del archivo", Blocking: false, Resolution: PendingResolved, ResolutionNote: "CSV según la persona"},
			}},
		},
		Stories: &SpecifyStories{
			Stories: []SpecifyStory{{ID: "US1", ActorID: "A1", Need: "Exportar mi historial autorizado", Outcome: "Consultarlo fuera de la aplicación"}},
			Criteria: []SpecifyCriterion{
				{ID: "C1", StoryIDs: []string{"US1"}, Condition: "Tiene registros autorizados", ExpectedResult: "El archivo contiene solo esos registros"},
				{ID: "C2", StoryIDs: []string{"US1"}, Condition: "No tiene registros", ExpectedResult: "Se comunica que no hay registros"},
			},
			Deliveries: []SpecifyDelivery{{ID: "D1", Title: "Exportación del historial propio", Outcome: "La persona obtiene su historial", StoryIDs: []string{"US1"}, CriterionIDs: []string{"C1", "C2"}}},
		},
	}
}

func TestRenderSpecifyPrompt_IsRepeatable(t *testing.T) {
	a, issues := RenderSpecifyPrompt(completeSpecifyContent())
	if len(issues) != 0 || a == "" {
		t.Fatalf("render = %q, issues %v", a, issues)
	}
	b, _ := RenderSpecifyPrompt(completeSpecifyContent())
	if a != b || SpecifyPromptHash(a) != SpecifyPromptHash(b) {
		t.Fatal("same content rendered differently")
	}
}

func TestRenderSpecifyPrompt_KeepsIDsSectionAndListOrder(t *testing.T) {
	c := completeSpecifyContent()
	text, _ := RenderSpecifyPrompt(c)
	order := []string{"A1", "Exportar el historial propio", "Historiales de otras personas", "P1", "US1", "C1", "C2", "D1"}
	last := -1
	for _, want := range order {
		i := strings.Index(text, want)
		if i < 0 || i < last {
			t.Fatalf("%q missing or out of order in:\n%s", want, text)
		}
		last = i
	}
	// The declared order of a list is kept, not sorted.
	c.Stories.Criteria[0], c.Stories.Criteria[1] = c.Stories.Criteria[1], c.Stories.Criteria[0]
	swapped, _ := RenderSpecifyPrompt(c)
	if c2, c1 := strings.Index(swapped, "- C2 ["), strings.Index(swapped, "- C1 ["); c2 < 0 || c1 < 0 || c2 > c1 {
		t.Fatalf("criteria order not preserved:\n%s", swapped)
	}
	if !strings.Contains(text, "Restricciones") || !strings.Contains(text, "ninguna declarada") {
		t.Fatalf("an explicitly empty list must be rendered as declared empty:\n%s", text)
	}
}

func TestRenderSpecifyPrompt_ProtectsBeforeRendering(t *testing.T) {
	c := completeSpecifyContent()
	c.Intent.Objective = "Exportar <private>dato-interno-042</private> el historial"
	c.Stories.Criteria[0].ExpectedResult = "Usa la clave AKIAIOSFODNN7EXAMPLE solo en pruebas"
	text, issues := RenderSpecifyPrompt(c)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	if strings.Contains(text, "dato-interno-042") || strings.Contains(text, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("protected content reached the reviewable text:\n%s", text)
	}
	if text != ProtectText(text) {
		t.Fatal("the rendered text must already be protected, so later persistence cannot change it")
	}
}

func TestRenderSpecifyPrompt_RejectsEssentialDataRemovedByProtection(t *testing.T) {
	c := completeSpecifyContent()
	c.Intent.Objective = "<private>todo el objetivo es privado</private>"
	text, issues := RenderSpecifyPrompt(c)
	if text != "" || len(issues) != 1 || issues[0].Code != "essential_redacted" || issues[0].Path != "intent.objective" {
		t.Fatalf("render = %q, issues %+v; want essential_redacted at intent.objective", text, issues)
	}
	c = completeSpecifyContent()
	c.Stories = nil
	if text, issues = RenderSpecifyPrompt(c); text != "" || len(issues) != 1 || issues[0].Path != "stories" {
		t.Fatalf("render without stories = %q, %+v", text, issues)
	}
}

func TestSpecifyPromptHash_IsExactBytes(t *testing.T) {
	sum := sha256.Sum256([]byte("texto \n"))
	if got := SpecifyPromptHash("texto \n"); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("hash = %s, want sha256 of the exact bytes", got)
	}
	if SpecifyPromptHash("texto") == SpecifyPromptHash("texto \n") {
		t.Fatal("exact hash must distinguish trailing whitespace")
	}
	if SocraticaPromptHash("texto") != SocraticaPromptHash("texto \n") {
		t.Fatal("the provenance hash keeps its normalisation")
	}
}
