package usecases

import (
	"context"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"strings"
)

// HCD conduce la ceremonia de las unidades de trabajo (feature 045 v2) sobre
// el núcleo ligero: un documento, evidencia y aprobaciones. El paso nunca se
// guarda: se deriva de domain.HCDState en cada consulta.
type HCD struct {
	store     ports.HCDStore
	ws        ports.HCDWorkspace
	clock     ports.ClockPort
	newID     func() string
	template  string
	decisions HCDDecisionSink
	project   string
}

// HCDDecisionSink es la parte de MemoryRepository que HCD necesita: las
// decisiones se guardan como memorias, sin tabla propia (FR-022).
type HCDDecisionSink interface {
	Insert(m *domain.Memory) (int64, error)
}

func NewHCD(store ports.HCDStore, ws ports.HCDWorkspace, clock ports.ClockPort, newID func() string, template string) *HCD {
	return &HCD{store: store, ws: ws, clock: clock, newID: newID, template: template}
}

// ConfigureDecisions activa el reflejo de decisiones en la memoria del proyecto.
func (h *HCD) ConfigureDecisions(sink HCDDecisionSink, project string) {
	h.decisions, h.project = sink, project
}

type HCDStartInput struct {
	Kind, Lane, Title, Why, Intent, Client string
}

type HCDTaskView struct {
	domain.Task
	Status domain.TaskStatus
}

// HCDView es el estado derivado de una unidad en un momento dado.
type HCDView struct {
	State              domain.HCDState
	Unit               domain.Unit
	Stage              domain.Stage
	Next               domain.NextStep
	Tasks              []HCDTaskView
	Unverified         []string
	RedMissing         []string
	DefinitionRevision string
	WorkRevision       string
	Approved           bool
	Warnings           []string
}

func (h *HCD) Start(ctx context.Context, in HCDStartInput) (HCDView, error) {
	kind, err := domain.ParseKind(in.Kind)
	if err != nil {
		return HCDView{}, err
	}
	lane, err := domain.ParseLane(in.Lane)
	if err != nil {
		return HCDView{}, err
	}
	title, why := domain.ProtectHCDField(in.Title), domain.ProtectHCDField(in.Why)
	if title == "" || why == "" {
		return HCDView{}, fmt.Errorf("%w: la unidad necesita título y motivo del carril", domain.ErrValidation)
	}
	n, err := h.store.NextNumber(ctx)
	if err != nil {
		return HCDView{}, err
	}
	now := h.clock.Now()
	id := fmt.Sprintf("HCD-%03d", n)
	unit := domain.Unit{ID: id, Number: n, Title: title, Slug: domain.Slugify(title), Kind: kind, Lane: lane, Why: why, State: domain.UnitActive, CreatedAt: now, UpdatedAt: now}
	unit.DocPath = "hcd/" + id + "-" + unit.Slug + ".md"
	if err = h.ws.WriteDoc(unit.DocPath, []byte(domain.RenderUnitTemplate(h.template, unit, in.Intent))); err != nil {
		return HCDView{}, err
	}
	if err = h.store.Create(ctx, unit); err != nil {
		return HCDView{}, err
	}
	if err = h.store.Activate(ctx, id); err != nil {
		return HCDView{}, err
	}
	h.recordDecision(unit, "carril "+string(lane), "Carril `"+string(lane)+"` para «"+title+"» ("+string(kind)+"). Motivo: "+why)
	return h.Show(ctx, id)
}

// Show es una consulta: no escribe estado, documento ni evidencia. Sin id usa
// la unidad activa.
func (h *HCD) Show(ctx context.Context, id string) (HCDView, error) {
	unit, err := h.unit(ctx, id)
	if err != nil {
		return HCDView{}, err
	}
	data, err := h.ws.ReadDoc(unit.DocPath)
	if err != nil {
		return HCDView{}, err
	}
	s := domain.HCDState{Unit: unit}
	s.Doc, s.Pending = domain.ParseUnitDoc(data, unit.Kind, unit.Lane)
	if s.Fingerprint, err = h.ws.Fingerprint(ctx); err != nil {
		return HCDView{}, err
	}
	if s.Evidence, err = h.store.Evidence(ctx, unit.ID); err != nil {
		return HCDView{}, err
	}
	if s.Approvals, err = h.store.Approvals(ctx, unit.ID); err != nil {
		return HCDView{}, err
	}
	return viewOf(s), nil
}

