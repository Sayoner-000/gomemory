package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"mem/application/ports"
)

func TestHCD014DoctorCoverageMatrix(t *testing.T) {
	c := buildDoctorCompression(&Deps{}, t.TempDir())
	if len(c.Coverage) != 12 {
		t.Fatalf("want 3 agents x 4 classes, got %+v", c.Coverage)
	}
	seen := map[string]bool{}
	for _, row := range c.Coverage {
		key := row.Scope.Agent + ":" + row.Scope.ContentClass
		if seen[key] || row.Status != "unverified" || row.Reason == "" {
			t.Fatalf("invalid default row: %+v", row)
		}
		seen[key] = true
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Coverage []ports.CompressionCoverageAssessment `json:"coverage"`
	}
	if err := json.Unmarshal(b, &decoded); err != nil || len(decoded.Coverage) != 12 {
		t.Fatalf("coverage absent from JSON: %s (%v)", b, err)
	}
}

func TestHCD014DoctorRejectsUnconsumedHook(t *testing.T) {
	scope := ports.CompressionCoverageScope{Agent: "codex", Version: "0.166.0", Mode: "interactive", ContentClass: "external_output"}
	digest := func(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
	base := ports.CompressionCoverageEvidence{Scope: scope, Mechanism: "hook", Configured: true, Executed: true, Consumed: true, Visibility: "provider_request", OriginalSHA256: digest("original"), DeliveredSHA256: digest("reduced"), OriginalBytes: 100, DeliveredBytes: 20, Source: "synthetic fixture"}
	tests := []struct {
		name   string
		change func(*ports.CompressionCoverageEvidence)
		want   string
	}{
		{"captured", func(*ports.CompressionCoverageEvidence) {}, "verified"},
		{"absent", func(e *ports.CompressionCoverageEvidence) {
			e.Configured = false
			e.Executed = false
			e.Consumed = false
		}, "unverified"},
		{"ignored", func(e *ports.CompressionCoverageEvidence) { e.Consumed = false }, "unverified"},
		{"stdout_only", func(e *ports.CompressionCoverageEvidence) { e.Visibility = "cli_stdout_only" }, "unverified"},
		{"callback_only", func(e *ports.CompressionCoverageEvidence) { e.Visibility = "hook_callback" }, "unverified"},
		{"no_visibility", func(e *ports.CompressionCoverageEvidence) { e.Visibility = "" }, "unverified"},
		{"old_version", func(e *ports.CompressionCoverageEvidence) { e.Scope.Version = "0.159.0" }, "unverified"},
		{"other_mode", func(e *ports.CompressionCoverageEvidence) { e.Scope.Mode = "exec" }, "unverified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := base
			tt.change(&e)
			deps := &Deps{CompressionCoverage: &ports.CompressionCoverageSnapshot{Scopes: []ports.CompressionCoverageScope{scope}, Evidence: []ports.CompressionCoverageEvidence{e}}}
			c := buildDoctorCompression(deps, t.TempDir())
			found := false
			for _, row := range c.Coverage {
				if row.Scope == scope {
					found = true
					if row.Status != tt.want {
						t.Fatalf("got %+v want %s", row, tt.want)
					}
				}
			}
			if !found {
				t.Fatal("requested scope absent from diagnostic")
			}
		})
	}
}
