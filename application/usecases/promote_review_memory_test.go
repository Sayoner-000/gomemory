package usecases

import (
	"strings"
	"testing"

	"mem/domain"
)

// memoriaEscrita registra lo insertado y simula el upsert por topic_key que ya
// implementa la persistencia real (feature 008): misma clave ⇒ misma fila.
type memoriaEscrita struct {
	porClave map[string]*domain.Memory
	orden    []string
	inserts  int
}

func nuevaMemoriaEscrita() *memoriaEscrita {
	return &memoriaEscrita{porClave: map[string]*domain.Memory{}}
}

func (m *memoriaEscrita) Insert(mem *domain.Memory) (int64, error) {
	m.inserts++
	if existente, ok := m.porClave[mem.TopicKey]; ok {
		mem.ID = existente.ID
		copia := *mem
		m.porClave[mem.TopicKey] = &copia
		return existente.ID, nil
	}
	mem.ID = int64(len(m.orden) + 1)
	copia := *mem
	m.porClave[mem.TopicKey] = &copia
	m.orden = append(m.orden, mem.TopicKey)
	return mem.ID, nil
}

func (m *memoriaEscrita) filas() int { return len(m.orden) }

func aprendizajePromovible() domain.ReviewLearning {
	return domain.ReviewLearning{
		ReviewID: "acr_test", Category: "concurrency", Component: "memory-store",
		Problem:    "dos escrituras concurrentes operaban sobre el mismo estado obsoleto",
		RootCause:  "el estado se leía antes de entrar en la transacción",
		Resolution: "la lectura se movió dentro de la transacción",
		Verification: []string{
			"go test ./internal/memory/...",
		},
		Confidence: "high",
	}
}

// escenarioAprobado deja una revisión aprobada con un defecto confirmado y
// resuelto: la única combinación que la especificación autoriza a promover.
func escenarioAprobado(t *testing.T) (*memoryReviewRepository, *memoryConsensusRepository) {
	t.Helper()
	reviews, ledger := escenarioReRevisable(t)

	// Los dos revisores validan la ronda corregida. Sin esto la revisión queda
	// INCOMPLETE y no puede aprobarse.
	ronda, _ := reviews.GetReview("proj", "acr_test")
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		resultado := domain.ReviewerResult{
			Reviewer: revisor, Round: ronda.Round, Status: domain.ReviewerResultSuccess,
		}
		if err := reviews.UpsertReviewerResult("proj", "acr_test", &resultado); err != nil {
			t.Fatalf("UpsertReviewerResult: %v", err)
		}
	}
	reRevisionUnanime(t, reviews, ledger,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved})

	// El helper se llama "aprobado" y hasta la funcionalidad 028 no lo estaba: la
	// revisión se quedaba sin finalizar y nadie lo notaba porque la promoción no
	// miraba el veredicto. Ahora sí lo mira, así que el helper tiene que cumplir
	// lo que su nombre promete.
	review, err := FinalizeReview(reviews, ledger, "proj", "acr_test")
	if err != nil {
		t.Fatalf("FinalizeReview: %v", err)
	}
	if review.Verdict != domain.VerdictApproved {
		t.Fatalf("el escenario debe quedar APPROVED, quedó %s", review.Verdict)
	}
	return reviews, ledger
}

// TestPromoteReviewMemory_SoloConfirmadoYResuelto cubre FR-033: promover un
// defecto sin resolver guardaría como aprendizaje algo que aún no se sabe
// arreglar, y ese "conocimiento" se serviría en cada sesión futura.
func TestPromoteReviewMemory_SoloConfirmadoYResuelto(t *testing.T) {
	reviews, ledger := escenarioAprobado(t)
	escritas := nuevaMemoriaEscrita()

	out, err := PromoteReviewMemory(reviews, ledger, escritas, PromoteReviewMemoryInput{
		Project: "proj", ReviewID: "acr_test",
		Learnings: map[string]domain.ReviewLearning{"C-001": aprendizajePromovible()},
	})
	if err != nil {
		t.Fatalf("PromoteReviewMemory: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("se promovieron %d memorias, se esperaba 1", len(out))
	}
	if escritas.filas() != 1 {
		t.Errorf("filas escritas = %d, se esperaba 1", escritas.filas())
	}
}

// TestPromoteReviewMemory_RechazaSinResolver: el mismo hallazgo confirmado, sin
// resolución, no se promueve.
func TestPromoteReviewMemory_RechazaSinResolver(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)
	reRevisionUnanime(t, reviews, ledger,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentUnresolved})
	escritas := nuevaMemoriaEscrita()

	_, err := PromoteReviewMemory(reviews, ledger, escritas, PromoteReviewMemoryInput{
		Project: "proj", ReviewID: "acr_test",
		Learnings: map[string]domain.ReviewLearning{"C-001": aprendizajePromovible()},
	})
	if err == nil {
		t.Fatal("se promovió un defecto sin resolver")
	}
	if escritas.inserts != 0 {
		t.Errorf("un rechazo escribió %d memoria(s)", escritas.inserts)
	}
}

// TestPromoteReviewMemory_RechazaSospechoso: un SUSPECT nunca tuvo
// corroboración, así que su "aprendizaje" podría ser un falso positivo.
func TestPromoteReviewMemory_RechazaSospechoso(t *testing.T) {
	reviews, ledger := escenarioAprobado(t)
	escritas := nuevaMemoriaEscrita()

	_, err := PromoteReviewMemory(reviews, ledger, escritas, PromoteReviewMemoryInput{
		Project: "proj", ReviewID: "acr_test",
		Learnings: map[string]domain.ReviewLearning{"S-001": aprendizajePromovible()},
	})
	if err == nil {
		t.Fatal("se promovió un hallazgo SUSPECT")
	}
}

