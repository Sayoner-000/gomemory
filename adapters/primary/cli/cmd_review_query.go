package cli

import (
	"fmt"
	"strconv"
	"strings"

	"mem/application/usecases"
	"mem/domain"
)

// cmdReviewStatus implementa `mem review status [<review-id>]`.
//
// Sin argumento resuelve la revisión ABIERTA del proyecto. Deliberadamente no
// cae en «la más reciente»: si la última terminó, decir su veredicto ante un
// `status` a secas se leería como el estado de un trabajo en curso, que es
// justo la confusión que esta consulta debe evitar.
func cmdReviewStatus(deps *Deps, args []string) {
	var review *domain.Review
	if len(args) > 0 {
		review = mustGetReview(deps, args[0])
	} else {
		review = activeReview(deps)
		if review == nil {
			fmt.Println("No hay ninguna revisión abierta en este proyecto.")
			fmt.Println("Usa `mem review history` para ver las cerradas.")
			return
		}
	}

	fmt.Printf("%s  ·  %s %s\n", review.ID, review.Target.Type, review.Target.Revision)
	if review.Status.Terminal() {
		fmt.Printf("veredicto: %s\n", review.Verdict)
	} else {
		fmt.Printf("etapa: %s (ronda %d de %d)\n", review.Status, review.Round, review.MaxFixRounds)
	}
	fmt.Printf("alcance: %s\n", alcanceDeRevision(review.FixAuthorized))
	resumirHallazgos(deps, review.ID, review.Round)
}

// cmdReviewHistory implementa `mem review history [--limit N]`.
func cmdReviewHistory(deps *Deps, args []string) {
	limit := 20
	if len(args) >= 2 && args[0] == "--limit" {
		n, err := strconv.Atoi(args[1])
		if err != nil || n <= 0 {
			fail("--limit requiere un entero positivo, se recibió %q", args[1])
		}
		limit = n
	} else if len(args) > 0 {
		fail("uso: mem review history [--limit N]")
	}

	reviews, err := deps.ReviewRepo.ListReviews(deps.Project, limit)
	if err != nil {
		fail("listar revisiones: %v", err)
	}
	if len(reviews) == 0 {
		fmt.Println("Este proyecto no tiene revisiones todavía.")
		return
	}
	for _, review := range reviews {
		estado := string(review.Status)
		if review.Status.Terminal() {
			estado = string(review.Verdict)
		}
		fmt.Printf("%-16s  %-12s  %-18s  %s\n",
			review.ID, review.Target.Type, estado, review.Target.Revision)
	}
}

