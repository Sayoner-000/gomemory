package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/domain"
)

// Ida y vuelta de cada registro de control HCD por el almacén real (SQLite temporal).
func TestHCDStore_ControlRecordsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newHCDTestStore(t)
	id, err := s.NextUnitID(ctx)
	if err != nil || id != "HCD-001" {
		t.Fatalf("NextUnitID=%q %v", id, err)
	}
	u := domain.Unit{ID: id, Kind: domain.KindFix, Title: "T", Slug: "t", Approach: "hcd", State: domain.StateDraft, StateRevision: 1, Implementer: "dev"}
	if err = s.CreateUnit(ctx, u); err != nil {
		t.Fatal(err)
	}
	if next, _ := s.NextUnitID(ctx); next != "HCD-002" {
		t.Fatalf("NextUnitID tras crear=%q", next)
	}
	if err = s.SaveNormativeRevision(ctx, "playbook", "abc123abc123", "contenido", "default", "initial", true); err != nil {
		t.Fatal(err)
	}
	if urgent, e := s.ListUrgentRevisions(ctx); e != nil || len(urgent) != 1 {
		t.Fatalf("urgent=%v %v", urgent, e)
	}

	// Decisiones
	did, err := s.NextDecisionID(ctx, id)
	if err != nil || did != "D-01" {
		t.Fatalf("NextDecisionID=%q %v", did, err)
	}
	d := domain.HCDDecision{ID: did, UnitID: id, Kind: "approach", Status: "accepted", Summary: "HCD", Revision: "r1", Refs: []string{id}, Source: "start", MemorySync: "pending"}
	if err = s.SaveDecision(ctx, d); err != nil {
		t.Fatal(err)
	}
	d.Summary, d.Status = "HCD v2", "superseded"
	if err = s.UpdateDecision(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err = s.MarkDecisionSynced(ctx, id, did, 7); err != nil {
		t.Fatal(err)
	}
	ds, err := s.ListDecisions(ctx, id)
	if err != nil || len(ds) != 1 || ds[0].Summary != "HCD v2" || ds[0].MemorySync != "synced" || ds[0].MemoryID != 7 || len(ds[0].Refs) != 1 {
		t.Fatalf("decisions=%+v %v", ds, err)
	}

	// Aclaraciones
	cid, err := s.NextClarificationID(ctx, id)
	if err != nil || cid != "CL-01" {
		t.Fatalf("NextClarificationID=%q %v", cid, err)
	}
	cl := domain.Clarification{ID: cid, UnitID: id, TargetRef: "C-01", Question: "¿?", Recommendation: "A", Status: "open", Options: []string{"A", "B"}, Blocking: true}
	if err = s.SaveClarification(ctx, cl); err != nil {
		t.Fatal(err)
	}
	if got, e := s.GetClarification(ctx, id, cid); e != nil || !got.Blocking || len(got.Options) != 2 {
		t.Fatalf("clarification=%+v %v", got, e)
	}
	if open, e := s.ListOpenClarifications(ctx, id); e != nil || len(open) != 1 {
		t.Fatalf("open=%v %v", open, e)
	}
	if _, e := s.GetClarification(ctx, id, "CL-99"); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("missing clarification err=%v", e)
	}

	// Avisos de aprobación (un solo uso) y aprobaciones
	prompt := domain.ApprovalPrompt{ID: "P-01", UnitID: id, Object: "definition", Revision: "r1", AskedAt: "2026-10-28T00:00:00Z", Options: []string{"A", "B"}}
	if err = s.SaveApprovalPrompt(ctx, prompt); err != nil {
		t.Fatal(err)
	}
	if got, e := s.GetApprovalPrompt(ctx, "P-01", id); e != nil || got.Consumed || len(got.Options) != 2 {
		t.Fatalf("prompt=%+v %v", got, e)
	}
	if err = s.ConsumeApprovalPrompt(ctx, "P-01", id); err != nil {
		t.Fatal(err)
	}
	if e := s.ConsumeApprovalPrompt(ctx, "P-01", id); e == nil {
		t.Fatal("un aviso consumido no se consume otra vez")
	}
	aid, err := s.NextApprovalID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveApproval(ctx, domain.Approval{ID: aid, UnitID: id, Object: "definition", Revision: "r1", Origin: "conversation", Quote: "apruebo", Client: "codex", ApprovedAt: "2026-10-28T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if as, e := s.ListApprovals(ctx, id); e != nil || len(as) != 1 || as[0].Quote != "apruebo" {
		t.Fatalf("approvals=%+v %v", as, e)
	}

	// Enlaces
	if err = s.AddLink(ctx, domain.Link{UnitID: id, FromKind: "decision", FromID: did, ToKind: "reference", ToID: "x", Relation: "justifies"}); err != nil {
		t.Fatal(err)
	}
	if ls, e := s.ListLinks(ctx, id); e != nil || len(ls) != 1 {
		t.Fatalf("links=%v %v", ls, e)
	}

	// Unidad activa y heartbeat
	if err = s.SetFocus(ctx, domain.Focus{UnitID: id, Client: "codex", SessionID: "s1", SetAt: "2026-10-28T00:00:00Z", SetBy: "start"}); err != nil {
		t.Fatal(err)
	}
	if f, e := s.GetFocus(ctx); e != nil || f.UnitID != id {
		t.Fatalf("focus=%+v %v", f, e)
	}
	if active, e := s.ActiveHCDUnit(ctx); e != nil || active.ID != id {
		t.Fatalf("active=%+v %v", active, e)
	}
	if err = s.RecordHeartbeat(ctx, domain.Heartbeat{ID: "HB-1", UnitID: id, SessionID: "s1", Client: "codex", Stage: "slicing", StateRevision: 1, Files: []string{"a.go"}, Drift: []string{"files_without_execution"}, SeenAt: "2026-10-28T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if hs, e := s.ListHeartbeats(ctx, id); e != nil || len(hs) != 1 || len(hs[0].Drift) != 1 {
		t.Fatalf("heartbeats=%+v %v", hs, e)
	}
	if at, e := s.HCDActivityAt(ctx, id); e != nil || at == "" {
		t.Fatalf("activity=%q %v", at, e)
	}
	if err = s.ClearFocus(ctx); err != nil {
		t.Fatal(err)
	}
	if _, e := s.ActiveHCDUnit(ctx); e == nil {
		t.Fatal("sin foco no hay unidad activa")
	}
	if us, e := s.ListHCDUnits(ctx); e != nil || len(us) != 1 {
		t.Fatalf("units=%v %v", us, e)
	}

	// Bloqueos
	bid, err := s.NextBlockID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveBlock(ctx, domain.Block{ID: bid, UnitID: id, TargetRef: "T-001", Reason: "falta permiso", Status: "open", Critical: true}); err != nil {
		t.Fatal(err)
	}
	if bs, e := s.ListBlocks(ctx, id); e != nil || len(bs) != 1 || !bs[0].Critical {
		t.Fatalf("blocks=%+v %v", bs, e)
	}

	// Ejecuciones y evidencia
	x := domain.Execution{ID: "X-1", UnitID: id, TaskID: "T-001", DefinitionRev: "r1", DeliveryRev: "r2", State: "granted", GrantedAt: "2026-10-28T00:00:00Z", ExpiresAt: "2026-10-29T00:00:00Z"}
	if err = s.SaveExecution(ctx, x); err != nil {
		t.Fatal(err)
	}
	if got, e := s.GetExecution(ctx, id, "X-1"); e != nil || got.State != "granted" {
		t.Fatalf("execution=%+v %v", got, e)
	}
	if xs, e := s.ListExecutions(ctx, id); e != nil || len(xs) != 1 {
		t.Fatalf("executions=%v %v", xs, e)
	}
	ev := domain.Evidence{ID: "E-1", UnitID: id, ExecutionID: "X-1", Procedure: "p", Environment: "e", Result: "pass", Covers: []string{"T-001"}, SubjectRef: []string{"tree:abc"}, InspectRef: []string{"https://x"}, DefinitionRev: "r1", DeliveryRev: "r2", PlaybookRev: "p", ConstitutionRev: "c", Executor: "codex", RecordedAt: "2026-10-28T00:00:00Z"}
	if err = s.SaveEvidence(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if es, e := s.ListEvidence(ctx, id); e != nil || len(es) != 1 || es[0].Result != "pass" || len(es[0].SubjectRef) != 1 {
		t.Fatalf("evidence=%+v %v", es, e)
	}
	if _, e := s.LastVerificationRevision(ctx, id); e != nil {
		t.Fatal(e)
	}

	// Gobierno: excepciones y adopciones
	eid, err := s.NextGovernanceID(ctx, "exception")
	if err != nil || eid != "E-01" {
		t.Fatalf("NextGovernanceID=%q %v", eid, err)
	}
	if _, e := s.NextGovernanceID(ctx, "otra"); !errors.Is(e, domain.ErrValidation) {
		t.Fatalf("kind inválido err=%v", e)
	}
	if err = s.SaveException(ctx, domain.Exception{ID: eid, UnitID: id, RuleID: "HCD-MEM-01", Scope: "unidad", Reason: "r", Owner: "dev", ApprovalID: aid, Status: "active", ExpiresAt: "2026-12-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if xs, e := s.ListExceptions(ctx, id); e != nil || len(xs) != 1 {
		t.Fatalf("exceptions=%v %v", xs, e)
	}
	if err = s.SaveAdoption(ctx, domain.PlaybookAdoption{ID: "AD-01", UnitID: id, Doc: "playbook", FromRev: "a", ToRev: "b", ImpactJSON: "{}", Reason: "import", Actor: "dev", ApprovalID: aid, AdoptedAt: "2026-10-28T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if rev, e := s.GetNormativeRevision(ctx, "playbook", "abc123abc123"); e != nil || rev.Content != "contenido" || !rev.Urgent {
		t.Fatalf("normative=%+v %v", rev, e)
	}

	// Migraciones
	mid, err := s.NextMigrationID(ctx)
	if err != nil || mid != "M-01" {
		t.Fatalf("NextMigrationID=%q %v", mid, err)
	}
	m := domain.Migration{ID: mid, SourceDir: "specs/001-x", SourceRev: "abc", Status: domain.MigrationProposed, Items: []domain.MigrationItem{{Source: "spec.md:1", Kind: "heading", Text: "t", Class: domain.MigrationConsolidated}}}
	if err = s.SaveMigration(ctx, m); err != nil {
		t.Fatal(err)
	}
	m.UnitID, m.Status = id, domain.MigrationActivated
	if err = s.SaveMigration(ctx, m); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMigration(ctx, mid)
	if err != nil || got.Status != domain.MigrationActivated || got.UnitID != id || got.Counts[domain.MigrationConsolidated] != 1 || got.Revision == "" {
		t.Fatalf("migration=%+v %v", got, err)
	}
	if all, e := s.ListMigrations(ctx); e != nil || len(all) != 1 {
		t.Fatalf("migrations=%v %v", all, e)
	}
	if _, e := s.GetMigration(ctx, "M-99"); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("missing migration err=%v", e)
	}
}

// SUBSTR(id,4) sobre «APR-01» leía «-01»: la segunda aprobación salía APR-00 y
// la tercera repetía APR-01 y chocaba con la clave primaria.
func TestHCDStore_ApprovalIDsIncrease(t *testing.T) {
	ctx := context.Background()
	s := newHCDTestStore(t)
	u := domain.Unit{ID: "HCD-001", Kind: domain.KindFix, Title: "T", Slug: "t", Approach: "hcd", State: domain.StateDraft, StateRevision: 1}
	if err := s.CreateUnit(ctx, u); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"APR-01", "APR-02", "APR-03"} {
		id, err := s.NextApprovalID(ctx, u.ID)
		if err != nil || id != want {
			t.Fatalf("NextApprovalID=%q %v; want %s", id, err, want)
		}
		if err = s.SaveApproval(ctx, domain.Approval{ID: id, UnitID: u.ID, Object: "definition", Revision: "abc123abc123", Origin: "terminal", Quote: "apruebo", Client: "terminal", ApprovedAt: "2026-10-28T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}
	}
}

// ACR C-006: reutilizar un operation-id en otra unidad devolvía el resultado
// guardado de la primera aunque el payload coincidiera.
func TestHCDStore_OperationIDIsScopedToUnitAndCommand(t *testing.T) {
	ctx := context.Background()
	s := newHCDTestStore(t)
	if _, _, err := s.RecordOperation(ctx, "op-1", "HCD-001", "define", "hash", `{"unit":"HCD-001"}`); err != nil {
		t.Fatal(err)
	}
	if got, replayed, err := s.RecordOperation(ctx, "op-1", "HCD-001", "define", "hash", `{"unit":"HCD-001"}`); err != nil || !replayed || got != `{"unit":"HCD-001"}` {
		t.Fatalf("same operation replay=%q %v %v", got, replayed, err)
	}
	if _, _, err := s.RecordOperation(ctx, "op-1", "HCD-002", "define", "hash", `{"unit":"HCD-002"}`); !errors.Is(err, domain.ErrOperationConflict) {
		t.Fatalf("other unit err=%v", err)
	}
	if _, _, err := s.RecordOperation(ctx, "op-1", "HCD-001", "slice", "hash", `{}`); !errors.Is(err, domain.ErrOperationConflict) {
		t.Fatalf("other command err=%v", err)
	}
}

// El almacén es el último límite antes de SQLite: título, implementador y
// cita se protegen aquí aunque un caso de uso olvide hacerlo.
func TestHCDStore_ProtectsUnitFieldsAndQuotes(t *testing.T) {
	ctx := context.Background()
	s := newHCDTestStore(t)
	const secret = "valor-sintetico-42"
	private := "<private>" + secret + "</private>\x1b]0;x\x07"
	u := domain.Unit{ID: "HCD-001", Kind: domain.KindFix, Title: private, Slug: "a", Approach: "hcd", State: domain.StateDraft, StateRevision: 1, Implementer: private}
	if err := s.CreateUnit(ctx, u); err != nil {
		t.Fatal(err)
	}
	u.StateRevision = 2
	if err := s.UpdateUnit(ctx, u, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApproval(ctx, domain.Approval{ID: "APR-01", UnitID: u.ID, Object: "close", Revision: "r1", Origin: "conversation", Quote: "apruebo " + private, Client: "codex", ApprovedAt: "2026-10-28T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	var title, implementer, quote string
	if err := s.db.QueryRow("SELECT title, implementer FROM hcd_units WHERE id=?", u.ID).Scan(&title, &implementer); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT quote FROM hcd_approvals WHERE id='APR-01'").Scan(&quote); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(title+implementer+quote, secret) || strings.ContainsRune(title+implementer, 0x1b) {
		t.Fatalf("valor sin proteger: %q %q %q", title, implementer, quote)
	}
}
