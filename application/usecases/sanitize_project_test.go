package usecases_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// Dobles en memoria del saneamiento (spec 037, US2). fakeStore simula la
// base: rows es el estado confirmado; una transacción trabaja sobre una copia
// que solo se publica si el callback termina sin error.
type fakeStore struct {
	rows       map[string]domain.SanitizeCandidate // clave category/id
	failApply  error
	compactErr error
	onBegin    func(rows map[string]domain.SanitizeCandidate)
	log        *[]string
}

func key(c domain.SanitizeCandidate) string { return string(c.Category) + "/" + c.RecordID }

func (s *fakeStore) list(rows map[string]domain.SanitizeCandidate) []domain.SanitizeCandidate {
	var out []domain.SanitizeCandidate
	for _, c := range rows {
		if c.Category == domain.SanitizeMemoryReview || domain.NeedsSanitize(c.Text) {
			out = append(out, c)
		}
	}
	return out
}

func (s *fakeStore) Candidates(context.Context, string) ([]domain.SanitizeCandidate, error) {
	*s.log = append(*s.log, "preview")
	return s.list(s.rows), nil
}

type fakeTx struct {
	s    *fakeStore
	rows map[string]domain.SanitizeCandidate
}

func (t *fakeTx) Candidates(context.Context, string) ([]domain.SanitizeCandidate, error) {
	return t.s.list(t.rows), nil
}

func (t *fakeTx) Apply(_ context.Context, c domain.SanitizeCandidate, text string) (bool, error) {
	if t.s.failApply != nil {
		return false, t.s.failApply
	}
	cur, ok := t.rows[key(c)]
	if !ok || cur.Text != c.Text {
		return false, nil
	}
	if c.Category == domain.SanitizeOriginal {
		delete(t.rows, key(c))
		return true, nil
	}
	cur.Text = text
	t.rows[key(c)] = cur
	return true, nil
}

func (s *fakeStore) WithTransaction(ctx context.Context, project string, fn func(ports.PrivacyTransaction) error) error {
	*s.log = append(*s.log, "begin")
	if s.onBegin != nil {
		s.onBegin(s.rows)
	}
	copyRows := map[string]domain.SanitizeCandidate{}
	for k, v := range s.rows {
		copyRows[k] = v
	}
	if err := fn(&fakeTx{s: s, rows: copyRows}); err != nil {
		*s.log = append(*s.log, "rollback")
		return err
	}
	if err := ctx.Err(); err != nil {
		*s.log = append(*s.log, "rollback")
		return err
	}
	s.rows = copyRows
	*s.log = append(*s.log, "commit")
	return nil
}

func (s *fakeStore) Compact(context.Context) error {
	*s.log = append(*s.log, "compact")
	return s.compactErr
}

type fakeArtifacts struct {
	reports     []domain.SanitizeReport
	publishErr  error
	backups     []domain.SanitizeArtifact
	residues    []domain.SanitizeArtifact
	needContext bool
	unreadable  []domain.SanitizeArtifact
	log         *[]string
}

func (a *fakeArtifacts) PublishReport(_ context.Context, r domain.SanitizeReport) error {
	*a.log = append(*a.log, "publish:"+string(r.Status))
	if a.publishErr != nil {
		return a.publishErr
	}
	for i := range a.reports {
		if a.reports[i].RunID == r.RunID {
			a.reports[i] = r
			return nil
		}
	}
	a.reports = append(a.reports, r)
	return nil
}

func (a *fakeArtifacts) PendingReports(_ context.Context, project string) ([]domain.SanitizeReport, []domain.SanitizeArtifact, error) {
	var out []domain.SanitizeReport
	for _, r := range a.reports {
		if r.Project == project && r.Status == domain.SanitizeStatusPendingCleanup {
			out = append(out, r)
		}
	}
	return out, a.unreadable, nil
}

func (a *fakeArtifacts) CleanBackups(context.Context, string) ([]domain.SanitizeArtifact, error) {
	*a.log = append(*a.log, "backups")
	return a.backups, nil
}

func (a *fakeArtifacts) ContextNeedsRefresh(context.Context) (bool, error) { return a.needContext, nil }

