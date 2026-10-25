package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// ModeSpecify prepares a need for speckit-specify (feature 042). Its content is
// structured and validated by gomemory; refine/scrutiny keep their contracts.
const ModeSpecify SocraticaMode = "specify"

// PayloadPolicy is fixed when a flow is created and never changes afterwards,
// even if the project's private setting is toggled later.
type PayloadPolicy string

const (
	PayloadPersistent PayloadPolicy = "persistent"
	PayloadEphemeral  PayloadPolicy = "ephemeral"
)

// PayloadPolicyFor: only a private specify flow keeps its texts in memory;
// every other flow, including private refine/scrutiny, stays persistent.
func PayloadPolicyFor(mode SocraticaMode, private bool) PayloadPolicy {
	if mode == ModeSpecify && private {
		return PayloadEphemeral
	}
	return PayloadPersistent
}

// EffectivePayloadPolicy treats rows written before the policy existed as
// persistent, which is how they were stored.
func (f RefinementFlow) EffectivePayloadPolicy() PayloadPolicy {
	if f.PayloadPolicy == "" {
		return PayloadPersistent
	}
	return f.PayloadPolicy
}

type SpecifyStage string

const (
	StageIntent     SpecifyStage = "intent"
	StageScope      SpecifyStage = "scope"
	StageStories    SpecifyStage = "stories"
	StageValidation SpecifyStage = "validation"
	StageReview     SpecifyStage = "review"
	StageApproved   SpecifyStage = "approved"
	StageDelivery   SpecifyStage = "delivery"
)

// PreparationSchemaVersion versions the preparation aggregate, independently
// of the SQLite catalog. A newer version is kept but never mutated.
const PreparationSchemaVersion = 1

// ErrUnsupportedPreparation: the stored aggregate is newer than this binary.
// It is kept as is and refused on mutation, never overwritten.
var ErrUnsupportedPreparation = errors.New("socratica: unsupported preparation version")

// ErrPrivateStateUnavailable: the ephemeral payload of a private flow is not
// in this instance. It is never rebuilt from input nor turned into permission.
var ErrPrivateStateUnavailable = errors.New("private_state_unavailable")

const (
	PendingOpen     = "open"
	PendingResolved = "resolved"
)

const (
	OutcomeCompleted = "completed"
	OutcomeFailed    = "failed"
	OutcomeUncertain = "uncertain"
)

// SpecifyPreparation is the optional aggregate of a specify flow. Checks,
// Review and Stage are computed by gomemory, never accepted from the agent.
type SpecifyPreparation struct {
	SchemaVersion int             `json:"schema_version"`
	Stage         SpecifyStage    `json:"stage"`
	Content       SpecifyContent  `json:"content"`
	Checks        []SpecifyCheck  `json:"checks,omitempty"`
	Review        *SpecifyReview  `json:"review,omitempty"`
	Outcome       *SpecifyOutcome `json:"outcome,omitempty"`
}

// SpecifyContent sections may be missing while the draft is completed.
type SpecifyContent struct {
	Intent  *SpecifyIntent  `json:"intent,omitempty"`
	Scope   *SpecifyScope   `json:"scope,omitempty"`
	Stories *SpecifyStories `json:"stories,omitempty"`
}

type SpecifyIntent struct {
	Objective      string         `json:"objective"`
	ExpectedResult string         `json:"expected_result"`
	Actors         []SpecifyActor `json:"actors"`
}

type SpecifyActor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DeclaredList: Declared=true with no Items states explicitly that nothing
// was identified; an undeclared list is still missing.
type DeclaredList struct {
	Declared bool     `json:"declared"`
	Items    []string `json:"items"`
}

type SpecifyPending struct {
	ID             string `json:"id"`
	Description    string `json:"description"`
	Blocking       bool   `json:"blocking"`
	Resolution     string `json:"resolution"`
	ResolutionNote string `json:"resolution_note,omitempty"`
}

type PendingList struct {
	Declared bool             `json:"declared"`
	Items    []SpecifyPending `json:"items"`
}

type SpecifyScope struct {
	Included    []string     `json:"included"`
	Excluded    DeclaredList `json:"excluded"`
	Constraints DeclaredList `json:"constraints"`
	Assumptions DeclaredList `json:"assumptions"`
	Pending     PendingList  `json:"pending"`
}

type SpecifyStory struct {
	ID      string `json:"id"`
	ActorID string `json:"actor_id"`
	Need    string `json:"need"`
	Outcome string `json:"outcome"`
}

