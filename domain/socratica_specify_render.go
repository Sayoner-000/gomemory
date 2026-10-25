package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// SpecifyPromptHash is the SHA-256 of the exact UTF-8 bytes of the reviewed
// text. Unlike SocraticaPromptHash (trimmed, for hook provenance) it proves
// identity byte for byte between review, approval and delivery.
func SpecifyPromptHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// specifyRenderer protects each text before writing it, so the reviewed text
// never changes when it is later persisted, and records the essential fields
// that protection emptied.
type specifyRenderer struct {
	b      strings.Builder
	issues []SpecifyIssue
}

func (r *specifyRenderer) text(path, s string) string {
	safe := strings.TrimSpace(ProtectText(s))
	if safe == "" && strings.TrimSpace(s) != "" {
		r.issues = append(r.issues, SpecifyIssue{Code: "essential_redacted", Path: path, Message: "la protección de privacidad retiró un dato esencial; reformúlalo sin contenido privado"})
	}
	// Continuation lines stay inside their list item.
	return strings.ReplaceAll(safe, "\n", "\n  ")
}

func (r *specifyRenderer) line(parts ...string) {
	for _, p := range parts {
		r.b.WriteString(p)
	}
	r.b.WriteByte('\n')
}

func (r *specifyRenderer) list(title, path string, l DeclaredList) {
	r.line("### ", title)
	if len(l.Items) == 0 {
		r.line("- (ninguna declarada)")
	}
	for i, item := range l.Items {
		r.line("- ", r.text(path+".items["+strconv.Itoa(i)+"]", item))
	}
}

// ids renders a reference list, protected like any other text.
func (r *specifyRenderer) ids(path string, ids []string) string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = r.text(path+"["+strconv.Itoa(i)+"]", id)
	}
	return strings.Join(out, ", ")
}

// RenderSpecifyPrompt produces the reviewable entry for speckit-specify from
// complete content: fixed section order, declared list order and stable ids.
// It returns no text when a section is missing or protection removed an
// essential field; the issues say which.
func RenderSpecifyPrompt(c SpecifyContent) (string, []SpecifyIssue) {
	for _, s := range []struct {
		path    string
		missing bool
	}{{"intent", c.Intent == nil}, {"scope", c.Scope == nil}, {"stories", c.Stories == nil}} {
		if s.missing {
			return "", []SpecifyIssue{{Code: "section_missing", Path: s.path, Message: "falta la sección " + s.path}}
		}
	}
	r := &specifyRenderer{}
	in, sc, st := c.Intent, c.Scope, c.Stories
	r.line("# Necesidad preparada para especificar")
	r.line()
	r.line("## Intención")
	r.line("- Objetivo: ", r.text("intent.objective", in.Objective))
	r.line("- Resultado esperado: ", r.text("intent.expected_result", in.ExpectedResult))
	r.line("- Actores:")
	for i, a := range in.Actors {
		r.line("  - ", r.text("intent.actors["+strconv.Itoa(i)+"].id", a.ID), ": ", r.text("intent.actors["+strconv.Itoa(i)+"].name", a.Name))
	}
	r.line()
	r.line("## Alcance")
	r.line("### Incluido")
	for i, item := range sc.Included {
		r.line("- ", r.text("scope.included["+strconv.Itoa(i)+"]", item))
	}
	r.list("Excluido", "scope.excluded", sc.Excluded)
	r.list("Restricciones", "scope.constraints", sc.Constraints)
	r.list("Supuestos", "scope.assumptions", sc.Assumptions)
	r.line("### Pendientes")
	if len(sc.Pending.Items) == 0 {
		r.line("- (ninguna declarada)")
	}
	for i, p := range sc.Pending.Items {
		kind, state := "no bloqueante", "abierto"
		if p.Blocking {
			kind = "bloqueante"
		}
		if p.Resolution == PendingResolved {
			state = "resuelto"
		}
		path := "scope.pending.items[" + strconv.Itoa(i) + "]"
		entry := "- " + r.text(path+".id", p.ID) + " (" + kind + ", " + state + "): " + r.text(path+".description", p.Description)
		if p.ResolutionNote != "" {
			entry += " — Resolución: " + r.text(path+".resolution_note", p.ResolutionNote)
		}
		r.line(entry)
	}
	r.line()
	r.line("## Historias de usuario")
	for i, s := range st.Stories {
		path := "stories.stories[" + strconv.Itoa(i) + "]"
		r.line("- ", r.text(path+".id", s.ID), " (actor ", r.text(path+".actor_id", s.ActorID), "): ", r.text(path+".need", s.Need), " → ", r.text(path+".outcome", s.Outcome))
	}
	r.line()
	r.line("## Criterios de aceptación")
	for i, cr := range st.Criteria {
		path := "stories.criteria[" + strconv.Itoa(i) + "]"
		r.line("- ", r.text(path+".id", cr.ID), " [", r.ids(path+".story_ids", cr.StoryIDs), "]: Condición: ", r.text(path+".condition", cr.Condition), " → Resultado esperado: ", r.text(path+".expected_result", cr.ExpectedResult))
	}
	r.line()
	r.line("## Entregas")
	for i, d := range st.Deliveries {
		path := "stories.deliveries[" + strconv.Itoa(i) + "]"
		r.line("- ", r.text(path+".id", d.ID), ": ", r.text(path+".title", d.Title), " — ", r.text(path+".outcome", d.Outcome), " (historias: ", r.ids(path+".story_ids", d.StoryIDs), "; criterios: ", r.ids(path+".criterion_ids", d.CriterionIDs), ")")
	}
	if len(r.issues) > 0 {
		return "", r.issues
	}
	return r.b.String(), nil
}
