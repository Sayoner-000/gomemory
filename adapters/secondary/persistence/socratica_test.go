package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func socraticaTestRepo(t *testing.T) *SocraticaStore {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	return NewSocraticaStore(db, time.Second)
}
func socraticaTestFlow(id, client string) domain.RefinementFlow {
	return domain.RefinementFlow{ID: id, SchemaVersion: 1, Project: "p", Client: client, Mode: domain.ModeRefine, OriginalPrompt: "original", ProposedPrompt: "proposal", Phase: domain.PhaseReview, LastStablePhase: domain.PhaseReview, CreatedAt: time.Unix(100, 0).UTC(), UpdatedAt: time.Unix(100, 0).UTC()}
}
func TestSocraticaRepository(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := socraticaTestFlow("f", "claude")
	f.Scrutiny = &domain.ScrutinySynthesis{Restatement: "claim"}
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFlow(ctx, "f")
	if err != nil || got.Scrutiny == nil || got.Scrutiny.Restatement != "claim" {
		t.Fatal(got, err)
	}
	if _, found, err := r.ActiveFlow(ctx, "p", "claude"); err != nil || !found {
		t.Fatal(found, err)
	}
	if err := r.CreateFlow(ctx, socraticaTestFlow("second", "claude")); err == nil {
		t.Fatal("accepted second pending flow")
	}
	if err := r.CreateFlow(ctx, socraticaTestFlow("other", "codex")); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, dup, err := r.Approve(ctx, "f", fmt.Sprintf("dispatch%d", i), "proposal")
			if err != nil {
				t.Error(err)
				return
			}
			if !dup {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("approve winners=%d", wins)
	}
	got, err = r.GetFlow(ctx, "f")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Ack(ctx, "f", "foreign", "", "n"); err == nil {
		t.Fatal("foreign ack accepted")
	}
	wins = 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := r.Ack(ctx, "f", got.DispatchID, "", fmt.Sprintf("nonce%d", i))
			if err != nil {
				t.Error(err)
			}
			if res.Acked {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("ack winners=%d", wins)
	}
	got, err = r.GetFlow(ctx, "f")
	if err != nil {
		t.Fatal(err)
	}
	if res, err := r.Ack(ctx, "f", got.DispatchID, "", got.AckNonce); err != nil || !res.Acked || res.AlreadyDispatched || res.Granted {
		t.Fatal(res, err)
	}
	if res, err := r.Ack(ctx, "f", got.DispatchID, "", "new"); err != nil || res.Acked || !res.AlreadyDispatched || res.Granted {
		t.Fatal(res, err)
	}
	if err = r.RedactFlow(ctx, "f"); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.ConsumeDispatch(ctx, got.DispatchID, "proposal"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := r.ConsumeDispatch(ctx, got.DispatchID, "proposal"); err != nil || ok {
		t.Fatal(ok, err)
	}
	if _, _, err = r.FindByDispatchID(ctx, got.DispatchID); err != nil {
		t.Fatal(err)
	}
	if ids, err := r.PurgeTerminal(ctx, "p", time.Now()); err != nil || len(ids) != 1 || ids[0] != "f" {
		t.Fatal(ids, err)
	}
}
func TestSocraticaRepositoryRollbackPrivacyHistory(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := socraticaTestFlow("f", "claude")
	f.OriginalPrompt = "original <private>hidden value</private>"
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	f, err := r.GetFlow(ctx, "f")
	if err != nil || strings.Contains(f.OriginalPrompt, "hidden value") {
		t.Fatal(f, err)
	}
	stop := errors.New("history failed")
	err = r.WithTransaction(ctx, func(tx ports.SocraticaRepository) error {
		next := f
		next.Phase = domain.PhaseApproved
		if err := tx.SaveTransition(ctx, f, next); err != nil {
			return err
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
	got, err := r.GetFlow(ctx, "f")
	if err != nil || got.Phase != domain.PhaseReview {
		t.Fatal(got, err)
	}
	for i := 0; i < 25; i++ {
		h := domain.RefinementHistory{ID: fmt.Sprint(i), FlowID: fmt.Sprint(i), Project: "p", Client: "claude", OriginalPrompt: "original", ProposedPrompt: "proposal", CompletedAt: time.Unix(int64(i), 0).UTC()}
		if err = r.UpsertHistory(ctx, h, 20); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := r.ListHistory(ctx, "p", "claude", 100)
	if err != nil || len(rows) != 20 || rows[0].ProposedPrompt != "proposal" {
		t.Fatal(rows, err)
	}
}

func TestSocraticaDispatchLock(t *testing.T) {
	lock := NewSocraticaDispatchLock(t.TempDir())
	release, acquired, err := lock.TryAcquire("flow")
	if err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	defer release()
	_, acquired, err = lock.TryAcquire("flow")
	if err != nil || acquired {
		t.Fatal(acquired, err)
	}
	release()
	release2, acquired, err := lock.TryAcquire("flow")
	if err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	release2()
	if _, _, err := lock.TryAcquire("../outside"); err == nil {
		t.Fatal("accepted path traversal")
	}
}

func TestSocraticaRepositoryClaimsAndRecovery(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := socraticaTestFlow("failed", "claude")
	f.Phase = domain.PhaseFailed
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	approved, _, err := r.Approve(ctx, f.ID, "dispatch", "original")
	if err != nil || approved.ApprovedPrompt != "original" {
		t.Fatal(approved, err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for _, owner := range []string{"a", "b"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			ok, err := r.ClaimDispatch(ctx, f.ID, "dispatch", owner, time.Unix(100, 0))
			if err != nil {
				t.Error(err)
			}
			if ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(owner)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatal(wins)
	}
	claimed, err := r.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Un propietario ajeno no afecta filas, y lo dice (H-02).
	if err = r.ReleaseDispatch(ctx, f.ID, "foreign"); !errors.Is(err, ports.ErrSocraticaNoMatch) {
		t.Fatal(err)
	}
	if ok, err := r.ClaimDispatch(ctx, f.ID, "dispatch", "new", time.Unix(999999, 0)); err != nil || ok {
		t.Fatal(ok, err)
	}
	if _, err = r.Ack(ctx, f.ID, "dispatch", "", "nonce"); err == nil {
		t.Fatal("MCP ack accepted local claim")
	}
	if err = r.ForceResend(ctx, f.ID, "dispatch", "new-dispatch"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Ack(ctx, f.ID, "dispatch", claimed.DispatchClaimOwner, "nonce"); err == nil {
		t.Fatal("old owner ack accepted")
	}
	if ok, err := r.ClaimDispatch(ctx, f.ID, "new-dispatch", "new", time.Unix(999999, 0)); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err = r.MarkDispatched(ctx, f.ID, "new-dispatch"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.PurgeStale(ctx, "p", time.Unix(1000000, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.GetFlow(ctx, f.ID); err != nil {
		t.Fatal("terminal purged as pending", err)
	}
	pending := socraticaTestFlow("next", "claude")
	if err = r.CreateFlow(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if err = r.ForceResend(ctx, f.ID, "new-dispatch", "third"); err == nil {
		t.Fatal("resend created second pending flow")
	}
	if err = r.DeleteFlow(ctx, pending.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSocraticaRepositoryReadsHistoricalPrivate(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := socraticaTestFlow("historical", "claude")
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`UPDATE socratica_flows SET original_prompt=?,questions_json=? WHERE id=?`, "public <private>hidden</private>", `[{"text":"public <private>hidden</private>","options":["A","B"]}]`, f.ID); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFlow(ctx, f.ID)
	if err != nil || strings.Contains(got.OriginalPrompt, "hidden") || strings.Contains(got.Questions[0].Text, "hidden") {
		t.Fatal("historical private data served", err)
	}
}

func TestSocraticaRepositoryQueryTimeout(t *testing.T) {
	r := socraticaTestRepo(t)
	ctx := context.Background()
	f := socraticaTestFlow("blocked", "claude")
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	r.timeout = 100 * time.Millisecond
	start := time.Now()
	_, _, err = r.Approve(ctx, f.ID, "dispatch", "proposal")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("timeout elapsed=%s err=%v", time.Since(start), err)
	}
}

func BenchmarkSocraticaApprove(b *testing.B) {
	db, err := sql.Open("sqlite", filepath.Join(b.TempDir(), "bench.db")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err = migrate(db); err != nil {
		b.Fatal(err)
	}
	r := NewSocraticaStore(db, time.Second)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f := socraticaTestFlow(fmt.Sprint(i), "claude")
		if err = r.CreateFlow(ctx, f); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, _, err = r.Approve(ctx, f.ID, "d"+f.ID, "proposal"); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err = r.DeleteFlow(ctx, f.ID); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

// TestSocraticaRepository_Complementos cubre el resto de T005: rechazo tipado
// del ack, fases que Approve no toca, redacción, purgas con ids, historial que
// sobrevive al flujo (I6), timeout de SaveTransition y migración aditiva.
func TestSocraticaRepository_Complementos(t *testing.T) {
	ctx := context.Background()
	t.Run("approve only from review or failed", func(t *testing.T) {
		r := socraticaTestRepo(t)
		f := socraticaTestFlow("q", "claude")
		f.Phase = domain.PhaseQuestions
		if err := r.CreateFlow(ctx, f); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.Approve(ctx, "q", "d", "x"); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatal(err)
		}
		got, err := r.GetFlow(ctx, "q")
		if err != nil || got.Phase != domain.PhaseQuestions || got.DispatchID != "" {
			t.Fatal(got, err)
		}
	})
	t.Run("ack rejection is typed", func(t *testing.T) {
		r := socraticaTestRepo(t)
		if err := r.CreateFlow(ctx, socraticaTestFlow("f", "claude")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.Approve(ctx, "f", "d", "proposal"); err != nil {
			t.Fatal(err)
		}
		if ok, err := r.ClaimDispatch(ctx, "f", "d", "owner", time.Unix(1, 0)); err != nil || !ok {
			t.Fatal(ok, err)
		}
		for _, tc := range [][2]string{{"d", "other"}, {"x", "owner"}, {"d", ""}} {
			if _, err := r.Ack(ctx, "f", tc[0], tc[1], "n"); !errors.Is(err, ports.ErrAckRejected) {
				t.Fatal(tc, err)
			}
		}
		if res, err := r.Ack(ctx, "f", "d", "owner", "n"); err != nil || !res.Acked || !res.Granted {
			t.Fatal(res, err)
		}
	})
	t.Run("redact keeps dispatch identity", func(t *testing.T) {
		r := socraticaTestRepo(t)
		f := socraticaTestFlow("f", "claude")
		f.Questions = []domain.Question{{Text: "q", Options: []string{"a", "b"}}}
		f.Answers = []domain.Answer{{QuestionIndex: 0, Choice: "a"}}
		f.Scrutiny = &domain.ScrutinySynthesis{Restatement: "s"}
		if err := r.CreateFlow(ctx, f); err != nil {
			t.Fatal(err)
		}
		approved, _, err := r.Approve(ctx, "f", "d", "proposal")
		if err != nil {
			t.Fatal(err)
		}
		if err = r.RedactFlow(ctx, "f"); err != nil {
			t.Fatal(err)
		}
		got, err := r.GetFlow(ctx, "f")
		if err != nil || got.OriginalPrompt != "" || got.ProposedPrompt != "" || got.ApprovedPrompt != "" || len(got.Questions) != 0 || len(got.Answers) != 0 || got.Scrutiny != nil {
			t.Fatal(got, err)
		}
		if got.DispatchID != "d" || got.ApprovedHash != approved.ApprovedHash {
			t.Fatal("dispatch identity lost", got)
		}
	})
	t.Run("purges return ids and keep approved", func(t *testing.T) {
		r := socraticaTestRepo(t)
		old := time.Unix(100, 0).UTC()
		for _, tc := range []struct {
			id, client string
			phase      domain.SocraticaPhase
		}{{"stale", "claude", domain.PhaseQuestions}, {"approved", "codex", domain.PhaseApproved}, {"done", "opencode", domain.PhaseCancelled}} {
			f := socraticaTestFlow(tc.id, tc.client)
			f.Phase = tc.phase
			f.UpdatedAt = old
			if err := r.CreateFlow(ctx, f); err != nil {
				t.Fatal(err)
			}
		}
		ids, err := r.PurgeStale(ctx, "p", old.Add(time.Hour))
		if err != nil || len(ids) != 1 || ids[0] != "stale" {
			t.Fatal(ids, err)
		}
		if _, err = r.GetFlow(ctx, "approved"); err != nil {
			t.Fatal("approved without ack purged", err)
		}
		if ids, err = r.PurgeTerminal(ctx, "other-project", old.Add(time.Hour)); err != nil || len(ids) != 0 {
			t.Fatal(ids, err)
		}
		if ids, err = r.PurgeTerminal(ctx, "p", old.Add(time.Hour)); err != nil || len(ids) != 1 || ids[0] != "done" {
			t.Fatal(ids, err)
		}
	})
	t.Run("prune terminal per client", func(t *testing.T) {
		r := socraticaTestRepo(t)
		for i := 0; i < 3; i++ {
			for _, client := range []string{"claude", "codex"} {
				f := socraticaTestFlow(fmt.Sprintf("%s%d", client, i), client)
				f.Phase = domain.PhaseDispatched
				f.UpdatedAt = time.Unix(int64(100+i), 0).UTC()
				if err := r.CreateFlow(ctx, f); err != nil {
					t.Fatal(err)
				}
			}
		}
		ids, err := r.PruneTerminal(ctx, "p", "claude", 2)
		if err != nil || len(ids) != 1 || ids[0] != "claude0" {
			t.Fatal(ids, err)
		}
		if _, err = r.GetFlow(ctx, "codex0"); err != nil {
			t.Fatal("other client pruned", err)
		}
	})
	t.Run("history survives the flow (I6)", func(t *testing.T) {
		r := socraticaTestRepo(t)
		f := socraticaTestFlow("f", "claude")
		f.Phase = domain.PhaseCancelled
		f.UpdatedAt = time.Unix(100, 0).UTC()
		if err := r.CreateFlow(ctx, f); err != nil {
			t.Fatal(err)
		}
		h := domain.RefinementHistory{ID: "f", FlowID: "f", Project: "p", Client: "claude", Mode: domain.ModeRefine, OriginalPrompt: "original", ProposedPrompt: "first", CompletedAt: time.Unix(100, 0).UTC()}
		if err := r.UpsertHistory(ctx, h, 20); err != nil {
			t.Fatal(err)
		}
		h.ProposedPrompt = "edited"
		if err := r.UpsertHistory(ctx, h, 20); err != nil {
			t.Fatal(err)
		}
		if _, err := r.PurgeTerminal(ctx, "p", time.Unix(200, 0)); err != nil {
			t.Fatal(err)
		}
		rows, err := r.ListHistory(ctx, "p", "claude", 20)
		if err != nil || len(rows) != 1 || rows[0].OriginalPrompt != "original" || rows[0].ProposedPrompt != "edited" {
			t.Fatal(rows, err)
		}
	})
	t.Run("SaveTransition honours the query timeout", func(t *testing.T) {
		r := socraticaTestRepo(t)
		f := socraticaTestFlow("blocked", "claude")
		if err := r.CreateFlow(ctx, f); err != nil {
			t.Fatal(err)
		}
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		r.timeout = 100 * time.Millisecond
		next := f
		next.Phase = domain.PhaseApproved
		start := time.Now()
		err = r.SaveTransition(ctx, f, next)
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
			t.Fatalf("elapsed=%s err=%v", time.Since(start), err)
		}
		_ = tx.Rollback()
		r.timeout = time.Second
		if got, err := r.GetFlow(ctx, "blocked"); err != nil || got.Phase != domain.PhaseReview {
			t.Fatal("partial change", got, err)
		}
	})
	t.Run("older database migrates additively", func(t *testing.T) {
		r := socraticaTestRepo(t)
		if _, err := r.db.Exec(`DROP TABLE socratica_flows; DROP TABLE socratica_history`); err != nil {
			t.Fatal(err)
		}
		// La base simulada es anterior al versionado del esquema.
		if _, err := r.db.Exec(`PRAGMA user_version = 0`); err != nil {
			t.Fatalf("reset legacy schema version: %v", err)
		}
		for i := 0; i < 2; i++ {
			if err := migrate(r.db); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.CreateFlow(ctx, socraticaTestFlow("f", "claude")); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSocraticaDispatchLock_Remove(t *testing.T) {
	lock := NewSocraticaDispatchLock(t.TempDir())
	release, ok, err := lock.TryAcquire("flow")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	release()
	if err = lock.Remove("flow"); err != nil {
		t.Fatal(err)
	}
	if err = lock.Remove("flow"); err != nil {
		t.Fatal("missing lock file must not fail", err)
	}
	if err = lock.Remove("../x"); err == nil {
		t.Fatal("accepted path traversal")
	}
}

// TestSocraticaMutations_SinFilaNoEsExito cierra H-02 (auditoría 2026-10-16):
// una mutación que no afecta a ninguna fila no puede devolver éxito.
func TestSocraticaMutations_SinFilaNoEsExito(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	for name, call := range map[string]func() error{
		"ReleaseDispatch": func() error { return r.ReleaseDispatch(ctx, "nope", "owner") },
		"MarkDispatched":  func() error { return r.MarkDispatched(ctx, "nope", "d") },
		"ForceResend":     func() error { return r.ForceResend(ctx, "nope", "d", "d2") },
		"DeleteFlow":      func() error { return r.DeleteFlow(ctx, "nope") },
		"RedactFlow":      func() error { return r.RedactFlow(ctx, "nope") },
	} {
		if err := call(); !errors.Is(err, ports.ErrSocraticaNoMatch) {
			t.Errorf("%s sobre un flujo inexistente devolvió %v", name, err)
		}
	}
}
