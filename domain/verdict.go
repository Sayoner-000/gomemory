package domain

type Verdict string

const (
	VerdictApproved   Verdict = "APPROVED"
	VerdictEscalated  Verdict = "ESCALATED"
	VerdictIncomplete Verdict = "INCOMPLETE"
)

func (v Verdict) Valid() bool {
	return v == VerdictApproved || v == VerdictEscalated || v == VerdictIncomplete
}

type ReJudgmentState string

const (
	ReJudgmentResolved   ReJudgmentState = "RESOLVED"
	ReJudgmentUnresolved ReJudgmentState = "UNRESOLVED"
	ReJudgmentRegressed  ReJudgmentState = "REGRESSED"
)

func (s ReJudgmentState) Valid() bool {
	return s == ReJudgmentResolved || s == ReJudgmentUnresolved || s == ReJudgmentRegressed
}

// DeriveVerdict calcula el estado terminal de una revisión a partir de lo
// PERSISTIDO. Es pura y es el punto que la especificación exige que no dependa
// del prompt (§44): ningún llamador puede pasar un veredicto, solo hechos.
//
// Devuelve la cadena vacía —que no es un Verdict válido— cuando la revisión
// sigue viva: hay defectos severos por resolver y presupuesto para intentarlo.
// Esa distinción es deliberada; FinalizeReview la traduce en «aún no se puede
// finalizar» en vez de inventar un terminal.
//
// Orden de precedencia, y no es arbitrario:
//  1. INCOMPLETE manda sobre todo (INV-010). Si un revisor falló, no sabemos lo
//     suficiente para aprobar NI para escalar: escalar afirmaría que el
//     protocolo se ejecutó y encontró algo irresoluble, y no es el caso.
//  2. Una contradicción severa escala de inmediato, sin gastar rondas: los dos
//     revisores discrepan sobre el mismo comportamiento, así que no hay defecto
//     acordado que corregir — corregir «algo» sería elegir un bando a ciegas.
//  3. Con defectos severos sin resolver: ESCALATED si el presupuesto se agotó,
//     abierto si aún queda ronda.
func DeriveVerdict(
	review Review,
	results []ReviewerResult,
	consensus []ConsensusFinding,
	fixes []FixDelta,
) Verdict {
	seen := map[Reviewer]bool{}
	for _, result := range results {
		if result.Status == ReviewerResultFailure {
			return VerdictIncomplete
		}
		if result.Status == ReviewerResultSuccess {
			seen[result.Reviewer] = true
		}
	}
	if !seen[ReviewerA] || !seen[ReviewerB] {
		return VerdictIncomplete
	}

	// Sin consenso no hay veredicto posible: aprobar aquí no sería decidir, sería
	// no haber mirado. Se detectó en una prueba de punta a punta donde el paso de
	// consenso falló, nadie lo notó y la revisión salió APPROVED «con 0
	// hallazgos» pese a que ambos revisores habían reportado un HIGH.
	//
	// La condición es «hay hallazgos que clasificar y nadie los clasificó», no
	// «faltan hallazgos de consenso»: una revisión donde ambos revisores no
	// encontraron nada aprueba legítimamente sin ninguna entrada de consenso.
	if len(consensus) == 0 && hayHallazgosSinClasificar(results) {
		return ""
	}

	pendienteSevero := false
	for _, finding := range consensus {
		if !finding.Severity.Severe() {
			continue
		}
		if finding.Status == ConsensusContradiction {
			return VerdictEscalated
		}
		if finding.Status == ConsensusConfirmed && finding.RejudgmentState != ReJudgmentResolved {
			pendienteSevero = true
		}
	}
	if !pendienteSevero {
		return VerdictApproved
	}

	if _, err := review.NextFixRound(len(fixes)); err != nil {
		return VerdictEscalated
	}
	return ""
}

// hayHallazgosSinClasificar indica si algún revisor reportó hallazgos.
func hayHallazgosSinClasificar(results []ReviewerResult) bool {
	for _, result := range results {
		if len(result.Findings) > 0 {
			return true
		}
	}
	return false
}
