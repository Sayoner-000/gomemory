package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

// seedLedgerHistorico inserta un ledger de revisión guardado antes de la
// redacción al escribir: textos crudos en claim, evidence y verification. Las
// listas se serializan con json.Marshal, como las escribe el adaptador, así
// que los bloques privados quedan como \u003cprivate\u003e. El proyecto
// "otro" tiene el mismo ledger y no debe tocarse.
func seedLedgerHistorico(t *testing.T, db *sql.DB) {
	t.Helper()
	lista, err := json.Marshal([]string{protectedIn, "evidencia pública"})
	if err != nil {
		t.Fatal(err)
	}
	limpia, err := json.Marshal([]string{"evidencia pública"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		project string
		base    int
	}{{"proj", 0}, {"otro", 100}} {
		stmts := []struct {
			q    string
			args []any
		}{
			{`INSERT INTO reviews (id, project, review_id, target_type, target_revision, target_digest) VALUES (?, ?, 'rv1', 'diff', 'r', 'd')`, []any{p.base + 1, p.project}},
			{`INSERT INTO reviewer_results (id, review_id, reviewer, status) VALUES (?, ?, 'A', 'completed')`, []any{p.base + 1, p.base + 1}},
			{`INSERT INTO findings (id, reviewer_result_id, local_id, severity, claim, evidence_class, evidence) VALUES (?, ?, 'F1', 'HIGH', ?, 'deterministic', ?)`, []any{p.base + 1, p.base + 1, protectedIn, string(lista)}},
			{`INSERT INTO findings (id, reviewer_result_id, local_id, severity, claim, evidence_class, evidence) VALUES (?, ?, 'F2', 'LOW', 'hallazgo público', 'deterministic', ?)`, []any{p.base + 2, p.base + 1, string(limpia)}},
			{`INSERT INTO consensus_findings (id, review_id, consensus_local_id, status, severity, claim, source_finding_ids) VALUES (?, ?, 'C1', 'CONFIRMED', 'HIGH', ?, ?)`, []any{p.base + 1, p.base + 1, protectedIn, "[1]"}},
			{`INSERT INTO rejudgments (id, review_id, round, consensus_finding_id, reviewer, state, evidence) VALUES (?, ?, 1, ?, 'A', 'RESOLVED', ?)`, []any{p.base + 1, p.base + 1, p.base + 1, string(lista)}},
			{`INSERT INTO fix_rounds (id, review_id, round, base_target_digest, fixed_target_digest, verification) VALUES (?, ?, 1, 'd', 'd2', ?)`, []any{p.base + 1, p.base + 1, string(lista)}},
		}
		for _, s := range stmts {
			if _, err := db.Exec(s.q, s.args...); err != nil {
				t.Fatalf("seed %q: %v", s.q, err)
			}
		}
	}
}

// TestLedger_LectoresDepuranFilasHistoricas: ninguna lectura del ledger
// reemite el texto crudo de filas anteriores a la redacción al escribir.
func TestLedger_LectoresDepuranFilasHistoricas(t *testing.T) {
	db := openTestDB(t)
	seedLedgerHistorico(t, db)
	repo := NewReviewRepository(db).(*ReviewRepository)

	finding, err := repo.GetFinding("proj", "rv1", 1)
	if err != nil || finding == nil {
		t.Fatalf("GetFinding: %v %v", finding, err)
	}
	assertProtected(t, "GetFinding.claim", finding.Claim)
	assertProtected(t, "GetFinding.evidence", finding.Evidence[0])

	findings, err := repo.ListFindings("proj", "rv1", 0)
	if err != nil || len(findings) != 2 {
		t.Fatalf("ListFindings: %v %v", findings, err)
	}
	assertProtected(t, "ListFindings.claim", findings[0].Claim)

	consensus, err := repo.GetConsensusFinding("proj", "rv1", "C1")
	if err != nil || consensus == nil {
		t.Fatalf("GetConsensusFinding: %v %v", consensus, err)
	}
	assertProtected(t, "GetConsensusFinding.claim", consensus.Claim)

	all, err := repo.ListConsensusFindings("proj", "rv1", 0)
	if err != nil || len(all) != 1 {
		t.Fatalf("ListConsensusFindings: %v %v", all, err)
	}
	assertProtected(t, "ListConsensusFindings.claim", all[0].Claim)

	judgments, err := repo.ListReJudgments("proj", "rv1", "C1")
	if err != nil || len(judgments) != 1 {
		t.Fatalf("ListReJudgments: %v %v", judgments, err)
	}
	assertProtected(t, "ListReJudgments.evidence", judgments[0].Evidence[0])

	deltas, err := repo.ListFixDeltas("proj", "rv1")
	if err != nil || len(deltas) != 1 {
		t.Fatalf("ListFixDeltas: %v %v", deltas, err)
	}
	assertProtected(t, "ListFixDeltas.verification", deltas[0].Verification[0])
}

// TestLedger_SaneamientoCubreElLedger: Candidates enumera las columnas del
// ledger del proyecto (también los bloques escapados dentro del JSON) y Apply
// las deja protegidas con JSON válido, sin tocar filas limpias ni otro proyecto.
func TestLedger_SaneamientoCubreElLedger(t *testing.T) {
	db := openTestDB(t)
	seedLedgerHistorico(t, db)
	store := NewPrivacyMaintenanceStore(db)
	ctx := context.Background()

	cands, err := store.Candidates(ctx, "proj")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	want := []string{
		"review_consensus:1",
		"review_finding:1",
		"review_finding_evidence:1",
		"review_fix_verification:1",
		"review_rejudgment:1",
	}
	if got := candidateIDs(cands); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("candidatos = %v, quiero %v", got, want)
	}

	err = store.WithTransaction(ctx, "proj", func(tx ports.PrivacyTransaction) error {
		for _, c := range cands {
			if err := c.Validate("proj"); err != nil {
				return err
			}
			ok, err := tx.Apply(ctx, c, domain.ProtectText(c.Text))
			if err != nil {
				return err
			}
			if !ok {
				t.Errorf("Apply %s:%s no cambió la fila", c.Category, c.RecordID)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}

	assertProtected(t, "findings.claim", rawColumn(t, db, `SELECT claim FROM findings WHERE id = 1`))
	assertProtected(t, "consensus_findings.claim", rawColumn(t, db, `SELECT claim FROM consensus_findings WHERE id = 1`))
	for _, q := range []string{
		`SELECT evidence FROM findings WHERE id = 1`,
		`SELECT evidence FROM rejudgments WHERE id = 1`,
		`SELECT verification FROM fix_rounds WHERE id = 1`,
	} {
		var items []string
		if err := json.Unmarshal([]byte(rawColumn(t, db, q)), &items); err != nil || len(items) != 2 {
			t.Fatalf("%s: JSON inválido tras sanear: %v %v", q, items, err)
		}
		assertProtected(t, q, items[0])
		if items[1] != "evidencia pública" {
			t.Fatalf("%s: alteró un elemento limpio: %q", q, items[1])
		}
	}
	if got := rawColumn(t, db, `SELECT claim FROM findings WHERE id = 101`); got != protectedIn {
		t.Fatalf("saneó otro proyecto: %q", got)
	}

	left, err := store.Candidates(ctx, "proj")
	if err != nil {
		t.Fatalf("Candidates tras sanear: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("quedan candidatos tras sanear: %v", candidateIDs(left))
	}
}
