package domain

import (
	"strings"
	"testing"
)

func aprendizajeDePrueba() ReviewLearning {
	return ReviewLearning{
		ReviewID:     "acr_01JXYZ",
		Category:     "concurrency",
		Component:    "memory-store",
		Problem:      "dos escrituras concurrentes operaban sobre el mismo estado obsoleto",
		RootCause:    "el estado se leía antes de entrar en la transacción",
		Resolution:   "la lectura del estado se movió dentro de la transacción protegida",
		Verification: []string{"test de regresión concurrente", "go test ./internal/memory/..."},
		Confidence:   "high",
	}
}

// TestReviewLearning_TopicKeyAgrupaPorPatronNoPorRevision es lo que hace que la
// deduplicación funcione (FR-034): dos revisiones distintas del MISMO patrón de
// fallo deben compartir clave para que la segunda actualice la memoria de la
// primera. Incluir el review_id en la clave daría una memoria nueva por
// revisión y el conocimiento se fragmentaría en vez de reforzarse.
func TestReviewLearning_TopicKeyAgrupaPorPatronNoPorRevision(t *testing.T) {
	a := aprendizajeDePrueba()
	b := aprendizajeDePrueba()
	b.ReviewID = "acr_OTRA"
	b.Problem = "otra redacción del mismo problema"

	if a.TopicKey() != b.TopicKey() {
		t.Errorf("dos revisiones del mismo patrón dieron claves distintas:\n  %q\n  %q", a.TopicKey(), b.TopicKey())
	}
	if strings.Contains(a.TopicKey(), a.ReviewID) {
		t.Errorf("la clave %q incluye el review_id: cada revisión crearía su propia memoria", a.TopicKey())
	}

	distinto := aprendizajeDePrueba()
	distinto.Category = "security"
	if distinto.TopicKey() == a.TopicKey() {
		t.Error("patrones de categoría distinta comparten clave: se pisarían entre sí")
	}
}

// TestReviewLearning_MemoriaSoloLlevaConocimientoDestilado cubre FR-031 y
// AC-008 por CONSTRUCCIÓN: la estructura no tiene dónde meter un transcript ni
// una cadena de razonamiento, así que no depende de que quien llame se acuerde
// de excluirlos.
func TestReviewLearning_MemoriaSoloLlevaConocimientoDestilado(t *testing.T) {
	memoria, err := aprendizajeDePrueba().Memory("proj")
	if err != nil {
		t.Fatalf("Memory: %v", err)
	}

	if memoria.Type != Learning {
		t.Errorf("tipo = %s, se esperaba %s", memoria.Type, Learning)
	}
	if memoria.TopicKey == "" {
		t.Error("la memoria promovida sin TopicKey no se deduplicaría nunca")
	}
	for _, esperado := range []string{"concurrency", "memory-store", "transacción", "acr_01JXYZ"} {
		if !strings.Contains(memoria.Content, esperado) {
			t.Errorf("el contenido no menciona %q:\n%s", esperado, memoria.Content)
		}
	}
	if !strings.Contains(memoria.Content, "go test ./internal/memory/...") {
		t.Error("la verificación no llegó al contenido: sin ella nadie puede reproducir la comprobación")
	}
}

// TestReviewLearning_ExigeLoIndispensable: una memoria sin causa raíz ni
// resolución no es conocimiento reutilizable, es un titular. Guardarla
// ensuciaría el contexto de todas las sesiones futuras.
func TestReviewLearning_ExigeLoIndispensable(t *testing.T) {
	casos := map[string]func(*ReviewLearning){
		"sin problema":   func(l *ReviewLearning) { l.Problem = "" },
		"sin causa raíz": func(l *ReviewLearning) { l.RootCause = "" },
		"sin resolución": func(l *ReviewLearning) { l.Resolution = "" },
		"sin categoría":  func(l *ReviewLearning) { l.Category = "" },
	}
	for nombre, romper := range casos {
		t.Run(nombre, func(t *testing.T) {
			l := aprendizajeDePrueba()
			romper(&l)
			if _, err := l.Memory("proj"); err == nil {
				t.Fatal("se construyó una memoria incompleta")
			}
		})
	}
}

// TestPromotableFindings_SoloConfirmadoYResuelto: la condición de promoción es
// una regla del sistema, no del prompt. Cualquier otra combinación queda fuera.
func TestPromotableFindings_SoloConfirmadoYResuelto(t *testing.T) {
	consenso := []ConsensusFinding{
		{ConsensusLocalID: "C-002", Status: ConsensusConfirmed, RejudgmentState: ReJudgmentResolved},
		{ConsensusLocalID: "C-001", Status: ConsensusConfirmed, RejudgmentState: ReJudgmentResolved},
		{ConsensusLocalID: "C-003", Status: ConsensusConfirmed, RejudgmentState: ReJudgmentUnresolved},
		{ConsensusLocalID: "C-004", Status: ConsensusConfirmed, RejudgmentState: ReJudgmentRegressed},
		{ConsensusLocalID: "S-001", Status: ConsensusSuspect, RejudgmentState: ReJudgmentResolved},
		{ConsensusLocalID: "C-005", Status: ConsensusConfirmed},
	}

	got := PromotableFindings(consenso)
	quiero := []string{"C-001", "C-002"}
	if len(got) != len(quiero) {
		t.Fatalf("promovibles = %v, se esperaba %v", got, quiero)
	}
	for i := range quiero {
		if got[i] != quiero[i] {
			t.Fatalf("promovibles = %v, se esperaba %v (orden estable)", got, quiero)
		}
	}
}
