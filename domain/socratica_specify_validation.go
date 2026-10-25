package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ErrBlockingPending: an edit would remove or downgrade an open blocking
// pending without the note that identifies the decision or evidence.
var ErrBlockingPending = errors.New("blocking_pending")

// Codes of SpecifyIssue. Messages never contain the user's text, only paths.
const (
	IssueMissing          = "missing"
	IssuePlaceholder      = "placeholder"
	IssueEssentialMarker  = "essential_marker"
	IssueNotDeclared      = "not_declared"
	IssueInvalidValue     = "invalid_value"
	IssueDuplicateID      = "duplicate_id"
	IssueInvalidReference = "invalid_reference"
	IssueUncoveredStory   = "uncovered_story"
	IssueStoryNoDelivery  = "story_without_delivery"
	IssueForeignCriterion = "foreign_criterion"
	IssueDeliveryGap      = "delivery_gap"
	IssueBlockingPending  = "blocking_pending"
	IssueSectionMissing   = "section_missing"
	IssueUnsupportedStage = "unsupported_stage"
)

var fillerTexts = map[string]bool{
	"tbd": true, "tba": true, "n/a": true, "na": true, "xxx": true, "lorem ipsum": true,
	"por definir": true, "a definir": true, "pendiente": true, "pendiente de definir": true,
}

// markerPattern finds a standalone bracketed placeholder such as [formato].
// Indexes (items[0]), links ([x](y)) and the privacy label are filtered below.
var markerPattern = regexp.MustCompile(`(^|[^\p{L}\p{N}\])])\[([^\[\]\n]{1,80})\]`)

func isFiller(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	if fillerTexts[t] {
		return true
	}
	return !strings.ContainsFunc(t, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) })
}

func hasEssentialMarker(s string) bool {
	for _, m := range markerPattern.FindAllStringSubmatchIndex(s, -1) {
		inner, end := s[m[4]:m[5]], m[1]
		if strings.HasPrefix(inner, "REDACTED:") || (end < len(s) && s[end] == '(') {
			continue
		}
		return true
	}
	return false
}

type specifyValidator struct{ issues []SpecifyIssue }

func (v *specifyValidator) add(code, path, message string) {
	v.issues = append(v.issues, SpecifyIssue{Code: code, Path: path, Message: message})
}

// text reports one problem at most per field: blank, filler or marker. A
// field in any of those states does not count as resolved information.
func (v *specifyValidator) text(path, s string) {
	switch {
	case strings.TrimSpace(s) == "":
		v.add(IssueMissing, path, "falta información en "+path)
	case isFiller(s):
		v.add(IssuePlaceholder, path, "texto de relleno en "+path+"; descríbelo o decláralo pendiente")
	case hasEssentialMarker(s):
		v.add(IssueEssentialMarker, path, "marcador sin resolver en "+path+"; resuélvelo o regístralo como pendiente bloqueante")
	}
}

func (v *specifyValidator) id(path, id string, seen map[string]bool) {
	switch {
	case strings.TrimSpace(id) == "":
		v.add(IssueMissing, path, "falta el identificador en "+path)
	case seen[id]:
		v.add(IssueDuplicateID, path, "identificador repetido en "+path)
	default:
		seen[id] = true
	}
}

func (v *specifyValidator) declared(path string, l DeclaredList) {
	if !l.Declared {
		v.add(IssueNotDeclared, path, "declara "+path+" con declared=true; usa items=[] si no se identifica ninguno")
		return
	}
	for i, item := range l.Items {
		v.text(path+".items["+strconv.Itoa(i)+"]", item)
	}
}

func (v *specifyValidator) intent(i *SpecifyIntent) {
	if i == nil {
		v.add(IssueSectionMissing, "intent", "falta la sección intent")
		return
	}
	v.text("intent.objective", i.Objective)
	v.text("intent.expected_result", i.ExpectedResult)
	if len(i.Actors) == 0 {
		v.add(IssueMissing, "intent.actors", "falta al menos un actor")
	}
	seen := map[string]bool{}
	for n, a := range i.Actors {
		p := "intent.actors[" + strconv.Itoa(n) + "]"
		v.id(p+".id", a.ID, seen)
		v.text(p+".name", a.Name)
	}
}