func viewOf(s domain.HCDState) HCDView {
	v := HCDView{State: s, Unit: s.Unit, Stage: s.Stage(), Next: s.NextAction(), Unverified: s.Unverified(), RedMissing: s.RedMissing(), DefinitionRevision: s.DefinitionRevision(), WorkRevision: s.WorkRevision(), Approved: s.Approved()}
	status := s.TaskStatuses()
	for _, t := range s.Doc.Tasks {
		v.Tasks = append(v.Tasks, HCDTaskView{Task: t, Status: status[t.ID]})
	}
	if s.Fingerprint == domain.FingerprintNoGit {
		v.Warnings = append(v.Warnings, "el proyecto no usa git: la huella no identifica el código evaluado y el rojo se acepta por orden")
	}
	return v
}

// Analyze es una consulta de coherencia (FR-017).
func (h *HCD) Analyze(ctx context.Context, id string) (HCDView, []domain.HCDFinding, error) {
	view, err := h.Show(ctx, id)
	if err != nil {
		return HCDView{}, nil, err
	}
	return view, view.State.Analyze(), nil
}

type HCDClarifyInput struct {
	ID       string
	Ask      string
	Question string
	Answer   string
}

// Clarify abre una pregunta (Ask) o resuelve una abierta (Question + Answer)
// en el documento. La respuesta se refleja como decisión (FR-010).
func (h *HCD) Clarify(ctx context.Context, in HCDClarifyInput) (HCDView, string, error) {
	view, err := h.mutable(ctx, in.ID)
	if err != nil {
		return HCDView{}, "", err
	}
	data, err := h.ws.ReadDoc(view.Unit.DocPath)
	if err != nil {
		return view, "", err
	}
	ask, answer := domain.ProtectHCDField(in.Ask), domain.ProtectHCDField(in.Answer)
	var qid string
	switch {
	case ask != "" && in.Question == "" && answer == "":
		data, qid = domain.AddClarification(data, view.State.Doc, ask)
	case ask == "" && in.Question != "" && answer != "":
		qid = in.Question
		if data, err = domain.AnswerClarification(data, qid, answer); err != nil {
			return view, "", err
		}
	default:
		return view, "", fmt.Errorf("%w: usa --ask \"pregunta\", o --id Qn con --answer \"respuesta\"", domain.ErrValidation)
	}
	if err = h.ws.WriteDoc(view.Unit.DocPath, data); err != nil {
		return view, "", err
	}
	if answer != "" {
		h.recordDecision(view.Unit, "aclaración "+qid, "Respuesta a "+qid+": "+answer)
	}
	next, err := h.Show(ctx, view.Unit.ID)
	return next, qid, err
}

type HCDApproveInput struct {
	ID, Quote, Client string
}

// Approve registra la aprobación de la revisión exacta de la definición
// (FR-011). Solo existe en el carril completo; la Vía rápida aprueba al cierre.
func (h *HCD) Approve(ctx context.Context, in HCDApproveInput) (HCDView, error) {
	view, err := h.mutable(ctx, in.ID)
	if err != nil {
		return HCDView{}, err
	}
	if view.Unit.Lane != domain.LaneFull {
		return view, fmt.Errorf("%w: la Vía rápida no aprueba la definición; se aprueba al cerrar", domain.ErrNotAllowed)
	}
	if view.Stage != domain.StageApprove {
		return view, fmt.Errorf("%w: la unidad está en %s: %s", domain.ErrNotAllowed, view.Stage, view.Next.Why)
	}
	if !domain.ValidQuote(in.Quote, view.DefinitionRevision) {
		return view, domain.ErrInvalidQuote
	}
	if err = h.approve(ctx, view, domain.ApprovalApprove, view.DefinitionRevision, in.Quote, in.Client); err != nil {
		return view, err
	}
	h.recordDecision(view.Unit, "definición aprobada", "Definición aprobada sobre la revisión "+view.DefinitionRevision+".")
	return h.Show(ctx, view.Unit.ID)
}