type SpecifyCriterion struct {
	ID             string   `json:"id"`
	StoryIDs       []string `json:"story_ids"`
	Condition      string   `json:"condition"`
	ExpectedResult string   `json:"expected_result"`
}

type SpecifyDelivery struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Outcome      string   `json:"outcome"`
	StoryIDs     []string `json:"story_ids"`
	CriterionIDs []string `json:"criterion_ids"`
}

type SpecifyStories struct {
	Stories    []SpecifyStory     `json:"stories"`
	Criteria   []SpecifyCriterion `json:"criteria"`
	Deliveries []SpecifyDelivery  `json:"deliveries"`
}

// SpecifyIssue is one entry of pending_checks: a missing field or a block.
type SpecifyIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SpecifyCheck records a stage check against the hash of the projection it
// used; a check whose InputHash is stale does not enable the next stage.
type SpecifyCheck struct {
	Stage     SpecifyStage   `json:"stage"`
	InputHash string         `json:"input_hash"`
	Passed    bool           `json:"passed"`
	Missing   []SpecifyIssue `json:"missing,omitempty"`
}

// SpecifyReview binds the reviewed text to its revision. PromptHash is the
// SHA-256 of the exact UTF-8 bytes, unlike the trimmed SocraticaPromptHash.
type SpecifyReview struct {
	Revision   int    `json:"revision"`
	PromptHash string `json:"prompt_hash"`
}

// SpecifyOutcome is what the agent reports after a consumed dispatch. It is
// information, never permission to execute.
type SpecifyOutcome struct {
	DispatchID    string         `json:"dispatch_id"`
	Status        string         `json:"status"`
	SpecPath      string         `json:"spec_path,omitempty"`
	ChecklistPath string         `json:"checklist_path,omitempty"`
	Traceability  []SpecifyTrace `json:"traceability,omitempty"`
	Message       string         `json:"message,omitempty"`
}

type SpecifyTrace struct {
	CriterionID   string   `json:"criterion_id"`
	StoryIDs      []string `json:"story_ids"`
	SpecReference string   `json:"spec_reference"`
}

// NewSpecifyPreparation is the initial aggregate created by start.
func NewSpecifyPreparation() *SpecifyPreparation {
	return &SpecifyPreparation{SchemaVersion: PreparationSchemaVersion, Stage: StageIntent}
}

// Clone deep-copies the aggregate so a pure transition never mutates the
// flow it received.
func (p *SpecifyPreparation) Clone() *SpecifyPreparation {
	if p == nil {
		return nil
	}
	c := *p
	c.Content = p.Content.Clone()
	c.Checks = make([]SpecifyCheck, len(p.Checks))
	for i, ch := range p.Checks {
		c.Checks[i] = ch
		c.Checks[i].Missing = slices.Clone(ch.Missing)
	}
	if p.Checks == nil {
		c.Checks = nil
	}
	if p.Review != nil {
		r := *p.Review
		c.Review = &r
	}
	if p.Outcome != nil {
		o := *p.Outcome
		o.Traceability = make([]SpecifyTrace, len(p.Outcome.Traceability))
		for i, t := range p.Outcome.Traceability {
			o.Traceability[i] = t
			o.Traceability[i].StoryIDs = slices.Clone(t.StoryIDs)
		}
		if p.Outcome.Traceability == nil {
			o.Traceability = nil
		}
		c.Outcome = &o
	}
	return &c
}

func (c SpecifyContent) Clone() SpecifyContent {
	var out SpecifyContent
	if c.Intent != nil {
		i := *c.Intent
		i.Actors = slices.Clone(c.Intent.Actors)
		out.Intent = &i
	}
	if c.Scope != nil {
		s := *c.Scope
		s.Included = slices.Clone(c.Scope.Included)
		s.Excluded.Items = slices.Clone(c.Scope.Excluded.Items)
		s.Constraints.Items = slices.Clone(c.Scope.Constraints.Items)
		s.Assumptions.Items = slices.Clone(c.Scope.Assumptions.Items)
		s.Pending.Items = slices.Clone(c.Scope.Pending.Items)
		out.Scope = &s
	}
	if c.Stories != nil {
		s := SpecifyStories{Stories: slices.Clone(c.Stories.Stories)}
		if c.Stories.Criteria != nil {
			s.Criteria = make([]SpecifyCriterion, len(c.Stories.Criteria))
			for i, cr := range c.Stories.Criteria {
				s.Criteria[i] = cr
				s.Criteria[i].StoryIDs = slices.Clone(cr.StoryIDs)
			}
		}
		if c.Stories.Deliveries != nil {
			s.Deliveries = make([]SpecifyDelivery, len(c.Stories.Deliveries))
			for i, d := range c.Stories.Deliveries {
				s.Deliveries[i] = d
				s.Deliveries[i].StoryIDs = slices.Clone(d.StoryIDs)
				s.Deliveries[i].CriterionIDs = slices.Clone(d.CriterionIDs)
			}
		}
		out.Stories = &s
	}
	return out
}