func (v *specifyValidator) scope(s *SpecifyScope) {
	if s == nil {
		v.add(IssueSectionMissing, "scope", "falta la sección scope")
		return
	}
	if len(s.Included) == 0 {
		v.add(IssueMissing, "scope.included", "describe qué queda incluido en el alcance")
	}
	for n, item := range s.Included {
		v.text("scope.included["+strconv.Itoa(n)+"]", item)
	}
	v.declared("scope.excluded", s.Excluded)
	v.declared("scope.constraints", s.Constraints)
	v.declared("scope.assumptions", s.Assumptions)
	if !s.Pending.Declared {
		v.add(IssueNotDeclared, "scope.pending", "declara scope.pending con declared=true; usa items=[] si no hay pendientes")
		return
	}
	seen := map[string]bool{}
	for n, p := range s.Pending.Items {
		path := "scope.pending.items[" + strconv.Itoa(n) + "]"
		v.id(path+".id", p.ID, seen)
		v.text(path+".description", p.Description)
		switch p.Resolution {
		case PendingOpen:
		case PendingResolved:
			if strings.TrimSpace(p.ResolutionNote) == "" {
				v.add(IssueMissing, path+".resolution_note", "un pendiente resuelto necesita la nota con la decisión o la evidencia")
			}
		default:
			v.add(IssueInvalidValue, path+".resolution", "resolution debe ser open o resolved")
		}
	}
}

// refs checks that every id of list exists in known and reports the position.
// refs checks that every reference exists and appears once (uniqueItems in
// preparation.schema.json).
func (v *specifyValidator) refs(path string, list []string, known map[string]bool) {
	seen := map[string]bool{}
	for n, id := range list {
		at := path + "[" + strconv.Itoa(n) + "]"
		switch {
		case !known[id]:
			v.add(IssueInvalidReference, at, "referencia inexistente")
		case seen[id]:
			v.add(IssueDuplicateID, at, "referencia repetida")
		}
		seen[id] = true
	}
}

func (v *specifyValidator) stories(c SpecifyContent) {
	st := c.Stories
	if st == nil {
		v.add(IssueSectionMissing, "stories", "falta la sección stories")
		return
	}
	actors := map[string]bool{}
	if c.Intent == nil {
		v.add(IssueSectionMissing, "intent", "falta la sección intent: sin sus actores no se pueden comprobar las historias")
	} else {
		for _, a := range c.Intent.Actors {
			actors[a.ID] = true
		}
	}
	if len(st.Stories) == 0 {
		v.add(IssueMissing, "stories.stories", "falta al menos una historia")
	}
	if len(st.Criteria) == 0 {
		v.add(IssueMissing, "stories.criteria", "falta al menos un criterio verificable")
	}
	if len(st.Deliveries) == 0 {
		v.add(IssueMissing, "stories.deliveries", "falta al menos una entrega")
	}
	storyIDs, criterionIDs := map[string]bool{}, map[string]bool{}
	for n, s := range st.Stories {
		p := "stories.stories[" + strconv.Itoa(n) + "]"
		v.id(p+".id", s.ID, storyIDs)
		if c.Intent != nil && !actors[s.ActorID] {
			v.add(IssueInvalidReference, p+".actor_id", "el actor de la historia no existe en intent.actors")
		}
		v.text(p+".need", s.Need)
		v.text(p+".outcome", s.Outcome)
	}
	deliveryIDs := map[string]bool{}
	covered := map[string]bool{} // stories with at least one criterion
	related := map[string]map[string]bool{}
	for n, cr := range st.Criteria {
		p := "stories.criteria[" + strconv.Itoa(n) + "]"
		v.id(p+".id", cr.ID, criterionIDs)
		if len(cr.StoryIDs) == 0 {
			v.add(IssueMissing, p+".story_ids", "el criterio debe relacionarse con al menos una historia")
		}
		v.refs(p+".story_ids", cr.StoryIDs, storyIDs)
		v.text(p+".condition", cr.Condition)
		v.text(p+".expected_result", cr.ExpectedResult)
		related[cr.ID] = map[string]bool{}
		for _, sid := range cr.StoryIDs {
			covered[sid] = true
			related[cr.ID][sid] = true
		}
	}
	inDelivery := map[string]bool{}
	for n, d := range st.Deliveries {
		p := "stories.deliveries[" + strconv.Itoa(n) + "]"
		v.id(p+".id", d.ID, deliveryIDs)
		v.text(p+".title", d.Title)
		v.text(p+".outcome", d.Outcome)
		if len(d.StoryIDs) == 0 {
			v.add(IssueMissing, p+".story_ids", "la entrega debe relacionarse con al menos una historia")
		}
		if len(d.CriterionIDs) == 0 {
			v.add(IssueMissing, p+".criterion_ids", "la entrega debe relacionarse con al menos un criterio")
		}
		v.refs(p+".story_ids", d.StoryIDs, storyIDs)
		v.refs(p+".criterion_ids", d.CriterionIDs, criterionIDs)
		deliveryStories := map[string]bool{}
		for _, sid := range d.StoryIDs {
			deliveryStories[sid] = true
			inDelivery[sid] = true
		}
		checked := map[string]bool{}
		for k, cid := range d.CriterionIDs {
			if !criterionIDs[cid] {
				continue
			}
			foreign := true
			for sid := range related[cid] {
				if deliveryStories[sid] {
					foreign, checked[sid] = false, true
				}
			}
			if foreign {
				v.add(IssueForeignCriterion, p+".criterion_ids["+strconv.Itoa(k)+"]", "el criterio no comprueba ninguna historia de esta entrega")
			}
		}
		for k, sid := range d.StoryIDs {
			if storyIDs[sid] && !checked[sid] {
				v.add(IssueDeliveryGap, p+".story_ids["+strconv.Itoa(k)+"]", "ningún criterio de la entrega comprueba esta historia")
			}
		}
	}
	for n, s := range st.Stories {
		p := "stories.stories[" + strconv.Itoa(n) + "]"
		if storyIDs[s.ID] && !covered[s.ID] {
			v.add(IssueUncoveredStory, p, "la historia necesita al menos un criterio verificable")
		}
		if storyIDs[s.ID] && !inDelivery[s.ID] {
			v.add(IssueStoryNoDelivery, p, "la historia debe participar en una entrega")
		}
	}
}

