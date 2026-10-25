package review

import (
	"fmt"
	"mem/adapters/primary/cli/clikit"
	"os"
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
func cmdReviewStatus(deps *clikit.Deps, args []string) {
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

	fmt.Printf("%s  ·  %s %s\n", review.ID, review.Target.Type, domain.StripTerminalControls(review.Target.Revision))
	if review.Status.Terminal() {
		fmt.Printf("veredicto: %s\n", review.Verdict)
	} else {
		fmt.Printf("etapa: %s (ronda %d de %d)\n", review.Status, review.Round, review.MaxFixRounds)
	}
	fmt.Printf("alcance: %s\n", alcanceDeRevision(review.FixAuthorized))
	resumirHallazgos(deps, review)
}

// cmdReviewHistory implementa `mem review history [--limit N]`.
func cmdReviewHistory(deps *clikit.Deps, args []string) {
	limit := 20
	if len(args) >= 2 && args[0] == "--limit" {
		n, err := strconv.Atoi(args[1])
		if err != nil || n <= 0 {
			clikit.Fail("--limit requiere un entero positivo, se recibió %q", args[1])
		}
		limit = n
	} else if len(args) > 0 {
		clikit.Fail("uso: mem review history [--limit N]")
	}

	reviews, err := deps.ReviewRepo.ListReviews(deps.Project, limit)
	if err != nil {
		clikit.Fail("listar revisiones: %v", err)
	}
	if len(reviews) == 0 {
		fmt.Println("Este proyecto no tiene revisiones todavía.")
		return
	}
	rows := make([][]string, 0, len(reviews))
	for _, review := range reviews {
		estado := string(review.Status)
		if review.Status.Terminal() {
			estado = string(review.Verdict)
		}
		rows = append(rows, []string{review.ID, string(review.Target.Type), estado, domain.StripTerminalControls(review.Target.Revision)})
	}
	if clikit.PrintListPanel("review history", "Revisiones", fmt.Sprint(len(rows)), []string{"ID", "Target", "Estado", "Revisión"}, rows) {
		return
	}
	for _, r := range rows {
		fmt.Printf("%-16s  %-12s  %-18s  %s\n", r[0], r[1], r[2], r[3])
	}
}

// cmdReviewShow implementa `mem review show <review-id>`: el linaje completo de
// data-model.md, que es lo que permite reconstruir qué se revisó, qué se
// detectó, qué corrección se intentó y cómo acabó.
func cmdReviewShow(deps *clikit.Deps, args []string) {
	if len(args) != 1 {
		clikit.Fail("uso: mem review show <review-id>")
	}
	review := mustGetReview(deps, args[0])
	// Claims, reasons, revisions and paths come from agents: the report is
	// plain text, so the whole of it is written without control sequences.
	out := clikit.TerminalSafe{W: os.Stdout}

	fmt.Fprintf(out, "# Revisión %s\n\n", review.ID)
	fmt.Fprintln(out, "## Target")
	fmt.Fprintf(out, "- tipo: %s\n- revisión: %s\n- digest original: %s\n",
		review.Target.Type, review.Target.Revision, review.Target.Digest())
	if vigente := review.ActiveTargetDigest(); vigente != review.Target.Digest() {
		fmt.Fprintf(out, "- digest vigente: %s (ronda %d)\n", vigente, review.Round)
	}
	if len(review.Target.Scope) > 0 {
		fmt.Fprintf(out, "- alcance: %s\n", strings.Join(review.Target.Scope, ", "))
	}
	fmt.Fprintf(out, "- independencia: %s", review.IndependenceLevel)
	if review.IndependenceReason != "" {
		fmt.Fprintf(out, " (%s)", review.IndependenceReason)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "\n## Política\n- alcance: %s\n- rondas máximas: %d\n- severidades corregibles: %s\n",
		alcanceDeRevision(review.FixAuthorized), review.MaxFixRounds,
		severidadesLegibles(review.AutoFixSeverities))

	estado, err := usecases.BuildReviewStatus(deps.ReviewRepo, deps.ConsensusRepo, deps.Project, review)
	if err != nil {
		clikit.Fail("%v", err)
	}

	// El informe humano muestra el historial de todas las rondas.
	fmt.Fprintln(out, "\n## Revisores")
	huboAlguno := false
	for ronda, results := range estado.ResultsByRound {
		for _, result := range results {
			huboAlguno = true
			fmt.Fprintf(out, "- ronda %d · revisor %s · %s · %d hallazgo(s)\n",
				ronda, result.Reviewer, result.Status, len(result.Findings))
		}
	}
	if !huboAlguno {
		fmt.Fprintln(out, "- (ningún resultado de revisor todavía)")
	}

	fmt.Fprintln(out, "\n## Consenso")
	if len(estado.Findings) == 0 {
		fmt.Fprintln(out, "- (sin consenso calculado)")
	}
	for _, finding := range estado.Findings {
		fmt.Fprintf(out, "- %s · %s · %s", finding.ConsensusLocalID, finding.Status, finding.Severity)
		if len(finding.SourceFindingIDs) > 0 {
			fmt.Fprintf(out, " · fuentes %s", idsLegibles(finding.SourceFindingIDs))
		}
		if finding.Addressed {
			fmt.Fprintf(out, " · corregido en ronda %d", finding.AddressedByRound)
		} else if finding.Status == domain.ConsensusConfirmed {
			fmt.Fprint(out, " · sin corregir")
		}
		fmt.Fprintln(out)
		// El linaje por revisor es la mitad de la auditoría: sin él no se puede
		// saber si un RESOLVED lo respaldan dos revisores o uno solo (FR-023).
		porRevisor := finding.ByReviewer
		if len(porRevisor) > 0 || finding.RejudgmentState != "" {
			vigente := finding.Vigente
			fmt.Fprintf(out, "  re-juicio  A=%s  B=%s  ·  agregado: %s\n",
				estadoOGuion(porRevisor[domain.ReviewerA]),
				estadoOGuion(porRevisor[domain.ReviewerB]),
				estadoOGuion(vigente))
			if vigente == "" && finding.RejudgmentState != "" {
				fmt.Fprintf(out, "             verificado en la ronda %d; la ronda vigente es la %d y exige volver a verificarlo\n",
					finding.RejudgmentRound, review.Round)
			}
		}
		if finding.Claim != "" {
			fmt.Fprintf(out, "  %s\n", finding.Claim)
		}
	}

	fmt.Fprintln(out, "\n## Correcciones")
	fixes := estado.Fixes
	if len(fixes) == 0 {
		fmt.Fprintln(out, "- (ninguna)")
	}
	for _, fix := range fixes {
		fmt.Fprintf(out, "- ronda %d · %s → %s · aborda %s\n",
			fix.Round, fix.BaseTargetDigest, fix.FixedTargetDigest,
			strings.Join(fix.AddressedConsensusIDs, ", "))
		if len(fix.ModifiedPaths) > 0 {
			fmt.Fprintf(out, "  rutas: %s\n", strings.Join(fix.ModifiedPaths, ", "))
		}
	}

	fmt.Fprintln(out, "\n## Veredicto")
	if review.Status.Terminal() {
		fmt.Fprintf(out, "%s\n", review.Verdict)
	} else {
		fmt.Fprintf(out, "(sin veredicto: la revisión está en %s)\n", review.Status)
	}
}

// mustGetReview resuelve una revisión o termina con un error explícito.
//
// Un identificador inexistente NO puede devolver una revisión vacía: quien
// consulta leería «sin hallazgos» y concluiría que no hay defectos, cuando lo
// que pasa es que se equivocó de identificador.
func mustGetReview(deps *clikit.Deps, reviewID string) *domain.Review {
	review, err := deps.ReviewRepo.GetReview(deps.Project, reviewID)
	if err != nil {
		clikit.Fail("leer la revisión %s: %v", reviewID, err)
	}
	if review == nil {
		clikit.Fail("la revisión %s no existe en este proyecto (usa `mem review history`)", reviewID)
	}
	return review
}

// activeReview devuelve la revisión abierta más reciente, o nil si todas
// terminaron.
func activeReview(deps *clikit.Deps) *domain.Review {
	reviews, err := deps.ReviewRepo.ListReviews(deps.Project, 50)
	if err != nil {
		clikit.Fail("listar revisiones: %v", err)
	}
	for i := range reviews {
		if !reviews[i].Status.Terminal() {
			return &reviews[i]
		}
	}
	return nil
}

func resumirHallazgos(deps *clikit.Deps, review *domain.Review) {
	estado, err := usecases.BuildReviewStatus(deps.ReviewRepo, deps.ConsensusRepo, deps.Project, review)
	if err != nil {
		clikit.Fail("%v", err)
	}
	if len(estado.Findings) == 0 {
		return
	}
	porEstado := estado.Counts.ByStatus
	porSeveridad := estado.Counts.BySeverity
	porReJuicio := estado.Counts.ByRejudgment
	pendientes := estado.Counts.Pending
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
