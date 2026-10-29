package domain

import (
	"regexp"
	"strings"
)

type StoryState string

const (
	StoryDraft      StoryState = "draft"
	StoryReady      StoryState = "ready"
	StoryInProgress StoryState = "in_progress"
	StorySatisfied  StoryState = "satisfied"
	StoryBlocked    StoryState = "blocked"
	StoryRejected   StoryState = "rejected"
)

type CriterionState string

const (
	CriterionDefined       CriterionState = "defined"
	CriterionCovered       CriterionState = "covered"
	CriterionVerified      CriterionState = "verified"
	CriterionFailed        CriterionState = "failed"
	CriterionBlocked       CriterionState = "blocked"
	CriterionNotApplicable CriterionState = "not_applicable"
)

type VerificationKind string

const (
	VerificationAutomated  VerificationKind = "automated"
	VerificationManual     VerificationKind = "manual"
	VerificationInspection VerificationKind = "inspection"
	VerificationReview     VerificationKind = "review"
	VerificationRealSystem VerificationKind = "real-system"
)

type Story struct {
	ID, Title, Actor, Text string
	State                  StoryState
}

type Criterion struct {
	ID, Title    string
	Stories      []string
	Condition    string
	Verification VerificationKind
	State        CriterionState
}

type Definition struct {
	ID                                                  string
	Kind                                                Kind
	Title, Implementer                                  string
	Approach, Playbook                                  string
	Problem, Actor, ExpectedResult                      string
	Included, Excluded, Restrictions                    string
	ObservedFailure, Reproduction, RootCause            string
	CurrentBehavior, Change, Compatibility              string
	TechnicalProblem, Beneficiary, ImprovementCondition string
	Stories                                             []Story
	Criteria                                            []Criterion
}

func ValidateDefinition(def Definition, kind Kind) []PendingCheck {
	var pending []PendingCheck
	add := func(field, value string) {
		if isHCDPending(value) {
			pending = append(pending, PendingCheck{Code: "DEF-PENDING", Field: field, Message: field + " requiere un valor definido"})
		}
	}
	add("ID", def.ID)
	add("Título", def.Title)
	add("Implementador", def.Implementer)
	add("Problema", def.Problem)
	add("Actor", def.Actor)
	add("Resultado esperado", def.ExpectedResult)
	add("Incluido", def.Included)
	add("Excluido", def.Excluded)
	add("Restricciones", def.Restrictions)
	add("Playbook", def.Playbook)
	if len(def.Stories) == 0 {
		pending = append(pending, PendingCheck{Code: "DEF-STORY", Field: "Historias", Message: "se requiere al menos una historia"})
	}
	for _, story := range def.Stories {
		prefix := story.ID
		if prefix == "" {
			prefix = "Historia"
		}
		if isHCDPending(story.Title) || isHCDPending(story.Actor) || isHCDPending(story.Text) {
			pending = append(pending, PendingCheck{Code: "DEF-PENDING", Field: prefix, Message: "la historia tiene campos pendientes"})
		}
		if len(def.CriteriaForStory(story.ID)) == 0 {
			pending = append(pending, PendingCheck{Code: "DEF-CRITERION", Field: prefix, Message: "la historia requiere al menos un criterio"})
		}
	}
	switch kind {
	case KindFeature:
		add("Actor", def.Actor)
	case KindEvolution:
		add("Comportamiento actual", def.CurrentBehavior)
		add("Cambio", def.Change)
		add("Compatibilidad", def.Compatibility)
	case KindFix:
		add("Fallo observado", def.ObservedFailure)
		add("Reproducción", def.Reproduction)
		if def.RootCause != "pendiente" {
			add("Causa raíz", def.RootCause)
		}
	case KindTechnical:
		add("Problema técnico", def.TechnicalProblem)
		add("Beneficiario", def.Beneficiary)
		add("Condición de mejora", def.ImprovementCondition)
	default:
		pending = append(pending, PendingCheck{Code: "DEF-KIND", Field: "Tipo", Message: "tipo de unidad desconocido"})
	}
	for _, criterion := range def.Criteria {
		if isHCDPending(criterion.ID) || isHCDPending(criterion.Condition) {
			pending = append(pending, PendingCheck{Code: "DEF-PENDING", Field: criterion.ID, Message: "el criterio tiene campos pendientes"})
		}
		if len(criterion.Stories) == 0 || criterion.Verification == "" {
			pending = append(pending, PendingCheck{Code: "DEF-CRITERION", Field: criterion.ID, Message: "el criterio requiere historia y verificación"})
		}
	}
	return pending
}

// CriteriaForStory returns criteria linked by their story ID from the optional
// Criteria collection. Kept as a method to make relationship validation explicit.
func (d Definition) CriteriaForStory(storyID string) []Criterion {
	var out []Criterion
	for _, criterion := range d.Criteria {
		for _, id := range criterion.Stories {
			if id == storyID {
				out = append(out, criterion)
				break
			}
		}
	}
	return out
}

// hcdPlaceholder reconoce los marcadores sin rellenar de las plantillas: «…» suelto
// («Como …, quiero …») y títulos entre llaves («{Condición}»).
var hcdPlaceholder = regexp.MustCompile(`(^|[\s,(|])…($|[\s,.)|])|^\{[^}]*\}$`)

func isHCDPending(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "pendiente" || value == "[pendiente]" || value == "todo" {
		return true
	}
	return strings.HasPrefix(value, "[needs clarification:") || hcdPlaceholder.MatchString(value)
}
