package usecases

import (
	"testing"

	"mem/domain"
)

func TestBuildConsensusValidatesIndependentSourcesAndEvidence(t *testing.T) {
	reviews := newMemoryReviewRepository()
	ledger := newMemoryConsensusRepository()
	target, _ := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:frozen", nil)
	review := &domain.Review{ID: "acr_consensus", Project: "proj", Target: target, Status: domain.ReviewConsensusReady}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{
		{LocalID: "A-001", Severity: domain.SeverityHigh, Claim: "race", EvidenceClass: domain.EvidenceReproduced, Evidence: []string{"go test -race falla"}},
		{LocalID: "A-002", Severity: domain.SeverityMedium, Claim: "único", EvidenceClass: domain.EvidenceStaticAnalysis, Evidence: []string{"ruta alcanzable"}},
		{LocalID: "A-003", Severity: domain.SeverityHigh, Claim: "sin evidencia", EvidenceClass: domain.EvidenceDeterministic},
	}}
	b := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{
		{LocalID: "B-001", Severity: domain.SeverityHigh, Claim: "race equivalente", EvidenceClass: domain.EvidenceTestFailure, Evidence: []string{"mismo fallo"}},
		{LocalID: "B-003", Severity: domain.SeverityHigh, Claim: "pareja", EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"traza"}},
	}}
	if err := reviews.UpsertReviewerResult("proj", review.ID, &a); err != nil {
		t.Fatal(err)
	}
	if err := reviews.UpsertReviewerResult("proj", review.ID, &b); err != nil {
		t.Fatal(err)
	}

	// La clasificación cubre los CINCO hallazgos. Hasta la funcionalidad 028 este
	// test clasificaba solo dos y pasaba: la cobertura parcial no era un error, era
	// simplemente no mencionar los otros tres. Ese silencio es el defecto que
	// permitía aprobar una revisión con un HIGH oculto.
	findings, err := BuildConsensus(reviews, ledger, BuildConsensusInput{
		Project: "proj", ReviewID: review.ID,
		Matches: []ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: b.Findings[0].ID,
			Severity: domain.SeverityHigh, Claim: "race",
		}},
		Unmatched: []ConsensusUnmatched{
			{Status: domain.ConsensusSuspect, FindingID: a.Findings[1].ID},
			{Status: domain.ConsensusSuspect, FindingID: a.Findings[2].ID},
			{Status: domain.ConsensusInfo, FindingID: b.Findings[1].ID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 4 {
		t.Fatalf("se esperaban 4 clasificaciones que cubran los 5 hallazgos: %#v", findings)
	}
	if findings[0].Status != domain.ConsensusConfirmed || len(findings[0].SourceFindingIDs) != 2 {
		t.Fatalf("consenso confirmado inválido: %#v", findings[0])
	}
	if findings[1].Status != domain.ConsensusSuspect || len(findings[1].SourceFindingIDs) != 1 {
		t.Fatalf("hallazgo único no quedó SUSPECT: %#v", findings[1])
	}

	// Omitir un solo hallazgo basta para rechazar la clasificación entera (FR-001).
	_, err = BuildConsensus(reviews, newMemoryConsensusRepository(), BuildConsensusInput{
		Project: "proj", ReviewID: review.ID,
		Matches: []ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: b.Findings[0].ID,
			Claim: "race",
		}},
		Unmatched: []ConsensusUnmatched{
			{Status: domain.ConsensusSuspect, FindingID: a.Findings[1].ID},
			{Status: domain.ConsensusSuspect, FindingID: a.Findings[2].ID},
		},
	})
	if err == nil {
		t.Fatal("dejar un hallazgo sin clasificar debe rechazarse")
	}

	_, err = BuildConsensus(reviews, newMemoryConsensusRepository(), BuildConsensusInput{
		Project: "proj", ReviewID: review.ID,
		Matches: []ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: a.Findings[1].ID,
			Severity: domain.SeverityHigh,
		}},
	})
	if err == nil {
		t.Fatal("dos fuentes del mismo revisor no pueden confirmar un hallazgo")
	}

	_, err = BuildConsensus(reviews, newMemoryConsensusRepository(), BuildConsensusInput{
		Project: "proj", ReviewID: review.ID,
		Matches: []ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[2].ID, FindingIDB: b.Findings[1].ID,
			Severity: domain.SeverityHigh,
		}},
	})
	if err == nil {
		t.Fatal("un hallazgo sin evidencia no puede quedar CONFIRMED")
	}
}

// escenarioConsensuable deja una revisión lista para clasificar, con dos hallazgos
// corroborables por revisor distinto y uno suelto.
func escenarioConsensuable(t *testing.T) (*memoryReviewRepository, *memoryConsensusRepository, []int64) {
	t.Helper()
	reviews := newMemoryReviewRepository()
	ledger := newMemoryConsensusRepository().enlazar(reviews)
	target, _ := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:frozen", nil)
	review := &domain.Review{
		ID: "acr_idem", Project: "proj", Target: target, CurrentTargetDigest: "sha256:frozen",
		MaxFixRounds: 2, FixAuthorized: true, Status: domain.ReviewConsensusReady,
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{
		{LocalID: "A-001", Severity: domain.SeverityHigh, Claim: "race", EvidenceClass: domain.EvidenceReproduced, Evidence: []string{"go test -race falla"}},
		{LocalID: "A-002", Severity: domain.SeverityMedium, Claim: "único", EvidenceClass: domain.EvidenceStaticAnalysis, Evidence: []string{"ruta alcanzable"}},
	}}
	b := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{
		{LocalID: "B-001", Severity: domain.SeverityHigh, Claim: "race equivalente", EvidenceClass: domain.EvidenceTestFailure, Evidence: []string{"mismo fallo"}},
	}}
	if err := reviews.UpsertReviewerResult("proj", review.ID, &a); err != nil {
		t.Fatal(err)
	}
	if err := reviews.UpsertReviewerResult("proj", review.ID, &b); err != nil {
		t.Fatal(err)
	}
	return reviews, ledger, []int64{a.Findings[0].ID, a.Findings[1].ID, b.Findings[0].ID}
}

