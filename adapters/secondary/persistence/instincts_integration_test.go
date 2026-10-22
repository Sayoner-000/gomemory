package persistence

import (
	"context"
	"database/sql"
	"errors"
	"mem/adapters/secondary/expandstate"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type instinctClock struct{}

func (instinctClock) Now() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }

func TestInstinctSanitizeAllStoresAndContext(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", home)
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	project := ProjectKey(root)
	now := instinctClock{}.Now()
	repo := NewInstinctRepository(db, project)
	g, err := OpenInstinctGlobal(home)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	q := expandstate.NewQueue(filepath.Join(root, MemDir))
	obs := domain.Observation{ID: "one", Kind: "cmd", Key: "cmd:go-test", Sample: "safe"}
	if err := repo.Fold(ctx, []domain.Observation{obs}, now); err != nil {
		t.Fatal(err)
	}
	leak := "ghp_" + strings.Repeat("a", 36)
	if _, err := db.Exec(`UPDATE instinct_stats SET description=?,confidence=.7 WHERE project=?`, leak, project); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{project, "other"} {
		if err := g.RecordProject(ctx, p, domain.Instinct{Slug: "global", Kind: "cmd", Confidence: .7, Description: "safe"}, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := g.db.Exec(`UPDATE instinct_global SET description=?`, leak); err != nil {
		t.Fatal(err)
	}
	// Seed historical bytes directly; the queue writer already redacts new observations.
	raw := `{"id":"queue","kind":"cmd","key":"cmd:go-test","sample":"` + leak + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(root, MemDir, "expand-observations.ndjson"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewPrivacyMaintenanceStore(db)
	u := &usecases.SanitizeProject{Repo: store, UoW: store, Artifacts: NewPrivacyArtifacts(home, project, filepath.Join(root, MemDir, "context.md")), Clock: instinctClock{}, NewRunID: func() string { return "expand-sanitize" }, InstinctGlobal: g, InstinctQueue: q}
	preview, err := u.Preview(ctx, project)
	if err != nil || len(preview.Categories) != 3 {
		t.Fatalf("preview: %+v %v", preview.Categories, err)
	}
	report, err := u.Apply(ctx, project)
	if err != nil || len(report.Categories) != 3 || report.RowsChanged != 3 {
		t.Fatalf("apply: %+v %v", report, err)
	}
	var desc string
	if err := db.QueryRow(`SELECT description FROM instinct_stats WHERE project=?`, project).Scan(&desc); err != nil || strings.Contains(desc, leak) {
		t.Fatal("project residue", err)
	}
	candidates, err := g.SanitizeCandidates(ctx, project)
	if err != nil || len(candidates) > 0 {
		t.Fatal("global residue", err)
	}
	batch, err := q.ReadBatch(ctx, 10)
	if err != nil || len(batch) != 1 || strings.Contains(batch[0].Sample, leak) {
		t.Fatal("queue residue", err)
	}
	builder := usecases.New(NewMemoryRepository(db), NewSessionRepository(db), NewRelationRepository(db), root, project)
	builder.Instincts = repo
	builder.InstinctGlobal = g
	builder.ExpandSettings = NewSettingsRepository()
	out, err := builder.Build()
	if err != nil || !strings.Contains(out, "## Instincts") || !strings.Contains(out, "global") {
		t.Fatal(out, err)
	}
	settings := builder.ExpandSettings.Read(root)
	settings.ExpandOff = []string{"instincts"}
	if err := builder.ExpandSettings.Write(root, settings); err != nil {
		t.Fatal(err)
	}
	out, err = builder.Build()
	if err != nil || strings.Contains(out, "## Instincts") {
		t.Fatal("switch did not omit section", err)
	}
}

// B-001: una propuesta global se puede confirmar o rechazar en un proyecto
// que nunca la observó, y el rechazo persiste e impide inyectarla allí.
func TestInstinctGlobalProposalReviewableInNewProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", home)
	ctx := context.Background()
	now := instinctClock{}.Now()
	g, err := OpenInstinctGlobal(home)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	for _, p := range []string{"a", "b"} {
		if err := g.RecordProject(ctx, p, domain.Instinct{Slug: "cmd:go-test", Kind: "cmd", Confidence: .8, Description: "ejecuta go test"}, now); err != nil {
			t.Fatal(err)
		}
	}
	open := func() (*usecases.Instincts, func()) {
		root := t.TempDir()
		db, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		project := ProjectKey(root)
		return &usecases.Instincts{Repo: NewInstinctRepository(db, project), Global: g, Project: project,
			Memories: NewMemoryRepository(db), Now: func() time.Time { return now }}, func() { _ = db.Close() }
	}

	rejecting, closeC := open()
	defer closeC()
	if err := rejecting.Reject(ctx, "cmd:go-test"); err != nil {
		t.Fatalf("rechazar una propuesta global: %v", err)
	}
	if err := rejecting.Confirm(ctx, "cmd:go-test"); err == nil {
		t.Error("el rechazo es final")
	}
	if list, err := rejecting.List(ctx, false); err != nil || len(list) != 0 {
		t.Errorf("un rechazo local oculta la propuesta: %+v %v", list, err)
	}
	if inj, err := usecases.InstinctContext(ctx, rejecting.Repo, g); err != nil || len(inj) != 0 {
		t.Errorf("un rechazo local impide inyectarla: %+v %v", inj, err)
	}

	confirming, closeD := open()
	defer closeD()
	if err := confirming.Confirm(ctx, "cmd:go-test"); err != nil {
		t.Fatalf("confirmar una propuesta global: %v", err)
	}
	if list, err := confirming.List(ctx, true); err != nil || len(list) != 1 || list[0].Status != "confirmed" || list[0].Scope != "global" {
		t.Errorf("confirmada en el proyecto: %+v %v", list, err)
	}
	if err := confirming.Reject(ctx, "nada"); err == nil {
		t.Error("un slug desconocido no existe")
	}
}

// A-001: con un lector activo el checkpoint no puede vaciar el WAL; el
// saneamiento global lo informa como pendiente recuperable y, al liberarse el
// lector, repetirlo retira el texto antiguo aunque ya no queden filas que
// cambiar.
func TestInstinctGlobalSanitizeBusyCheckpointIsPending(t *testing.T) {
	home := t.TempDir()
	ctx := context.Background()
	g, err := OpenInstinctGlobal(home)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	canary := "ghp_" + strings.Repeat("c", 36)
	for _, p := range []string{"a", "b"} {
		if err := g.RecordProject(ctx, p, domain.Instinct{Slug: "cmd:go-test", Kind: "cmd", Confidence: .8, Description: "safe"}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := g.db.Exec(`UPDATE instinct_global SET description=?`, canary); err != nil {
		t.Fatal(err)
	}
	reader, err := sql.Open("sqlite", filepath.Join(home, "instincts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	rtx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := rtx.QueryRow(`SELECT COUNT(*) FROM instinct_global`).Scan(&n); err != nil {
		t.Fatal(err)
	}

	changed, err := g.Sanitize(ctx, domain.ProtectText)
	if changed != 1 || !isSanitizeBusy(err) {
		t.Fatalf("con lector activo: changed=%d err=%v; se esperaba pendiente (busy)", changed, err)
	}
	_ = rtx.Rollback()

	changed, err = g.Sanitize(ctx, domain.ProtectText)
	if err != nil || changed != 0 {
		t.Fatalf("tras liberar el lector: changed=%d err=%v", changed, err)
	}
	for _, f := range []string{"instincts.db", "instincts.db-wal"} {
		if data, err := os.ReadFile(filepath.Join(home, f)); err == nil && strings.Contains(string(data), canary) {
			t.Errorf("%s conserva el canario", f)
		}
	}
}

func isSanitizeBusy(err error) bool {
	var se *domain.SanitizeError
	return errors.As(err, &se) && se.Code == domain.SanitizeCodeBusy
}

// B-001 (2.ª ronda del ACR de 040): una credencial en un comando observado no
// llega a la clave ni al slug; los registros antiguos se sanean en los tres
// almacenes fusionando las colisiones, y listado y contexto quedan limpios.
func TestInstinctKeysNeverKeepSecrets(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", home)
	ctx := context.Background()
	now := instinctClock{}.Now()
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	project := ProjectKey(root)
	repo := NewInstinctRepository(db, project)
	g, err := OpenInstinctGlobal(home)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	q := expandstate.NewQueue(filepath.Join(root, MemDir))
	secret := "sk-" + strings.Repeat("z", 24)

	// Observación nueva: la clave nace protegida.
	uc := &usecases.Instincts{Queue: q, Repo: repo, Global: g, Project: project, Now: func() time.Time { return now }, NewID: func() string { return "new" }}
	if err := uc.Observe(ctx, domain.ExpandSignals{Event: domain.ExpandPostTool, Tool: domain.ToolShell, Command: "curl " + secret}); err != nil {
		t.Fatal(err)
	}
	if batch, _ := q.ReadBatch(ctx, 0); len(batch) != 1 || batch[0].Key != "cmd:curl" {
		t.Fatalf("clave de la observación nueva: %+v", batch)
	}

	// Registros antiguos contaminados que chocan con los limpios.
	dirty := "cmd:curl-" + secret
	for _, slug := range []string{"cmd:curl", dirty} {
		if _, err := db.Exec(`INSERT INTO instinct_stats(project,slug,kind,description,observations,first_seen,last_seen) VALUES(?,?,?,?,4,?,?)`, project, slug, "cmd", "curl", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	for _, rec := range []struct{ slug, project string }{{"cmd:curl", "a"}, {dirty, "b"}} {
		if err := g.RecordProject(ctx, rec.project, domain.Instinct{Slug: rec.slug, Kind: "cmd", Confidence: .8, Description: "curl"}, now); err != nil {
			t.Fatal(err)
		}
	}
	legacy := `{"id":"old","kind":"cmd","key":"` + dirty + `","sample":"curl"}` + "\n"
	f, err := os.OpenFile(filepath.Join(root, MemDir, "expand-observations.ndjson"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(legacy)
	_ = f.Close()

	store := NewPrivacyMaintenanceStore(db)
	u := &usecases.SanitizeProject{Repo: store, UoW: store, Artifacts: NewPrivacyArtifacts(home, project, filepath.Join(root, MemDir, "context.md")), Clock: instinctClock{}, NewRunID: func() string { return "keys" }, InstinctGlobal: g, InstinctQueue: q}
	if preview, err := u.Preview(ctx, project); err != nil || len(preview.Categories) == 0 {
		t.Fatalf("la vista previa debe mostrar las claves: %+v %v", preview, err)
	}
	if _, err := u.Apply(ctx, project); err != nil {
		t.Fatalf("apply: %v", err)
	}

	var obs int
	if err := db.QueryRow(`SELECT observations FROM instinct_stats WHERE project=? AND slug='cmd:curl'`, project).Scan(&obs); err != nil || obs != 8 {
		t.Errorf("la colisión se fusiona sumando observaciones: %d %v", obs, err)
	}
	list, err := uc.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	var projects []string
	for _, i := range list {
		if strings.Contains(i.Slug, secret) {
			t.Errorf("slug con secreto en el listado: %q", i.Slug)
		}
		if i.Slug == "cmd:curl" {
			projects = i.Projects
		}
	}
	if len(projects) != 2 {
		t.Errorf("la fusión global une los proyectos: %v", projects)
	}
	for _, path := range []string{filepath.Join(root, MemDir, "expand-observations.ndjson"), filepath.Join(home, "instincts.db"), filepath.Join(home, "instincts.db-wal")} {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), secret) {
			t.Errorf("%s conserva el secreto", filepath.Base(path))
		}
	}
	var leaks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM instinct_stats WHERE slug LIKE ?`, "%"+secret+"%").Scan(&leaks); err != nil || leaks != 0 {
		t.Errorf("instinct_stats conserva el secreto: %d %v", leaks, err)
	}
	ctxText, err := usecases.InstinctContext(ctx, repo, g)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range ctxText {
		if strings.Contains(i.Slug+i.Description, secret) {
			t.Errorf("el contexto conserva el secreto: %+v", i)
		}
	}
}

// A-001 (ronda 4): sin pasar por mem sanitize, un slug histórico con
// credencial —local, global u observación pendiente— no llega al listado ni
// al contexto, y sigue pudiendo revisarse por su forma protegida.
func TestInstinctHistoricSlugsProtectedOnRead(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", home)
	ctx := context.Background()
	now := instinctClock{}.Now()
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	project := ProjectKey(root)
	repo := NewInstinctRepository(db, project)
	g, err := OpenInstinctGlobal(home)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	secret := "ghp_" + strings.Repeat("d", 36)
	stamp := now.Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO instinct_stats(project,slug,kind,description,observations,confidence,first_seen,last_seen) VALUES(?,?,?,?,4,.7,?,?)`, project, "cmd:curl-"+secret, "cmd", "curl", stamp, stamp); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a", "b"} {
		if err := g.RecordProject(ctx, p, domain.Instinct{Slug: "cmd:wget-" + secret, Kind: "cmd", Confidence: .8, Description: "wget"}, now); err != nil {
			t.Fatal(err)
		}
	}
	q := expandstate.NewQueue(filepath.Join(root, MemDir))
	legacy := `{"id":"old","kind":"cmd","key":"cmd:git-` + secret + `","sample":"git"}` + "\n"
	if err := os.WriteFile(filepath.Join(root, MemDir, "expand-observations.ndjson"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	uc := &usecases.Instincts{Queue: q, Repo: repo, Global: g, Project: project, Now: func() time.Time { return now }}
	if err := uc.Fold(ctx); err != nil {
		t.Fatal(err)
	}
	var leaks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM instinct_stats WHERE slug LIKE ?`, "%"+secret+"%").Scan(&leaks); err != nil || leaks != 1 {
		t.Fatalf("el plegado no debe crear slugs con secreto (solo queda la fila histórica): %d %v", leaks, err)
	}
	list, err := uc.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	slugs := map[string]bool{}
	for _, i := range list {
		if strings.Contains(i.Slug, secret) {
			t.Errorf("slug con secreto en el listado: %q", i.Slug)
		}
		slugs[i.Slug] = true
	}
	for _, want := range []string{"cmd:curl", "cmd:wget", "cmd:git"} {
		if !slugs[want] {
			t.Errorf("falta %s en el listado: %v", want, slugs)
		}
	}
	ctxList, err := usecases.InstinctContext(ctx, repo, g)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range ctxList {
		if strings.Contains(i.Slug, secret) {
			t.Errorf("slug con secreto en el contexto: %q", i.Slug)
		}
	}
	// La revisión por la forma protegida alcanza a la fila histórica.
	if err := uc.Reject(ctx, "cmd:curl"); err != nil {
		t.Fatalf("rechazar por slug protegido: %v", err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM instinct_stats WHERE project=? AND slug=?`, project, "cmd:curl-"+secret).Scan(&status); err != nil || status != "rejected" {
		t.Fatalf("estado de la fila histórica: %q %v", status, err)
	}
	if err := uc.Reject(ctx, "cmd:wget"); err != nil {
		t.Fatalf("rechazar propuesta global por slug protegido: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM instinct_stats WHERE slug LIKE ?`, "%"+secret+"%").Scan(&leaks); err != nil || leaks != 1 {
		t.Fatalf("adoptar la propuesta global no debe copiar el secreto: %d %v", leaks, err)
	}
}
