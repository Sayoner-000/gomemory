package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/application/usecases"
	"mem/domain"
)

type reviewIdentityInput struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type reviewFindingInput struct {
	LocalID       string   `json:"local_id"`
	Location      string   `json:"location"`
	Severity      string   `json:"severity"`
	Category      string   `json:"category"`
	Claim         string   `json:"claim"`
	EvidenceClass string   `json:"evidence_class"`
	Evidence      []string `json:"evidence"`
	Confidence    string   `json:"confidence"`
}

type reviewMatchInput struct {
	Status     string `json:"status"`
	FindingIDA int64  `json:"finding_id_a"`
	FindingIDB int64  `json:"finding_id_b"`
	Severity   string `json:"severity"`
	Claim      string `json:"claim"`
}

type reviewLearnInput struct {
	Category     string   `json:"category"`
	Component    string   `json:"component,omitempty"`
	Problem      string   `json:"problem"`
	RootCause    string   `json:"root_cause"`
	Resolution   string   `json:"resolution"`
	Verification []string `json:"verification,omitempty"`
	Confidence   string   `json:"confidence,omitempty"`
}

// reviewJudgesInput es el veredicto de un revisor sobre un hallazgo corregido. La
// evidencia es obligatoria: un re-juicio sin ella es una afirmación sin respaldo, y
// es la afirmación que decide si la revisión puede aprobarse.
type reviewJudgesInput struct {
	State    string   `json:"state"`
	Evidence []string `json:"evidence"`
}

type reviewUnmatchedInput struct {
	Status    string `json:"status"`
	FindingID int64  `json:"finding_id"`
	// Severity es informativa desde la funcionalidad 028: la severidad persistida
	// se deriva de la fuente. Si viene y no coincide, la operación se rechaza.
	Severity string `json:"severity,omitempty"`
}