func clasificacionDe(ids []int64, estadoSuelto domain.ConsensusStatus) BuildConsensusInput {
	return BuildConsensusInput{
		Project: "proj", ReviewID: "acr_idem",
		Matches: []ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: ids[0], FindingIDB: ids[2], Claim: "race",
		}},
		Unmatched: []ConsensusUnmatched{{Status: estadoSuelto, FindingID: ids[1]}},
	}
}

// TestBuildConsensus_ReenvioExactoEsIdempotente cubre FR-005: antes el UPSERT
// sobrescribía sin preguntar y los consensus_local_id se asignaban por orden de
// llegada, así que reenviar la misma clasificación en otro orden reasignaba
// identificadores y rompía las referencias que ya tenían correcciones y re-juicios.
func TestBuildConsensus_ReenvioExactoEsIdempotente(t *testing.T) {
	reviews, ledger, ids := escenarioConsensuable(t)

	primera, err := BuildConsensusWithOutcome(reviews, ledger, clasificacionDe(ids, domain.ConsensusSuspect))
	if err != nil {
		t.Fatal(err)
	}
	if primera.Idempotent {
		t.Error("la primera clasificación no puede reportarse como idempotente")
	}

	segunda, err := BuildConsensusWithOutcome(reviews, ledger, clasificacionDe(ids, domain.ConsensusSuspect))
	if err != nil {
		t.Fatalf("el reenvío exacto debe aceptarse: %v", err)
	}
	if !segunda.Idempotent {
		t.Error("el reenvío exacto debe reportarse como idempotente")
	}
	if len(primera.Findings) != len(segunda.Findings) {
		t.Fatalf("el reenvío cambió el número de clasificaciones: %d vs %d",
			len(primera.Findings), len(segunda.Findings))
	}
	for i := range primera.Findings {
		if primera.Findings[i].ConsensusLocalID != segunda.Findings[i].ConsensusLocalID {
			t.Errorf("el identificador %s cambió a %s en el reenvío",
				primera.Findings[i].ConsensusLocalID, segunda.Findings[i].ConsensusLocalID)
		}
	}
}

func TestBuildConsensus_RechazaReemplazoDivergente(t *testing.T) {
	reviews, ledger, ids := escenarioConsensuable(t)
	if _, err := BuildConsensus(reviews, ledger, clasificacionDe(ids, domain.ConsensusSuspect)); err != nil {
		t.Fatal(err)
	}

	_, err := BuildConsensus(reviews, ledger, clasificacionDe(ids, domain.ConsensusInfo))
	if err == nil {
		t.Fatal("reclasificar la misma ronda debe rechazarse")
	}
	if !contieneTexto(err.Error(), "no admite reemplazo") {
		t.Errorf("el error debe explicar que la ronda ya está registrada: %v", err)
	}

	persistidos, err := ledger.ListAllConsensusFindings("proj", "acr_idem")
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range persistidos {
		if finding.Status == domain.ConsensusInfo {
			t.Error("el intento de reemplazo alteró el ledger")
		}
	}
}

// TestBuildConsensus_DerivaSeveridadYRechazaDegradacion cubre FR-003 en el borde del
// caso de uso, no solo en el dominio.
func TestBuildConsensus_DerivaSeveridadYRechazaDegradacion(t *testing.T) {
	reviews, ledger, ids := escenarioConsensuable(t)

	degradada := clasificacionDe(ids, domain.ConsensusSuspect)
	degradada.Matches[0].Severity = domain.SeverityLow
	if _, err := BuildConsensus(reviews, ledger, degradada); err == nil {
		t.Fatal("declarar una severidad menor que la derivada debe rechazarse")
	}

	findings, err := BuildConsensus(reviews, ledger, clasificacionDe(ids, domain.ConsensusSuspect))
	if err != nil {
		t.Fatal(err)
	}
	if findings[0].Severity != domain.SeverityHigh {
		t.Errorf("severidad persistida = %s, se esperaba la derivada HIGH", findings[0].Severity)
	}
}

func TestBuildConsensus_RechazaRevisionTerminal(t *testing.T) {
	reviews, ledger, ids := escenarioConsensuable(t)
	review, _ := reviews.GetReview("proj", "acr_idem")
	review.Status = domain.ReviewApproved
	if err := reviews.UpdateReview(review); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildConsensus(reviews, ledger, clasificacionDe(ids, domain.ConsensusSuspect)); err == nil {
		t.Fatal("una revisión aprobada no admite consenso nuevo")
	}
}

func contieneTexto(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
