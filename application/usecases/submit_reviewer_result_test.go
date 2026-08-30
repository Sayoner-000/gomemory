package usecases

import (
	"strings"
	"testing"

	"mem/domain"
)

func TestSubmitReviewerResultValidatesDigestIdempotencyAndFailure(t *testing.T) {
	repo := newMemoryReviewRepository()
	target, err := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:frozen", nil)
	if err != nil {
		t.Fatal(err)
	}
	review := &domain.Review{
		ID:      "acr_submit",
		Project: "proj",
		Target:  target,
		Round:   0,
		Status:  domain.ReviewAwaitingReviewers,
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatal(err)
	}

	result := domain.ReviewerResult{
		Reviewer: domain.ReviewerA,
		Status:   domain.ReviewerResultSuccess,
		Findings: []domain.Finding{{
			LocalID:       "A-001",
			Location:      "domain/verdict.go:42",
			Severity:      domain.SeverityHigh,
			Category:      "correctness",
			Claim:         "defecto",
			EvidenceClass: domain.EvidenceDeterministic,
			Evidence:      []string{"evidencia"},
			Confidence:    "high",
		}},
	}
	_, err = SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: review.ID, TargetDigest: "sha256:changed", Result: result,
	})
	if err == nil || !strings.Contains(err.Error(), "target changed") {
		t.Fatalf("digest distinto debe devolver target changed, got %v", err)
	}
	if len(repo.results[reviewKey("proj", review.ID)]) != 0 {
		t.Fatal("se persistió un resultado contra el target equivocado")
	}

	input := SubmitReviewerResultInput{
		Project: "proj", ReviewID: review.ID, TargetDigest: "sha256:frozen", Result: result,
	}
	first, err := SubmitReviewerResult(repo, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.FindingIDs["A-001"] == 0 {
		t.Fatal("el envío debe devolver los IDs persistidos para construir el consenso")
	}
	input.Result.Findings[0].Claim = "defecto actualizado"
	if _, err := SubmitReviewerResult(repo, input); err != nil {
		t.Fatal(err)
	}
	storedResults, _ := repo.ListReviewerResults("proj", review.ID, 0)
	if len(storedResults) != 1 || len(storedResults[0].Findings) != 1 {
		t.Fatalf("reenvío no idempotente: %#v", storedResults)
	}

	if _, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project:      "proj",
		ReviewID:     review.ID,
		TargetDigest: "sha256:frozen",
		Result: domain.ReviewerResult{
			Reviewer: domain.ReviewerB,
			Status:   domain.ReviewerResultFailure,
		},
	}); err != nil {
		t.Fatal(err)
	}
	storedReview, _ := repo.GetReview("proj", review.ID)
	if storedReview.Status != domain.ReviewIncomplete {
		t.Fatalf("un fallo de revisor debe bloquear APPROVED: status=%s", storedReview.Status)
	}
}

func hallazgoCompleto(localID string) domain.Finding {
	return domain.Finding{
		LocalID: localID, Location: "domain/verdict.go:42", Severity: domain.SeverityHigh,
		Category: "correctness", Claim: "defecto", EvidenceClass: domain.EvidenceDeterministic,
		Evidence: []string{"traza"}, Confidence: "high",
	}
}

func revisionConIdentidades(t *testing.T) *memoryReviewRepository {
	t.Helper()
	repo := newMemoryReviewRepository()
	target, _ := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:frozen", nil)
	review := &domain.Review{
		ID: "acr_ident", Project: "proj", Target: target, CurrentTargetDigest: "sha256:frozen",
		MaxFixRounds: 2, FixAuthorized: true, Status: domain.ReviewAwaitingReviewers,
		ReviewerA: domain.ReviewerIdentity{Provider: "anthropic", Model: "claude-opus-5"},
		ReviewerB: domain.ReviewerIdentity{Provider: "openai", Model: "gpt-5"},
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	return repo
}

// TestSubmitReviewerResult_ExigeLaIdentidadEsperada cubre FR-006: sin esta
// comprobación, la independencia que la revisión afirma tener no es verificable.
func TestSubmitReviewerResult_ExigeLaIdentidadEsperada(t *testing.T) {
	repo := revisionConIdentidades(t)
	suplantado := domain.ReviewerResult{
		Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
		Provider: "openai", Model: "gpt-5",
		Findings: []domain.Finding{hallazgoCompleto("A-001")},
	}
	_, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: "acr_ident", TargetDigest: "sha256:frozen", Result: suplantado,
	})
	if err == nil {
		t.Fatal("un resultado que declara otro proveedor debe rechazarse")
	}
	if len(repo.results[reviewKey("proj", "acr_ident")]) != 0 {
		t.Fatal("se persistió un resultado de identidad equivocada")
	}

	correcto := suplantado
	correcto.Provider, correcto.Model = "anthropic", "claude-opus-5"
	if _, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: "acr_ident", TargetDigest: "sha256:frozen", Result: correcto,
	}); err != nil {
		t.Fatalf("el revisor asignado fue rechazado: %v", err)
	}
}

