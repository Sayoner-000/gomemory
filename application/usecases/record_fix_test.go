package usecases

import (
	"strings"
	"testing"

	"mem/domain"
)

// escenarioCorregible deja una revisión lista para corregir: consenso cerrado,
// un CONFIRMED severo y un SUSPECT del mismo nivel para poder contrastar que la
// diferencia la marca la corroboración, no la severidad.
func escenarioCorregible(t *testing.T, maxRondas int) (*memoryReviewRepository, *memoryConsensusRepository, *domain.Review) {
	t.Helper()

	target, err := domain.NewTarget(domain.TargetDiff, "abc123", "sha256:base", []string{"internal/"})
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	review := &domain.Review{
		ID: "acr_test", Project: "proj", Target: target,
		MaxFixRounds:      maxRondas,
		AutoFixSeverities: []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
		Status:            domain.ReviewConsensusReady,
	}
	reviews := newMemoryReviewRepository()
	if err := reviews.CreateReview(review); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}

	ledger := newMemoryConsensusRepository()
	confirmado := &domain.ConsensusFinding{
		ReviewID: review.ID, ConsensusLocalID: "C-001",
		Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
		SourceFindingIDs: []int64{1, 2},
	}
	sospechoso := &domain.ConsensusFinding{
		ReviewID: review.ID, ConsensusLocalID: "S-001",
		Status: domain.ConsensusSuspect, Severity: domain.SeverityHigh,
		SourceFindingIDs: []int64{3},
	}
	for _, f := range []*domain.ConsensusFinding{confirmado, sospechoso} {
		if err := ledger.UpsertConsensusFinding("proj", review.ID, f); err != nil {
			t.Fatalf("UpsertConsensusFinding: %v", err)
		}
	}
	return reviews, ledger, review
}

func entradaDeCorreccion(ids ...string) RecordFixInput {
	return RecordFixInput{
		Project: "proj", ReviewID: "acr_test",
		AddressedConsensusIDs: ids,
		BaseTargetDigest:      "sha256:base",
		FixedTargetDigest:     "sha256:fixed",
		ModifiedPaths:         []string{"internal/memory/store.go"},
		Verification:          []string{"go test ./internal/memory/..."},
		DiffDigest:            "sha256:diff",
	}
}

// TestRecordFix_RegistraLaRondaYAvanzaLaRevision es el camino feliz: un
// CONFIRMED severo produce ronda 1 y deja la revisión en re-revisión.
func TestRecordFix_RegistraLaRondaYAvanzaLaRevision(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)

	delta, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-001"))
	if err != nil {
		t.Fatalf("RecordFix: %v", err)
	}
	if delta.Round != 1 {
		t.Errorf("Round = %d, se esperaba 1", delta.Round)
	}

	review, _ := reviews.GetReview("proj", "acr_test")
	if review.Round != 1 {
		t.Errorf("la revisión quedó en la ronda %d, se esperaba 1", review.Round)
	}
	if review.Status != domain.ReviewRejudging {
		t.Errorf("estado = %s, se esperaba %s", review.Status, domain.ReviewRejudging)
	}
}

// TestRecordFix_RechazaSospechoso cubre INV-005: un hallazgo de un solo revisor
// no puede disparar corrección automática.
func TestRecordFix_RechazaSospechoso(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)

	if _, err := RecordFix(reviews, ledger, entradaDeCorreccion("S-001")); err == nil {
		t.Fatal("se aceptó corregir un SUSPECT")
	}
	if fixes, _ := ledger.ListFixDeltas("proj", "acr_test"); len(fixes) != 0 {
		t.Errorf("un rechazo dejó %d ronda(s) registrada(s): debe ser atómico", len(fixes))
	}
}

// TestRecordFix_ExigeAlMenosUnHallazgoConfirmado cubre INV-006: una corrección
// sin defecto que la justifique no es una corrección, es un cambio suelto.
func TestRecordFix_ExigeAlMenosUnHallazgoConfirmado(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)

	if _, err := RecordFix(reviews, ledger, entradaDeCorreccion()); err == nil {
		t.Fatal("se aceptó una corrección sin hallazgos referenciados")
	}
	if _, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-404")); err == nil {
		t.Fatal("se aceptó una corrección contra un hallazgo inexistente")
	}
}

// TestRecordFix_RespetaElPresupuesto cubre INV-009: la tercera ronda no existe
// con max_fix_rounds=2, y no hay parámetro de entrada que la pida.
func TestRecordFix_RespetaElPresupuesto(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)

	for ronda := 1; ronda <= 2; ronda++ {
		if _, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-001")); err != nil {
			t.Fatalf("ronda %d: %v", ronda, err)
		}
	}
	_, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-001"))
	if err == nil {
		t.Fatal("se registró una tercera ronda con presupuesto de 2")
	}
	if !strings.Contains(err.Error(), "presupuesto") {
		t.Errorf("el error no explica el presupuesto agotado: %v", err)
	}
}

