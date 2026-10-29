package usecases

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"os"
	"strings"
	"time"
)

type HCD struct {
	specKitReader                                              ports.SpecKitReader
	hcdSettings                                                domain.HCDSettings
	memoryRepo                                                 ports.MemoryRepository
	relationRepo                                               ports.RelationRepository
	inspectSubject                                             func(context.Context, string) (bool, error)
	subjectRef                                                 func(context.Context, []string) (string, error)
	parseEvidence                                              func([]byte) (domain.Evidence, error)
	parseDelivery                                              func([]byte) (domain.Delivery, error)
	renderDelivery                                             func(string, domain.Delivery) []byte
	repo                                                       ports.HCDRepository
	uow                                                        ports.HCDUnitOfWork
	artifacts                                                  ports.HCDArtifactRepository
	contextRepo                                                ports.HCDContextRepository
	clock                                                      ports.ClockPort
	newID                                                      func() string
	project                                                    string
	topics                                                     ports.MemoryTopicQuerier
	definitionTemplate, playbookTemplate, constitutionTemplate string
	parseDefinition                                            func([]byte) (domain.Definition, error)
}

func (h *HCD) ConfigureDefinitionParser(parse func([]byte) (domain.Definition, error)) {
	h.parseDefinition = parse
}

func (h *HCD) ConfigureDocuments(project string, topics ports.MemoryTopicQuerier, definitionTemplate, playbookTemplate, constitutionTemplate string) {
	h.project = project
	h.topics = topics
	h.definitionTemplate = definitionTemplate
	h.playbookTemplate = playbookTemplate
	h.constitutionTemplate = constitutionTemplate
}

type HCDStartInput struct {
	Kind                                        domain.Kind
	Title, Slug, Implementer, Client, SessionID string
}

func (h *HCD) Start(ctx context.Context, input HCDStartInput) (domain.Unit, error) {
	// Antes del slug: un valor privado del título no debe llegar al nombre de
	// la carpeta, a definition.md ni a SQLite.
	input.Title, input.Implementer = domain.ProtectHCDField(input.Title), domain.ProtectHCDField(input.Implementer)
	if strings.TrimSpace(input.Title) == "" || (input.Kind != domain.KindFix && input.Kind != domain.KindFeature && input.Kind != domain.KindEvolution && input.Kind != domain.KindTechnical) {
		return domain.Unit{}, domain.ErrValidation
	}
	if input.Slug == "" {
		input.Slug = hcdSlug(input.Title)
	}
	playbook, err := ResolvePinnedDoc(h.topics, h.project, domain.TopicHCDPlaybook, h.playbookTemplate)
	if err != nil {
		return domain.Unit{}, err
	}
	constitution, err := ResolvePinnedDoc(h.topics, h.project, domain.TopicConstitution, h.constitutionTemplate)
	if err != nil {
		return domain.Unit{}, err
	}
	if !domain.ConstitutionAdmitsHCD(constitution.Content) {
		return domain.Unit{}, domain.ErrConstitutionConflict
	}
	playbookRev := domain.ContentRevision([]byte(playbook.Content))
	constitutionRev := domain.ContentRevision([]byte(constitution.Content))
	// El ID se reserva dentro de la transacción: el INSERT toma el lock de
	// escritura y la clave primaria rechaza a un Start concurrente con el mismo
	// ID antes de que escriba su definition.md. El artefacto se escribe con ese
	// lock tomado, así que la limpieza solo puede borrar lo que escribió esta
	// llamada.
	var unit domain.Unit
	var unitDir string
	written := false
	err = h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		id, err := repo.NextUnitID(ctx)
		if err != nil {
			return err
		}
		unitDir = strings.TrimPrefix(id, "HCD-") + "-" + input.Slug
		exists, err := h.artifacts.Exists(ctx, unitDir, "definition.md")
		if err != nil {
			return err
		}
		if exists {
			return domain.ErrAlreadyExists
		}
		definition := strings.ReplaceAll(h.definitionTemplate, "HCD-001", id)
		definition = strings.ReplaceAll(definition, "{Título}", input.Title)
		definition = strings.ReplaceAll(definition, "Tipo: fix", "Tipo: "+string(input.Kind))
		definition = strings.ReplaceAll(definition, "{persona}", input.Implementer)
		definition = strings.ReplaceAll(definition, "{revisión}", playbookRev)
		unit = domain.Unit{ID: id, Kind: input.Kind, Title: input.Title, Slug: input.Slug, Approach: "hcd", State: domain.StateDraft, StateRevision: 1, DefinitionRev: domain.ContentRevision([]byte(definition)), PlaybookRev: playbookRev, ConstitutionRev: constitutionRev, Implementer: input.Implementer}
		if err := repo.CreateUnit(ctx, unit); err != nil {
			return err
		}
		if err := repo.SaveNormativeRevision(ctx, "playbook", playbookRev, playbook.Content, docSource(playbook.FromMemory), "initial", false); err != nil {
			return err
		}
		if err := repo.SaveNormativeRevision(ctx, "constitution", constitutionRev, constitution.Content, docSource(constitution.FromMemory), "initial", false); err != nil {
			return err
		}
		decision := domain.HCDDecision{UnitID: id, Kind: "approach", Status: "accepted", Summary: "HCD", Revision: unit.DefinitionRev, Refs: []string{id}, Source: "start", MemorySync: "pending"}
		if err := recordDecisionInRepo(ctx, repo, &decision); err != nil {
			return err
		}
		if err := repo.SetFocus(ctx, domain.Focus{UnitID: id, Client: input.Client, SessionID: input.SessionID, SetAt: h.clock.Now().UTC().Format(time.RFC3339Nano), SetBy: "start"}); err != nil {
			return err
		}
		if err := h.artifacts.Write(ctx, unitDir, "definition.md", []byte(definition)); err != nil {
			return err
		}
		// Si el commit falla después, el archivo es de esta llamada.
		written = true
		return nil
	})
	if err != nil {
		if !written {
			return domain.Unit{}, fmt.Errorf("start HCD unit: %w", err)
		}
		cctx, cancel := compensationContext(ctx)
		cleanupErr := h.artifacts.Delete(cctx, unitDir, "definition.md")
		cancel()
		if cleanupErr != nil {
			return domain.Unit{}, fmt.Errorf("start HCD unit: %w (artifact cleanup: %v)", err, cleanupErr)
		}
		return domain.Unit{}, fmt.Errorf("start HCD unit: %w", err)
	}
	h.syncDecisionsAfterCommit(ctx, unit.ID)
	return unit, nil
}