// MaxSocraticaQuestions is the cumulative question budget of one flow.
const MaxSocraticaQuestions = 3

// ErrStageBlocked: the requested stage cannot be checked yet because an earlier
// stage has no current passing check, or the stage is not checkable at all.
var ErrStageBlocked = errors.New("stage_blocked")

// ErrOutcomeConflict: a different outcome was already communicated for this
// dispatch; changing it needs an explicit correction.
var ErrOutcomeConflict = errors.New("outcome_conflict")

// ErrReviewStale: the content, checks, revision or text no longer match what
// was reviewed. The caller must check and review again; nothing is approved.
var ErrReviewStale = errors.New("review_stale")

// ErrRevisionConflict: the agent edited from a revision that is no longer the
// current one. The caller must read the flow again; nothing is applied.
var ErrRevisionConflict = errors.New("revision_conflict")

// specifyStageRank orders the preparation stages; unknown or later stages
// (review, approved, delivery) rank after validation.
func specifyStageRank(s SpecifyStage) int {
	switch s {
	case StageIntent:
		return 0
	case StageScope:
		return 1
	case StageStories:
		return 2
	case StageValidation:
		return 3
	}
	return 4
}

// ParseSpecifySection decodes the value of prepare(set) for one section. Unknown
// fields are rejected, which also refuses computed data (checks, stage, hashes,
// permissions) smuggled inside the value. Incomplete drafts are accepted: their
// gaps are reported by the stage checks, not here.
func ParseSpecifySection(section SpecifyStage, raw []byte) (SpecifyContent, error) {
	var c SpecifyContent
	var target any
	switch section {
	case StageIntent:
		c.Intent = &SpecifyIntent{}
		target = c.Intent
	case StageScope:
		c.Scope = &SpecifyScope{}
		target = c.Scope
	case StageStories:
		c.Stories = &SpecifyStories{}
		target = c.Stories
	default:
		return SpecifyContent{}, fmt.Errorf("unsupported section %q", section)
	}
	// A missing or null value would decode into an empty section and wipe the
	// stored one: value must be the whole section, as an object.
	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] != '{' {
		return SpecifyContent{}, fmt.Errorf("invalid %s section: value must be an object with the whole section", section)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return SpecifyContent{}, fmt.Errorf("invalid %s section: %w", section, err)
	}
	if dec.More() {
		return SpecifyContent{}, fmt.Errorf("invalid %s section: trailing data", section)
	}
	return c, nil
}

// ProtectSpecifyContent applies ProtectText to every textual leaf, so what is
// stored, checked, rendered and reviewed is the same protected content.
func ProtectSpecifyContent(c SpecifyContent) (SpecifyContent, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return SpecifyContent{}, err
	}
	var tree any
	if err = json.Unmarshal(raw, &tree); err != nil {
		return SpecifyContent{}, err
	}
	var protect func(any) any
	protect = func(v any) any {
		switch x := v.(type) {
		case string:
			return ProtectText(x)
		case []any:
			for i := range x {
				x[i] = protect(x[i])
			}
		case map[string]any:
			for k, e := range x {
				x[k] = protect(e)
			}
		}
		return v
	}
	if raw, err = json.Marshal(protect(tree)); err != nil {
		return SpecifyContent{}, err
	}
	var out SpecifyContent
	err = json.Unmarshal(raw, &out)
	return out, err
}

// requireSpecifyDraft holds the guards shared by the preparation operations:
// a specify flow with a supported preparation, in one of the given phases, with
// no claimed or consumed dispatch, edited from the revision the agent read.
func requireSpecifyDraft(f RefinementFlow, expectedRevision int, phases ...SocraticaPhase) error {
	if f.Mode != ModeSpecify || f.Preparation == nil {
		return ErrInvalidTransition
	}
	if f.Preparation.SchemaVersion > PreparationSchemaVersion {
		return ErrUnsupportedPreparation
	}
	if !slices.Contains(phases, f.Phase) {
		return ErrInvalidTransition
	}
	// A claimed or consumed dispatch can no longer be edited from under it.
	if f.DispatchClaimOwner != "" || f.DispatchAck {
		return ErrInvalidTransition
	}
	if expectedRevision != f.Revision {
		return ErrRevisionConflict
	}
	return nil
}