// ValidateSpecifyStage checks the content a stage depends on and returns the
// missing or invalid items, in a stable order. It never invents business
// decisions: ambiguity of meaning is for the human review. The validation
// stage adds the open blocking pendings and runs every previous check.
func ValidateSpecifyStage(c SpecifyContent, stage SpecifyStage) []SpecifyIssue {
	v := &specifyValidator{}
	switch stage {
	case StageIntent:
		v.intent(c.Intent)
	case StageScope:
		v.scope(c.Scope)
	case StageStories:
		v.stories(c)
	case StageValidation:
		v.intent(c.Intent)
		v.scope(c.Scope)
		v.stories(c)
		if c.Scope != nil {
			for n, p := range c.Scope.Pending.Items {
				if p.Blocking && p.Resolution == PendingOpen {
					v.add(IssueBlockingPending, "scope.pending.items["+strconv.Itoa(n)+"]", "hay un pendiente bloqueante abierto; resuélvelo con una nota antes de revisar")
				}
			}
		}
	default:
		v.add(IssueUnsupportedStage, string(stage), "etapa no soportada")
	}
	return v.issues
}

// SpecifyInputHash fingerprints the projection a stage check used. It covers
// the sections the stage depends on and no more, so editing stories keeps the
// intent and scope checks current.
func SpecifyInputHash(c SpecifyContent, stage SpecifyStage) string {
	var projection SpecifyContent
	switch stage {
	case StageIntent:
		projection.Intent = c.Intent
	case StageScope:
		projection.Intent, projection.Scope = c.Intent, c.Scope
	default:
		projection = c
	}
	raw, _ := json.Marshal(projection) // fixed field order: deterministic
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// CheckPendingContinuity refuses to drop or downgrade an open blocking pending
// silently. To remove one it must be resolved, or reclassified, with a note
// that identifies the decision or evidence.
func CheckPendingContinuity(old, next *SpecifyScope) error {
	if old == nil {
		return nil
	}
	var problems []string
	for i, p := range old.Pending.Items {
		if !p.Blocking || p.Resolution != PendingOpen {
			continue
		}
		path := "scope.pending.items[" + strconv.Itoa(i) + "] (" + p.ID + ")"
		var current *SpecifyPending
		if next != nil {
			for j := range next.Pending.Items {
				if next.Pending.Items[j].ID == p.ID {
					current = &next.Pending.Items[j]
					break
				}
			}
		}
		switch {
		case current == nil:
			problems = append(problems, path+" fue eliminado sin resolverlo")
		case current.Blocking && current.Resolution == PendingOpen:
		case strings.TrimSpace(current.ResolutionNote) == "":
			problems = append(problems, path+" cambió sin la nota con la decisión o la evidencia")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrBlockingPending, strings.Join(problems, "; "))
	}
	return nil
}