func (h *HCD) Define(ctx context.Context, unitID string, expectedRevision int64, operationID string, definition domain.Definition, content []byte) (domain.Unit, error) {
	content = []byte(domain.ProtectText(string(content)))
	unit, err := h.repo.GetUnit(ctx, unitID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			units, e := h.List(ctx)
			if e != nil {
				return domain.Unit{}, e
			}
			for _, candidate := range units {
				if (candidate.ID == unitID || candidate.SourceDir == unitID) && candidate.Approach != "hcd" {
					return domain.Unit{}, domain.ErrApproachMismatch
				}
			}
		}
		return domain.Unit{}, err
	}
	unitDir := strings.TrimPrefix(unit.ID, "HCD-") + "-" + unit.Slug
	previous, readErr := h.artifacts.Read(ctx, unitDir, "definition.md")
	previousExists := readErr == nil
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return domain.Unit{}, readErr
	}
	wrote := false
	result, err := h.mutate(ctx, unitID, expectedRevision, operationID, hcdPayloadHash(content), "define", func(repo ports.HCDRepository, current domain.Unit) (domain.Unit, any, error) {
		wasApproved := current.State != domain.StateDraft
		if err := h.artifacts.Write(ctx, unitDir, "definition.md", content); err != nil {
			return current, nil, err
		}
		wrote = true
		current.Title = domain.ProtectHCDField(definition.Title)
		if implementer := domain.ProtectHCDField(definition.Implementer); implementer != "" {
			current.Implementer = implementer
		}
		current.DefinitionRev = domain.ContentRevision(content)
		current.State = domain.StateDraft
		current.ApprovedRev = ""
		for _, story := range definition.Stories {
			if err := repo.AddLink(ctx, domain.Link{UnitID: unitID, FromKind: "unit", FromID: unitID, ToKind: "story", ToID: story.ID, Relation: "belongs_to"}); err != nil {
				return current, nil, err
			}
			for _, criterion := range definition.CriteriaForStory(story.ID) {
				if err := repo.AddLink(ctx, domain.Link{UnitID: unitID, FromKind: "story", FromID: story.ID, ToKind: "criterion", ToID: criterion.ID, Relation: "covers"}); err != nil {
					return current, nil, err
				}
			}
		}
		if wasApproved {
			refs := make([]string, 0, len(definition.Stories))
			for _, story := range definition.Stories {
				refs = append(refs, story.ID)
			}
			decision := domain.HCDDecision{UnitID: unitID, Kind: "scope_change", Status: "proposed", Summary: "Cambiar el alcance de la definición", Revision: domain.ContentRevision(content), Refs: refs, Source: "define", MemorySync: "pending"}
			if err := recordDecisionInRepo(ctx, repo, &decision); err != nil {
				return current, nil, err
			}
		}
		return current, current, nil
	})
	if err != nil && wrote {
		var cleanup error
		cctx, cancel := compensationContext(ctx)
		if previousExists {
			cleanup = h.artifacts.Write(cctx, unitDir, "definition.md", previous)
		} else {
			cleanup = h.artifacts.Delete(cctx, unitDir, "definition.md")
		}
		cancel()
		if cleanup != nil {
			return domain.Unit{}, fmt.Errorf("define HCD unit: %w (artifact rollback: %v)", err, cleanup)
		}
	}
	if err != nil {
		return domain.Unit{}, err
	}
	var updated domain.Unit
	if err := json.Unmarshal([]byte(result), &updated); err != nil {
		return domain.Unit{}, err
	}
	return updated, nil
}