type HCDVerifyInput struct {
	ID         string
	Tasks      []string
	Command    string
	Manual     string
	ExpectFail bool
}

// verifiable son los pasos en los que ya hay tareas autorizadas que comprobar.
var verifiable = map[domain.Stage]bool{domain.StageImplement: true, domain.StageVerify: true, domain.StageClose: true}

// Verify ejecuta la comprobación y registra la evidencia con la revisión de
// trabajo y la huella del código evaluado (FR-013). Reescribe el bloque
// Evidencia y marca las tareas que pasaron.
func (h *HCD) Verify(ctx context.Context, in HCDVerifyInput) (HCDView, domain.Evidence, error) {
	view, err := h.mutable(ctx, in.ID)
	if err != nil {
		return HCDView{}, domain.Evidence{}, err
	}
	if !verifiable[view.Stage] {
		return view, domain.Evidence{}, fmt.Errorf("%w: la unidad está en %s: %s", domain.ErrNotAllowed, view.Stage, view.Next.Why)
	}
	known := map[string]bool{}
	for _, t := range view.Tasks {
		known[t.ID] = true
	}
	if len(in.Tasks) == 0 {
		return view, domain.Evidence{}, fmt.Errorf("%w: indica la tarea con --task", domain.ErrValidation)
	}
	for _, id := range in.Tasks {
		if !known[id] {
			return view, domain.Evidence{}, fmt.Errorf("%w: la tarea %s no existe", domain.ErrValidation, id)
		}
	}
	command, manual := strings.TrimSpace(in.Command), domain.ProtectHCDField(in.Manual)
	if (command == "") == (manual == "") {
		return view, domain.Evidence{}, fmt.Errorf("%w: usa un comando o --manual, no ambos", domain.ErrValidation)
	}
	if in.ExpectFail && command == "" {
		return view, domain.Evidence{}, fmt.Errorf("%w: el rojo solo se observa ejecutando un comando", domain.ErrValidation)
	}
	ev := domain.Evidence{ID: h.newID(), UnitID: view.Unit.ID, Tasks: in.Tasks, ExpectFail: in.ExpectFail, Revision: view.WorkRevision, Fingerprint: view.State.Fingerprint, CreatedAt: h.clock.Now()}
	if command != "" {
		ev.Kind, ev.Command = domain.EvidenceCommand, domain.ProtectText(command)
		exit, excerpt, err := h.ws.Run(ctx, command)
		if err != nil {
			return view, domain.Evidence{}, err
		}
		ev.ExitCode, ev.Excerpt = exit, domain.ProtectText(excerpt)
	} else {
		ev.Kind, ev.Excerpt = domain.EvidenceManual, manual
	}
	if err = h.store.AddEvidence(ctx, ev); err != nil {
		return view, domain.Evidence{}, err
	}
	data, err := h.ws.ReadDoc(view.Unit.DocPath)
	if err != nil {
		return view, ev, err
	}
	if ev.Passed() {
		data = domain.MarkTasksDone(data, ev.Tasks)
	}
	all := append(append([]domain.Evidence{}, view.State.Evidence...), ev)
	data = domain.ReplaceSection(data, domain.SectionEvidence, renderEvidence(all, view.WorkRevision, view.State.Fingerprint), domain.SectionNext)
	if err = h.ws.WriteDoc(view.Unit.DocPath, data); err != nil {
		return view, ev, err
	}
	next, err := h.Show(ctx, view.Unit.ID)
	return next, ev, err
}

type HCDCloseInput struct {
	ID, Quote, Client string
}