// prepareSet replaces one section when the expected revision is current. A
// material change bumps the revision and drops everything derived from the old
// content: later checks, the review, the proposal and an unconsumed approval.
// An identical replacement is a no-op that keeps the revision.
func prepareSet(f RefinementFlow, ev Event) (RefinementFlow, error) {
	if err := requireSpecifyDraft(f, ev.ExpectedRevision, PhaseCaptured, PhaseQuestions, PhaseReview, PhaseApproved); err != nil {
		return f, err
	}
	prep := f.Preparation
	changed := false
	switch ev.Section {
	case StageIntent:
		if ev.Content.Intent == nil {
			return f, ErrInvalidTransition
		}
		changed = !reflect.DeepEqual(prep.Content.Intent, ev.Content.Intent)
		prep.Content.Intent = ev.Content.Intent
	case StageScope:
		if ev.Content.Scope == nil {
			return f, ErrInvalidTransition
		}
		if err := CheckPendingContinuity(prep.Content.Scope, ev.Content.Scope); err != nil {
			return f, err
		}
		changed = !reflect.DeepEqual(prep.Content.Scope, ev.Content.Scope)
		prep.Content.Scope = ev.Content.Scope
	case StageStories:
		if ev.Content.Stories == nil {
			return f, ErrInvalidTransition
		}
		changed = !reflect.DeepEqual(prep.Content.Stories, ev.Content.Stories)
		prep.Content.Stories = ev.Content.Stories
	default:
		return f, ErrInvalidTransition
	}
	if !changed {
		return f, nil
	}
	f.Revision++
	rank := specifyStageRank(ev.Section)
	kept := prep.Checks[:0:0]
	for _, c := range prep.Checks {
		if specifyStageRank(c.Stage) < rank {
			kept = append(kept, c)
		}
	}
	prep.Checks = kept
	if len(prep.Checks) == 0 {
		prep.Checks = nil
	}
	prep.Review = nil
	if specifyStageRank(prep.Stage) > rank {
		prep.Stage = ev.Section
	}
	f.ProposedPrompt, f.ApprovedPrompt, f.ApprovedHash = "", "", ""
	f.DispatchID = ""
	if f.Phase == PhaseReview || f.Phase == PhaseApproved {
		f.Phase, f.LastStablePhase = PhaseCaptured, PhaseCaptured
	}
	return f, nil
}

var checkableStages = []SpecifyStage{StageIntent, StageScope, StageStories, StageValidation}

// checkIsCurrent: the stage has a passing check whose inputs are still the
// current content. A stale or forged check never enables the next stage.
func checkIsCurrent(p *SpecifyPreparation, stage SpecifyStage) bool {
	for _, c := range p.Checks {
		if c.Stage == stage {
			return c.Passed && c.InputHash == SpecifyInputHash(p.Content, stage)
		}
	}
	return false
}

// RewindStaleChecks drops the checks from the first stage whose inputs changed
// outside prepare (a sanitize pass) and moves the stage back to it, so the next
// step suggested is a validation that can pass, never one blocked by a stale
// earlier check. It reports whether anything changed.
func RewindStaleChecks(p *SpecifyPreparation) bool {
	for _, stage := range checkableStages {
		if specifyStageRank(stage) >= specifyStageRank(p.Stage) {
			return false
		}
		if checkIsCurrent(p, stage) {
			continue
		}
		rank := specifyStageRank(stage)
		kept := p.Checks[:0:0]
		for _, c := range p.Checks {
			if specifyStageRank(c.Stage) < rank {
				kept = append(kept, c)
			}
		}
		p.Checks = kept
		if len(p.Checks) == 0 {
			p.Checks = nil
		}
		p.Stage = stage
		return true
	}
	return false
}

