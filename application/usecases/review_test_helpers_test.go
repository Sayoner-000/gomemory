package usecases

import (
	"fmt"
	"sort"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

type memoryReviewRepository struct {
	reviews   map[string]*domain.Review
	results   map[string][]domain.ReviewerResult
	consensus *memoryConsensusRepository
}

func newMemoryReviewRepository() *memoryReviewRepository {
	return &memoryReviewRepository{
		reviews: make(map[string]*domain.Review),
		results: make(map[string][]domain.ReviewerResult),
	}
}

func reviewKey(project, reviewID string) string { return project + ":" + reviewID }

func (r *memoryReviewRepository) CreateReview(review *domain.Review) error {
	key := reviewKey(review.Project, review.ID)
	if _, exists := r.reviews[key]; exists {
		return fmt.Errorf("duplicate review")
	}
	copy := *review
	r.reviews[key] = &copy
	return nil
}

func (r *memoryReviewRepository) GetReview(project, reviewID string) (*domain.Review, error) {
	review := r.reviews[reviewKey(project, reviewID)]
	if review == nil {
		return nil, nil
	}
	copy := *review
	return &copy, nil
}

func (r *memoryReviewRepository) UpdateReview(review *domain.Review) error {
	copy := *review
	r.reviews[reviewKey(review.Project, review.ID)] = &copy
	return nil
}

// SetReviewStatusAtomically refleja la implementación real: compara el estado sobre el
// que se derivó el veredicto y escribe SOLO status y verdict. Si el doble escribiera
// la revisión entera, los tests darían verde sobre una invariante que el adaptador de
// verdad sí tiene y el doble no.
func (r *memoryReviewRepository) SetReviewStatusAtomically(
	project, reviewID string, transition ports.StatusTransition,
) error {
	review := r.reviews[reviewKey(project, reviewID)]
	if review == nil {
		return fmt.Errorf("review %s not found", reviewID)
	}
	if review.Status != transition.ExpectedStatus {
		return fmt.Errorf(
			"la revisión pasó a %s mientras se finalizaba: vuelve a derivar el veredicto",
			review.Status,
		)
	}
	if review.Round != transition.ExpectedRound {
		return fmt.Errorf(
			"la revisión avanzó a la ronda %d mientras se finalizaba: vuelve a derivar el veredicto",
			review.Round,
		)
	}
	if transition.ExpectedDigest != "" && review.ActiveTargetDigest() != transition.ExpectedDigest {
		return fmt.Errorf(
			"el target vigente cambió mientras se finalizaba: el veredicto ya no corresponde a %s",
			transition.ExpectedDigest,
		)
	}
	if transition.ExpectedRejudgmentMark != "" {
		marca, err := r.rejudgmentMarkDe(project, reviewID)
		if err != nil {
			return err
		}
		if marca != transition.ExpectedRejudgmentMark {
			return fmt.Errorf("los re-juicios cambiaron mientras se finalizaba: vuelve a derivar el veredicto")
		}
	}
	if transition.ExpectedReviewerResultsMark != "" {
		marca, err := r.ReviewerResultsMark(project, reviewID, review.Round)
		if err != nil {
			return err
		}
		if marca != transition.ExpectedReviewerResultsMark {
			return fmt.Errorf("los resultados de revisor cambiaron mientras se finalizaba: vuelve a derivar el veredicto")
		}
	}
	review.Status = transition.NextStatus
	if transition.Verdict != "" {
		review.Verdict = transition.Verdict
	}
	return nil
}

// ledger enlaza el repositorio de revisiones con el de consenso para poder calcular
// la marca de re-juicios, que es lo que el adaptador real deriva de sus filas.
func (r *memoryReviewRepository) rejudgmentMarkDe(project, reviewID string) (string, error) {
	if r.consensus == nil {
		return "", nil
	}
	return r.consensus.marcaDeReJuicios(project, reviewID), nil
}

func (r *memoryReviewRepository) RejudgmentMark(project, reviewID string) (string, error) {
	return r.rejudgmentMarkDe(project, reviewID)
}

func (r *memoryReviewRepository) ReviewerResultsMark(project, reviewID string, round int) (string, error) {
	results, err := r.ListReviewerResults(project, reviewID, round)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%#v", results), nil
}

func (r *memoryReviewRepository) ListReviews(project string, limit int) ([]domain.Review, error) {
	var out []domain.Review
	for _, review := range r.reviews {
		if review.Project == project {
			out = append(out, *review)
		}
	}
	return out, nil
}

func (r *memoryReviewRepository) UpsertReviewerResult(project, reviewID string, result *domain.ReviewerResult) error {
	key := reviewKey(project, reviewID)
	items := r.results[key]
	for i := range items {
		if items[i].Reviewer == result.Reviewer && items[i].Round == result.Round {
			for j := range result.Findings {
				if result.Findings[j].ID == 0 {
					result.Findings[j].ID = items[i].Findings[0].ID
				}
			}
			items[i] = *result
			r.results[key] = items
			return nil
		}
	}
	result.ID = int64(len(items) + 1)
	for i := range result.Findings {
		result.Findings[i].ID = int64(len(items)*100 + i + 1)
		result.Findings[i].ReviewerResultID = result.ID
	}
	r.results[key] = append(items, *result)
	return nil
}

func (r *memoryReviewRepository) UpsertReviewerResultAtomically(
	project, reviewID string,
	expectedStatus domain.ReviewStatus,
	expectedRound int,
	result *domain.ReviewerResult,
) error {
	review := r.reviews[reviewKey(project, reviewID)]
	if review == nil {
		return fmt.Errorf("review %s not found", reviewID)
	}
	if review.Status.Terminal() {
		return fmt.Errorf("la revisión está en estado terminal %s y no admite cambios", review.Status)
	}
	if review.Status != expectedStatus || review.Round != expectedRound {
		return fmt.Errorf(
			"la revisión cambió a %s ronda %d mientras se enviaba el resultado; vuelve a intentarlo",
			review.Status, review.Round,
		)
	}
	faseEsperada := domain.ReviewAwaitingReviewers
	if review.Round > 0 {
		faseEsperada = domain.ReviewRejudging
	}
	if review.Status != faseEsperada {
		return fmt.Errorf(
			"la revisión está en %s y los resultados de la ronda %d ya no se pueden modificar",
			review.Status, review.Round,
		)
	}
	if result.Round != review.Round {
		return fmt.Errorf("el resultado pertenece a la ronda %d, la vigente es la %d", result.Round, review.Round)
	}
	return r.UpsertReviewerResult(project, reviewID, result)
}

func (r *memoryReviewRepository) ListReviewerResults(project, reviewID string, round int) ([]domain.ReviewerResult, error) {
	var out []domain.ReviewerResult
	for _, result := range r.results[reviewKey(project, reviewID)] {
		if result.Round == round {
			out = append(out, result)
		}
	}
	return out, nil
}

func (r *memoryReviewRepository) GetFinding(project, reviewID string, findingID int64) (*domain.Finding, error) {
	for _, result := range r.results[reviewKey(project, reviewID)] {
		for _, finding := range result.Findings {
			if finding.ID == findingID {
				copy := finding
				return &copy, nil
			}
		}
	}
	return nil, nil
}

func (r *memoryReviewRepository) ListFindings(project, reviewID string, round int) ([]domain.Finding, error) {
	var out []domain.Finding
	for _, result := range r.results[reviewKey(project, reviewID)] {
		if result.Round == round {
			out = append(out, result.Findings...)
		}
	}
	return out, nil
}

type memoryConsensusRepository struct {
	findings   map[string][]domain.ConsensusFinding
	fixes      map[string][]domain.FixDelta
	rejudgment map[string][]domain.ReJudgment
	reviews    *memoryReviewRepository
}

func newMemoryConsensusRepository() *memoryConsensusRepository {
	return &memoryConsensusRepository{
		findings:   make(map[string][]domain.ConsensusFinding),
		fixes:      make(map[string][]domain.FixDelta),
		rejudgment: make(map[string][]domain.ReJudgment),
	}
}

// enlazar deja al ledger en memoria hablar con el repositorio de revisiones, que es
// lo que la transición atómica de corrección necesita para avanzar la revisión.
func (r *memoryConsensusRepository) enlazar(reviews *memoryReviewRepository) *memoryConsensusRepository {
	r.reviews = reviews
	reviews.consensus = r
	return r
}

// marcaDeReJuicios reproduce la marca del adaptador real: cambia con cualquier alta o
// modificación de un re-juicio. Si el doble no la moviera, el test de la carrera de
// finalización pasaría contra un doble que no tiene la comprobación.
func (r *memoryConsensusRepository) marcaDeReJuicios(project, reviewID string) string {
	prefijo := reviewKey(project, reviewID) + ":"
	partes := make([]string, 0)
	for key, judgments := range r.rejudgment {
		if !strings.HasPrefix(key, prefijo) {
			continue
		}
		for _, judgment := range judgments {
			partes = append(partes, fmt.Sprintf("%s|%d|%s|%s",
				judgment.ConsensusLocalID, judgment.Round, judgment.Reviewer, judgment.State))
		}
	}
	sort.Strings(partes)
	return strings.Join(partes, ",")
}

func (r *memoryConsensusRepository) UpsertConsensusFinding(project, reviewID string, finding *domain.ConsensusFinding) error {
	key := reviewKey(project, reviewID)
	for i := range r.findings[key] {
		if r.findings[key][i].ConsensusLocalID == finding.ConsensusLocalID {
			r.findings[key][i] = *finding
			return nil
		}
	}
	finding.ID = int64(len(r.findings[key]) + 1)
	r.findings[key] = append(r.findings[key], *finding)
	return nil
}

func (r *memoryConsensusRepository) GetConsensusFinding(project, reviewID, localID string) (*domain.ConsensusFinding, error) {
	for _, finding := range r.findings[reviewKey(project, reviewID)] {
		if finding.ConsensusLocalID == localID {
			copy := finding
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *memoryConsensusRepository) ListConsensusFindings(project, reviewID string, round int) ([]domain.ConsensusFinding, error) {
	var out []domain.ConsensusFinding
	for _, finding := range r.findings[reviewKey(project, reviewID)] {
		if finding.Round == round {
			out = append(out, finding)
		}
	}
	return out, nil
}

// ReplaceConsensusRound refleja la implementación real: la comprobación de "ya existe"
// y la escritura son un solo paso. El doble no puede reproducir una carrera, pero sí
// las reglas de idempotencia y rechazo que el caso de uso delegó aquí.
func (r *memoryConsensusRepository) ReplaceConsensusRound(
	project, reviewID string, expectedRound int, fingerprint string,
	findings []domain.ConsensusFinding,
) ([]domain.ConsensusFinding, bool, error) {
	if r.reviews != nil {
		review, err := r.reviews.GetReview(project, reviewID)
		if err != nil {
			return nil, false, err
		}
		if review != nil {
			if err := review.EnsureMutable(); err != nil {
				return nil, false, err
			}
			if review.Round != expectedRound {
				return nil, false, fmt.Errorf(
					"la revisión avanzó a la ronda %d mientras se construía el consenso de la ronda %d",
					review.Round, expectedRound,
				)
			}
		}
	}
	existentes, err := r.ListConsensusFindings(project, reviewID, expectedRound)
	if err != nil {
		return nil, false, err
	}
	if len(existentes) > 0 {
		if domain.ClassificationFingerprint(existentes) != fingerprint {
			return nil, false, fmt.Errorf(
				"la ronda %d ya tiene un consenso registrado y no admite reemplazo", expectedRound,
			)
		}
		return existentes, true, nil
	}
	persistidos := make([]domain.ConsensusFinding, len(findings))
	copy(persistidos, findings)
	for i := range persistidos {
		if err := r.UpsertConsensusFinding(project, reviewID, &persistidos[i]); err != nil {
			return nil, false, err
		}
	}
	return persistidos, false, nil
}

func (r *memoryConsensusRepository) ListAllConsensusFindings(project, reviewID string) ([]domain.ConsensusFinding, error) {
	return append([]domain.ConsensusFinding(nil), r.findings[reviewKey(project, reviewID)]...), nil
}

func (r *memoryConsensusRepository) UpsertFixDelta(project, reviewID string, delta *domain.FixDelta) error {
	key := reviewKey(project, reviewID)
	for i := range r.fixes[key] {
		if r.fixes[key][i].Round == delta.Round {
			r.fixes[key][i] = *delta
			return nil
		}
	}
	r.fixes[key] = append(r.fixes[key], *delta)
	return nil
}

func (r *memoryConsensusRepository) ListFixDeltas(project, reviewID string) ([]domain.FixDelta, error) {
	return append([]domain.FixDelta(nil), r.fixes[reviewKey(project, reviewID)]...), nil
}

func rejudgmentKey(project, reviewID, localID string) string {
	return reviewKey(project, reviewID) + ":" + localID
}

func (r *memoryConsensusRepository) UpsertReJudgment(project, reviewID string, judgment *domain.ReJudgment) error {
	if err := judgment.Validate(); err != nil {
		return err
	}
	finding, err := r.GetConsensusFinding(project, reviewID, judgment.ConsensusLocalID)
	if err != nil {
		return err
	}
	if finding == nil {
		return fmt.Errorf("el hallazgo de consenso %s no existe en esta revisión", judgment.ConsensusLocalID)
	}
	key := rejudgmentKey(project, reviewID, judgment.ConsensusLocalID)
	reemplazado := false
	for i := range r.rejudgment[key] {
		existente := r.rejudgment[key][i]
		if existente.Reviewer == judgment.Reviewer && existente.Round == judgment.Round {
			r.rejudgment[key][i] = *judgment
			reemplazado = true
			break
		}
	}
	if !reemplazado {
		r.rejudgment[key] = append(r.rejudgment[key], *judgment)
	}
	finding.RejudgmentState = domain.AggregateReJudgmentForRound(r.rejudgment[key], judgment.Round)
	finding.RejudgmentRound = judgment.Round
	return r.UpsertConsensusFinding(project, reviewID, finding)
}

func (r *memoryConsensusRepository) ListReJudgments(project, reviewID, localID string) ([]domain.ReJudgment, error) {
	return append([]domain.ReJudgment(nil), r.rejudgment[rejudgmentKey(project, reviewID, localID)]...), nil
}

func (r *memoryConsensusRepository) RecordFixAtomically(
	project, reviewID string, transition ports.FixTransition,
) error {
	key := reviewKey(project, reviewID)
	if len(r.fixes[key]) != transition.ExpectedRounds {
		return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
	}
	if r.reviews != nil && transition.ExpectedStatus != "" {
		actual, err := r.reviews.GetReview(project, reviewID)
		if err != nil {
			return err
		}
		if actual != nil && actual.Status != transition.ExpectedStatus {
			return fmt.Errorf(
				"la revisión pasó a %s mientras se registraba la corrección de la ronda %d",
				actual.Status, transition.NextRound,
			)
		}
	}
	if r.reviews != nil && transition.ExpectedBaseDigest != "" {
		actual, err := r.reviews.GetReview(project, reviewID)
		if err != nil {
			return err
		}
		if actual != nil && actual.ActiveTargetDigest() != transition.ExpectedBaseDigest {
			return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
		}
	}
	for _, existente := range r.fixes[key] {
		if existente.Round == transition.Delta.Round {
			return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
		}
	}
	r.fixes[key] = append(r.fixes[key], *transition.Delta)
	// Abrir una ronda invalida el veredicto de re-juicio de la anterior, igual que en
	// el adaptador real: si el doble lo conservara, el test de arrastre de RESOLVED
	// pasaría contra un doble que no tiene la corrección.
	for i := range r.findings[key] {
		r.findings[key][i].RejudgmentState = ""
		r.findings[key][i].RejudgmentRound = 0
	}
	if r.reviews == nil {
		return nil
	}
	review, err := r.reviews.GetReview(project, reviewID)
	if err != nil {
		return err
	}
	if review == nil {
		return fmt.Errorf("review %s not found", reviewID)
	}
	review.Round = transition.NextRound
	review.Status = transition.NextStatus
	review.CurrentTargetDigest = transition.CurrentTargetDigest
	return r.reviews.UpdateReview(review)
}

// CountPromotedMemories: el repositorio en memoria no guarda memorias promovidas,
// así que informa cero. Los tests que miden esta métrica usan SQLite real.
func (r *memoryReviewRepository) CountPromotedMemories(project, reviewID string) (int, int, error) {
	return 0, 0, nil
}
