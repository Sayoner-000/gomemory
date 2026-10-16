package persistence

import (
	"context"
	"strings"
	"testing"

	"mem/domain"
)

// TestSanitize_CubreLastPrompt cierra H-01 (auditoría 2026-10-16):
// sessions.last_prompt se protege al escribir, pero las filas anteriores a esa
// protección quedaban fuera de mem sanitize.
func TestSanitize_CubreLastPrompt(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	if _, err := r.db.Exec(`INSERT INTO sessions(id, project, summary, last_prompt) VALUES ('s1','p','', ?), ('s2','otro','', ?)`,
		"envía <private>SYNTH-TOKEN-0001</private> al cliente", "<private>SYNTH-TOKEN-0002</private>"); err != nil {
		t.Fatal(err)
	}
	got, err := readSanitizeCandidates(ctx, r.db, "p")
	if err != nil {
		t.Fatal(err)
	}
	var found *domain.SanitizeCandidate
	for i := range got {
		if got[i].Field == "last_prompt" {
			found = &got[i]
		}
	}
	if found == nil || found.RecordID != "s1" {
		t.Fatalf("last_prompt no aparece como candidato: %+v", got)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := found.Validate("p"); err != nil {
		t.Fatal(err)
	}
	if changed, err := (&privacyTx{tx: tx, project: "p"}).Apply(ctx, *found, domain.ProtectText(found.Text)); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var mine, other string
	if err = r.db.QueryRow(`SELECT last_prompt FROM sessions WHERE id='s1'`).Scan(&mine); err != nil {
		t.Fatal(err)
	}
	if err = r.db.QueryRow(`SELECT last_prompt FROM sessions WHERE id='s2'`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(mine, "SYNTH-TOKEN-0001") || !strings.Contains(mine, "al cliente") {
		t.Fatalf("no se depuró: %q", mine)
	}
	if !strings.Contains(other, "SYNTH-TOKEN-0002") {
		t.Fatal("tocó otro proyecto")
	}
}

// TestSanitize_CubreTargetScope cierra H-04 (auditoría 2026-10-16): el alcance
// de una revisión es texto libre del llamador y quedaba fuera del ledger.
func TestSanitize_CubreTargetScope(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	if _, err := r.db.Exec(`INSERT INTO reviews(project, review_id, target_type, target_revision, target_digest, target_scope, independence_reason) VALUES ('p','acr_1','implementation','r1','abc',?, 'same-model')`,
		`["pkg/a.go","clave <private>SYNTH-TOKEN-0003</private>"]`); err != nil {
		t.Fatal(err)
	}
	got, err := readSanitizeCandidates(ctx, r.db, "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Field != "target_scope" {
		t.Fatalf("target_scope no aparece como candidato: %+v", got)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if changed, err := (&privacyTx{tx: tx, project: "p"}).Apply(ctx, got[0], domain.ProtectText(got[0].Text)); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var scope string
	if err = r.db.QueryRow(`SELECT target_scope FROM reviews WHERE review_id='acr_1'`).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(scope, "SYNTH-TOKEN-0003") || !strings.Contains(scope, "pkg/a.go") {
		t.Fatalf("no se depuró: %q", scope)
	}
}