// prepareValidate checks one stage computed by gomemory from the current
// content: the agent supplies neither validity nor hashes. A failure keeps the
// draft and the missing items and unlocks nothing; the final pass renders the
// protected text, binds it to the revision and opens the review, never an
// approval.
func prepareValidate(f RefinementFlow, ev Event) (RefinementFlow, error) {
	if err := requireSpecifyDraft(f, ev.ExpectedRevision, PhaseCaptured); err != nil {
		return f, err
	}
	at := slices.Index(checkableStages, ev.Section)
	if at < 0 {
		return f, fmt.Errorf("%w: stage %q cannot be checked", ErrStageBlocked, ev.Section)
	}
	prep := f.Preparation
	for _, prev := range checkableStages[:at] {
		if !checkIsCurrent(prep, prev) {
			return f, fmt.Errorf("%w: check %s first (missing or outdated)", ErrStageBlocked, prev)
		}
	}
	issues := ValidateSpecifyStage(prep.Content, ev.Section)
	var text string
	if ev.Section == StageValidation && len(issues) == 0 {
		text, issues = RenderSpecifyPrompt(prep.Content)
	}
	check := SpecifyCheck{Stage: ev.Section, InputHash: SpecifyInputHash(prep.Content, ev.Section), Passed: len(issues) == 0, Missing: issues}
	kept := make([]SpecifyCheck, 0, len(prep.Checks)+1)
	for _, c := range prep.Checks {
		if c.Stage != ev.Section {
			kept = append(kept, c)
		}
	}
	prep.Checks = append(kept, check)
	slices.SortStableFunc(prep.Checks, func(a, b SpecifyCheck) int { return specifyStageRank(a.Stage) - specifyStageRank(b.Stage) })
	switch {
	case !check.Passed:
		prep.Stage = ev.Section
	case ev.Section == StageValidation:
		f.ProposedPrompt = text
		prep.Review = &SpecifyReview{Revision: f.Revision, PromptHash: SpecifyPromptHash(text)}
		f.Phase, f.LastStablePhase = PhaseReview, PhaseReview
		prep.Stage = StageReview
	default:
		if next := checkableStages[at+1]; specifyStageRank(next) > specifyStageRank(prep.Stage) {
			prep.Stage = next
		}
	}
	return f, nil
}

// specifyIdentity proves that text is, byte for byte, the render of the current
// content, that the review record names this revision and this text, and that
// every stage check is current. Storage edited behind the flow (privacy
// sanitisation, tampering) makes it fail, so an old authorisation never covers
// a new version.
func specifyIdentity(f RefinementFlow, text string) error {
	p := f.Preparation
	if p == nil || p.Review == nil || p.Review.Revision != f.Revision || text == "" {
		return ErrReviewStale
	}
	for _, stage := range checkableStages {
		if !checkIsCurrent(p, stage) {
			return ErrReviewStale
		}
	}
	rendered, issues := RenderSpecifyPrompt(p.Content)
	if len(issues) != 0 || rendered != text || SpecifyPromptHash(rendered) != p.Review.PromptHash {
		return ErrReviewStale
	}
	return nil
}

// dropSpecifyApproval returns an unconsumed approval to preparation, keeping
// the draft: the person has to check and review again.
func dropSpecifyApproval(f *RefinementFlow) {
	f.Phase, f.LastStablePhase = PhaseCaptured, PhaseCaptured
	f.ProposedPrompt, f.ApprovedPrompt, f.ApprovedHash, f.DispatchID = "", "", "", ""
	if f.Preparation != nil {
		f.Preparation.Review = nil
		f.Preparation.Checks = nil
		f.Preparation.Stage = StageIntent
	}
}

// withdrawSpecifyPreparation removes the active preparation and any pending
// authorisation on cancel. A preparation of a newer version is kept intact.
func withdrawSpecifyPreparation(f *RefinementFlow) {
	f.ApprovedPrompt, f.ApprovedHash, f.DispatchID = "", "", ""
	if f.Preparation != nil && f.Preparation.SchemaVersion <= PreparationSchemaVersion {
		f.Preparation = nil
	}
}

// validRelativePath accepts only a plain relative path with forward slashes:
// no absolute or drive paths, URL schemes, home shortcuts, backslashes, control
// characters, empty or dot segments. The backend never opens it.
func validRelativePath(p string) bool {
	if p == "" || p != strings.TrimSpace(p) || strings.ContainsAny(p, `\:`) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") {
		return false
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return ProtectText(p) == p
}

// cleanOutcomeText removes terminal controls and protected content from text
// the agent communicates.
func cleanOutcomeText(s string) string {
	return strings.TrimSpace(StripTerminalControls(ProtectText(s)))
}