func (a *fakeArtifacts) VerifyResidues(context.Context, string) ([]domain.SanitizeArtifact, error) {
	*a.log = append(*a.log, "verify")
	return a.residues, nil
}

type fakeContext struct{ writes int }

func (f *fakeContext) Build() (string, error) { return "", nil }
func (f *fakeContext) WriteFile() error       { f.writes++; return nil }

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

const leak037 = "público <private>SECRETO_037</private> AKIAIOSFODNN7EXAMPLE"

func newSanitizeFixture() (*usecases.SanitizeProject, *fakeStore, *fakeArtifacts, *fakeContext, *[]string) {
	log := &[]string{}
	store := &fakeStore{log: log, rows: map[string]domain.SanitizeCandidate{
		"session/s1":               {Category: domain.SanitizeSession, RecordID: "s1", Project: "p", Field: "summary", Text: leak037},
		"relation/7":               {Category: domain.SanitizeRelation, RecordID: "7", Project: "p", Field: "reasoning", Text: leak037},
		"session/limpia":           {Category: domain.SanitizeSession, RecordID: "limpia", Project: "p", Field: "summary", Text: "público"},
		"compression_original/abc": {Category: domain.SanitizeOriginal, RecordID: "abc", Project: "p", Field: "content", Text: leak037},
		"memory_review/9":          {Category: domain.SanitizeMemoryReview, RecordID: "9", Project: "p", Field: "content", Text: "Z</private>"},
	}}
	arts := &fakeArtifacts{log: log}
	ctxw := &fakeContext{}
	uc := &usecases.SanitizeProject{
		Repo: store, UoW: store, Artifacts: arts, Context: ctxw,
		Clock:    fixedClock{time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)},
		NewRunID: func() string { return "run-1" },
	}
	return uc, store, arts, ctxw, log
}

// TestSanitize_PreviewNoMuta cubre US2.1 (FR-006).
func TestSanitize_PreviewNoMuta(t *testing.T) {
	uc, store, arts, _, log := newSanitizeFixture()
	before := len(store.rows)
	r, err := uc.Preview(context.Background(), "p")
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if r.Status != domain.SanitizeStatusPreview || r.RowsChanged != 0 || len(arts.reports) != 0 {
		t.Fatalf("preview con efectos: %+v reports=%d", r, len(arts.reports))
	}
	if len(store.rows) != before || store.rows["session/s1"].Text != leak037 {
		t.Fatal("preview modificó filas")
	}
	for _, step := range *log {
		if step == "begin" || strings.HasPrefix(step, "publish") {
			t.Fatalf("preview ejecutó %q", step)
		}
	}
	got := map[domain.SanitizeCategory]int{}
	for _, c := range r.Categories {
		got[c.Category] = c.AffectedCount
	}
	want := map[domain.SanitizeCategory]int{domain.SanitizeSession: 1, domain.SanitizeRelation: 1, domain.SanitizeOriginal: 1, domain.SanitizeMemoryReview: 1}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("categoría %s = %d, quiero %d", k, got[k], v)
		}
	}
}

// TestSanitize_ApplyCompleto cubre US2.2/US2.3 (FR-007): informe pendiente
// antes de la transacción, cambios solo en texto, limpieza verificada y
// segunda aplicación sin cambios.
func TestSanitize_ApplyCompleto(t *testing.T) {
	uc, store, arts, ctxw, log := newSanitizeFixture()
	arts.needContext = true
	r, err := uc.Apply(context.Background(), "p")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if (*log)[0] != "preview" || (*log)[1] != "publish:pending_cleanup" || (*log)[2] != "begin" {
		t.Fatalf("orden incorrecto: %v", *log)
	}
	if r.Status != domain.SanitizeStatusCompleted || r.RowsChanged != 3 || r.RunID != "run-1" || r.StartedAt == nil || r.FinishedAt == nil {
		t.Fatalf("informe final: %+v", r)
	}
	if got := store.rows["session/s1"]; domain.NeedsSanitize(got.Text) || !strings.Contains(got.Text, "público") || got.RecordID != "s1" {
		t.Fatalf("sesión mal saneada: %+v", got)
	}
	if _, ok := store.rows["compression_original/abc"]; ok {
		t.Fatal("original protegido no eliminado")
	}
	if store.rows["memory_review/9"].Text != "Z</private>" {
		t.Fatal("memory_review no debe modificarse")
	}
	if ctxw.writes != 1 {
		t.Fatalf("contexto regenerado %d veces", ctxw.writes)
	}
	if arts.reports[0].Status != domain.SanitizeStatusCompleted {
		t.Fatalf("informe persistido: %+v", arts.reports[0])
	}

	again, err := uc.Apply(context.Background(), "p")
	if err != nil || again.RowsChanged != 0 || again.Status != domain.SanitizeStatusCompleted {
		t.Fatalf("segundo apply: %+v err=%v", again, err)
	}
	if again.ExternalCopies == "" {
		t.Fatal("falta el aviso de copias externas")
	}
}

