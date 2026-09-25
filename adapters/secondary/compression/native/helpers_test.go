package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mem/domain"
)

const corpusDir = "../../../../tests/testdata/compression_corpus"

func readCorpus(t testing.TB, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(corpusDir, name))
	if err != nil {
		t.Fatalf("leer muestra %s: %v", name, err)
	}
	return string(b)
}

type expectations struct {
	Corpus  float64 `json:"corpus_min_saving_vs_structural_pct"`
	Samples map[string]struct {
		Type   string  `json:"type"`
		MinRaw float64 `json:"min_saving_vs_raw_pct"`
	} `json:"samples"`
}

func loadExpectations(t testing.TB) expectations {
	t.Helper()
	var e expectations
	if err := json.Unmarshal([]byte(readCorpus(t, "expectations.json")), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

var maxTh = domain.ThresholdsFor(domain.AggressivenessMax)