func (h *HCD) Validate(definition domain.Definition, kind domain.Kind) []domain.PendingCheck {
	return domain.ValidateDefinition(definition, kind)
}

type HCDApproveInput struct {
	UnitID                                                 string
	ExpectedRevision                                       int64
	OperationID, Object, Revision, Quote, Client, PromptID string
	Definition                                             domain.Definition
}

func (h *HCD) RequestApproval(ctx context.Context, unitID string, expected int64, object, revision string, definition domain.Definition, options ...string) (domain.ApprovalPrompt, error) {
	var prompt domain.ApprovalPrompt
	if len(options) == 0 {
		options = []string{"A", "R"}
	}
	err := h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		u, err := repo.GetUnit(ctx, unitID)
		if err != nil {
			return err
		}
		if u.StateRevision != expected || u.DefinitionRev != revision || object != "definition" {
			return domain.ErrRevisionConflict
		}
		if u.ApprovedRev == revision && u.State == domain.StateReady {
			return domain.ErrAlreadyExists
		}
		if checks := domain.ValidateDefinition(definition, u.Kind); len(checks) > 0 {
			return domain.ErrValidation
		}
		open, err := repo.ListOpenClarifications(ctx, unitID)
		if err != nil {
			return err
		}
		for _, c := range open {
			if c.Blocking {
				return domain.ErrBlocked
			}
		}
		prompt = domain.ApprovalPrompt{ID: h.newID(), UnitID: unitID, Object: object, Revision: revision, Options: append([]string(nil), options...), AskedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)}
		return repo.SaveApprovalPrompt(ctx, prompt)
	})
	return prompt, err
}

