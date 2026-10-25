package usecases

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"mem/domain"
)

// TestBuildReviewStatus_LinajeCompleto fija lo que antes armaba cada superficie
// por su cuenta: vigencia con el criterio del veredicto, re-juicios solo de la
// ronda vigente, la última corrección que abordó cada hallazgo y los resultados
// de todas las rondas.
func TestBuildReviewStatus_LinajeCompleto(t *testing.T) {
	const project = "p"
	reviews := newMemoryReviewRepository()
	consensus := newMemoryConsensusRepository()
	review := &domain.Review{ID: "r1", Project: project, Round: 2}
	key := reviewKey(project, review.ID)

	consensus.findings[key] = []domain.ConsensusFinding{
		{ConsensusLocalID: "C-001", Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
			RejudgmentState: domain.ReJudgmentResolved, RejudgmentRound: 2},
		// Re-juzgado en una ronda anterior: cuenta como pendiente, no como UNRESOLVED.
		{ConsensusLocalID: "C-002", Status: domain.ConsensusSuspect, Severity: domain.SeverityLow,
			RejudgmentState: domain.ReJudgmentUnresolved, RejudgmentRound: 1},
	}
	consensus.fixes[key] = []domain.FixDelta{
		{Round: 1, AddressedConsensusIDs: []string{"C-001"}},
		{Round: 2, AddressedConsensusIDs: []string{"C-001"}},
	}
	consensus.rejudgment[rejudgmentKey(project, review.ID, "C-001")] = []domain.ReJudgment{
		{Round: 1, ConsensusLocalID: "C-001", Reviewer: domain.ReviewerB, State: domain.ReJudgmentUnresolved},
		{Round: 2, ConsensusLocalID: "C-001", Reviewer: domain.ReviewerA, State: domain.ReJudgmentResolved},
	}
	reviews.results[key] = []domain.ReviewerResult{
		{Reviewer: domain.ReviewerA, Round: 0}, {Reviewer: domain.ReviewerB, Round: 0},
		{Reviewer: domain.ReviewerA, Round: 2},
	}

	got, err := BuildReviewStatus(reviews, consensus, project, review)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.ResultsByRound) != 3 || len(got.ResultsByRound[0]) != 2 || len(got.ResultsByRound[1]) != 0 || len(got.ResultsByRound[2]) != 1 {
		t.Fatalf("resultados por ronda = %+v", got.ResultsByRound)
	}
	c1, c2 := got.Findings[0], got.Findings[1]
	if c1.Vigente != domain.ReJudgmentResolved || !c1.Addressed || c1.AddressedByRound != 2 {
		t.Errorf("C-001 = vigente %q, abordado %v en ronda %d", c1.Vigente, c1.Addressed, c1.AddressedByRound)
	}
	if len(c1.ByReviewer) != 1 || c1.ByReviewer[domain.ReviewerA] != domain.ReJudgmentResolved {
		t.Errorf("C-001 solo debe traer el re-juicio de la ronda vigente: %v", c1.ByReviewer)
	}
	if c2.Vigente != "" || c2.Addressed {
		t.Errorf("C-002 = vigente %q, abordado %v", c2.Vigente, c2.Addressed)
	}

	want := ReviewCounts{
		ByStatus:     map[domain.ConsensusStatus]int{domain.ConsensusConfirmed: 1, domain.ConsensusSuspect: 1},
		BySeverity:   map[domain.Severity]int{domain.SeverityHigh: 1, domain.SeverityLow: 1},
		ByRejudgment: map[domain.ReJudgmentState]int{domain.ReJudgmentResolved: 1},
		Pending:      1,
	}
	if got.Counts.Pending != want.Pending || !maps.Equal(got.Counts.ByStatus, want.ByStatus) ||
		!maps.Equal(got.Counts.BySeverity, want.BySeverity) || !maps.Equal(got.Counts.ByRejudgment, want.ByRejudgment) {
		t.Errorf("conteos = %+v, se esperaba %+v", got.Counts, want)
	}
}

type fixDeltasFailing struct{ *memoryConsensusRepository }

func (fixDeltasFailing) ListFixDeltas(string, string) ([]domain.FixDelta, error) {
	return nil, errors.New("disco lleno")
}

func TestBuildReviewStatus_ErrorConContexto(t *testing.T) {
	_, err := BuildReviewStatus(newMemoryReviewRepository(), fixDeltasFailing{newMemoryConsensusRepository()}, "p", &domain.Review{ID: "r1"})
	if err == nil || !strings.Contains(err.Error(), "listar correcciones: disco lleno") {
		t.Fatalf("err = %v", err)
	}
}

func TestPromotableReviewFindings_AplicaLaReglaDeDominio(t *testing.T) {
	consensus := newMemoryConsensusRepository()
	findings := []domain.ConsensusFinding{
		{ConsensusLocalID: "C-001", Status: domain.ConsensusConfirmed, RejudgmentState: domain.ReJudgmentResolved, RejudgmentRound: 1},
		{ConsensusLocalID: "C-002", Status: domain.ConsensusSuspect},
	}
	consensus.findings[reviewKey("p", "r1")] = findings
	got, err := PromotableReviewFindings(consensus, "p", "r1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := domain.PromotableFindings(findings, 1); strings.Join(got, ",") != strings.Join(want, ",") || len(got) == 0 {
		t.Fatalf("promovibles = %v, se esperaba %v (no vacío)", got, want)
	}
}
