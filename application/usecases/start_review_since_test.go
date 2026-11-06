package usecases

import (
	"errors"
	"slices"
	"testing"

	"mem/domain"
)

// HCD-032 VS-02: StartReviewSince hereda de una revisión anterior terminada y
// fija el delta.

type fakeRevisionDiff struct {
	files    []string
	err      error
	from, to string
}

func (f *fakeRevisionDiff) ChangedFiles(from, to string) ([]string, error) {
	f.from, f.to = from, to
	return f.files, f.err
}

// revisionAnteriorTerminada deja una revisión aprobada con un CONFIRMED HIGH
// resuelto (heredable) y un CONFIRMED HIGH sin resolver (no heredable).
func revisionAnteriorTerminada(t *testing.T, status domain.ReviewStatus) (*memoryReviewRepository, *memoryConsensusRepository) {
	t.Helper()
	reviews := newMemoryReviewRepository()
	ledger := newMemoryConsensusRepository().enlazar(reviews)
	target, _ := domain.NewTarget(domain.TargetCommit, "abc123", "sha256:prev", nil)
	prev := &domain.Review{ID: "acr_prev", Project: "proj", Target: target, Round: 1, Status: status, FixAuthorized: true}
	if err := reviews.CreateReview(prev); err != nil {
		t.Fatal(err)
	}
	a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{
		{LocalID: "A-1", Location: "domain/rules.go:42", Severity: domain.SeverityHigh, Claim: "resuelto"},
		{LocalID: "A-2", Location: "adapters/x.go:7", Severity: domain.SeverityHigh, Claim: "abierto"},
	}}
	if err := reviews.UpsertReviewerResult("proj", "acr_prev", &a); err != nil {
		t.Fatal(err)
	}
	for _, c := range []domain.ConsensusFinding{
		{ConsensusLocalID: "C-1", Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh, Claim: "resuelto",
			SourceFindingIDs: []int64{a.Findings[0].ID}, RejudgmentState: domain.ReJudgmentResolved, RejudgmentRound: 1},
		{ConsensusLocalID: "C-2", Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh, Claim: "abierto",
			SourceFindingIDs: []int64{a.Findings[1].ID}},
	} {
		c := c
		if err := ledger.UpsertConsensusFinding("proj", "acr_prev", &c); err != nil {
			t.Fatal(err)
		}
	}
	return reviews, ledger
}

func TestStartReview_Since(t *testing.T) {
	reviews, ledger := revisionAnteriorTerminada(t, domain.ReviewApproved)
	diff := &fakeRevisionDiff{files: []string{"adapters/x.go"}}
	in := StartReviewInput{Project: "proj", TargetType: domain.TargetCommit, Revision: "def456", Digest: "sha256:new",
		Scope: []string{"todo"}, SinceReviewID: "acr_prev"}

	review, err := StartReviewSince(reviews, ledger, diff, in)
	if err != nil {
		t.Fatal(err)
	}
	if review.SinceReviewID != "acr_prev" || diff.from != "abc123" || diff.to != "def456" {
		t.Fatalf("el delta se calcula entre los commits de ambas revisiones: %+v (from=%s to=%s)", review, diff.from, diff.to)
	}
	if review.DeltaSource != domain.DeltaSourceGit || !slices.Equal(review.DeltaScope, []string{"adapters/x.go"}) {
		t.Fatalf("delta de git: %+v", review)
	}
	inherited, _ := reviews.InheritedFindings("proj", review.ID)
	if len(inherited) != 1 || inherited[0].Claim != "resuelto" || !slices.Equal(inherited[0].Files, []string{"domain/rules.go"}) {
		t.Fatalf("solo se hereda el resuelto, con sus archivos: %+v", inherited)
	}

	// Sin git (o sin commit) se usa el scope del llamador y se declara.
	review2, err := StartReviewSince(reviews, ledger, &fakeRevisionDiff{err: errors.New("no es un commit")}, in)
	if err != nil {
		t.Fatal(err)
	}
	if review2.DeltaSource != domain.DeltaSourceScope || !slices.Equal(review2.DeltaScope, []string{"todo"}) {
		t.Fatalf("sin git el delta es el scope declarado: %+v", review2)
	}

	// La anterior debe existir, ser del proyecto y estar terminada.
	for _, bad := range []StartReviewInput{
		{Project: "proj", TargetType: domain.TargetCommit, Revision: "x", Digest: "d", SinceReviewID: "acr_no_existe"},
		{Project: "otro", TargetType: domain.TargetCommit, Revision: "x", Digest: "d", SinceReviewID: "acr_prev"},
	} {
		if _, err := StartReviewSince(reviews, ledger, diff, bad); err == nil {
			t.Errorf("%+v: debe rechazarse", bad)
		}
	}
	abierta, l2 := revisionAnteriorTerminada(t, domain.ReviewConsensusReady)
	if _, err := StartReviewSince(abierta, l2, diff, in); err == nil {
		t.Error("una revisión anterior aún abierta no puede heredarse")
	}

	// StartReview sin las dependencias no puede ignorar --since.
	if _, err := StartReview(reviews, in); err == nil {
		t.Error("StartReview con SinceReviewID debe exigir StartReviewSince")
	}
	// Y sin --since, todo como antes.
	plain := in
	plain.SinceReviewID = ""
	if r, err := StartReview(reviews, plain); err != nil || r.SinceReviewID != "" || len(r.DeltaScope) != 0 {
		t.Fatalf("sin enlace, comportamiento actual: %+v %v", r, err)
	}
}
