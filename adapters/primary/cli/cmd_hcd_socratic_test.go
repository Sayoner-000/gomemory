package cli

import (
	"strings"
	"testing"
)

func TestHCD057SocraticCLI(t *testing.T) {
	// Verify that CLI help and usage include review command for HCD
	var foundReviewInHCD bool
	for _, cat := range helpIndex {
		for _, e := range cat.commands {
			if e.command == "hcd" && strings.Contains(e.summary, "review") {
				foundReviewInHCD = true
			}
		}
	}
	if !foundReviewInHCD {
		t.Fatal("expected 'review' mentioned in hcd help entry description")
	}

	// Verify that detailed helpReference includes review command line
	ref := helpReference()
	if !strings.Contains(ref, "review --kind socratic|splash") {
		t.Fatalf("expected 'review --kind socratic|splash' in helpReference, got:\n%s", ref)
	}
}
