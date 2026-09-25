package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/compression/native"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
)

// T025 — SC-001, SC-002 y SC-005 sobre el corpus real, con el almacén real.
func TestCompressionCorpus(t *testing.T) {
	root := repoRootIntegration(t)
	corpus := filepath.Join(root, "tests", "testdata", "compression_corpus")
	var exp struct {
		Corpus  float64 `json:"corpus_min_saving_vs_structural_pct"`
		Samples map[string]struct {
			MinRaw float64 `json:"min_saving_vs_raw_pct"`
		} `json:"samples"`
	}
	raw, err := os.ReadFile(filepath.Join(corpus, "expectations.json"))
	if err != nil || json.Unmarshal(raw, &exp) != nil {
		t.Fatalf("expectations.json: %v", err)
	}

	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store := persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
	engine := native.NewEngine(store, nil, nil, "corpus")

	var totalStructural, totalMax int
	for name, want := range exp.Samples {
		in, err := os.ReadFile(filepath.Join(corpus, name))
		if err != nil {
			t.Fatal(err)
		}
		st, _ := compression.StructuralCompressor{}.Compress(string(in), ports.CompressionOptions{Level: ports.CompressionStructural})
		mx, _ := engine.Compress(string(in), ports.CompressionOptions{Level: ports.CompressionMax})
		totalStructural += st.Tokens
		totalMax += mx.Tokens
		saving := 100 * (1 - float64(mx.Tokens)/float64(mx.RawTokens))
		t.Logf("%-20s raw=%6d structural=%6d max=%6d (-%.1f%%) %s %s", name, mx.RawTokens, st.Tokens, mx.Tokens, saving, mx.Compressor, mx.FallbackReason)
		if saving+1e-9 < want.MinRaw {
			t.Errorf("%s: ahorro %.1f%% < %.0f%%", name, saving, want.MinRaw)
		}
		// SC-005: cada ref recupera el bloque original, que contiene lo omitido.
		for _, ref := range mx.Refs {
			orig, _, ok, err := store.Get(context.Background(), ref)
			if err != nil || !ok {
				t.Errorf("%s: ref %s no recuperable: %v", name, ref, err)
				continue
			}
			if !strings.Contains(string(in), orig) {
				t.Errorf("%s: el original de %s no es un fragmento literal de la entrada", name, ref)
			}
		}
		if len(mx.Refs) == 1 && mx.Refs[0] != "" {
			if orig, _, _, _ := store.Get(context.Background(), mx.Refs[0]); orig != string(in) && !strings.Contains(mx.Content, "```") {
				t.Errorf("%s: la ref de un bloque único debe devolver la entrada byte a byte", name)
			}
		}
	}
	saving := 100 * (1 - float64(totalMax)/float64(totalStructural))
	t.Logf("corpus: structural=%d max=%d → -%.1f%% frente a structural", totalStructural, totalMax, saving)
	if saving < exp.Corpus {
		t.Errorf("SC-001: ahorro del corpus %.1f%% < %.0f%% frente a structural", saving, exp.Corpus)
	}
}

// FR-015 — las memorias guardadas nunca se alteran por la compresión.
func TestMemoriesNeverAltered(t *testing.T) {
	bin := buildMemBinary(t)
	env := storeAislado(t)
	dir := proyectoCompresion(t, `{"context_compression_level": "max", "budget": -1}`)
	contenido, err := os.ReadFile(filepath.Join(repoRootIntegration(t), "tests", "testdata", "compression_corpus", "gotest-fail.log"))
	if err != nil {
		t.Fatal(err)
	}
	correrMem(t, bin, dir, env, "", "session", "start")
	if r := correrMem(t, bin, dir, env, "", "save", "-t", "log de fallo grande", "-y", "bugfix", string(contenido)); r.code != 0 {
		t.Fatalf("save: %s", r.stderr)
	}
	for _, args := range [][]string{{"context"}, {"search", "fallo"}, {"pack", "build", "--task", "log de fallo", "--max-tokens", "40000"}, {"get", "1"}} {
		correrMem(t, bin, dir, env, "", args...)
	}
	db := baseDelStore(t, env)
	defer func() { _ = db.Close() }()
	var guardado string
	if err := db.QueryRow(`SELECT content FROM memories WHERE title = 'log de fallo grande'`).Scan(&guardado); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(guardado) != strings.TrimSpace(string(contenido)) {
		t.Error("la memoria guardada cambió tras entregarla comprimida")
	}
	if strings.Contains(guardado, "⟦mem⟧") {
		t.Error("la memoria guardada contiene marcadores")
	}
}