// cmdReviewShow implementa `mem review show <review-id>`: el linaje completo de
// data-model.md, que es lo que permite reconstruir qué se revisó, qué se
// detectó, qué corrección se intentó y cómo acabó.
func cmdReviewShow(deps *Deps, args []string) {
	if len(args) != 1 {
		fail("uso: mem review show <review-id>")
	}
	review := mustGetReview(deps, args[0])

	fmt.Printf("# Revisión %s\n\n", review.ID)
	fmt.Println("## Target")
	fmt.Printf("- tipo: %s\n- revisión: %s\n- digest original: %s\n",
		review.Target.Type, review.Target.Revision, review.Target.Digest())
	if vigente := review.ActiveTargetDigest(); vigente != review.Target.Digest() {
		fmt.Printf("- digest vigente: %s (ronda %d)\n", vigente, review.Round)
	}
	if len(review.Target.Scope) > 0 {
		fmt.Printf("- alcance: %s\n", strings.Join(review.Target.Scope, ", "))
	}
	fmt.Printf("- independencia: %s", review.IndependenceLevel)
	if review.IndependenceReason != "" {
		fmt.Printf(" (%s)", review.IndependenceReason)
	}
	fmt.Println()
	fmt.Printf("\n## Política\n- alcance: %s\n- rondas máximas: %d\n- severidades corregibles: %s\n",
		alcanceDeRevision(review.FixAuthorized), review.MaxFixRounds,
		severidadesLegibles(review.AutoFixSeverities))

	fmt.Println("\n## Revisores")
	huboAlguno := false
	for ronda := 0; ronda <= review.Round; ronda++ {
		results, err := deps.ReviewRepo.ListReviewerResults(deps.Project, review.ID, ronda)
		if err != nil {
			fail("listar resultados de revisor: %v", err)
		}
		for _, result := range results {
			huboAlguno = true
			fmt.Printf("- ronda %d · revisor %s · %s · %d hallazgo(s)\n",
				ronda, result.Reviewer, result.Status, len(result.Findings))
		}
	}
	if !huboAlguno {
		fmt.Println("- (ningún resultado de revisor todavía)")
	}

	fmt.Println("\n## Consenso")
	findings, err := deps.ConsensusRepo.ListAllConsensusFindings(deps.Project, review.ID)
	if err != nil {
		fail("listar consenso: %v", err)
	}
	if len(findings) == 0 {
		fmt.Println("- (sin consenso calculado)")
	}
	fixesPorHallazgo := map[string]int{}
	deltas, err := deps.ConsensusRepo.ListFixDeltas(deps.Project, review.ID)
	if err != nil {
		fail("listar correcciones: %v", err)
	}
	for _, delta := range deltas {
		for _, localID := range delta.AddressedConsensusIDs {
			fixesPorHallazgo[localID] = delta.Round
		}
	}
	for _, finding := range findings {
		fmt.Printf("- %s · %s · %s", finding.ConsensusLocalID, finding.Status, finding.Severity)
		if len(finding.SourceFindingIDs) > 0 {
			fmt.Printf(" · fuentes %s", idsLegibles(finding.SourceFindingIDs))
		}
		if ronda, ok := fixesPorHallazgo[finding.ConsensusLocalID]; ok {
			fmt.Printf(" · corregido en ronda %d", ronda)
		} else if finding.Status == domain.ConsensusConfirmed {
			fmt.Print(" · sin corregir")
		}
		fmt.Println()
		// El linaje por revisor es la mitad de la auditoría: sin él no se puede
		// saber si un RESOLVED lo respaldan dos revisores o uno solo (FR-023).
		porRevisor, err := usecases.ReJudgmentsByReviewer(
			deps.ConsensusRepo, deps.Project, review.ID, finding.ConsensusLocalID, review.Round)
		if err != nil {
			fail("listar re-juicios: %v", err)
		}
		if len(porRevisor) > 0 || finding.RejudgmentState != "" {
			// Con el MISMO criterio de vigencia que el veredicto: mostrar la columna a
			// secas anunciaba como resuelto lo que la finalización cuenta como pendiente.
			vigente := finding.EstadoVigente(review.Round)
			fmt.Printf("  re-juicio  A=%s  B=%s  ·  agregado: %s\n",
				estadoOGuion(porRevisor[domain.ReviewerA]),
				estadoOGuion(porRevisor[domain.ReviewerB]),
				estadoOGuion(vigente))
			if vigente == "" && finding.RejudgmentState != "" {
				fmt.Printf("             verificado en la ronda %d; la ronda vigente es la %d y exige volver a verificarlo\n",
					finding.RejudgmentRound, review.Round)
			}
		}
		if finding.Claim != "" {
			fmt.Printf("  %s\n", finding.Claim)
		}
	}

	fmt.Println("\n## Correcciones")
	fixes := deltas
	if len(fixes) == 0 {
		fmt.Println("- (ninguna)")
	}
	for _, fix := range fixes {
		fmt.Printf("- ronda %d · %s → %s · aborda %s\n",
			fix.Round, fix.BaseTargetDigest, fix.FixedTargetDigest,
			strings.Join(fix.AddressedConsensusIDs, ", "))
		if len(fix.ModifiedPaths) > 0 {
			fmt.Printf("  rutas: %s\n", strings.Join(fix.ModifiedPaths, ", "))
		}
	}

	fmt.Println("\n## Veredicto")
	if review.Status.Terminal() {
		fmt.Printf("%s\n", review.Verdict)
	} else {
		fmt.Printf("(sin veredicto: la revisión está en %s)\n", review.Status)
	}
}