func (h *HCD) Approve(ctx context.Context, input HCDApproveInput) (domain.Unit, error) {
	if input.Object == "" {
		input.Object = "definition"
	}
	if input.Object != "definition" || input.Revision == "" {
		return domain.Unit{}, domain.ErrValidation
	}
	payload, _ := json.Marshal(input)
	encoded, err := h.mutate(ctx, input.UnitID, input.ExpectedRevision, input.OperationID, hcdPayloadHash(payload), "approve", func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		if u.ApprovedRev == input.Revision && u.State == domain.StateReady {
			return u, u, domain.ErrAlreadyExists
		}
		if u.DefinitionRev != input.Revision {
			return u, nil, domain.ErrRevisionConflict
		}
		if checks := domain.ValidateDefinition(input.Definition, u.Kind); len(checks) > 0 {
			return u, nil, domain.ErrValidation
		}
		open, e := repo.ListOpenClarifications(ctx, u.ID)
		if e != nil {
			return u, nil, e
		}
		for _, c := range open {
			if c.Blocking {
				return u, nil, domain.ErrBlocked
			}
		}
		quote := strings.TrimSpace(domain.ProtectText(input.Quote))
		valid := domain.ValidQuote(quote, input.Revision)
		if input.PromptID != "" {
			p, e := repo.GetApprovalPrompt(ctx, input.PromptID, u.ID)
			if e != nil {
				return u, nil, e
			}
			if p.Consumed || p.Object != input.Object || p.Revision != input.Revision {
				return u, nil, domain.ErrInvalidQuote
			}
			if strings.EqualFold(quote, "a") || strings.EqualFold(quote, "recommended") {
				valid = true
			}
			if !valid {
				return u, nil, domain.ErrInvalidQuote
			}
			if e = repo.ConsumeApprovalPrompt(ctx, p.ID, u.ID); e != nil {
				return u, nil, e
			}
		} else if !valid {
			return u, nil, domain.ErrInvalidQuote
		}
		id, e := repo.NextApprovalID(ctx, u.ID)
		if e != nil {
			return u, nil, e
		}
		origin := "conversation"
		if strings.EqualFold(input.Client, "terminal") {
			origin = "terminal"
		}
		if e = repo.SaveApproval(ctx, domain.Approval{ID: id, UnitID: u.ID, Object: input.Object, Revision: input.Revision, Origin: origin, Quote: quote, Client: input.Client, ApprovedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)}); e != nil {
			return u, nil, e
		}
		d := domain.HCDDecision{UnitID: u.ID, Kind: "approval", Status: "accepted", Summary: "Aprobación de la definición", Revision: input.Revision, Refs: []string{u.ID, input.Object, id}, Source: "approve", MemorySync: "pending"}
		if e = recordDecisionInRepo(ctx, repo, &d); e != nil {
			return u, nil, e
		}
		u.State = domain.StateReady
		u.ApprovedRev = input.Revision
		return u, u, nil
	})
	if err != nil {
		return domain.Unit{}, err
	}
	var u domain.Unit
	err = json.Unmarshal([]byte(encoded), &u)
	return u, err
}

func docSource(fromMemory bool) string {
	if fromMemory {
		return "project"
	}
	return "default"
}
func hcdSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	dash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func NewHCD(repo ports.HCDRepository, uow ports.HCDUnitOfWork, artifacts ports.HCDArtifactRepository, contextRepo ports.HCDContextRepository, clock ports.ClockPort, newID func() string) *HCD {
	if newID == nil {
		newID = defaultHCDID
	}
	return &HCD{repo: repo, uow: uow, artifacts: artifacts, contextRepo: contextRepo, clock: clock, newID: newID}
}

func defaultHCDID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err == nil {
		return hex.EncodeToString(data[:])
	}
	return fmt.Sprintf("fallback-%d", time.Now().UTC().UnixNano())
}

type hcdMutation func(ports.HCDRepository, domain.Unit) (domain.Unit, any, error)
type hcdReplay struct{ result string }

func (e hcdReplay) Error() string { return "hcd operation replay" }

