package persistence

import (
	"slices"
	"testing"

	"mem/domain"
)

// HCD-032 VS-02: el enlace con la revisión anterior, el delta, los heredados y
// settled_ref sobreviven a la ida y vuelta por la base.
func TestReviewInheritance_RoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := NewReviewRepository(db)
	target, err := domain.NewTarget(domain.TargetCommit, "def456", "digest-new", []string{"domain/rules.go"})
	if err != nil {
		t.Fatal(err)
	}
	inherited := []domain.SettledFinding{
		{ConsensusID: 10, FromReviewID: "acr_prev", Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
			Claim: "resuelto antes", Files: []string{"adapters/x.go", "domain/rules.go"}},
		{ConsensusID: 11, FromReviewID: "acr_prev", Status: domain.ConsensusSuspect, Severity: domain.SeverityMedium,
			Claim: "sospecha", Files: []string{"domain/expand.go"}},
	}
	review := &domain.Review{
		ID: "acr_new", Project: "proj", Target: target, Status: domain.ReviewAwaitingReviewers, FixAuthorized: true,
		SinceReviewID: "acr_prev", DeltaScope: []string{"domain/rules.go"}, DeltaSource: domain.DeltaSourceGit,
		Inherited: inherited,
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetReview("proj", "acr_new")
	if err != nil || got == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	if got.SinceReviewID != "acr_prev" || got.DeltaSource != domain.DeltaSourceGit || !slices.Equal(got.DeltaScope, []string{"domain/rules.go"}) {
		t.Fatalf("enlace y delta deben persistir: %+v", got)
	}
	gotInherited, err := repo.InheritedFindings("proj", "acr_new")
	if err != nil {
		t.Fatal(err)
	}
	if len(gotInherited) != 2 || gotInherited[0].ConsensusID != 10 || !slices.Equal(gotInherited[0].Files, inherited[0].Files) ||
		gotInherited[1].Status != domain.ConsensusSuspect || gotInherited[0].FromReviewID != "acr_prev" {
		t.Fatalf("los heredados deben persistir íntegros: %+v", gotInherited)
	}

	// Una revisión sin enlace se lee como hoy.
	plain, _ := domain.NewTarget(domain.TargetCommit, "aaa", "digest-plain", nil)
	if err := repo.CreateReview(&domain.Review{ID: "acr_plain", Project: "proj", Target: plain, Status: domain.ReviewAwaitingReviewers}); err != nil {
		t.Fatal(err)
	}
	if p, _ := repo.GetReview("proj", "acr_plain"); p.SinceReviewID != "" || len(p.DeltaScope) != 0 || p.DeltaSource != "" {
		t.Fatalf("sin enlace no hay delta: %+v", p)
	}
	if inh, _ := repo.InheritedFindings("proj", "acr_plain"); len(inh) != 0 {
		t.Fatalf("sin enlace no hay heredados: %+v", inh)
	}

	// settled_ref y settled viajan con el hallazgo y cambian la marca de resultados.
	before, err := repo.ReviewerResultsMark("proj", "acr_new", 0)
	if err != nil {
		t.Fatal(err)
	}
	result := &domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{{
		LocalID: "A-1", Location: "domain/expand.go:3", Severity: domain.SeverityMedium, Category: "c", Claim: "sospecha",
		EvidenceClass: domain.EvidenceStaticAnalysis, Evidence: []string{"e"}, Confidence: "high", SettledRef: 11, Settled: true,
	}}}
	if err := repo.UpsertReviewerResult("proj", "acr_new", result); err != nil {
		t.Fatal(err)
	}
	results, err := repo.ListReviewerResults("proj", "acr_new", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(results[0].Findings) != 1 || results[0].Findings[0].SettledRef != 11 || !results[0].Findings[0].Settled {
		t.Fatalf("settled_ref y settled deben persistir: %+v", results)
	}
	after, _ := repo.ReviewerResultsMark("proj", "acr_new", 0)
	if before == after {
		t.Fatal("la marca de resultados debe cambiar con el hallazgo asentado")
	}
}
