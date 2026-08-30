package domain

import "testing"

// fuentes construye el conjunto de hallazgos de una ronda con dos revisores.
func fuentes() []ConsensusSource {
	return []ConsensusSource{
		{FindingID: 1, Reviewer: ReviewerA, Severity: SeverityHigh, Claim: "race", Confirmable: true},
		{FindingID: 2, Reviewer: ReviewerA, Severity: SeverityMedium, Claim: "único", Confirmable: true},
		{FindingID: 3, Reviewer: ReviewerB, Severity: SeverityHigh, Claim: "race equivalente", Confirmable: true},
		{FindingID: 4, Reviewer: ReviewerB, Severity: SeverityLow, Claim: "estilo", Confirmable: true},
	}
}

func clasificacionCompleta() ConsensusClassification {
	return ConsensusClassification{
		Matches: []ConsensusPair{
			{Status: ConsensusConfirmed, FindingIDA: 1, FindingIDB: 3, Claim: "race"},
		},
		Unmatched: []ConsensusSingle{
			{Status: ConsensusSuspect, FindingID: 2},
			{Status: ConsensusInfo, FindingID: 4},
		},
	}
}

// TestValidateCoverage_ExigeClasificarTodo es la regresión del defecto que motivó la
// funcionalidad: BuildConsensus validaba hallazgo a hallazgo pero nunca el conjunto,
// así que omitir un HIGH de la clasificación no producía ningún error.
func TestValidateCoverage_ExigeClasificarTodo(t *testing.T) {
	parcial := ConsensusClassification{
		Matches:   clasificacionCompleta().Matches,
		Unmatched: []ConsensusSingle{{Status: ConsensusSuspect, FindingID: 2}},
	}
	_, err := ValidateCoverage(fuentes(), parcial)
	if err == nil {
		t.Fatal("una clasificación que deja un hallazgo fuera debe rechazarse")
	}
	if !contiene(err.Error(), "sin clasificar") {
		t.Errorf("el error debe nombrar los hallazgos sin clasificar: %v", err)
	}

	if _, err := ValidateCoverage(fuentes(), clasificacionCompleta()); err != nil {
		t.Fatalf("una clasificación completa fue rechazada: %v", err)
	}
}

func TestValidateCoverage_RechazaDuplicadosYCruces(t *testing.T) {
	casos := map[string]ConsensusClassification{
		"id repetido en unmatched": {
			Matches: []ConsensusPair{{Status: ConsensusConfirmed, FindingIDA: 1, FindingIDB: 3}},
			Unmatched: []ConsensusSingle{
				{Status: ConsensusSuspect, FindingID: 2},
				{Status: ConsensusInfo, FindingID: 2},
				{Status: ConsensusInfo, FindingID: 4},
			},
		},
		"id emparejado y no emparejado a la vez": {
			Matches: []ConsensusPair{{Status: ConsensusConfirmed, FindingIDA: 1, FindingIDB: 3}},
			Unmatched: []ConsensusSingle{
				{Status: ConsensusSuspect, FindingID: 1},
				{Status: ConsensusInfo, FindingID: 2},
				{Status: ConsensusInfo, FindingID: 4},
			},
		},
		"pareja consigo misma": {
			Matches: []ConsensusPair{{Status: ConsensusConfirmed, FindingIDA: 1, FindingIDB: 1}},
			Unmatched: []ConsensusSingle{
				{Status: ConsensusSuspect, FindingID: 2},
				{Status: ConsensusInfo, FindingID: 3},
				{Status: ConsensusInfo, FindingID: 4},
			},
		},
		"desconocido o de otra ronda": {
			Matches: []ConsensusPair{{Status: ConsensusConfirmed, FindingIDA: 1, FindingIDB: 99}},
			Unmatched: []ConsensusSingle{
				{Status: ConsensusSuspect, FindingID: 2},
				{Status: ConsensusInfo, FindingID: 3},
				{Status: ConsensusInfo, FindingID: 4},
			},
		},
		"dos fuentes del mismo revisor": {
			Matches: []ConsensusPair{{Status: ConsensusConfirmed, FindingIDA: 1, FindingIDB: 2}},
			Unmatched: []ConsensusSingle{
				{Status: ConsensusSuspect, FindingID: 3},
				{Status: ConsensusInfo, FindingID: 4},
			},
		},
		"emparejado con estado de no emparejado": {
			Matches: []ConsensusPair{{Status: ConsensusSuspect, FindingIDA: 1, FindingIDB: 3}},
			Unmatched: []ConsensusSingle{
				{Status: ConsensusSuspect, FindingID: 2},
				{Status: ConsensusInfo, FindingID: 4},
			},
		},
		"no emparejado con estado de emparejado": {
			Matches: []ConsensusPair{{Status: ConsensusConfirmed, FindingIDA: 1, FindingIDB: 3}},
			Unmatched: []ConsensusSingle{
				{Status: ConsensusConfirmed, FindingID: 2},
				{Status: ConsensusInfo, FindingID: 4},
			},
		},
	}
	for nombre, clasificacion := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := ValidateCoverage(fuentes(), clasificacion); err == nil {
				t.Error("se aceptó una clasificación inválida")
			}
		})
	}
}

