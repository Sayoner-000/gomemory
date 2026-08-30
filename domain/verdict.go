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
	// no haber mirado.
	//
	// La condición NO es «no hay ninguna fila de consenso». Esa fue la primera
	// versión y dejaba pasar el caso que de verdad importa: con una fila inocua
	// —un SUSPECT cualquiera— un hallazgo HIGH omitido de la clasificación se
	// volvía invisible al veredicto y la revisión salía APPROVED con el defecto
	// intacto. Lo que se comprueba es que TODO hallazgo fuente esté respaldado por
	// alguna clasificación (FR-004).
	//
	// Una revisión donde ambos revisores no encontraron nada sigue aprobando
	// legítimamente sin ninguna entrada de consenso: no hay nada que clasificar.
	if hayFuentesSinClasificar(results, consensus) {
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
		if finding.Status == ConsensusConfirmed && !finding.ResueltoEn(review.Round) {
			pendienteSevero = true
		}
	}
	if !pendienteSevero {
		return VerdictApproved
	}

	// Una revisión de solo lectura con un defecto severo abierto NO puede quedarse
	// esperando: su alcance prohíbe la corrección que la desbloquearía.
	//
	// Es el defecto reproducido en acr_96710834: con dos resultados success,
	// consenso persistido y un CONFIRMED HIGH, review_finalize devolvía "review is
	// not ready to finalize" y dejaba la revisión en consensus_ready para siempre,
	// porque el presupuesto de rondas permitía una corrección que nadie estaba
	// autorizado a hacer. El estado era irrecuperable sin editar la base a mano
	// (FR-019).
	if !review.FixAuthorized {
		return VerdictEscalated
	}

	if _, err := review.NextFixRound(len(fixes)); err != nil {
		return VerdictEscalated
	}
	return ""
}

// hayFuentesSinClasificar indica si algún hallazgo reportado por los revisores no
// aparece como fuente de ninguna clasificación de consenso.
//
// BuildConsensus ya impide crear una clasificación incompleta, pero el veredicto no
// puede confiar en eso: se deriva de lo PERSISTIDO, y el ledger puede contener rondas
// escritas por una versión anterior que sí lo permitía. Fallar cerrado aquí es lo que
// hace que esas revisiones no puedan aprobarse por accidente.
func hayFuentesSinClasificar(results []ReviewerResult, consensus []ConsensusFinding) bool {
	clasificados := map[int64]bool{}
	for _, finding := range consensus {
		for _, id := range finding.SourceFindingIDs {
			clasificados[id] = true
		}
	}
	for _, result := range results {
		for _, finding := range result.Findings {
			if !clasificados[finding.ID] {
				return true
			}
		}
	}
	return false
}
