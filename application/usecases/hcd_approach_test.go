package usecases_test

import (
	"context"
	"errors"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/speckit"
	"mem/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestHCDApproachDetectionAndClassificationDoesNotEditArtifacts(t *testing.T) {
	uc, repo, _, _, root := newHCDFlow(t)
	uc.ConfigureSpecKitReader(speckit.Reader{})
	ctx := context.Background()
	legacy := filepath.Join(root, "specs", "001-legacy")
	if e := os.MkdirAll(legacy, 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(legacy, "spec.md"), []byte("legacy source"), 0600); e != nil {
		t.Fatal(e)
	}
	files, e := hcdfiles.NewStore(root)
	if e != nil {
		t.Fatal(e)
	}
	d := validApprovalDefinition(domain.Unit{ID: "HCD-002", Kind: domain.KindFix, Title: "Huérfana", Implementer: "dev", PlaybookRev: "playbook"})
	content := hcdfiles.RenderDefinition(d)
	for _, dir := range []string{"002-orphan", "003-mixed"} {
		if e = files.Write(ctx, dir, "definition.md", content); e != nil {
			t.Fatal(e)
		}
	}
	if e = os.WriteFile(filepath.Join(root, "hcd", "features", "003-mixed", "spec.md"), []byte("mixed source"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = repo.CreateUnit(ctx, domain.Unit{ID: "HCD-003", Slug: "mixed", Approach: "hcd", State: domain.StateDraft, StateRevision: 1}); e != nil {
		t.Fatal(e)
	}
	list, e := uc.List(ctx)
	if e != nil {
		t.Fatal(e)
	}
	expected := map[string]string{"001-legacy": "speckit", "HCD-002": "unclassified", "HCD-003": "unclassified"}
	for _, u := range list {
		if expected[u.ID] != u.Approach {
			t.Fatalf("approach=%+v", u)
		}
		delete(expected, u.ID)
	}
	if len(expected) > 0 {
		t.Fatalf("missing=%v", expected)
	}
	if _, e = uc.Define(ctx, "001-legacy", 1, "no-legacy", d, content); !errors.Is(e, domain.ErrApproachMismatch) {
		t.Fatalf("legacy mutation=%v", e)
	}
	classified, e := uc.Classify(ctx, "hcd/features/002-orphan", "hcd", "apruebo", "codex")
	if e != nil || classified.Approach != "hcd" {
		t.Fatalf("classify=%+v %v", classified, e)
	}
	after, e := files.Read(ctx, "002-orphan", "definition.md")
	if e != nil || string(after) != string(content) {
		t.Fatal("classification edited definition")
	}
	legacyAfter, e := os.ReadFile(filepath.Join(legacy, "spec.md"))
	if e != nil || string(legacyAfter) != "legacy source" {
		t.Fatal("classification edited legacy")
	}
	_, e = uc.Classify(ctx, "hcd/features/003-mixed", "speckit", "apruebo", "codex")
	if e != nil {
		t.Fatal(e)
	}
	list, e = uc.List(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range list {
		if u.ID == "HCD-003" && u.Approach != "speckit" {
			t.Fatalf("explicit classification ignored: %+v", u)
		}
	}
}