// TestSanitize_SinInformeNoTocaDatos: si el informe pendiente no se puede
// persistir, se aborta antes de abrir la transacción.
func TestSanitize_SinInformeNoTocaDatos(t *testing.T) {
	uc, store, arts, _, log := newSanitizeFixture()
	arts.publishErr = errors.New("disco lleno")
	r, err := uc.Apply(context.Background(), "p")
	if !hasCode(err, domain.SanitizeCodeFailed) || r.Status != domain.SanitizeStatusFailed {
		t.Fatalf("quiero SANITIZE_FAILED/failed: %+v %v", r, err)
	}
	for _, step := range *log {
		if step == "begin" {
			t.Fatal("abrió transacción sin informe")
		}
	}
	if store.rows["session/s1"].Text != leak037 {
		t.Fatal("modificó datos")
	}
}

// TestSanitize_RollbackAntesDelCommit cubre US2.4: error o cancelación en la
// transacción no deja cambios y el informe queda pendiente.
func TestSanitize_RollbackAntesDelCommit(t *testing.T) {
	uc, store, arts, _, log := newSanitizeFixture()
	store.failApply = errors.New("disk I/O error SECRETO_037")
	r, err := uc.Apply(context.Background(), "p")
	if !hasCode(err, domain.SanitizeCodeFailed) || r.Status != domain.SanitizeStatusPendingCleanup {
		t.Fatalf("quiero SANITIZE_FAILED/pending: %+v %v", r, err)
	}
	if strings.Contains(err.Error(), "SECRETO_037") {
		t.Fatalf("el error revela la causa cruda: %v", err)
	}
	if !contains(*log, "rollback") || store.rows["session/s1"].Text != leak037 {
		t.Fatalf("sin rollback: %v", *log)
	}
	if arts.reports[0].Status != domain.SanitizeStatusPendingCleanup {
		t.Fatalf("informe no quedó pendiente: %+v", arts.reports[0])
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store.failApply = nil
	if _, err := uc.Apply(ctx, "p"); !hasCode(err, domain.SanitizeCodeTimeout) {
		t.Fatalf("cancelación: quiero SANITIZE_TIMEOUT, got %v", err)
	}
	if store.rows["session/s1"].Text != leak037 {
		t.Fatal("cancelación confirmó cambios")
	}
}

// TestSanitize_FalloPosteriorAlCommitQuedaPendiente cubre US2.4/US2.5: tras
// el commit, un checkpoint bloqueado o un residuo deja pending_cleanup y un
// reintento posterior completa (y cierra también el informe anterior).
func TestSanitize_FalloPosteriorAlCommitQuedaPendiente(t *testing.T) {
	uc, store, arts, _, _ := newSanitizeFixture()
	store.compactErr = domain.NewSanitizeError(domain.SanitizeCodeBusy, errors.New("busy"))
	r, err := uc.Apply(context.Background(), "p")
	if !hasCode(err, domain.SanitizeCodeBusy) || r.Status != domain.SanitizeStatusPendingCleanup || r.RowsChanged != 3 {
		t.Fatalf("quiero BUSY/pending con filas ya saneadas: %+v %v", r, err)
	}
	if domain.NeedsSanitize(store.rows["session/s1"].Text) {
		t.Fatal("las filas confirmadas no deben revertirse")
	}

	store.compactErr = nil
	arts.residues = []domain.SanitizeArtifact{{Identity: "backups/x.json", Category: "backup", Status: domain.ArtifactPending, ErrorCode: "unsafe_link"}}
	uc.NewRunID = func() string { return "run-2" }
	r, err = uc.Apply(context.Background(), "p")
	if !hasCode(err, domain.SanitizeCodePendingCleanup) || r.Status != domain.SanitizeStatusPendingCleanup || len(r.Pending) == 0 {
		t.Fatalf("residuo: quiero PENDING_CLEANUP: %+v %v", r, err)
	}

	arts.residues = nil
	uc.NewRunID = func() string { return "run-3" }
	r, err = uc.Apply(context.Background(), "p")
	if err != nil || r.Status != domain.SanitizeStatusCompleted {
		t.Fatalf("reintento: %+v %v", r, err)
	}
	for _, rep := range arts.reports {
		if rep.Status != domain.SanitizeStatusCompleted {
			t.Fatalf("informe %s sigue %s tras completar", rep.RunID, rep.Status)
		}
	}
}

func hasCode(err error, code string) bool {
	var se *domain.SanitizeError
	return errors.As(err, &se) && se.Code == code
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// TestSanitize_InformeIlegibleImpideCompleted cubre C-002 (acr_9ba278e2):
// con un informe anterior ilegible, Apply no declara completed y la vista
// previa lo avisa; al retirarlo, el reintento completa.
func TestSanitize_InformeIlegibleImpideCompleted(t *testing.T) {
	uc, _, arts, _, _ := newSanitizeFixture()
	arts.unreadable = []domain.SanitizeArtifact{{Identity: "maintenance/privacy/run-roto.json", Category: "report", Status: domain.ArtifactPending, ErrorCode: "unreadable_report"}}

	prev, err := uc.Preview(context.Background(), "p")
	if err != nil || len(prev.Pending) == 0 || len(prev.Artifacts) != 1 {
		t.Fatalf("la vista previa debe avisar del informe ilegible: %+v %v", prev, err)
	}
	r, err := uc.Apply(context.Background(), "p")
	if !hasCode(err, domain.SanitizeCodePendingCleanup) || r.Status != domain.SanitizeStatusPendingCleanup {
		t.Fatalf("quiero PENDING_CLEANUP: %+v %v", r, err)
	}
	found := false
	for _, a := range r.Artifacts {
		found = found || a.ErrorCode == "unreadable_report"
	}
	if !found {
		t.Fatalf("el informe ilegible no figura en artifacts: %+v", r.Artifacts)
	}

	arts.unreadable = nil
	uc.NewRunID = func() string { return "run-2" }
	if r, err := uc.Apply(context.Background(), "p"); err != nil || r.Status != domain.SanitizeStatusCompleted {
		t.Fatalf("sin el informe ilegible debe completar: %+v %v", r, err)
	}
}

// TestSanitize_CategoriasReflejanLoAplicado cubre S-01 (acr_9ba278e2): si un
// escritor limpia una fila entre la vista previa y la transacción, el
// informe final describe lo que se aplicó, no la lectura previa.
func TestSanitize_CategoriasReflejanLoAplicado(t *testing.T) {
	uc, store, _, _, _ := newSanitizeFixture()
	store.onBegin = func(rows map[string]domain.SanitizeCandidate) {
		r := rows["relation/7"]
		r.Text = "razonamiento ya limpio"
		rows["relation/7"] = r
	}
	r, err := uc.Apply(context.Background(), "p")
	if err != nil || r.Status != domain.SanitizeStatusCompleted {
		t.Fatalf("Apply: %+v %v", r, err)
	}
	for _, c := range r.Categories {
		if c.Category == domain.SanitizeRelation {
			t.Fatalf("el informe conserva una categoría que no se aplicó: %+v", r.Categories)
		}
	}
	if r.RowsChanged != 2 {
		t.Fatalf("rows_changed = %d, quiero 2", r.RowsChanged)
	}
}
