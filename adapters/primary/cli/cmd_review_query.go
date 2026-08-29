package cli

import (
	"fmt"
	"strconv"
	"strings"

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
	resumirHallazgos(deps, review.ID)
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
	fmt.Printf("- tipo: %s\n- revisión: %s\n- digest: %s\n",
		review.Target.Type, review.Target.Revision, review.Target.Digest())
	if len(review.Target.Scope) > 0 {
		fmt.Printf("- alcance: %s\n", strings.Join(review.Target.Scope, ", "))
	}
	fmt.Printf("- independencia: %s", review.IndependenceLevel)
	if review.IndependenceReason != "" {
		fmt.Printf(" (%s)", review.IndependenceReason)
	}
	fmt.Println()

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
	for _, finding := range findings {
		fmt.Printf("- %s · %s · %s", finding.ConsensusLocalID, finding.Status, finding.Severity)
		if finding.RejudgmentState != "" {
			fmt.Printf(" · %s", finding.RejudgmentState)
		}
		if finding.Claim != "" {
			fmt.Printf("\n  %s", finding.Claim)
		}
		fmt.Println()
	}

	fmt.Println("\n## Correcciones")
	fixes, err := deps.ConsensusRepo.ListFixDeltas(deps.Project, review.ID)
	if err != nil {
		fail("listar correcciones: %v", err)
	}
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

func resumirHallazgos(deps *Deps, reviewID string) {
	findings, err := deps.ConsensusRepo.ListAllConsensusFindings(deps.Project, reviewID)
	if err != nil {
		fail("listar consenso: %v", err)
	}
	if len(findings) == 0 {
		return
	}
	porEstado := map[domain.ConsensusStatus]int{}
	for _, finding := range findings {
		porEstado[finding.Status]++
	}
	fmt.Printf("hallazgos: %d confirmado(s), %d sospechoso(s), %d contradicción(es)\n",
		porEstado[domain.ConsensusConfirmed],
		porEstado[domain.ConsensusSuspect],
		porEstado[domain.ConsensusContradiction],
	)
}
