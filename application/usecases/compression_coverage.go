package usecases

import (
	"encoding/hex"
	"strings"

	"mem/application/ports"
)

// BuildCompressionCoverageMatrix no traslada capturas a otro alcance. Los
// agentes sin snapshot también aparecen, con versión/modalidad desconocidas.
func BuildCompressionCoverageMatrix(snapshot *ports.CompressionCoverageSnapshot) []ports.CompressionCoverageAssessment {
	rows := make([]ports.CompressionCoverageAssessment, 0, 12)
	for _, agent := range []string{"claude", "codex", "opencode"} {
		var scopes []ports.CompressionCoverageScope
		if snapshot != nil {
			for _, s := range snapshot.Scopes {
				if s.Agent == agent {
					scopes = append(scopes, s)
				}
			}
		}
		if len(scopes) == 0 {
			scopes = append(scopes, ports.CompressionCoverageScope{Agent: agent})
		}
		seen := map[ports.CompressionCoverageScope]bool{}
		for _, s := range scopes {
			for _, class := range []string{"own_output", "external_output", "history", "outbound_request"} {
				s.ContentClass = class
				if seen[s] {
					continue
				}
				seen[s] = true
				var match ports.CompressionCoverageEvidence
				matches := 0
				if snapshot != nil {
					for _, e := range snapshot.Evidence {
						if e.Scope == s {
							match, matches = e, matches+1
						}
					}
				}
				assessment := AssessCompressionCoverage(s, match)
				if matches > 1 {
					assessment.Status, assessment.Reason = "unverified", "multiple evidence records require reconciliation"
				}
				rows = append(rows, assessment)
			}
		}
	}
	return rows
}

// AssessCompressionCoverage no ejecuta sondas ni lee configuración. Evalúa
// evidencia suministrada para un alcance exacto: instalar/ejecutar un hook no
// demuestra que el host consumió su resultado ni que ese resultado se redujo.
func AssessCompressionCoverage(scope ports.CompressionCoverageScope, e ports.CompressionCoverageEvidence) ports.CompressionCoverageAssessment {
	out := ports.CompressionCoverageAssessment{Scope: scope, Status: "unverified", Reason: "no matching consumption evidence"}
	if !validCompressionScope(scope) || scope != e.Scope {
		return out
	}
	out.Mechanism = e.Mechanism
	if strings.TrimSpace(e.Mechanism) == "" || strings.TrimSpace(e.Source) == "" {
		out.Reason = "missing mechanism or provenance"
		return out
	}
	if e.Unavailable {
		if e.Executed || e.Consumed || e.Partial || strings.TrimSpace(e.Reason) == "" {
			out.Reason = "inconsistent unavailability evidence"
			return out
		}
		out.Status, out.Reason = "unavailable", e.Reason
		return out
	}
	if !e.Configured || !e.Executed || !e.Consumed {
		out.Reason = "configuration or execution alone does not prove consumption"
		return out
	}
	if e.Visibility != "host_input" && e.Visibility != "provider_request" {
		out.Reason = "callback or CLI output does not prove host input consumption"
		return out
	}
	if !validCompressionDigest(e.OriginalSHA256) || !validCompressionDigest(e.DeliveredSHA256) {
		out.Reason = "missing or invalid content digests"
		return out
	}
	if strings.EqualFold(e.OriginalSHA256, e.DeliveredSHA256) {
		out.Reason = "unchanged content does not prove compression"
		return out
	}
	if e.OriginalBytes <= 0 || e.DeliveredBytes <= 0 || e.DeliveredBytes >= e.OriginalBytes {
		out.Reason = "no measured reduction in delivered bytes"
		return out
	}
	out.Status, out.Reason = "verified", "reduced delivery observed within the declared scope"
	if e.Partial {
		out.Status, out.Reason = "partial", "reduced delivery observed with declared coverage limits"
	}
	return out
}

func validCompressionScope(s ports.CompressionCoverageScope) bool {
	if s.Agent != "codex" && s.Agent != "claude" && s.Agent != "opencode" {
		return false
	}
	if strings.TrimSpace(s.Version) == "" || strings.TrimSpace(s.Mode) == "" {
		return false
	}
	switch s.ContentClass {
	case "own_output", "external_output", "history", "outbound_request":
		return true
	default:
		return false
	}
}

func validCompressionDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