// normalizeOutcome validates an outcome against the prepared content and
// returns it cleaned. It checks format and coherence of references only: the
// backend never reads the paths nor judges the generated document.
func normalizeOutcome(o SpecifyOutcome, dispatchID string, content SpecifyContent) (SpecifyOutcome, error) {
	if o.DispatchID != "" && o.DispatchID != dispatchID {
		return o, fmt.Errorf("%w: the outcome names another dispatch", ErrInvalidTransition)
	}
	out := SpecifyOutcome{DispatchID: dispatchID, Status: o.Status, SpecPath: o.SpecPath, ChecklistPath: o.ChecklistPath, Message: cleanOutcomeText(o.Message)}
	switch o.Status {
	case OutcomeCompleted:
	case OutcomeFailed, OutcomeUncertain:
		if out.Message == "" {
			return o, errors.New("a failed or uncertain outcome needs a message")
		}
	default:
		return o, fmt.Errorf("unsupported outcome status %q", o.Status)
	}
	// Checked in a fixed order so the reported field is deterministic.
	for _, path := range []struct{ field, value string }{{"spec_path", out.SpecPath}, {"checklist_path", out.ChecklistPath}} {
		switch {
		case path.value == "" && o.Status == OutcomeCompleted:
			return o, fmt.Errorf("a completed outcome needs %s", path.field)
		case path.value != "" && !validRelativePath(path.value):
			return o, fmt.Errorf("%s must be a plain relative path", path.field)
		}
	}
	criteria := map[string][]string{}
	stories := map[string]bool{}
	if content.Stories != nil {
		for _, c := range content.Stories.Criteria {
			criteria[c.ID] = c.StoryIDs
		}
		for _, s := range content.Stories.Stories {
			stories[s.ID] = true
		}
	}
	seen := map[string]bool{}
	for _, t := range o.Traceability {
		linked, ok := criteria[t.CriterionID]
		switch {
		case !ok:
			return o, fmt.Errorf("traceability names an unknown criterion %q", t.CriterionID)
		case seen[t.CriterionID]:
			return o, fmt.Errorf("traceability repeats criterion %q", t.CriterionID)
		case len(t.StoryIDs) == 0:
			return o, fmt.Errorf("traceability of %q needs its stories", t.CriterionID)
		}
		seen[t.CriterionID] = true
		for _, sid := range t.StoryIDs {
			if !stories[sid] || !slices.Contains(linked, sid) {
				return o, fmt.Errorf("traceability of %q names story %q that is not one of its stories", t.CriterionID, sid)
			}
		}
		ref := cleanOutcomeText(t.SpecReference)
		if ref == "" {
			return o, fmt.Errorf("traceability of %q needs a reference into the specification", t.CriterionID)
		}
		out.Traceability = append(out.Traceability, SpecifyTrace{CriterionID: t.CriterionID, StoryIDs: slices.Clone(t.StoryIDs), SpecReference: ref})
	}
	if o.Status == OutcomeCompleted && content.Stories != nil {
		for _, c := range content.Stories.Criteria {
			if !seen[c.ID] {
				return o, fmt.Errorf("a completed outcome needs traceability for criterion %q", c.ID)
			}
		}
	}
	return out, nil
}

// reportOutcome records the result the agent communicated for the consumed
// dispatch. It is information, never permission: nothing about the dispatch,
// the approval or the grant changes. changed is false for an identical repeat.
func reportOutcome(f RefinementFlow, ev Event) (RefinementFlow, bool, error) {
	if f.Mode != ModeSpecify {
		return f, false, ErrInvalidTransition
	}
	if f.Preparation == nil {
		return f, false, ErrPrivateStateUnavailable
	}
	if f.Preparation.SchemaVersion > PreparationSchemaVersion {
		return f, false, ErrUnsupportedPreparation
	}
	if f.Phase != PhaseDispatched || !f.DispatchAck || ev.DispatchID == "" || ev.DispatchID != f.DispatchID {
		return f, false, ErrInvalidTransition
	}
	o, err := normalizeOutcome(ev.Outcome, f.DispatchID, f.Preparation.Content)
	if err != nil {
		return f, false, err
	}
	if prev := f.Preparation.Outcome; prev != nil {
		if reflect.DeepEqual(*prev, o) {
			return f, false, nil
		}
		if !ev.CorrectPrevious {
			return f, false, ErrOutcomeConflict
		}
	}
	f.Preparation.Outcome = &o
	f.Preparation.Stage = StageDelivery
	return f, true, nil
}
