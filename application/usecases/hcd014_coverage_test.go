package usecases_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"mem/application/ports"
	"mem/application/usecases"
)

func TestHCD014CoverageEvidenceContract(t *testing.T) {
	scope := ports.CompressionCoverageScope{Agent: "codex", Version: "0.166.0", Mode: "interactive", ContentClass: "external_output"}
	digest := func(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
	base := ports.CompressionCoverageEvidence{
		Scope: scope, Mechanism: "native_hook", Configured: true, Executed: true,
		Consumed: true, OriginalSHA256: digest("synthetic original"), DeliveredSHA256: digest("synthetic reduced"),
		Reason: "synthetic capture of host input", Source: "fixture:host-input",
		Visibility:    "provider_request",
		OriginalBytes: 200, DeliveredBytes: 100,
	}
	tests := []struct {
		name   string
		change func(*ports.CompressionCoverageEvidence)
		want   string
	}{
		{"complete", func(*ports.CompressionCoverageEvidence) {}, "verified"},
		{"longer_delivery", func(e *ports.CompressionCoverageEvidence) { e.DeliveredBytes = 201 }, "unverified"},
		{"same_size", func(e *ports.CompressionCoverageEvidence) { e.DeliveredBytes = 200 }, "unverified"},
		{"missing_size", func(e *ports.CompressionCoverageEvidence) { e.OriginalBytes = 0 }, "unverified"},
		{"negative_size", func(e *ports.CompressionCoverageEvidence) { e.DeliveredBytes = -1 }, "unverified"},
		{"configured_only", func(e *ports.CompressionCoverageEvidence) { e.Executed = false; e.Consumed = false }, "unverified"},
		{"executed_not_consumed", func(e *ports.CompressionCoverageEvidence) { e.Consumed = false }, "unverified"},
		{"consumed_without_execution", func(e *ports.CompressionCoverageEvidence) { e.Executed = false }, "unverified"},
		{"different_version", func(e *ports.CompressionCoverageEvidence) { e.Scope.Version = "0.159.0" }, "unverified"},
		{"different_mode", func(e *ports.CompressionCoverageEvidence) { e.Scope.Mode = "exec" }, "unverified"},
		{"different_agent", func(e *ports.CompressionCoverageEvidence) { e.Scope.Agent = "claude" }, "unverified"},
		{"different_class", func(e *ports.CompressionCoverageEvidence) { e.Scope.ContentClass = "history" }, "unverified"},
		{"missing_mechanism", func(e *ports.CompressionCoverageEvidence) { e.Mechanism = "" }, "unverified"},
		{"missing_provenance", func(e *ports.CompressionCoverageEvidence) { e.Source = "" }, "unverified"},
		{"missing_hash", func(e *ports.CompressionCoverageEvidence) { e.DeliveredSHA256 = "" }, "unverified"},
		{"invalid_hash", func(e *ports.CompressionCoverageEvidence) { e.OriginalSHA256 = "zz" }, "unverified"},
		{"unchanged", func(e *ports.CompressionCoverageEvidence) { e.DeliveredSHA256 = e.OriginalSHA256 }, "unverified"},
		{"partial", func(e *ports.CompressionCoverageEvidence) { e.Partial = true }, "partial"},
		{"unavailable", func(e *ports.CompressionCoverageEvidence) {
			e.Configured = false
			e.Executed = false
			e.Consumed = false
			e.Unavailable = true
		}, "unavailable"},
		{"contradictory_unavailable", func(e *ports.CompressionCoverageEvidence) { e.Unavailable = true }, "unverified"},
		{"unavailable_without_reason", func(e *ports.CompressionCoverageEvidence) {
			e.Configured = false
			e.Executed = false
			e.Consumed = false
			e.Unavailable = true
			e.Reason = ""
		}, "unverified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := base
			tt.change(&e)
			got := usecases.AssessCompressionCoverage(scope, e)
			if got.Status != tt.want || got.Reason == "" || got.Scope != scope {
				t.Fatalf("status=%q reason=%q scope=%+v, want %q for %+v", got.Status, got.Reason, got.Scope, tt.want, scope)
			}
		})
	}
	t.Run("empty_scope", func(t *testing.T) {
		if got := usecases.AssessCompressionCoverage(ports.CompressionCoverageScope{}, ports.CompressionCoverageEvidence{}); got.Status != "unverified" {
			t.Fatalf("empty scope promoted: %+v", got)
		}
	})
}
