package domain

import "testing"

func revisionConPolitica(maxRondas int, severidades ...Severity) Review {
	if len(severidades) == 0 {
		severidades = []Severity{SeverityCritical, SeverityHigh}
	}
	return Review{MaxFixRounds: maxRondas, AutoFixSeverities: severidades}
}

// TestAuthorizeFix_SoloConfirmadosYSeveridadAdmitida cubre INV-005 y INV-006:
// la corrección automática es una autorización, no un permiso general. Un
// SUSPECT no tiene corroboración independiente y una severidad fuera de
// política no la pidió nadie.
func TestAuthorizeFix_SoloConfirmadosYSeveridadAdmitida(t *testing.T) {
	review := revisionConPolitica(2)

	casos := []struct {
		nombre    string
		hallazgo  ConsensusFinding
		explicita bool
		quiero    bool
	}{
		{"confirmado crítico entra", ConsensusFinding{Status: ConsensusConfirmed, Severity: SeverityCritical}, false, true},
		{"confirmado alto entra", ConsensusFinding{Status: ConsensusConfirmed, Severity: SeverityHigh}, false, true},
		{"sospechoso nunca entra", ConsensusFinding{Status: ConsensusSuspect, Severity: SeverityCritical}, false, false},
		{"contradicción nunca entra", ConsensusFinding{Status: ConsensusContradiction, Severity: SeverityHigh}, false, false},
		{"confirmado medio queda fuera de política", ConsensusFinding{Status: ConsensusConfirmed, Severity: SeverityMedium}, false, false},
		{"confirmado medio con autorización explícita entra", ConsensusFinding{Status: ConsensusConfirmed, Severity: SeverityMedium}, true, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := AuthorizeFix(review, c.hallazgo, c.explicita)
			if c.quiero && err != nil {
				t.Fatalf("AuthorizeFix() = %v, se esperaba autorización", err)
			}
			if !c.quiero && err == nil {
				t.Fatal("AuthorizeFix() autorizó lo que debía rechazar")
			}
		})
	}
}

// TestAuthorizeFix_LaAutorizacionExplicitaNoLevantaLaCorroboracion: la bandera
// de autorización explícita amplía la POLÍTICA de severidad, no la invariante.
// Un SUSPECT sigue sin poder corregirse aunque alguien lo pida (INV-005).
func TestAuthorizeFix_LaAutorizacionExplicitaNoLevantaLaCorroboracion(t *testing.T) {
	review := revisionConPolitica(2)
	sospechoso := ConsensusFinding{Status: ConsensusSuspect, Severity: SeverityCritical}

	if err := AuthorizeFix(review, sospechoso, true); err == nil {
		t.Fatal("un SUSPECT se autorizó con la bandera explícita: la corroboración no es negociable")
	}
}

// TestNextFixRound_NoExcedeElPresupuesto cubre INV-009: el presupuesto no se
// puede agotar «por accidente» ni ampliar desde la entrada.
func TestNextFixRound_NoExcedeElPresupuesto(t *testing.T) {
	review := revisionConPolitica(2)

	if ronda, err := review.NextFixRound(0); err != nil || ronda != 1 {
		t.Fatalf("primera ronda = (%d, %v), se esperaba (1, nil)", ronda, err)
	}
	if ronda, err := review.NextFixRound(1); err != nil || ronda != 2 {
		t.Fatalf("segunda ronda = (%d, %v), se esperaba (2, nil)", ronda, err)
	}
	if _, err := review.NextFixRound(2); err == nil {
		t.Fatal("la tercera ronda se aceptó con max_fix_rounds=2")
	}
}

// TestNextFixRound_PresupuestoSinConfigurarUsaElDefecto: una revisión sin
// política explícita no puede quedar sin techo — sería un bucle infinito
// silencioso, que es justo lo que INV-009 existe para impedir.
func TestNextFixRound_PresupuestoSinConfigurarUsaElDefecto(t *testing.T) {
	sinPolitica := Review{}

	if _, err := sinPolitica.NextFixRound(DefaultMaxFixRounds); err == nil {
		t.Fatalf("sin MaxFixRounds configurado no se aplicó el defecto de %d", DefaultMaxFixRounds)
	}
	if ronda, err := sinPolitica.NextFixRound(0); err != nil || ronda != 1 {
		t.Fatalf("primera ronda sin política = (%d, %v), se esperaba (1, nil)", ronda, err)
	}
}