// mustGetReview resuelve una revisión o termina con un error explícito.
//
// Un identificador inexistente NO puede devolver una revisión vacía: quien
// consulta leería «sin hallazgos» y concluiría que no hay defectos, cuando lo
// que pasa es que se equivocó de identificador.
func mustGetReview(deps *Deps, reviewID string) *domain.Review {
	review, err := deps.ReviewRepo.GetReview(deps.Project, reviewID)
	if err != nil {
		fail("leer la revisión %s: %v", reviewID, err)
	}
	if review == nil {
		fail("la revisión %s no existe en este proyecto (usa `mem review history`)", reviewID)
	}
	return review
}

// activeReview devuelve la revisión abierta más reciente, o nil si todas
// terminaron.
func activeReview(deps *Deps) *domain.Review {
	reviews, err := deps.ReviewRepo.ListReviews(deps.Project, 50)
	if err != nil {
		fail("listar revisiones: %v", err)
	}
	for i := range reviews {
		if !reviews[i].Status.Terminal() {
			return &reviews[i]
		}
	}
	return nil
}

func resumirHallazgos(deps *Deps, reviewID string, ronda int) {
	findings, err := deps.ConsensusRepo.ListAllConsensusFindings(deps.Project, reviewID)
	if err != nil {
		fail("listar consenso: %v", err)
	}
	if len(findings) == 0 {
		return
	}
	porEstado := map[domain.ConsensusStatus]int{}
	porSeveridad := map[domain.Severity]int{}
	porReJuicio := map[domain.ReJudgmentState]int{}
	pendientes := 0
	for _, finding := range findings {
		porEstado[finding.Status]++
		porSeveridad[finding.Severity]++
		// La ronda vigente decide, igual que en el veredicto: un re-juicio de una
		// ronda anterior cuenta como pendiente, no como resuelto.
		if vigente := finding.EstadoVigente(ronda); vigente == "" {
			pendientes++
		} else {
			porReJuicio[vigente]++
		}
	}
	fmt.Printf("hallazgos: %d confirmado(s), %d sospechoso(s), %d contradicción(es), %d informativo(s)\n",
		porEstado[domain.ConsensusConfirmed],
		porEstado[domain.ConsensusSuspect],
		porEstado[domain.ConsensusContradiction],
		porEstado[domain.ConsensusInfo],
	)
	fmt.Printf("severidad: CRITICAL %d · HIGH %d · MEDIUM %d · LOW %d · INFO %d\n",
		porSeveridad[domain.SeverityCritical], porSeveridad[domain.SeverityHigh],
		porSeveridad[domain.SeverityMedium], porSeveridad[domain.SeverityLow],
		porSeveridad[domain.SeverityInfo],
	)
	fmt.Printf("re-juicio: RESOLVED %d · UNRESOLVED %d · REGRESSED %d · PENDIENTE %d\n",
		porReJuicio[domain.ReJudgmentResolved], porReJuicio[domain.ReJudgmentUnresolved],
		porReJuicio[domain.ReJudgmentRegressed], pendientes,
	)
}

// alcanceDeRevision traduce el booleano a algo que un humano pueda leer sin
// consultar el contrato.
func alcanceDeRevision(fixAuthorized bool) string {
	if fixAuthorized {
		return "autorizada a corregir"
	}
	return "solo lectura"
}

func severidadesLegibles(severidades []domain.Severity) string {
	partes := make([]string, 0, len(severidades))
	for _, severidad := range severidades {
		partes = append(partes, string(severidad))
	}
	if len(partes) == 0 {
		return "(ninguna)"
	}
	return strings.Join(partes, ", ")
}

func idsLegibles(ids []int64) string {
	partes := make([]string, 0, len(ids))
	for _, id := range ids {
		partes = append(partes, strconv.FormatInt(id, 10))
	}
	return strings.Join(partes, ", ")
}

func estadoOGuion(estado domain.ReJudgmentState) string {
	if estado == "" {
		return "—"
	}
	return string(estado)
}