func registerReviewTools(server *mcp.Server, deps *Deps, project string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "review_start",
		Description: "Congela un target y abre una revisión adversarial sin ejecutar modelos.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		TargetType        string              `json:"target_type"`
		Revision          string              `json:"revision"`
		Digest            string              `json:"digest"`
		Scope             []string            `json:"scope,omitempty"`
		ReviewerA         reviewIdentityInput `json:"reviewer_a,omitempty"`
		ReviewerB         reviewIdentityInput `json:"reviewer_b,omitempty"`
		MaxFixRounds      int                 `json:"max_fix_rounds,omitempty"`
		AutoFixSeverities []string            `json:"auto_fix_severities,omitempty"`
		FixAuthorized     *bool               `json:"fix_authorized,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		severities := make([]domain.Severity, 0, len(in.AutoFixSeverities))
		for _, severity := range in.AutoFixSeverities {
			severities = append(severities, domain.Severity(severity))
		}
		review, err := usecases.StartReview(deps.ReviewRepo, usecases.StartReviewInput{
			Project: project, TargetType: domain.TargetType(in.TargetType), Revision: in.Revision,
			Digest: in.Digest, Scope: in.Scope, MaxFixRounds: in.MaxFixRounds, AutoFixSeverities: severities,
			Policy:        reviewPolicyDelProyecto(deps),
			FixAuthorized: in.FixAuthorized,
			ReviewerA:     usecases.ReviewerIdentity{Provider: in.ReviewerA.Provider, Model: in.ReviewerA.Model},
			ReviewerB:     usecases.ReviewerIdentity{Provider: in.ReviewerB.Provider, Model: in.ReviewerB.Model},
		})
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{
			"review_id": review.ID, "target_digest": review.Target.Digest(),
			"independence":        map[string]any{"level": review.IndependenceLevel, "reason": review.IndependenceReason},
			"fix_authorized":      review.FixAuthorized,
			"max_fix_rounds":      review.MaxFixRounds,
			"auto_fix_severities": review.AutoFixSeverities,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "review_submit",
		Description: "Registra el resultado estructurado de un revisor para el target congelado.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ReviewID     string               `json:"review_id"`
		Reviewer     string               `json:"reviewer"`
		TargetDigest string               `json:"target_digest"`
		Status       string               `json:"status"`
		Provider     string               `json:"provider,omitempty"`
		Model        string               `json:"model,omitempty"`
		Findings     []reviewFindingInput `json:"findings,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		findings := make([]domain.Finding, 0, len(in.Findings))
		for _, finding := range in.Findings {
			findings = append(findings, domain.Finding{
				LocalID: finding.LocalID, Location: finding.Location, Severity: domain.Severity(finding.Severity),
				Category: finding.Category, Claim: finding.Claim, EvidenceClass: domain.EvidenceClass(finding.EvidenceClass),
				Evidence: finding.Evidence, Confidence: finding.Confidence,
			})
		}
		out, err := usecases.SubmitReviewerResult(deps.ReviewRepo, usecases.SubmitReviewerResultInput{
			Project: project, ReviewID: in.ReviewID, TargetDigest: in.TargetDigest,
			Result: domain.ReviewerResult{Reviewer: domain.Reviewer(in.Reviewer), Provider: in.Provider,
				Model: in.Model, Status: domain.ReviewerResultStatus(in.Status), Findings: findings},
		})
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{
			"stored": true, "consensus_ready": out.ConsensusReady, "finding_ids": out.FindingIDs,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "review_consensus",
		Description: "Valida y persiste una clasificación de consenso propuesta por el agente orquestador.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ReviewID  string                 `json:"review_id"`
		Matches   []reviewMatchInput     `json:"matches,omitempty"`
		Unmatched []reviewUnmatchedInput `json:"unmatched,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		matches := make([]usecases.ConsensusMatch, 0, len(in.Matches))
		for _, match := range in.Matches {
			matches = append(matches, usecases.ConsensusMatch{
				Status: domain.ConsensusStatus(match.Status), FindingIDA: match.FindingIDA, FindingIDB: match.FindingIDB,
				Severity: domain.Severity(match.Severity), Claim: match.Claim,
			})
		}
		unmatched := make([]usecases.ConsensusUnmatched, 0, len(in.Unmatched))
		for _, item := range in.Unmatched {
			unmatched = append(unmatched, usecases.ConsensusUnmatched{
				Status: domain.ConsensusStatus(item.Status), FindingID: item.FindingID,
				Severity: domain.Severity(item.Severity),
			})
		}
		out, err := usecases.BuildConsensusWithOutcome(deps.ReviewRepo, deps.ConsensusRepo, usecases.BuildConsensusInput{
			Project: project, ReviewID: in.ReviewID, Matches: matches, Unmatched: unmatched,
		})
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{
			"consensus_findings": out.Findings, "idempotent": out.Idempotent,
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "review_fix_record",
		Description: "Registra una corrección ya aplicada fuera de gomemory y avanza la revisión a re-revisión. " +
			"Rechaza hallazgos no confirmados, severidades fuera de política sin autorización explícita, " +
			"y cualquier ronda por encima del presupuesto. El número de ronda se deriva, no se pide.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ReviewID              string   `json:"review_id"`
		AddressedConsensusIDs []string `json:"addressed_consensus_ids"`
		BaseTargetDigest      string   `json:"base_target_digest"`
		FixedTargetDigest     string   `json:"fixed_target_digest"`
		ModifiedPaths         []string `json:"modified_paths,omitempty"`
		Verification          []string `json:"verification,omitempty"`
		DiffDigest            string   `json:"diff_digest,omitempty"`
		ExplicitAuthorization bool     `json:"explicit_authorization,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		delta, err := usecases.RecordFix(deps.ReviewRepo, deps.ConsensusRepo, usecases.RecordFixInput{
			Project: project, ReviewID: in.ReviewID,
			AddressedConsensusIDs: in.AddressedConsensusIDs,
			BaseTargetDigest:      in.BaseTargetDigest,
			FixedTargetDigest:     in.FixedTargetDigest,
			ModifiedPaths:         in.ModifiedPaths,
			Verification:          in.Verification,
			DiffDigest:            in.DiffDigest,
			ExplicitAuthorization: in.ExplicitAuthorization,
		})
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{"fix_delta": delta})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "review_rejudge",
		Description: "Registra el resultado de la revalidación de los hallazgos confirmados tras una corrección " +
			"(RESOLVED, UNRESOLVED o REGRESSED). Exige una corrección previa registrada.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ReviewID  string                       `json:"review_id"`
		Reviewer  string                       `json:"reviewer"`
		Judgments map[string]reviewJudgesInput `json:"judgments"`
	}) (*mcp.CallToolResult, any, error) {
		judgments := make(map[string]usecases.ReJudgeEntry, len(in.Judgments))
		for localID, entry := range in.Judgments {
			judgments[localID] = usecases.ReJudgeEntry{
				State: domain.ReJudgmentState(entry.State), Evidence: entry.Evidence,
			}
		}
		findings, err := usecases.RejudgeReview(deps.ReviewRepo, deps.ConsensusRepo, usecases.RejudgeReviewInput{
			Project: project, ReviewID: in.ReviewID,
			Reviewer: domain.Reviewer(in.Reviewer), Judgments: judgments,
		})
		if err != nil {
			return nil, nil, err
		}
		salida := make([]map[string]any, 0, len(findings))
		for _, finding := range findings {
			porRevisor, err := usecases.ReJudgmentsByReviewer(
				deps.ConsensusRepo, project, in.ReviewID, finding.ConsensusLocalID)
			if err != nil {
				return nil, nil, err
			}
			salida = append(salida, map[string]any{
				"consensus_local_id": finding.ConsensusLocalID,
				"reviewer_states":    porRevisor,
				"aggregate_state":    finding.RejudgmentState,
			})
		}
		return reviewToolResult(map[string]any{"rejudged": salida})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "review_status",
		Description: "Consulta el estado persistido de una revisión sin ejecutar transiciones.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ReviewID string `json:"review_id"`
	}) (*mcp.CallToolResult, any, error) {
		review, err := deps.ReviewRepo.GetReview(project, in.ReviewID)
		if err != nil {
			return nil, nil, err
		}
		if review == nil {
			return nil, nil, fmt.Errorf("review %s not found", in.ReviewID)
		}
		return construirEstadoDeRevision(deps, project, review)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "review_finalize",
		Description: "Deriva el veredicto terminal exclusivamente desde el estado persistido.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ReviewID string `json:"review_id"`
	}) (*mcp.CallToolResult, any, error) {
		review, metrics, err := usecases.FinalizeReviewWithMetrics(deps.ReviewRepo, deps.ConsensusRepo, project, in.ReviewID)
		if err != nil {
			return nil, nil, err
		}
		// Qué es promovible lo decide gomemory; QUÉ dice el aprendizaje lo
		// redacta el agente con review_promote_memory. Informarlo aquí evita
		// que la regla de promoción dependa de que el agente la recuerde, sin
		// que gomemory tenga que inventar un conocimiento que no posee.
		findings, err := deps.ConsensusRepo.ListAllConsensusFindings(project, in.ReviewID)
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{
			"review_id":           review.ID,
			"verdict":             review.Verdict,
			"promotable_findings": domain.PromotableFindings(findings),
			"metrics":             nuevasMetricasDTO(metrics),
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "review_promote_memory",
		Description: "Convierte defectos confirmados Y resueltos en memoria reutilizable del proyecto. " +
			"Solo acepta problema, causa raíz, resolución y verificación: no hay dónde poner un transcript " +
			"ni una cadena de razonamiento. Dos revisiones del mismo patrón refuerzan una memoria, no crean dos.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ReviewID  string                      `json:"review_id"`
		Learnings map[string]reviewLearnInput `json:"learnings"`
	}) (*mcp.CallToolResult, any, error) {
		learnings := make(map[string]domain.ReviewLearning, len(in.Learnings))
		for localID, l := range in.Learnings {
			learnings[localID] = domain.ReviewLearning{
				Category: l.Category, Component: l.Component, Problem: l.Problem,
				RootCause: l.RootCause, Resolution: l.Resolution,
				Verification: l.Verification, Confidence: l.Confidence,
			}
		}
		promovidas, err := usecases.PromoteReviewMemory(
			deps.ReviewRepo, deps.ConsensusRepo, deps.MemoryRepo,
			usecases.PromoteReviewMemoryInput{Project: project, ReviewID: in.ReviewID, Learnings: learnings},
		)
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{"promoted": promovidas})
	})
}

