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
	if _, err := RejudgeReview(reviews, ledger, entradaDeReRevision(
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved},
	)); err != nil {
		t.Fatalf("RejudgeReview: %v", err)
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
	if _, err := RejudgeReview(reviews, ledger, entradaDeReRevision(
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentUnresolved},
	)); err != nil {
		t.Fatalf("RejudgeReview: %v", err)
	}
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
