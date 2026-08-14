package main

import (
	"testing"

	"mem/adapters/primary/cli"
)

func TestParsePackBuildFlags_RequiresTask(t *testing.T) {
	_, _, err := cli.ParsePackBuildFlags([]string{"--max-tokens", "500"}, "proj")
	if err == nil {
		t.Fatal("se esperaba error por --task ausente")
	}
}

func TestParsePackBuildFlags_RequiresMaxTokens(t *testing.T) {
	_, _, err := cli.ParsePackBuildFlags([]string{"--task", "hacer algo"}, "proj")
	if err == nil {
		t.Fatal("se esperaba error por --max-tokens ausente")
	}
}

func TestParsePackBuildFlags_RejectsNonPositiveMaxTokens(t *testing.T) {
	_, _, err := cli.ParsePackBuildFlags([]string{"--task", "hacer algo", "--max-tokens", "0"}, "proj")
	if err == nil {
		t.Fatal("se esperaba error por --max-tokens=0")
	}
}

func TestParsePackBuildFlags_DefaultsProjectFromCurrent(t *testing.T) {
	req, _, err := cli.ParsePackBuildFlags([]string{"--task", "hacer algo", "--max-tokens", "500"}, "proj-actual")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Project != "proj-actual" {
		t.Fatalf("Project = %q, se esperaba %q (default del proyecto actual)", req.Project, "proj-actual")
	}
	if req.MaxTokens != 500 {
		t.Fatalf("MaxTokens = %d, se esperaba 500", req.MaxTokens)
	}
}

func TestParsePackBuildFlags_ProjectFlagOverridesDefault(t *testing.T) {
	req, _, err := cli.ParsePackBuildFlags([]string{"--task", "x", "--max-tokens", "10", "--project", "otro"}, "proj-actual")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Project != "otro" {
		t.Fatalf("Project = %q, se esperaba %q", req.Project, "otro")
	}
}

func TestParsePackBuildFlags_NoSpeckitDisablesInclusion(t *testing.T) {
	req, _, err := cli.ParsePackBuildFlags([]string{"--task", "x", "--max-tokens", "10", "--no-speckit"}, "proj")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.IncludeSpecKit {
		t.Fatal("--no-speckit debería dejar IncludeSpecKit=false")
	}
}

func TestParsePackBuildFlags_JSONFlag(t *testing.T) {
	_, asJSON, err := cli.ParsePackBuildFlags([]string{"--task", "x", "--max-tokens", "10", "--json"}, "proj")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !asJSON {
		t.Fatal("--json debería devolver asJSON=true")
	}
}