// Close aplica el gate de cierre: paso close, ningún hallazgo critical y la
// cita del Implementador sobre la revisión de trabajo actual (FR-018).
func (h *HCD) Close(ctx context.Context, in HCDCloseInput) (HCDView, error) {
	view, err := h.mutable(ctx, in.ID)
	if err != nil {
		return HCDView{}, err
	}
	if view.Stage != domain.StageClose {
		return view, fmt.Errorf("%w: la unidad está en %s: %s", domain.ErrNotAllowed, view.Stage, view.Next.Why)
	}
	if findings := view.State.Analyze(); domain.HasCritical(findings) {
		return view, fmt.Errorf("%w: analyze tiene hallazgos critical: %s", domain.ErrNotAllowed, criticalSummary(findings))
	}
	if !domain.ValidQuote(in.Quote, view.WorkRevision) {
		return view, domain.ErrInvalidQuote
	}
	if err = h.approve(ctx, view, domain.ApprovalClose, view.WorkRevision, in.Quote, in.Client); err != nil {
		return view, err
	}
	view.Unit.State = domain.UnitClosed
	return h.finish(ctx, view, "cerrada", "Cerrada con aprobación sobre la revisión "+view.WorkRevision+".")
}

func (h *HCD) Cancel(ctx context.Context, in HCDCloseInput) (HCDView, error) {
	view, err := h.mutable(ctx, in.ID)
	if err != nil {
		return HCDView{}, err
	}
	if domain.ProtectHCDField(in.Quote) == "" {
		return view, domain.ErrInvalidQuote
	}
	if err = h.approve(ctx, view, domain.ApprovalCancel, view.WorkRevision, in.Quote, in.Client); err != nil {
		return view, err
	}
	view.Unit.State = domain.UnitCancelled
	return h.finish(ctx, view, "cancelada", "Cancelada: "+domain.ProtectHCDField(in.Quote))
}

// Promote pasa una unidad de la Vía rápida a la ceremonia completa sin
// reiniciar (FR-020): añade Historias y Slices y conserva la evidencia.
func (h *HCD) Promote(ctx context.Context, id string) (HCDView, error) {
	view, err := h.mutable(ctx, id)
	if err != nil {
		return HCDView{}, err
	}
	if view.Unit.Lane == domain.LaneFull {
		return view, fmt.Errorf("%w: la unidad ya está en el carril completo", domain.ErrNotAllowed)
	}
	data, err := h.ws.ReadDoc(view.Unit.DocPath)
	if err != nil {
		return view, err
	}
	if _, ok := view.State.Doc.Sections[domain.SectionStories]; !ok {
		data = domain.ReplaceSection(data, domain.SectionStories, domain.FullSectionsTemplate, domain.SectionClarifications)
	}
	if _, ok := view.State.Doc.Sections[domain.SectionSlices]; !ok {
		data = domain.ReplaceSection(data, domain.SectionSlices, "<!-- ### VS-01. comportamiento entregable\nTareas: 1\nCriterios: C-01 -->", domain.SectionChecklist)
	}
	if err = h.ws.WriteDoc(view.Unit.DocPath, data); err != nil {
		return view, err
	}
	view.Unit.Lane, view.Unit.UpdatedAt = domain.LaneFull, h.clock.Now()
	if err = h.store.Save(ctx, view.Unit); err != nil {
		return view, err
	}
	h.recordDecision(view.Unit, "promovida a la ceremonia completa", "La unidad pasa al carril completo; se conservan su documento y su evidencia.")
	return h.Show(ctx, view.Unit.ID)
}

// HasActive es la consulta ligera del hook pre-edición: solo lee la tabla,
// sin git ni documento, para caber en el plazo del hook (FR-025).
func (h *HCD) HasActive(ctx context.Context) (bool, error) {
	_, ok, err := h.store.Active(ctx)
	return ok, err
}

// ActiveHCDContext alimenta la sección «HCD activo» del contexto de arranque.
func (h *HCD) ActiveHCDContext(ctx context.Context) (domain.HCDActiveContext, bool, error) {
	view, err := h.Show(ctx, "")
	if errors.Is(err, domain.ErrNotFound) {
		return domain.HCDActiveContext{}, false, nil
	}
	if err != nil {
		return domain.HCDActiveContext{}, false, err
	}
	return domain.HCDActiveContext{Unit: view.Unit, Stage: view.Stage, NextAction: view.Next, Unverified: view.Unverified}, true, nil
}

