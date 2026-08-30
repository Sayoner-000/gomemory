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
		// El target vigente arranca igual que el original: la cadena de
		// correcciones se valida contra él, no contra un digest inventado.
		CurrentTargetDigest: "sha256:base",
		MaxFixRounds:        maxRondas,
		AutoFixSeverities:   []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
		FixAuthorized:       true,
		Status:              domain.ReviewConsensusReady,
	}
	reviews := newMemoryReviewRepository()
	if err := reviews.CreateReview(review); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}

	ledger := newMemoryConsensusRepository().enlazar(reviews)
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
	return correccionEncadenada("sha256:base", "sha256:fixed", ids...)
}

// correccionEncadenada construye una corrección que parte de un target concreto.
// Desde la funcionalidad 028 la cadena importa: la ronda N debe partir del target
// que dejó corregido la ronda N-1, y un digest inventado se rechaza (FR-009).
func correccionEncadenada(base, corregido string, ids ...string) RecordFixInput {
	return RecordFixInput{
		Project: "proj", ReviewID: "acr_test",
		AddressedConsensusIDs: ids,
		BaseTargetDigest:      base,
		FixedTargetDigest:     corregido,
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

	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:base", "sha256:r1", "C-001")); err != nil {
		t.Fatalf("ronda 1: %v", err)
	}
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:r1", "sha256:r2", "C-001")); err != nil {
		t.Fatalf("ronda 2: %v", err)
	}
	_, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:r2", "sha256:r3", "C-001"))
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

// TestRecordFix_ExigeElTargetVigente cubre FR-008 y FR-009. Sin esta comprobación,
// una corrección podía declarar como base una revisión del código que ya nadie
// estaba inspeccionando, y la cadena de evidencia dejaba de significar nada.
func TestRecordFix_ExigeElTargetVigente(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 3)

	_, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:inventado", "sha256:r1", "C-001"))
	if err == nil {
		t.Fatal("una corrección que no parte del target vigente debe rechazarse")
	}
	if !strings.Contains(err.Error(), "target vigente") {
		t.Errorf("el error debe nombrar el target vigente: %v", err)
	}

	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:base", "sha256:r1", "C-001")); err != nil {
		t.Fatalf("la primera corrección debe partir del original: %v", err)
	}
	review, _ := reviews.GetReview("proj", "acr_test")
	if review.CurrentTargetDigest != "sha256:r1" {
		t.Fatalf("el target vigente no avanzó: %q", review.CurrentTargetDigest)
	}

	// La ronda 2 ya no puede partir del original.
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:base", "sha256:r2", "C-001")); err == nil {
		t.Fatal("la ronda 2 no puede partir del target original")
	}
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:r1", "sha256:r2", "C-001")); err != nil {
		t.Fatalf("la ronda 2 debe partir del corregido por la ronda 1: %v", err)
	}
}

func TestRecordFix_RechazaRevisionDeSoloLectura(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)
	review, _ := reviews.GetReview("proj", "acr_test")
	review.FixAuthorized = false
	if err := reviews.UpdateReview(review); err != nil {
		t.Fatal(err)
	}
	_, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-001"))
	if err == nil {
		t.Fatal("una revisión de solo lectura no puede registrar correcciones")
	}
	if !strings.Contains(err.Error(), "solo lectura") {
		t.Errorf("el error debe explicar el alcance de solo lectura: %v", err)
	}
}

func TestRecordFix_RechazaRevisionTerminal(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)
	review, _ := reviews.GetReview("proj", "acr_test")
	review.Status = domain.ReviewApproved
	if err := reviews.UpdateReview(review); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-001")); err == nil {
		t.Fatal("una revisión aprobada no admite correcciones nuevas")
	}
}

// TestRecordFix_UnaSolaRondaGanaLaCarrera cubre FR-010 sobre el ledger en memoria.
// La versión real de esta garantía —con BEGIN IMMEDIATE y el UNIQUE de fix_rounds—
// se prueba contra SQLite en tests/integration/review_concurrent_fix_test.go.
func TestRecordFix_UnaSolaRondaGanaLaCarrera(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 3)
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:base", "sha256:r1", "C-001")); err != nil {
		t.Fatal(err)
	}
	// Una segunda corrección que cree seguir en la ronda 1 no puede pisar la
	// primera: parte de un target que ya no es el vigente.
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:base", "sha256:otro", "C-001")); err == nil {
		t.Fatal("una corrección rezagada sobrescribió la ronda ya registrada")
	}
	deltas, _ := ledger.ListFixDeltas("proj", "acr_test")
	if len(deltas) != 1 || deltas[0].FixedTargetDigest != "sha256:r1" {
		t.Fatalf("el ledger conserva la corrección equivocada: %#v", deltas)
	}
}