func reviewToolResult(value any) (*mcp.CallToolResult, any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, value, nil
}

// reviewPolicyDelProyecto lee la política de revisión configurada.
//
// Es el punto que faltaba: Settings tenía la política desde la funcionalidad 027 pero
// ningún llamador la leía, así que configurarla no producía ningún efecto (FR-017).
func reviewPolicyDelProyecto(deps *Deps) domain.ReviewPolicy {
	politica := domain.DefaultReviewPolicy()
	if deps.SettingsRepo == nil {
		return politica
	}
	ajustes := deps.SettingsRepo.Read(deps.Root)
	if ajustes.ReviewMaxFixRounds > 0 {
		politica.MaxFixRounds = ajustes.ReviewMaxFixRounds
	}
	if len(ajustes.ReviewAutoFixSeverities) > 0 {
		severidades := make([]domain.Severity, 0, len(ajustes.ReviewAutoFixSeverities))
		for _, severidad := range ajustes.ReviewAutoFixSeverities {
			severidades = append(severidades, domain.Severity(severidad))
		}
		politica.AutoFixSeverities = severidades
	}
	if ajustes.ReviewFixAuthorized != nil {
		politica.FixAuthorized = *ajustes.ReviewFixAuthorized
	}
	return politica
}

// reviewMetricsDTO publica las métricas con los nombres EXACTOS del contrato.
//
// El struct de la capa de aplicación no tenía etiquetas JSON, así que json.Marshal
// emitía FindingsTotal, FixRounds… en PascalCase, y omitía tres de los ocho campos
// prometidos. La serialización vive en el adaptador porque es un detalle del canal,
// no del dominio (FR-024).
type reviewMetricsDTO struct {
	Duration           int `json:"duration"`
	FindingsTotal      int `json:"findings_total"`
	FindingsConfirmed  int `json:"findings_confirmed"`
	FindingsSuspect    int `json:"findings_suspect"`
	Contradictions     int `json:"contradictions"`
	FixRounds          int `json:"fix_rounds"`
	MemoryPromoted     int `json:"memory_promoted"`
	MemoryDeduplicated int `json:"memory_deduplicated"`
}

