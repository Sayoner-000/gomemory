package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type hcd002Memory struct {
	Type, Title, Content string
	TopicKey             string `json:"topic_key"`
}

type hcd002Corpus struct {
	Profile struct {
		CuratedTarget int                `json:"curated_target"`
		CuratedMix    map[string]float64 `json:"curated_mix_pct"`
		CharsMedian   int                `json:"content_chars_median"`
		CharsP95      int                `json:"content_chars_p95"`
	}
	Projects map[string]struct {
		Bundle json.RawMessage
		SHA256 string
	}
	Cases []struct {
		ID, Category, Project string
		RequiredVisible       []string `json:"required_visible"`
		Recover               []struct{ Title, SHA256 string }
	}
}

func hcd002Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hcd002Rank(values []int, q float64) int {
	sort.Ints(values)
	return values[int(math.Ceil(q*float64(len(values))))-1]
}

// TestHCD002CorpusContract valida el corpus sintético fijado de HCD-002: es
// reproducible desde su generador, está calibrado con el perfil de uso real
// (tarea 1), declara literales y recuperaciones verificables y no contiene
// patrones de credenciales (C-04, C-05).
func TestHCD002CorpusContract(t *testing.T) {
	data, err := os.ReadFile("../fixtures/hcd002/corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	secret := regexp.MustCompile(`ghp_|github_pat_|sk-[A-Za-z0-9]{20}|AKIA[0-9A-Z]{16}|-----BEGIN|<private>`)
	if secret.Match(data) {
		t.Fatal("el corpus contiene un patrón de credencial o una marca privada")
	}
	if regen, err := exec.Command("python3", "../../scripts/hcd002_corpus.py").Output(); err != nil || !bytes.Equal(regen, data) {
		t.Fatalf("el corpus no es reproducible desde su generador: %v", err)
	}
	var c hcd002Corpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}

	memories := map[string][]hcd002Memory{}
	for name, p := range c.Projects {
		var canonical any
		if err := json.Unmarshal(p.Bundle, &canonical); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(canonical)
		if got := hcd002Hash(bytes.TrimSuffix(buf.Bytes(), []byte("\n"))); got != p.SHA256 {
			t.Errorf("huella del proyecto %s: %s != %s", name, got, p.SHA256)
		}
		var b struct{ Memories []hcd002Memory }
		if err := json.Unmarshal(p.Bundle, &b); err != nil {
			t.Fatal(err)
		}
		memories[name] = b.Memories
	}

	// Calibración del proyecto representativo con el perfil de la tarea 1.
	var curated []hcd002Memory
	pinned := 0
	for _, m := range memories["representativo"] {
		if m.TopicKey == "gomemory:work-rules" {
			pinned++
		}
		if m.Type != "checkpoint" {
			curated = append(curated, m)
		}
	}
	if pinned != 1 || math.Abs(float64(len(curated)-c.Profile.CuratedTarget)) > 0.05*float64(c.Profile.CuratedTarget) {
		t.Fatalf("representativo: %d curadas (objetivo %d), %d reglas fijadas", len(curated), c.Profile.CuratedTarget, pinned)
	}
	byType := map[string]int{}
	var lengths []int
	for _, m := range curated {
		byType[m.Type]++
		lengths = append(lengths, len(m.Content))
	}
	for kind, pct := range c.Profile.CuratedMix {
		if got := 100 * float64(byType[kind]) / float64(len(curated)); math.Abs(got-pct) > 3 {
			t.Errorf("mezcla %s: %.1f%% frente a %.1f%% del perfil", kind, got, pct)
		}
	}
	for q, want := range map[float64]int{0.5: c.Profile.CharsMedian, 0.95: c.Profile.CharsP95} {
		if got := hcd002Rank(append([]int(nil), lengths...), q); math.Abs(float64(got-want)) > 0.15*float64(want) {
			t.Errorf("longitud q%.2f: %d frente a %d del perfil (tolerancia 15%%)", q, got, want)
		}
	}

	// Casos: 20 identificadores únicos, cinco por categoría, con literales y
	// recuperaciones que existen en su proyecto.
	seen, perCategory := map[string]bool{}, map[string]int{}
	for _, cs := range c.Cases {
		if seen[cs.ID] {
			t.Errorf("caso duplicado %s", cs.ID)
		}
		seen[cs.ID] = true
		perCategory[cs.Category]++
		mems, ok := memories[cs.Project]
		if !ok || len(cs.RequiredVisible) == 0 {
			t.Errorf("%s: proyecto %q inexistente o sin literales visibles", cs.ID, cs.Project)
			continue
		}
		for _, lit := range cs.RequiredVisible {
			found := false
			for _, m := range mems {
				found = found || strings.Contains(m.Title, lit) || strings.Contains(m.Content, lit)
			}
			if !found {
				t.Errorf("%s: literal %q no existe en %s", cs.ID, lit, cs.Project)
			}
		}
		if (cs.Category == "recuperacion") != (len(cs.Recover) > 0) {
			t.Errorf("%s: solo los casos de recuperación declaran originales", cs.ID)
		}
		for _, r := range cs.Recover {
			found := false
			for _, m := range mems {
				found = found || (m.Title == r.Title && hcd002Hash([]byte(m.Content)) == r.SHA256)
			}
			if !found {
				t.Errorf("%s: original %q sin huella coincidente", cs.ID, r.Title)
			}
		}
	}
	for _, cat := range []string{"arranque", "compactacion", "full", "recuperacion"} {
		if perCategory[cat] != 5 {
			t.Errorf("categoría %s: %d casos, se esperaban 5", cat, perCategory[cat])
		}
	}
	if len(seen) != 20 {
		t.Errorf("se esperaban 20 casos, hay %d", len(seen))
	}
}