// mutate is the single write boundary for HCD unit state. It validates the
// state revision and recorded artifact revision, then stores the mutation and
// its idempotent result in one transaction.
func (h *HCD) mutate(ctx context.Context, unitID string, expectedRevision int64, operationID, payloadHash, command string, fn hcdMutation) (string, error) {
	if operationID == "" {
		operationID = h.newID()
	}
	cached, replayed, err := h.repo.RecordOperation(ctx, operationID, unitID, command, payloadHash, "")
	if err != nil {
		return "", err
	}
	if replayed {
		h.syncDecisionsAfterCommit(ctx, unitID)
		return cached, nil
	}
	approaches, e := h.List(ctx)
	if e != nil {
		return "", e
	}
	for _, candidate := range approaches {
		if (candidate.ID == unitID || candidate.SourceDir == unitID) && candidate.Approach != "hcd" {
			return "", domain.ErrApproachMismatch
		}
	}
	changed, err := h.reconcileDefinition(ctx, unitID)
	if err != nil {
		return "", err
	}
	if changed {
		if command == "close" {
			return "", domain.ErrNotAllowed
		}
		return "", domain.ErrRevisionConflict
	}
	var replayResult string
	err = h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		cached, replayed, err := repo.RecordOperation(ctx, operationID, unitID, command, payloadHash, "")
		if err != nil {
			return err
		}
		if replayed {
			replayResult = cached
			return hcdReplay{result: cached}
		}
		unit, err := repo.GetUnit(ctx, unitID)
		if err != nil {
			return err
		}
		if unit.Approach != "hcd" {
			return domain.ErrApproachMismatch
		}
		if unit.State == domain.StateCompleted || unit.State == domain.StateCancelled || unit.State == domain.StateSuperseded {
			return domain.ErrNotAllowed
		}
		if unit.StateRevision != expectedRevision {
			return domain.ErrRevisionConflict
		}
		if err = h.checkDefinitionRevision(ctx, unit); err != nil {
			return err
		}
		updated, result, err := fn(repo, unit)
		if err != nil {
			return err
		}
		updated.StateRevision = unit.StateRevision + 1
		if err = repo.UpdateUnit(ctx, updated, unit.StateRevision); err != nil {
			return err
		}
		if value, ok := result.(domain.Unit); ok {
			value.StateRevision = updated.StateRevision
			result = value
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		_, replayed, err = repo.RecordOperation(ctx, operationID, unitID, command, payloadHash, string(encoded))
		if err != nil {
			return err
		}
		if replayed {
			return fmt.Errorf("operation unexpectedly replayed inside transaction")
		}
		replayResult = string(encoded)
		return nil
	})
	var replay hcdReplay
	if errors.As(err, &replay) {
		return replay.result, nil
	}
	if err != nil {
		return "", err
	}
	h.syncDecisionsAfterCommit(ctx, unitID)
	return replayResult, nil
}

func (h *HCD) checkDefinitionRevision(ctx context.Context, unit domain.Unit) error {
	if h.artifacts == nil || unit.DefinitionRev == "" {
		return nil
	}
	unitDir := strings.TrimPrefix(unit.ID, "HCD-") + "-" + unit.Slug
	exists, err := h.artifacts.Exists(ctx, unitDir, "definition.md")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	actual, err := h.artifacts.Hash(ctx, unitDir, "definition.md")
	if err != nil {
		return err
	}
	if actual != unit.DefinitionRev {
		return fmt.Errorf("definition artifact changed outside HCD: %w", domain.ErrRevisionConflict)
	}
	return nil
}

func hcdPayloadHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// Reconcile an external definition as its own committed revision before the
// requested write. The caller must retry against that newly visible revision.
func (h *HCD) reconcileDefinition(ctx context.Context, id string) (bool, error) {
	if h.artifacts == nil {
		return false, nil
	}
	changed := false
	e := h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		u, e := repo.GetUnit(ctx, id)
		if e != nil {
			return e
		}
		if u.State == domain.StateCompleted || u.State == domain.StateCancelled || u.State == domain.StateSuperseded {
			return nil
		}
		dir := strings.TrimPrefix(u.ID, "HCD-") + "-" + u.Slug
		actual, e := h.artifacts.Hash(ctx, dir, "definition.md")
		// ACR C-005: un definition.md borrado es una edición externa más; sin
		// esto, toda mutación (incluido el define que lo repone) fallaba.
		if errors.Is(e, os.ErrNotExist) {
			actual, e = "", nil
		}
		if e != nil {
			return e
		}
		if actual == u.DefinitionRev {
			return nil
		}
		previous := u.StateRevision
		u.DefinitionRev = actual
		u.ApprovedRev = ""
		u.State = domain.StateDraft
		u.StateRevision++
		if e = repo.UpdateUnit(ctx, u, previous); e != nil {
			return e
		}
		changed = true
		return nil
	})
	return changed, e
}

// hcdCompensationTimeout acota una compensación de artefactos.
const hcdCompensationTimeout = 5 * time.Second

// compensationContext da a la compensación de un artefacto su propio plazo:
// si el fallo fue una cancelación, reutilizar ctx haría que Write y Delete
// retornaran sin deshacer nada y el disco quedaría distinto de SQLite.
func compensationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), hcdCompensationTimeout)
}
