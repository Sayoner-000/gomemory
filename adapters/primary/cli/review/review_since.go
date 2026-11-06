package review

import (
	"encoding/json"
	"fmt"
	"strings"

	"mem/adapters/primary/cli/clikit"
	"mem/application/usecases"
	"mem/domain"
)

// Superficies de la revisión acumulativa (HCD-032).

// settledInstructions es lo que reciben los revisores de una revisión enlazada:
// sin ella, el ahorro dependería de que el orquestador lo explicara bien.
const settledInstructions = "Revisión de seguimiento: examina solo los archivos del delta. " +
	"Los hallazgos de inherited ya están asentados: no los reportes de nuevo. Si reaparece uno, " +
	"repórtalo con settled_ref=<consensus_id>; el motor lo da por asentado si su archivo no cambió " +
	"y lo reabre si cambió."

// extraerModificadores separa los modificadores de `mem review` que pueden ir
// en cualquier posición: --read-only y --since <id> (o --since=<id>).
func extraerModificadores(args []string) (rest []string, soloLectura bool, since string, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--read-only":
			soloLectura = true
		case arg == "--since":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return nil, false, "", fmt.Errorf("--since requiere el identificador de la revisión anterior")
			}
			since = args[i+1]
			i++
		case strings.HasPrefix(arg, "--since="):
			since = strings.TrimPrefix(arg, "--since=")
			if since == "" {
				return nil, false, "", fmt.Errorf("--since requiere el identificador de la revisión anterior")
			}
		default:
			rest = append(rest, arg)
		}
	}
	return rest, soloLectura, since, nil
}

// findingsDesdeEntrada traduce los hallazgos de review_submit. Settled nunca se
// toma de la entrada: lo decide el motor con el delta guardado.
func findingsDesdeEntrada(in []reviewFindingInput) []domain.Finding {
	out := make([]domain.Finding, 0, len(in))
	for _, f := range in {
		out = append(out, domain.Finding{
			LocalID: f.LocalID, Location: f.Location, Severity: domain.Severity(f.Severity),
			Category: f.Category, Claim: f.Claim, EvidenceClass: domain.EvidenceClass(f.EvidenceClass),
			Evidence: f.Evidence, Confidence: f.Confidence, SettledRef: f.SettledRef,
		})
	}
	return out
}

// cargaHeredada es lo que el revisor de una revisión enlazada recibe de más:
// la lista de heredados y las instrucciones, tal como viajan en la respuesta.
func cargaHeredada(enlace map[string]any) string {
	data, err := json.Marshal(map[string]any{"inherited": enlace["inherited"], "instructions": enlace["instructions"]})
	if err != nil {
		return ""
	}
	return string(data)
}

// ahorroDeRevision mide el ahorro de una revisión enlazada para review_start
// (HCD-035). Es un proxy de entrada, no el consumo real de un modelo.
func ahorroDeRevision(deps *clikit.Deps, review *domain.Review, enlace map[string]any) map[string]any {
	s := usecases.ReviewSinceSavings(deps.RevisionFiles, deps.TokenCounter, review, cargaHeredada(enlace))
	out := map[string]any{"measurable": s.Measurable}
	if !s.Measurable {
		out["reason"] = s.Reason
		return out
	}
	out["scope_files"], out["delta_files"] = s.ScopeFiles, s.DeltaFiles
	out["scope_tokens"], out["delta_tokens"] = s.ScopeTokens, s.DeltaTokens
	out["inherited_tokens"], out["avoided_tokens"] = s.InheritedTokens, s.AvoidedTokens
	return out
}

// enlaceDeRevision describe el enlace de una revisión de seguimiento para
// review_start y review_status. Vacío si la revisión no está enlazada.
func enlaceDeRevision(review *domain.Review, inherited []domain.SettledFinding) map[string]any {
	if review == nil || review.SinceReviewID == "" {
		return map[string]any{}
	}
	files := review.DeltaScope
	if files == nil {
		files = []string{}
	}
	lista := make([]map[string]any, 0, len(inherited))
	for _, s := range inherited {
		lista = append(lista, map[string]any{
			"consensus_id": s.ConsensusID, "status": s.Status, "severity": s.Severity,
			"claim": s.Claim, "files": s.Files,
		})
	}
	return map[string]any{
		"since_review_id": review.SinceReviewID,
		"delta":           map[string]any{"source": review.DeltaSource, "files": files},
		"inherited":       lista,
		"instructions":    settledInstructions,
	}
}