// TestPromoteReviewMemory_DeduplicaPorPatron cubre AC-009: dos revisiones del
// mismo patrón refuerzan una memoria, no crean dos.
func TestPromoteReviewMemory_DeduplicaPorPatron(t *testing.T) {
	escritas := nuevaMemoriaEscrita()

	for i, reviewID := range []string{"acr_test", "acr_test"} {
		reviews, ledger := escenarioAprobado(t)
		aprendizaje := aprendizajePromovible()
		aprendizaje.ReviewID = reviewID
		aprendizaje.Resolution = "resolución de la revisión " + string(rune('A'+i))

		if _, err := PromoteReviewMemory(reviews, ledger, escritas, PromoteReviewMemoryInput{
			Project: "proj", ReviewID: "acr_test",
			Learnings: map[string]domain.ReviewLearning{"C-001": aprendizaje},
		}); err != nil {
			t.Fatalf("promoción %d: %v", i+1, err)
		}
	}

	if escritas.inserts != 2 {
		t.Fatalf("inserts = %d, se esperaban 2 llamadas", escritas.inserts)
	}
	if escritas.filas() != 1 {
		t.Errorf("filas = %d: el mismo patrón debe actualizar una sola memoria (AC-009)", escritas.filas())
	}
	guardada := escritas.porClave[aprendizajePromovible().TopicKey()]
	if !strings.Contains(guardada.Content, "revisión B") {
		t.Error("la segunda promoción no actualizó el contenido de la memoria existente")
	}
}

// TestPromoteReviewMemory_EnlazaConSuRevision cubre FR-038: sin el enlace, el
// linaje se corta justo donde el conocimiento sale del protocolo.
func TestPromoteReviewMemory_EnlazaConSuRevision(t *testing.T) {
	reviews, ledger := escenarioAprobado(t)
	escritas := nuevaMemoriaEscrita()

	out, err := PromoteReviewMemory(reviews, ledger, escritas, PromoteReviewMemoryInput{
		Project: "proj", ReviewID: "acr_test",
		Learnings: map[string]domain.ReviewLearning{"C-001": aprendizajePromovible()},
	})
	if err != nil {
		t.Fatalf("PromoteReviewMemory: %v", err)
	}
	if out[0].SourceReviewID == "" {
		t.Error("la memoria promovida no referencia su revisión de origen")
	}
}

// TestPromoteReviewMemory_ExigeVeredictoAprobado cubre FR-021 y el escenario 5 de la
// Historia 3. El caso de uso ya comprobaba que el hallazgo estuviera CONFIRMED y
// RESOLVED, pero no miraba el veredicto de la revisión que lo contiene: se podía
// promover conocimiento de una revisión que todavía iba a escalar.
func TestPromoteReviewMemory_ExigeVeredictoAprobado(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)
	reRevisionUnanime(t, reviews, ledger,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved})
	escritas := nuevaMemoriaEscrita()

	// El hallazgo está confirmado y resuelto, pero la revisión no ha finalizado.
	_, err := PromoteReviewMemory(reviews, ledger, escritas, PromoteReviewMemoryInput{
		Project: "proj", ReviewID: "acr_test",
		Learnings: map[string]domain.ReviewLearning{"C-001": aprendizajePromovible()},
	})
	if err == nil {
		t.Fatal("se promovió desde una revisión sin aprobar")
	}
	if escritas.inserts != 0 {
		t.Errorf("se escribieron %d memorias pese al rechazo", escritas.inserts)
	}

	// Tras aprobarla, la misma promoción procede.
	aprobadas, aprobadoLedger := escenarioAprobado(t)
	if _, err := PromoteReviewMemory(aprobadas, aprobadoLedger, escritas, PromoteReviewMemoryInput{
		Project: "proj", ReviewID: "acr_test",
		Learnings: map[string]domain.ReviewLearning{"C-001": aprendizajePromovible()},
	}); err != nil {
		t.Fatalf("una revisión aprobada debe poder promover: %v", err)
	}
}

// TestPromoteReviewMemory_RechazaRevisionTerminalNoAprobada: escalada e incompleta
// también son terminales, y ninguna de las dos es una aprobación.
func TestPromoteReviewMemory_RechazaRevisionTerminalNoAprobada(t *testing.T) {
	for _, veredicto := range []domain.Verdict{domain.VerdictEscalated, domain.VerdictIncomplete} {
		t.Run(string(veredicto), func(t *testing.T) {
			reviews, ledger := escenarioReRevisable(t)
			reRevisionUnanime(t, reviews, ledger,
				map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved})
			review, _ := reviews.GetReview("proj", "acr_test")
			review.Verdict = veredicto
			if err := reviews.UpdateReview(review); err != nil {
				t.Fatal(err)
			}
			_, err := PromoteReviewMemory(reviews, ledger, nuevaMemoriaEscrita(), PromoteReviewMemoryInput{
				Project: "proj", ReviewID: "acr_test",
				Learnings: map[string]domain.ReviewLearning{"C-001": aprendizajePromovible()},
			})
			if err == nil {
				t.Fatalf("una revisión %s no puede promover aprendizaje", veredicto)
			}
		})
	}
}