// TestRecordFix_ExigeUnaRevisionNuevaDelTarget cubre INV-007: una corrección
// produce una revisión inmutable NUEVA. Un digest corregido igual al base
// significa que nada cambió, y registrarlo dejaría el ledger afirmando un
// arreglo inexistente.
func TestRecordFix_ExigeUnaRevisionNuevaDelTarget(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)

	entrada := entradaDeCorreccion("C-001")
	entrada.FixedTargetDigest = entrada.BaseTargetDigest
	if _, err := RecordFix(reviews, ledger, entrada); err == nil {
		t.Fatal("se aceptó una corrección cuyo target no cambió")
	}
}

// TestRecordFix_AutorizacionExplicitaAmpliaLaPoliticaNoLaInvariante: la bandera
// deja pasar una severidad fuera de política, pero nunca un no confirmado.
func TestRecordFix_AutorizacionExplicitaAmpliaLaPoliticaNoLaInvariante(t *testing.T) {
	reviews, ledger, review := escenarioCorregible(t, 2)

	medio := &domain.ConsensusFinding{
		ReviewID: review.ID, ConsensusLocalID: "C-002",
		Status: domain.ConsensusConfirmed, Severity: domain.SeverityMedium,
		SourceFindingIDs: []int64{4, 5},
	}
	if err := ledger.UpsertConsensusFinding("proj", review.ID, medio); err != nil {
		t.Fatalf("UpsertConsensusFinding: %v", err)
	}

	sinBandera := entradaDeCorreccion("C-002")
	if _, err := RecordFix(reviews, ledger, sinBandera); err == nil {
		t.Fatal("un MEDIUM entró sin autorización explícita")
	}

	conBandera := entradaDeCorreccion("C-002")
	conBandera.ExplicitAuthorization = true
	if _, err := RecordFix(reviews, ledger, conBandera); err != nil {
		t.Fatalf("un MEDIUM autorizado explícitamente fue rechazado: %v", err)
	}

	sospechosoAutorizado := entradaDeCorreccion("S-001")
	sospechosoAutorizado.ExplicitAuthorization = true
	if _, err := RecordFix(reviews, ledger, sospechosoAutorizado); err == nil {
		t.Fatal("la bandera explícita dejó pasar un SUSPECT: la corroboración no es negociable")
	}
}

// TestInvariantesNoDependenDelPrompt es la tesis de la feature (§44 y SC-008)
// puesta a prueba: con la guía de participación AUSENTE —aquí no hay ningún
// texto de prompt, solo llamadas directas— los tres atajos que un agente podría
// intentar siguen rechazados.
//
// Si este test pasara con las invariantes implementadas solo en SKILL.md,
// pasaría también sin ellas. Que falle al quitarlas del código es lo que
// distingue «el protocolo se cumple» de «el protocolo se pide por favor».
func TestInvariantesNoDependenDelPrompt(t *testing.T) {
	t.Run("no se puede corregir un hallazgo sin corroborar", func(t *testing.T) {
		reviews, ledger, _ := escenarioCorregible(t, 2)
		entrada := entradaDeCorreccion("S-001")
		entrada.ExplicitAuthorization = true // el agente insiste
		if _, err := RecordFix(reviews, ledger, entrada); err == nil {
			t.Fatal("un SUSPECT se corrigió pese a la invariante INV-005")
		}
	})

	t.Run("no se puede exceder el presupuesto de rondas", func(t *testing.T) {
		reviews, ledger, _ := escenarioCorregible(t, 1)
		if _, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-001")); err != nil {
			t.Fatalf("primera ronda: %v", err)
		}
		segunda := entradaDeCorreccion("C-001")
		segunda.BaseTargetDigest = "sha256:fixed"
		segunda.FixedTargetDigest = "sha256:fixed2"
		if _, err := RecordFix(reviews, ledger, segunda); err == nil {
			t.Fatal("se registró una ronda por encima de max_fix_rounds=1 (INV-009)")
		}
	})

	t.Run("no se puede declarar un veredicto por parámetro", func(t *testing.T) {
		reviews, ledger, _ := escenarioCorregible(t, 2)
		// La firma de FinalizeReview no admite un veredicto: la única forma de
		// obtener APPROVED es que el estado persistido lo sustente. Con un
		// CONFIRMED severo sin resolver y presupuesto disponible, la revisión
		// ni siquiera puede finalizarse.
		for _, reviewer := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
			r := domain.ReviewerResult{Reviewer: reviewer, Status: domain.ReviewerResultSuccess}
			if err := reviews.UpsertReviewerResult("proj", "acr_test", &r); err != nil {
				t.Fatalf("UpsertReviewerResult: %v", err)
			}
		}
		if _, err := FinalizeReview(reviews, ledger, "proj", "acr_test"); err == nil {
			t.Fatal("se finalizó una revisión con un defecto severo sin resolver (INV-010)")
		}
	})
}
