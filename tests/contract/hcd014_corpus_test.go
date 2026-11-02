package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestHCD014CorpusContract(t *testing.T) {
	b, err := os.ReadFile("../fixtures/hcd014/corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Version int `json:"schema_version"`
		Cases   []struct {
			ID       string `json:"id"`
			Kind     string `json:"kind"`
			Output   string `json:"output"`
			SHA      string `json:"output_sha256"`
			Expected string `json:"expected_result"`
			Oracle   string `json:"oracle"`
			Command  string `json:"command"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != 1 || len(corpus.Cases) != 6 {
		t.Fatal("expected the six frozen cases")
	}
	want := map[string]string{"volume": "worker_399 job=399", "exact_read": "func Exact() string { return \"literal\" }", "error": "ERR_HCD014: missing fixture", "recovery": "job=237 checksum=HCD014-237", "compaction": "checkpoint=HCD014-CONTINUE", "no_gain": "ok"}
	seen := map[string]bool{}
	for _, c := range corpus.Cases {
		if seen[c.ID] || c.ID == "" || c.Output == "" || c.Command == "" {
			t.Fatal("empty/duplicate case")
		}
		seen[c.ID] = true
		h := sha256.Sum256([]byte(c.Output))
		if hex.EncodeToString(h[:]) != c.SHA {
			t.Fatalf("%s hash changed", c.ID)
		}
		if c.Oracle != "exact_string" || c.Expected != want[c.Kind] || c.Expected == "" {
			t.Fatalf("%s lacks exact fixed oracle", c.ID)
		}
		delete(want, c.Kind)
	}
	if len(want) != 0 {
		t.Fatalf("missing cases: %v", want)
	}
}
