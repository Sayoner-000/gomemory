package usecases

import (
	"fmt"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

type SubmitReviewerResultInput struct {
	Project      string
	ReviewID     string
	TargetDigest string
	Result       domain.ReviewerResult
}

type SubmitReviewerResultOutput struct {
	ConsensusReady bool
	FindingIDs     map[string]int64
}

func SubmitReviewerResult(repo ports.ReviewRepository, input SubmitReviewerResultInput) (SubmitReviewerResultOutput, error) {
	review, err := repo.GetReview(input.Project, input.ReviewID)
	if err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	if review == nil {
		return SubmitReviewerResultOutput{}, fmt.Errorf("review %s not found", input.ReviewID)
	}
	if err := review.EnsureMutable(); err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	// Contra el target VIGENTE, no contra el original: tras una corrección los
	// revisores inspeccionan la revisión corregida, y validar contra el digest
	// congelado al inicio rechazaría precisamente el resultado correcto (FR-011).
	if strings.TrimSpace(input.TargetDigest) != review.ActiveTargetDigest() {
		return SubmitReviewerResultOutput{}, fmt.Errorf("target changed")
	}
	if !input.Result.Reviewer.Valid() {
		return SubmitReviewerResultOutput{}, fmt.Errorf("reviewer must be A or B")
	}
	if input.Result.Status != domain.ReviewerResultSuccess && input.Result.Status != domain.ReviewerResultFailure {
		return SubmitReviewerResultOutput{}, fmt.Errorf("invalid reviewer result status")
	}
	// La identidad esperada se congela al iniciar la revisión. Sin comprobarla, la
	// independencia que la revisión afirma tener no es verificable: cualquiera
	// puede enviar los dos resultados declarando proveedores distintos (FR-006).
	if esperado := review.ExpectedReviewer(input.Result.Reviewer); esperado.Declared() {
		if !esperado.Matches(input.Result.Provider, input.Result.Model) {
			return SubmitReviewerResultOutput{}, fmt.Errorf(
				"el resultado declara %s/%s pero el revisor %s se asignó a %s",
				input.Result.Provider, input.Result.Model, input.Result.Reviewer, esperado,
			)
		}
	}
	// Tras una corrección, el resultado de un revisor es una VERIFICACIÓN, no una
	// ronda de descubrimiento nueva: la revalidación está acotada a lo ya
	// confirmado y un defecto descubierto ahora pertenece a una revisión nueva
	// sobre el target corregido (FR-023).
	//
	// Sin esta guarda, esos hallazgos quedaban huérfanos —ninguna clasificación de
	// consenso los cubría— y la revisión no podía finalizar NUNCA: DeriveVerdict
	// los veía sin clasificar y devolvía «aún no». El mismo bloqueo irrecuperable
	// que esta funcionalidad existe para cerrar, por otra puerta.
	if review.Round > 0 && len(input.Result.Findings) > 0 {
		return SubmitReviewerResultOutput{}, fmt.Errorf(
			"la ronda %d es de revalidación y no admite hallazgos nuevos: "+
				"declara el resultado con review_rejudge, y abre una revisión nueva "+
				"sobre el target corregido para lo que no estuviera confirmado",
			review.Round,
		)
	}
	for _, finding := range input.Result.Findings {
		if err := validarHallazgoEstructurado(finding); err != nil {
			return SubmitReviewerResultOutput{}, err
		}
	}
	existing, err := repo.ListReviewerResults(input.Project, input.ReviewID, review.Round)
	if err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	for _, result := range existing {
		if result.Reviewer == input.Result.Reviewer && result.Status == domain.ReviewerResultFailure {
			return SubmitReviewerResultOutput{}, fmt.Errorf("failed reviewer result is final for this round")
		}
	}
	input.Result.Round = review.Round
	input.Result.ReviewID = review.ID
	if err := repo.UpsertReviewerResultAtomically(
		input.Project, input.ReviewID, review.Status, review.Round, &input.Result,
	); err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	findingIDs := make(map[string]int64, len(input.Result.Findings))
	for _, finding := range input.Result.Findings {
		findingIDs[finding.LocalID] = finding.ID
	}
	if input.Result.Status == domain.ReviewerResultFailure {
		// Por TransitionTo y bajo comparación-y-cambio, no por asignación directa
		// más UpdateReview. Esto era lo último que quedaba de la escritura ciega: un
		// resultado que llegaba tarde reabría una revisión ya cerrada, borraba un
		// veredicto escrito o devolvía la ronda y el target a los valores que había
		// leído, con la corrección ya registrada en el ledger.
		anterior := review.Status
		if err := review.TransitionTo(domain.ReviewIncomplete); err != nil {
			return SubmitReviewerResultOutput{}, err
		}
		if err := repo.SetReviewStatusAtomically(input.Project, input.ReviewID, ports.StatusTransition{
			ExpectedStatus: anterior,
			ExpectedRound:  review.Round,
			ExpectedDigest: review.ActiveTargetDigest(),
			Verdict:        domain.VerdictIncomplete,
			NextStatus:     domain.ReviewIncomplete,
		}); err != nil {
			return SubmitReviewerResultOutput{}, err
		}
		return SubmitReviewerResultOutput{FindingIDs: findingIDs}, nil
	}
	results, err := repo.ListReviewerResults(input.Project, input.ReviewID, review.Round)
	if err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	seen := map[domain.Reviewer]bool{}
	for _, result := range results {
		if result.Status == domain.ReviewerResultSuccess {
			seen[result.Reviewer] = true
		}
	}
	ready := seen[domain.ReviewerA] && seen[domain.ReviewerB]
	// Solo se escribe cuando el avance es LEGAL en la máquina de estados. Al asignar
	// Status directamente, este punto producía dos transiciones que la máquina
	// prohíbe: consensus_ready -> consensus_ready al reenviar un resultado, y
	// rejudging -> consensus_ready en una ronda de revalidación, que además es
	// incorrecta de raíz —el consenso no se reconstruye en las rondas de
	// revalidación—. En ambos casos lo correcto es no mover el estado, no forzarlo.
	if ready && review.Status.CanTransitionTo(domain.ReviewConsensusReady) {
		anterior := review.Status
		if err := review.TransitionTo(domain.ReviewConsensusReady); err != nil {
			return SubmitReviewerResultOutput{}, err
		}
		// Sin veredicto: un avance de estado intermedio no emite juicio.
		if err := repo.SetReviewStatusAtomically(input.Project, input.ReviewID, ports.StatusTransition{
			ExpectedStatus: anterior,
			ExpectedRound:  review.Round,
			ExpectedDigest: review.ActiveTargetDigest(),
			NextStatus:     domain.ReviewConsensusReady,
		}); err != nil {
			return SubmitReviewerResultOutput{}, err
		}
	}
	return SubmitReviewerResultOutput{ConsensusReady: ready, FindingIDs: findingIDs}, nil
}

// validarHallazgoEstructurado exige los campos obligatorios en el BORDE del sistema
// (FR-007).
//
// Antes solo se consultaban al confirmar, vía Finding.Confirmable(): un hallazgo sin
// ubicación ni categoría entraba al ledger sin protestar y solo estorbaba mucho más
// tarde, cuando ya no había forma de pedirle al revisor que lo completara.
func validarHallazgoEstructurado(finding domain.Finding) error {
	obligatorios := []struct {
		campo string
		valor string
	}{
		{"local_id", finding.LocalID},
		{"location", finding.Location},
		{"severity", string(finding.Severity)},
		{"category", finding.Category},
		{"claim", finding.Claim},
		{"evidence_class", string(finding.EvidenceClass)},
		{"confidence", finding.Confidence},
	}
	for _, obligatorio := range obligatorios {
		if strings.TrimSpace(obligatorio.valor) == "" {
			return fmt.Errorf("el hallazgo %s omite el campo obligatorio %s",
				identificarHallazgo(finding), obligatorio.campo)
		}
	}
	if !finding.Severity.Valid() {
		return fmt.Errorf("el hallazgo %s declara una severidad desconocida: %q",
			identificarHallazgo(finding), finding.Severity)
	}
	if !finding.EvidenceClass.Valid() {
		return fmt.Errorf("el hallazgo %s declara una clase de evidencia desconocida: %q",
			identificarHallazgo(finding), finding.EvidenceClass)
	}
	for _, evidencia := range finding.Evidence {
		if strings.TrimSpace(evidencia) != "" {
			return nil
		}
	}
	return fmt.Errorf("el hallazgo %s omite el campo obligatorio evidence",
		identificarHallazgo(finding))
}

func identificarHallazgo(finding domain.Finding) string {
	if id := strings.TrimSpace(finding.LocalID); id != "" {
		return id
	}
	return "(sin local_id)"
}
