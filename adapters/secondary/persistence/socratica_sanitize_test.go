package persistence

import (
	"context"
	"encoding/json"
	"mem/domain"
	"strings"
	"testing"
)

func TestSanitize_CubreSocratica(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := socraticaTestFlow("flow", "claude")
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	secret := "public <private>hidden value</private>"
	raw, _ := json.Marshal(map[string]any{"nested": []string{secret}})
	if _, err := r.db.Exec(`UPDATE socratica_flows SET original_prompt=?,proposed_prompt=?,approved_prompt=?,questions_json=?,answers_json=?,scrutiny_json=?,last_error=? WHERE id='flow'`, secret, secret, secret, string(raw), string(raw), string(raw), secret); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`INSERT INTO socratica_history(id,flow_id,project,client,mode,original_prompt,proposed_prompt,approved_prompt,scrutiny_json,additions_json,words_before,words_after,completed_at) VALUES('h','flow','p','claude','refine',?,?,?,?,?,0,0,'2026-10-16T00:00:00Z')`, secret, secret, secret, string(raw), string(raw)); err != nil {
		t.Fatal(err)
	}
	got, err := readSanitizeCandidates(ctx, r.db, "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 12 {
		t.Fatalf("candidates=%d", len(got))
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	p := &privacyTx{tx: tx, project: "p"}
	for _, c := range got {
		if err := c.Validate("p"); err != nil {
			t.Fatal(err)
		}
		changed, err := p.Apply(ctx, c, domain.ProtectText(c.Text))
		if err != nil || !changed {
			t.Fatal(c.Category, changed, err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got, err = readSanitizeCandidates(ctx, r.db, "p")
	if err != nil || len(got) != 0 {
		t.Fatalf("remaining=%d error=%v", len(got), err)
	}
	var value string
	if err = r.db.QueryRow(`SELECT questions_json FROM socratica_flows WHERE id='flow'`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(value)) || strings.Contains(value, "hidden value") {
		t.Fatal("invalid or unredacted JSON")
	}
}