func nuevasMetricasDTO(m usecases.ReviewMetrics) reviewMetricsDTO {
	return reviewMetricsDTO{
		Duration: m.Duration, FindingsTotal: m.FindingsTotal,
		FindingsConfirmed: m.FindingsConfirmed, FindingsSuspect: m.FindingsSuspect,
		Contradictions: m.Contradictions, FixRounds: m.FixRounds,
		MemoryPromoted: m.MemoryPromoted, MemoryDeduplicated: m.MemoryDeduplicated,
	}
}

// construirEstadoDeRevision arma la respuesta de review_status.
//
// Antes devolvía cuatro campos y el contrato prometía "resumen de hallazgos por
// estado". SC-006 exige más: reconstruir el recorrido completo de cualquier hallazgo
// con UNA sola consulta, sin abrir la base ni ningún archivo interno (FR-022, FR-023).
func construirEstadoDeRevision(
	deps *Deps, project string, review *domain.Review,
) (*mcp.CallToolResult, any, error) {
	findings, err := deps.ConsensusRepo.ListAllConsensusFindings(project, review.ID)
	if err != nil {
		return nil, nil, err
	}
	fixes, err := deps.ConsensusRepo.ListFixDeltas(project, review.ID)
	if err != nil {
		return nil, nil, err
	}
	results, err := deps.ReviewRepo.ListReviewerResults(project, review.ID, review.Round)
	if err != nil {
		return nil, nil, err
	}

	// Qué ronda abordó cada hallazgo: es la mitad del linaje que un auditor
	// necesita y no estaba en ninguna respuesta.
	abordadoPor := map[string]int{}
	rondas := make([]map[string]any, 0, len(fixes))
	for _, fix := range fixes {
		for _, localID := range fix.AddressedConsensusIDs {
			abordadoPor[localID] = fix.Round
		}
		rondas = append(rondas, map[string]any{
			"round": fix.Round, "base_target_digest": fix.BaseTargetDigest,
			"fixed_target_digest":     fix.FixedTargetDigest,
			"addressed_consensus_ids": fix.AddressedConsensusIDs,
			"modified_paths":          fix.ModifiedPaths, "verification": fix.Verification,
		})
	}

	porEstado := map[string]int{"CONFIRMED": 0, "SUSPECT": 0, "CONTRADICTION": 0, "INFO": 0}
	porSeveridad := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0, "INFO": 0}
	porReJuicio := map[string]int{"RESOLVED": 0, "UNRESOLVED": 0, "REGRESSED": 0, "PENDING": 0}
	detalle := make([]map[string]any, 0, len(findings))
	for _, finding := range findings {
		porEstado[string(finding.Status)]++
		porSeveridad[string(finding.Severity)]++
		if finding.RejudgmentState == "" {
			porReJuicio["PENDING"]++
		} else {
			porReJuicio[string(finding.RejudgmentState)]++
		}
		reJuicios, err := usecases.ReJudgmentsByReviewer(
			deps.ConsensusRepo, project, review.ID, finding.ConsensusLocalID)
		if err != nil {
			return nil, nil, err
		}
		entrada := map[string]any{
			"consensus_local_id": finding.ConsensusLocalID,
			"status":             finding.Status,
			"severity":           finding.Severity,
			"round":              finding.Round,
			"source_finding_ids": finding.SourceFindingIDs,
			"rejudgments":        reJuicios,
			"aggregate_state":    finding.RejudgmentState,
		}
		if ronda, ok := abordadoPor[finding.ConsensusLocalID]; ok {
			entrada["addressed_by_round"] = ronda
		}
		detalle = append(detalle, entrada)
	}

	revisores := make([]map[string]any, 0, 2)
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		entrada := map[string]any{
			"reviewer": revisor,
			"expected": review.ExpectedReviewer(revisor).String(),
			"status":   "pending",
			"findings": 0,
		}
		for _, result := range results {
			if result.Reviewer == revisor {
				entrada["status"] = result.Status
				entrada["findings"] = len(result.Findings)
			}
		}
		revisores = append(revisores, entrada)
	}

	return reviewToolResult(map[string]any{
		"review_id":      review.ID,
		"status":         review.Status,
		"round":          review.Round,
		"verdict":        review.Verdict,
		"fix_authorized": review.FixAuthorized,
		"target": map[string]any{
			"type": review.Target.Type, "revision": review.Target.Revision,
			"original_digest": review.Target.Digest(),
			"current_digest":  review.ActiveTargetDigest(),
		},
		"policy": map[string]any{
			"max_fix_rounds": review.MaxFixRounds, "auto_fix_severities": review.AutoFixSeverities,
		},
		"reviewers": revisores,
		"counts": map[string]any{
			"by_status": porEstado, "by_severity": porSeveridad, "by_rejudgment": porReJuicio,
		},
		"findings":   detalle,
		"fix_rounds": rondas,
	})
}
