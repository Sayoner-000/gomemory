package domain

import (
	"slices"
	"testing"
)

// HCD-032 VS-01: herencia de hallazgos entre revisiones ACR.

func TestInheritableFindings(t *testing.T) {
	prev := Review{ID: "acr_prev", Round: 2}
	sources := map[int64]Finding{
		1: {ID: 1, Location: "domain/rules.go:42"},
		2: {ID: 2, Location: "adapters/x.go:10-20"},
		3: {ID: 3, Location: "domain/expand.go"},
		4: {ID: 4, Location: "a.go:1"},
		5: {ID: 5, Location: "b.go:1"},
		6: {ID: 6, Location: "c.go:1"},
	}
	consensus := []ConsensusFinding{
		{ID: 10, Status: ConsensusConfirmed, Severity: SeverityHigh, Claim: "resuelto", SourceFindingIDs: []int64{1, 2},
			RejudgmentState: ReJudgmentResolved, RejudgmentRound: 2},
		{ID: 11, Status: ConsensusSuspect, Severity: SeverityMedium, Claim: "sospecha", SourceFindingIDs: []int64{3}},
		{ID: 12, Status: ConsensusInfo, Severity: SeverityInfo, Claim: "nota", SourceFindingIDs: []int64{4}},
		// No heredables: severo sin resolver, resuelto en una ronda que ya no es la vigente,
		// y una contradicción.
		{ID: 13, Status: ConsensusConfirmed, Severity: SeverityCritical, Claim: "abierto", SourceFindingIDs: []int64{5}},
		{ID: 14, Status: ConsensusConfirmed, Severity: SeverityHigh, Claim: "resuelto antes", SourceFindingIDs: []int64{6},
			RejudgmentState: ReJudgmentResolved, RejudgmentRound: 1},
		{ID: 15, Status: ConsensusContradiction, Severity: SeverityLow, Claim: "discrepan", SourceFindingIDs: []int64{6}},
	}
	got := InheritableFindings(prev, consensus, sources)
	var ids []int64
	for _, s := range got {
		ids = append(ids, s.ConsensusID)
		if s.FromReviewID != "acr_prev" {
			t.Errorf("%d: debe recordar de qué revisión viene: %+v", s.ConsensusID, s)
		}
	}
	if !slices.Equal(ids, []int64{10, 11, 12}) {
		t.Fatalf("solo se heredan resueltos vigentes, SUSPECT e INFO: %v", ids)
	}
	if !slices.Equal(got[0].Files, []string{"adapters/x.go", "domain/rules.go"}) {
		t.Fatalf("los archivos salen de la ubicación de sus fuentes, sin línea y ordenados: %v", got[0].Files)
	}
}

func TestSettledStillValid(t *testing.T) {
	s := SettledFinding{ConsensusID: 10, Files: []string{"domain/rules.go", "adapters/x.go"}}
	if !SettledStillValid(s, []string{"domain/expand.go"}) {
		t.Error("si ninguno de sus archivos cambió, el asentado sigue valiendo")
	}
	if SettledStillValid(s, []string{"./domain/rules.go"}) {
		t.Error("si uno de sus archivos cambió, el asentado deja de valer")
	}
	if !SettledStillValid(s, nil) {
		t.Error("sin delta (nada cambió) el asentado vale")
	}
	if SettledStillValid(SettledFinding{ConsensusID: 11}, nil) {
		t.Error("un asentado sin archivos no se puede comprobar frente al delta: no vale")
	}
}

func TestDeriveVerdict_Settled(t *testing.T) {
	review := Review{ID: "acr_new", Round: 0, FixAuthorized: true}
	results := func(fA Finding) []ReviewerResult {
		return []ReviewerResult{
			{Reviewer: ReviewerA, Status: ReviewerResultSuccess, Findings: []Finding{fA}},
			{Reviewer: ReviewerB, Status: ReviewerResultSuccess},
		}
	}
	settled := Finding{ID: 1, Location: "domain/rules.go:1", Severity: SeverityHigh, SettledRef: 10, Settled: true}
	if v := DeriveVerdict(review, results(settled), nil, nil); v != VerdictApproved {
		t.Fatalf("un hallazgo con asentamiento vigente no exige consenso: %q", v)
	}
	reopened := settled
	reopened.Settled = false // su archivo cambió: se reabre
	if v := DeriveVerdict(review, results(reopened), nil, nil); v != "" {
		t.Fatalf("un hallazgo reabierto exige clasificarse como cualquier otro: %q", v)
	}
	// Las garantías actuales no cambian.
	failed := []ReviewerResult{{Reviewer: ReviewerA, Status: ReviewerResultSuccess, Findings: []Finding{settled}}, {Reviewer: ReviewerB, Status: ReviewerResultFailure}}
	if v := DeriveVerdict(review, failed, nil, nil); v != VerdictIncomplete {
		t.Fatalf("un revisor fallido sigue dando INCOMPLETE: %q", v)
	}
	if v := DeriveVerdict(review, results(settled)[:1], nil, nil); v != VerdictIncomplete {
		t.Fatalf("un revisor ausente sigue dando INCOMPLETE: %q", v)
	}
	open := Finding{ID: 2, Location: "x.go:1", Severity: SeverityHigh}
	both := results(settled)
	both[1].Findings = []Finding{open}
	consensus := []ConsensusFinding{{ID: 20, Status: ConsensusConfirmed, Severity: SeverityHigh, SourceFindingIDs: []int64{2}}}
	if v := DeriveVerdict(review, both, consensus, nil); v != "" {
		t.Fatalf("un HIGH confirmado sin resolver sigue impidiendo aprobar: %q", v)
	}
}