// TestSubmitReviewerResult_ExigeCamposObligatorios cubre FR-007. La validación va en
// el borde del sistema: antes estos campos solo se miraban al confirmar, así que un
// hallazgo incompleto entraba al ledger y solo estorbaba mucho más tarde.
func TestSubmitReviewerResult_ExigeCamposObligatorios(t *testing.T) {
	campos := map[string]func(*domain.Finding){
		"local_id":       func(f *domain.Finding) { f.LocalID = "" },
		"location":       func(f *domain.Finding) { f.Location = "" },
		"severity":       func(f *domain.Finding) { f.Severity = "" },
		"category":       func(f *domain.Finding) { f.Category = "" },
		"claim":          func(f *domain.Finding) { f.Claim = "" },
		"evidence_class": func(f *domain.Finding) { f.EvidenceClass = "" },
		"confidence":     func(f *domain.Finding) { f.Confidence = "" },
		"evidence":       func(f *domain.Finding) { f.Evidence = []string{"   "} },
	}
	for campo, mutilar := range campos {
		t.Run(campo, func(t *testing.T) {
			repo := revisionConIdentidades(t)
			finding := hallazgoCompleto("A-001")
			mutilar(&finding)
			_, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
				Project: "proj", ReviewID: "acr_ident", TargetDigest: "sha256:frozen",
				Result: domain.ReviewerResult{
					Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
					Provider: "anthropic", Model: "claude-opus-5",
					Findings: []domain.Finding{finding},
				},
			})
			if err == nil {
				t.Fatalf("un hallazgo sin %s debe rechazarse", campo)
			}
			if !strings.Contains(err.Error(), campo) {
				t.Errorf("el error debe nombrar el campo %s: %v", campo, err)
			}
			if len(repo.results[reviewKey("proj", "acr_ident")]) != 0 {
				t.Error("se persistió un resultado con un hallazgo incompleto")
			}
		})
	}
}

func TestSubmitReviewerResult_RechazaRevisionTerminal(t *testing.T) {
	repo := revisionConIdentidades(t)
	review, _ := repo.GetReview("proj", "acr_ident")
	review.Status = domain.ReviewEscalated
	if err := repo.UpdateReview(review); err != nil {
		t.Fatal(err)
	}
	_, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: "acr_ident", TargetDigest: "sha256:frozen",
		Result: domain.ReviewerResult{
			Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
			Provider: "anthropic", Model: "claude-opus-5",
			Findings: []domain.Finding{hallazgoCompleto("A-001")},
		},
	})
	if err == nil {
		t.Fatal("una revisión escalada no admite resultados nuevos")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("el error debe declarar el estado terminal: %v", err)
	}
}

// TestSubmitReviewerResult_ValidaContraElTargetVigente cubre FR-011: tras una
// corrección, los revisores inspeccionan la revisión corregida. Validar contra el
// digest congelado al inicio rechazaría precisamente el resultado correcto.
func TestSubmitReviewerResult_ValidaContraElTargetVigente(t *testing.T) {
	repo := revisionConIdentidades(t)
	review, _ := repo.GetReview("proj", "acr_ident")
	review.CurrentTargetDigest = "sha256:corregido"
	review.Status = domain.ReviewRejudging
	review.Round = 1
	if err := repo.UpdateReview(review); err != nil {
		t.Fatal(err)
	}

	// Sin hallazgos: una ronda posterior a la corrección es de VERIFICACIÓN, y
	// declarar hallazgos nuevos ahí se rechaza por separado. Lo que este test mide
	// es contra qué digest se valida, no qué se puede reportar.
	resultado := domain.ReviewerResult{
		Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
		Provider: "anthropic", Model: "claude-opus-5",
	}
	if _, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: "acr_ident", TargetDigest: "sha256:frozen", Result: resultado,
	}); err == nil {
		t.Fatal("el digest original ya no es el vigente tras una corrección")
	}
	if _, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: "acr_ident", TargetDigest: "sha256:corregido", Result: resultado,
	}); err != nil {
		t.Fatalf("el target corregido debe aceptarse: %v", err)
	}
}

// TestSubmitReviewerResult_RondaDeRevalidacionNoAdmiteHallazgos es la regresión del
// segundo defecto encontrado revisando la funcionalidad 028.
//
// Un hallazgo enviado tras una corrección quedaba huérfano: ninguna clasificación de
// consenso lo cubría, DeriveVerdict lo veía sin clasificar y devolvía «aún no».
// Resultado: la revisión no podía alcanzar NINGÚN estado terminal, ni aprobado ni
// escalado. El mismo bloqueo irrecuperable que la funcionalidad 028 existe para
// cerrar, entrando por otra puerta.
func TestSubmitReviewerResult_RondaDeRevalidacionNoAdmiteHallazgos(t *testing.T) {
	repo := revisionConIdentidades(t)
	review, _ := repo.GetReview("proj", "acr_ident")
	review.Round = 1
	review.Status = domain.ReviewRejudging
	review.CurrentTargetDigest = "sha256:corregido"
	if err := repo.UpdateReview(review); err != nil {
		t.Fatal(err)
	}

	conHallazgos := domain.ReviewerResult{
		Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
		Provider: "anthropic", Model: "claude-opus-5",
		Findings: []domain.Finding{hallazgoCompleto("A-900")},
	}
	_, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: "acr_ident", TargetDigest: "sha256:corregido",
		Result: conHallazgos,
	})
	if err == nil {
		t.Fatal("una ronda de revalidación no puede admitir hallazgos nuevos")
	}
	if !strings.Contains(err.Error(), "review_rejudge") {
		t.Errorf("el error debe encaminar al canal correcto: %v", err)
	}

	// La verificación sin hallazgos es exactamente lo que esa ronda espera.
	sinHallazgos := conHallazgos
	sinHallazgos.Findings = nil
	if _, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: "acr_ident", TargetDigest: "sha256:corregido",
		Result: sinHallazgos,
	}); err != nil {
		t.Fatalf("la verificación post-corrección debe aceptarse: %v", err)
	}
}