func (h *HCD) unit(ctx context.Context, id string) (domain.Unit, error) {
	if id != "" {
		return h.store.Get(ctx, id)
	}
	unit, ok, err := h.store.Active(ctx)
	if err != nil {
		return domain.Unit{}, err
	}
	if !ok {
		return domain.Unit{}, fmt.Errorf("%w: no hay unidad HCD activa", domain.ErrNotFound)
	}
	return unit, nil
}

// mutable carga una unidad activa y la deja como la unidad en foco.
func (h *HCD) mutable(ctx context.Context, id string) (HCDView, error) {
	view, err := h.Show(ctx, id)
	if err != nil {
		return HCDView{}, err
	}
	if view.Unit.State != domain.UnitActive {
		return view, fmt.Errorf("%w: la unidad %s está %s", domain.ErrNotAllowed, view.Unit.ID, view.Unit.State)
	}
	if !view.Unit.Active {
		if err = h.store.Activate(ctx, view.Unit.ID); err != nil {
			return view, err
		}
		view.Unit.Active = true
	}
	return view, nil
}

func (h *HCD) approve(ctx context.Context, view HCDView, action, revision, quote, client string) error {
	origin := "conversation"
	if client == "" || client == "terminal" {
		client, origin = "terminal", "terminal"
	}
	return h.store.AddApproval(ctx, domain.Approval{ID: h.newID(), UnitID: view.Unit.ID, Action: action, Revision: revision, Fingerprint: view.State.Fingerprint, Origin: origin, Client: client, Quote: domain.ProtectHCDField(quote), CreatedAt: h.clock.Now()})
}

func (h *HCD) finish(ctx context.Context, view HCDView, verb, next string) (HCDView, error) {
	view.Unit.Active, view.Unit.UpdatedAt = false, h.clock.Now()
	if err := h.store.Save(ctx, view.Unit); err != nil {
		return view, err
	}
	if data, err := h.ws.ReadDoc(view.Unit.DocPath); err == nil {
		_ = h.ws.WriteDoc(view.Unit.DocPath, domain.ReplaceSection(data, domain.SectionNext, next, ""))
	}
	h.recordDecision(view.Unit, verb, next)
	return h.Show(ctx, view.Unit.ID)
}

// recordDecision refleja la decisión en la memoria; un fallo no revierte la
// operación HCD, que ya está confirmada.
func (h *HCD) recordDecision(unit domain.Unit, what, content string) {
	if h.decisions == nil {
		return
	}
	_, _ = h.decisions.Insert(&domain.Memory{Project: h.project, Type: domain.Decision, Title: unit.ID + ": " + what, Content: content, Filepath: unit.DocPath})
}

func criticalSummary(findings []domain.HCDFinding) string {
	var parts []string
	for _, f := range findings {
		if f.Severity == domain.SeverityCritical {
			parts = append(parts, f.Message)
		}
	}
	return strings.Join(parts, "; ")
}

func renderEvidence(all []domain.Evidence, revision, fingerprint string) string {
	var b strings.Builder
	b.WriteString("<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->\n")
	for _, e := range all {
		result := "verde"
		switch {
		case e.ExpectFail && e.RedObserved():
			result = "rojo observado"
		case e.ExpectFail:
			result = "rojo NO observado (el comando pasó)"
		case e.Kind == domain.EvidenceManual:
			result = "manual"
		case !e.Passed():
			result = "falló"
		}
		vigencia := "vigente"
		if !e.Fresh(revision, fingerprint) {
			vigencia = "caducada"
		}
		fmt.Fprintf(&b, "- %s · tarea %s · %s · %s", e.CreatedAt.UTC().Format("2006-01-02 15:04"), strings.Join(e.Tasks, ","), result, vigencia)
		if e.Kind == domain.EvidenceCommand {
			fmt.Fprintf(&b, " · `%s` (salida %d)", strings.ReplaceAll(e.Command, "`", "'"), e.ExitCode)
		}
		b.WriteString("\n")
		// Sangrado: ninguna línea de salida puede abrir una sección `## `.
		for _, line := range strings.Split(strings.TrimRight(e.Excerpt, "\n"), "\n") {
			if strings.TrimSpace(line) != "" {
				b.WriteString("      " + line + "\n")
			}
		}
	}
	return b.String()
}
