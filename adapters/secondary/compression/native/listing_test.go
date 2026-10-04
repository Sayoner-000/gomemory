package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

func grepListingFixture(t testing.TB) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "tests", "contract", "testdata", "tool_output", "claude-grep-listing.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ev struct {
		ToolResponse struct {
			Content string `json:"content"`
		} `json:"tool_response"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatal(err)
	}
	return ev.ToolResponse.Content
}

func findListingFixture(t testing.TB) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "tests", "contract", "testdata", "tool_output", "listing-find.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// T067 (feature 035, US4, FR-023): las búsquedas por texto y los listados de
// rutas eran el grueso de las salidas de agentes y caían en prosa (0,3 % de
// ahorro). Se resumen conservando principio, final, errores y un conteo.
func TestClassify_Listados(t *testing.T) {
	for name, in := range map[string]string{"grep": grepListingFixture(t), "find": findListingFixture(t)} {
		if typ, _ := classify(in); typ != domain.ContentListing {
			t.Errorf("%s: classify = %s, se esperaba listing", name, typ)
		}
	}
	corto := strings.Join(strings.Split(grepListingFixture(t), "\n")[:29], "\n")
	if typ, _ := classify(corto); typ == domain.ContentListing {
		t.Error("menos de 30 líneas no es un listado")
	}
}

func TestCompressListing_CabezaColaErroresYConteo(t *testing.T) {
	lines := strings.Split(strings.TrimRight(grepListingFixture(t), "\n"), "\n")
	lines[200] = "adapters/x/y.go:77: panic: runtime error: index out of range"
	in := strings.Join(lines, "\n")
	th := domain.ThresholdsFor(domain.AggressivenessMax)

	out, n := compressListing(in, "abcdefabcdef", th)
	if n == 0 {
		t.Fatal("un listado de 400 líneas debe resumirse")
	}
	got := strings.Split(out, "\n")
	for i := 0; i < domain.ListingKeepHead; i++ {
		if got[i] != lines[i] {
			t.Fatalf("línea %d de la cabeza alterada: %q", i, got[i])
		}
	}
	for i := 1; i <= domain.ListingKeepTail; i++ {
		if got[len(got)-i] != lines[len(lines)-i] {
			t.Fatalf("línea %d de la cola alterada: %q", i, got[len(got)-i])
		}
	}
	if !strings.Contains(out, lines[200]) {
		t.Error("las líneas con error/FAIL/panic/fatal se conservan siempre")
	}
	if !strings.Contains(out, "ref=abcdefabcdef") || !strings.Contains(out, "líneas omitidas") {
		t.Errorf("lo omitido se marca con su ref y un conteo:\n%s", out)
	}
	if err := checkLiteral(in, out); err != nil {
		t.Errorf("guarda de literalidad: %v", err)
	}
	if ahorro := 1 - float64(len(out))/float64(len(in)); ahorro < 0.40 {
		t.Errorf("ahorro insuficiente: %.0f%% (mínimo 40%%)", 100*ahorro)
	}
}

func TestCompressListing_ConteoPorArchivo(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, "pkg/a.go:%d: línea\n", i)
	}
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "pkg/b.go:%d: línea\n", i)
	}
	out, _ := compressListing(b.String(), "r", domain.ThresholdsFor(domain.AggressivenessMax))
	if !strings.Contains(out, "pkg/a.go") || !strings.Contains(out, "pkg/b.go") {
		t.Errorf("el resumen cuenta lo omitido por archivo:\n%s", out)
	}
}

func TestEngine_ListadoDentroDelPresupuestoDelHook(t *testing.T) {
	in := grepListingFixture(t)
	e := NewEngine(newMemStore(), nil, nil, "p")
	start := time.Now()
	r, _ := e.Compress(in, toolOpts)
	if d := time.Since(start); d > domain.HookBudget {
		t.Errorf("comprimir el listado tardó %v (> %v)", d, domain.HookBudget)
	}
	if !r.Compressed || r.Compressor != string(domain.ContentListing) {
		t.Errorf("el motor debe usar el compresor de listados en modo salida de herramienta: %q %v", r.Compressor, r.Compressed)
	}
}