// TestValidateCoverage_DerivaLaSeveridad cubre el segundo defecto confirmado: la
// severidad venía del orquestador, así que un HIGH corroborado por ambos revisores
// podía persistirse como LOW y desaparecer del veredicto.
func TestValidateCoverage_DerivaLaSeveridad(t *testing.T) {
	clasificacion := clasificacionCompleta()
	clasificacion.Matches[0].DeclaredSeverity = ""
	resultado, err := ValidateCoverage(fuentes(), clasificacion)
	if err != nil {
		t.Fatal(err)
	}
	if resultado[0].Severity != SeverityHigh {
		t.Errorf("severidad derivada = %s, se esperaba HIGH (el máximo de sus fuentes)", resultado[0].Severity)
	}

	degradada := clasificacionCompleta()
	degradada.Matches[0].DeclaredSeverity = SeverityLow
	if _, err := ValidateCoverage(fuentes(), degradada); err == nil {
		t.Fatal("declarar una severidad menor que la derivada debe rechazarse")
	}

	coincidente := clasificacionCompleta()
	coincidente.Matches[0].DeclaredSeverity = SeverityHigh
	if _, err := ValidateCoverage(fuentes(), coincidente); err != nil {
		t.Fatalf("declarar la severidad correcta no puede fallar: %v", err)
	}
}

func TestValidateCoverage_ConfirmadoExigeEvidenciaDeAmbos(t *testing.T) {
	origen := fuentes()
	origen[2].Confirmable = false
	if _, err := ValidateCoverage(origen, clasificacionCompleta()); err == nil {
		t.Fatal("un CONFIRMED sin evidencia concreta de ambos revisores debe rechazarse")
	}
}

// TestValidateCoverage_IdentificadoresEstables comprueba que el mismo conjunto
// produce siempre los mismos consensus_local_id, independientemente del orden de
// llegada. Sin esto, reenviar la clasificación reasignaba identificadores y rompía
// las referencias que ya habían guardado las correcciones y los re-juicios.
func TestValidateCoverage_IdentificadoresEstables(t *testing.T) {
	primera, err := ValidateCoverage(fuentes(), clasificacionCompleta())
	if err != nil {
		t.Fatal(err)
	}
	desordenada := ConsensusClassification{
		Matches: clasificacionCompleta().Matches,
		Unmatched: []ConsensusSingle{
			{Status: ConsensusInfo, FindingID: 4},
			{Status: ConsensusSuspect, FindingID: 2},
		},
	}
	segunda, err := ValidateCoverage(fuentes(), desordenada)
	if err != nil {
		t.Fatal(err)
	}
	if len(primera) != len(segunda) {
		t.Fatalf("distinto número de clasificaciones: %d vs %d", len(primera), len(segunda))
	}
	for i := range primera {
		if primera[i].ConsensusLocalID != segunda[i].ConsensusLocalID ||
			primera[i].Severity != segunda[i].Severity {
			t.Errorf("clasificación %d no es estable: %#v vs %#v", i, primera[i], segunda[i])
		}
	}
	if primera[0].ConsensusLocalID != "C-001" {
		t.Errorf("el primer identificador debe ser C-001, fue %s", primera[0].ConsensusLocalID)
	}
}

// TestClassificationFingerprint distingue clasificaciones equivalentes de divergentes.
func TestClassificationFingerprint(t *testing.T) {
	primera, _ := ValidateCoverage(fuentes(), clasificacionCompleta())
	desordenada := ConsensusClassification{
		Matches: clasificacionCompleta().Matches,
		Unmatched: []ConsensusSingle{
			{Status: ConsensusInfo, FindingID: 4},
			{Status: ConsensusSuspect, FindingID: 2},
		},
	}
	segunda, _ := ValidateCoverage(fuentes(), desordenada)
	if ClassificationFingerprint(primera) != ClassificationFingerprint(segunda) {
		t.Error("el mismo conjunto en otro orden debe producir la misma huella")
	}

	distinta := ConsensusClassification{
		Matches: clasificacionCompleta().Matches,
		Unmatched: []ConsensusSingle{
			{Status: ConsensusInfo, FindingID: 2},
			{Status: ConsensusInfo, FindingID: 4},
		},
	}
	tercera, err := ValidateCoverage(fuentes(), distinta)
	if err != nil {
		t.Fatal(err)
	}
	if ClassificationFingerprint(primera) == ClassificationFingerprint(tercera) {
		t.Error("reclasificar un SUSPECT como INFO debe cambiar la huella")
	}
}

func contiene(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
