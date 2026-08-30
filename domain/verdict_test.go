package domain

import "testing"

func TestDeriveVerdictApprovedAndIncomplete(t *testing.T) {
	review := Review{MaxFixRounds: 2, FixAuthorized: true}
	tests := []struct {
		name      string
		results   []ReviewerResult
		consensus []ConsensusFinding
		want      Verdict
	}{
		{
			name: "sin confirmed severo queda aprobado",
			results: []ReviewerResult{
				{Reviewer: ReviewerA, Status: ReviewerResultSuccess},
				{Reviewer: ReviewerB, Status: ReviewerResultSuccess},
			},
			consensus: []ConsensusFinding{{Status: ConsensusSuspect, Severity: SeverityHigh}},
			want:      VerdictApproved,
		},
		{
			name: "fallo de un revisor queda incompleto",
			results: []ReviewerResult{
				{Reviewer: ReviewerA, Status: ReviewerResultSuccess},
				{Reviewer: ReviewerB, Status: ReviewerResultFailure},
			},
			want: VerdictIncomplete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveVerdict(review, tt.results, tt.consensus, nil); got != tt.want {
				t.Fatalf("DeriveVerdict() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestDeriveVerdictEscalated cubre las salidas de US2. Un veredicto vacío
// significa «aún hay trabajo»: la revisión sigue viva y admite otra ronda. Solo
// cuando el presupuesto se agota, o cuando ninguna ronda podría resolverlo, el
// estado pasa a ESCALATED — que es la forma de decir «esto lo decide una
// persona», no una variante de fallo.
func TestDeriveVerdictEscalated(t *testing.T) {
	ambosOK := []ReviewerResult{
		{Reviewer: ReviewerA, Status: ReviewerResultSuccess},
		{Reviewer: ReviewerB, Status: ReviewerResultSuccess},
	}
	confirmadoSevero := ConsensusFinding{Status: ConsensusConfirmed, Severity: SeverityHigh}
	regresado := ConsensusFinding{
		Status: ConsensusConfirmed, Severity: SeverityCritical, RejudgmentState: ReJudgmentRegressed,
	}
	contradiccionSevera := ConsensusFinding{Status: ConsensusContradiction, Severity: SeverityCritical}

	tests := []struct {
		name      string
		review    Review
		consensus []ConsensusFinding
		fixes     []FixDelta
		want      Verdict
	}{
		{
			name:      "confirmado severo sin resolver y con presupuesto queda abierto",
			review:    Review{MaxFixRounds: 2, FixAuthorized: true},
			consensus: []ConsensusFinding{confirmadoSevero},
			want:      "",
		},
		{
			name:      "presupuesto agotado con el defecto sin resolver escala",
			review:    Review{MaxFixRounds: 2, FixAuthorized: true},
			consensus: []ConsensusFinding{confirmadoSevero},
			fixes:     []FixDelta{{Round: 1}, {Round: 2}},
			want:      VerdictEscalated,
		},
		{
			name:      "una regresión con presupuesto agotado escala",
			review:    Review{MaxFixRounds: 1, FixAuthorized: true},
			consensus: []ConsensusFinding{regresado},
			fixes:     []FixDelta{{Round: 1}},
			want:      VerdictEscalated,
		},
		{
			name:      "contradicción severa escala sin gastar rondas",
			review:    Review{MaxFixRounds: 2, FixAuthorized: true},
			consensus: []ConsensusFinding{contradiccionSevera},
			want:      VerdictEscalated,
		},
		{
			name:      "resuelto tras una ronda aprueba",
			review:    Review{MaxFixRounds: 2, FixAuthorized: true},
			consensus: []ConsensusFinding{{Status: ConsensusConfirmed, Severity: SeverityHigh, RejudgmentState: ReJudgmentResolved}},
			fixes:     []FixDelta{{Round: 1}},
			want:      VerdictApproved,
		},
		{
			name:      "un fallo de revisor manda sobre el presupuesto agotado",
			review:    Review{MaxFixRounds: 1, FixAuthorized: true},
			consensus: []ConsensusFinding{confirmadoSevero},
			fixes:     []FixDelta{{Round: 1}},
			want:      VerdictIncomplete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := ambosOK
			if tt.want == VerdictIncomplete {
				results = []ReviewerResult{
					{Reviewer: ReviewerA, Status: ReviewerResultSuccess},
					{Reviewer: ReviewerB, Status: ReviewerResultFailure},
				}
			}
			if got := DeriveVerdict(tt.review, results, tt.consensus, tt.fixes); got != tt.want {
				t.Fatalf("DeriveVerdict() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDeriveVerdict_NoApruebaSinConsenso cubre el agujero que destapó la prueba
// de punta a punta: ambos revisores reportaron un HIGH, el paso de consenso
// nunca se ejecutó, y la revisión salió APPROVED con «0 hallazgos».
//
// Sin consenso no hay nada que decir sobre esos hallazgos: no se sabe si son el
// mismo defecto confirmado o dos sospechas sueltas. Aprobar ahí no es un
// veredicto, es no haber mirado — y es el fallo más peligroso posible en una
// funcionalidad cuyo propósito es no aprobar defectos.
func TestDeriveVerdict_NoApruebaSinConsenso(t *testing.T) {
	conHallazgos := []ReviewerResult{
		{Reviewer: ReviewerA, Status: ReviewerResultSuccess, Findings: []Finding{
			{LocalID: "A-001", Severity: SeverityHigh},
		}},
		{Reviewer: ReviewerB, Status: ReviewerResultSuccess, Findings: []Finding{
			{LocalID: "B-004", Severity: SeverityHigh},
		}},
	}
	if got := DeriveVerdict(Review{MaxFixRounds: 2, FixAuthorized: true}, conHallazgos, nil, nil); got == VerdictApproved {
		t.Fatal("se aprobó una revisión con hallazgos y sin consenso calculado")
	}

	// Sin hallazgos que clasificar, la ausencia de consenso es legítima.
	sinHallazgos := []ReviewerResult{
		{Reviewer: ReviewerA, Status: ReviewerResultSuccess},
		{Reviewer: ReviewerB, Status: ReviewerResultSuccess},
	}
	if got := DeriveVerdict(Review{MaxFixRounds: 2, FixAuthorized: true}, sinHallazgos, nil, nil); got != VerdictApproved {
		t.Fatalf("dos revisores sin hallazgos deben aprobar, se obtuvo %q", got)
	}
}

// TestDeriveVerdict_NoApruebaConFuentesSinClasificar es la regresión exacta del
// defecto CONFIRMED HIGH de la revisión acr_96710834: la comprobación anterior solo
// fallaba cerrado cuando NO existía ninguna fila de consenso. Bastaba una fila
// inocua —un SUSPECT cualquiera— para que un HIGH omitido de la clasificación fuera
// invisible al veredicto y la revisión saliera APPROVED.
func TestDeriveVerdict_NoApruebaConFuentesSinClasificar(t *testing.T) {
	results := []ReviewerResult{
		{Reviewer: ReviewerA, Status: ReviewerResultSuccess, Findings: []Finding{
			{ID: 1, LocalID: "A-001", Severity: SeverityHigh},
			{ID: 2, LocalID: "A-002", Severity: SeverityHigh},
		}},
		{Reviewer: ReviewerB, Status: ReviewerResultSuccess, Findings: []Finding{
			{ID: 3, LocalID: "B-001", Severity: SeverityHigh},
		}},
	}
	// Una única fila de consenso que solo cubre A-001 y B-001: A-002 queda fuera.
	parcial := []ConsensusFinding{
		{Status: ConsensusSuspect, Severity: SeverityLow, SourceFindingIDs: []int64{1}},
		{Status: ConsensusSuspect, Severity: SeverityLow, SourceFindingIDs: []int64{3}},
	}
	if got := DeriveVerdict(Review{MaxFixRounds: 2, FixAuthorized: true}, results, parcial, nil); got == VerdictApproved {
		t.Fatal("se aprobó una revisión con un hallazgo HIGH sin clasificar")
	}

	completa := append(parcial, ConsensusFinding{
		Status: ConsensusSuspect, Severity: SeverityLow, SourceFindingIDs: []int64{2},
	})
	if got := DeriveVerdict(Review{MaxFixRounds: 2, FixAuthorized: true}, results, completa, nil); got != VerdictApproved {
		t.Fatalf("con todo clasificado y sin confirmados severos debe aprobar, se obtuvo %q", got)
	}
}

// TestDeriveVerdict_SoloLecturaEscalaEnLugarDeBloquearse es la regresión exacta del
// bloqueo observado en acr_96710834-8273-49f3-bd11-42764b2f11d4 (FR-019).
func TestDeriveVerdict_SoloLecturaEscalaEnLugarDeBloquearse(t *testing.T) {
	results := []ReviewerResult{
		{Reviewer: ReviewerA, Status: ReviewerResultSuccess, Findings: []Finding{{ID: 1, Severity: SeverityHigh}}},
		{Reviewer: ReviewerB, Status: ReviewerResultSuccess, Findings: []Finding{{ID: 2, Severity: SeverityHigh}}},
	}
	consensus := []ConsensusFinding{{
		Status: ConsensusConfirmed, Severity: SeverityHigh,
		SourceFindingIDs: []int64{1, 2}, RejudgmentState: ReJudgmentUnresolved,
	}}

	soloLectura := Review{MaxFixRounds: 2, FixAuthorized: false}
	if got := DeriveVerdict(soloLectura, results, consensus, nil); got != VerdictEscalated {
		t.Fatalf("una revisión de solo lectura con HIGH confirmado debe escalar, se obtuvo %q", got)
	}

	// El contraste que da valor al caso de arriba: con corrección autorizada y
	// presupuesto disponible, la revisión sigue abierta. Si también escalara aquí,
	// el verde anterior no probaría nada.
	autorizada := Review{MaxFixRounds: 2, FixAuthorized: true}
	if got := DeriveVerdict(autorizada, results, consensus, nil); got != "" {
		t.Fatalf("con rondas disponibles la revisión sigue abierta, se obtuvo %q", got)
	}
}

// TestDeriveVerdict_PresupuestoAgotadoEscala cubre FR-020.
func TestDeriveVerdict_PresupuestoAgotadoEscala(t *testing.T) {
	results := []ReviewerResult{
		{Reviewer: ReviewerA, Status: ReviewerResultSuccess, Findings: []Finding{{ID: 1, Severity: SeverityHigh}}},
		{Reviewer: ReviewerB, Status: ReviewerResultSuccess, Findings: []Finding{{ID: 2, Severity: SeverityHigh}}},
	}
	consensus := []ConsensusFinding{{
		Status: ConsensusConfirmed, Severity: SeverityHigh,
		SourceFindingIDs: []int64{1, 2}, RejudgmentState: ReJudgmentUnresolved,
	}}
	fixes := []FixDelta{{Round: 1}, {Round: 2}}

	review := Review{MaxFixRounds: 2, FixAuthorized: true}
	if got := DeriveVerdict(review, results, consensus, fixes); got != VerdictEscalated {
		t.Fatalf("agotado el presupuesto con un defecto severo abierto debe escalar, se obtuvo %q", got)
	}
}
