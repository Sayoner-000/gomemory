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

type reviewUnmatchedInput struct {
	Status    string `json:"status"`
	FindingID int64  `json:"finding_id"`
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
	}) (*mcp.CallToolResult, any, error) {
		severities := make([]domain.Severity, 0, len(in.AutoFixSeverities))
		for _, severity := range in.AutoFixSeverities {
			severities = append(severities, domain.Severity(severity))
		}
		review, err := usecases.StartReview(deps.ReviewRepo, usecases.StartReviewInput{
			Project: project, TargetType: domain.TargetType(in.TargetType), Revision: in.Revision,
			Digest: in.Digest, Scope: in.Scope, MaxFixRounds: in.MaxFixRounds, AutoFixSeverities: severities,
			ReviewerA: usecases.ReviewerIdentity{Provider: in.ReviewerA.Provider, Model: in.ReviewerA.Model},
			ReviewerB: usecases.ReviewerIdentity{Provider: in.ReviewerB.Provider, Model: in.ReviewerB.Model},
		})
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{
			"review_id": review.ID, "target_digest": review.Target.Digest(),
			"independence": map[string]any{"level": review.IndependenceLevel, "reason": review.IndependenceReason},
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
			})
		}
		findings, err := usecases.BuildConsensus(deps.ReviewRepo, deps.ConsensusRepo, usecases.BuildConsensusInput{
			Project: project, ReviewID: in.ReviewID, Matches: matches, Unmatched: unmatched,
		})
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{"consensus_findings": findings})
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
		ReviewID string            `json:"review_id"`
		States   map[string]string `json:"states"`
	}) (*mcp.CallToolResult, any, error) {
		states := make(map[string]domain.ReJudgmentState, len(in.States))
		for localID, state := range in.States {
			states[localID] = domain.ReJudgmentState(state)
		}
		findings, err := usecases.RejudgeReview(deps.ReviewRepo, deps.ConsensusRepo, usecases.RejudgeReviewInput{
			Project: project, ReviewID: in.ReviewID, States: states,
		})
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(map[string]any{"rejudged": findings})
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
		return reviewToolResult(map[string]any{
			"review_id": review.ID, "status": review.Status, "round": review.Round, "verdict": review.Verdict,
		})
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
			"metrics":             metrics,
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
