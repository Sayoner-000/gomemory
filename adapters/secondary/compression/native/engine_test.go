package native

import (
	"errors"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/compression"
	"mem/application/ports"
)

var maxOpts = ports.CompressionOptions{Level: ports.CompressionMax}

// T022 — comportamiento del Engine.
func TestEngineStructuralLevelsUnchanged(t *testing.T) {
	in := readCorpus(t, "gotest-fail.log")
	e := NewEngine(newMemStore(), nil, nil, "p")
	for _, lvl := range []ports.CompressionLevel{ports.CompressionStructural, ports.CompressionNone} {
		want, _ := compression.StructuralCompressor{}.Compress(in, ports.CompressionOptions{Level: lvl})
		got, _ := e.Compress(in, ports.CompressionOptions{Level: lvl})
		if got.Content != want.Content {
			t.Errorf("nivel %d: la salida debe ser la de la v2.25.0 (INV-C2)", lvl)
		}
	}
}

func TestEngineSmallBlockIntact(t *testing.T) {
	in := "error corto   con   espacios"
	r, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(in, maxOpts)
	if r.Content != in {
		t.Errorf("FR-004: por debajo del umbral debe salir sin cambios, salió %q", r.Content)
	}
}

func TestEngineDeterministic(t *testing.T) {
	in := readCorpus(t, "golist.json")
	e := NewEngine(newMemStore(), nil, nil, "p")
	first, _ := e.Compress(in, maxOpts)
	for i := 0; i < 100; i++ {
		if r, _ := e.Compress(in, maxOpts); r.Content != first.Content {
			t.Fatalf("iteración %d: salida distinta (FR-003)", i)
		}
	}
}

func TestEngineFallbacks(t *testing.T) {
	big := strings.Repeat("2026-09-25 10:00:00 INFO línea de relleno para superar el tope\n", 40000)
	if r, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(big, maxOpts); r.FallbackReason != "too_large" {
		t.Errorf("entrada > 2 MB: fallback %q", r.FallbackReason)
	}
	prose := strings.Repeat("Una frase distinta número uno sobre el sistema. ", 1) + "Otra frase que añade contexto útil y no se repite en ningún sitio del texto de prueba."
	if r, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(prose+" "+prose[:40], maxOpts); r.Tokens > r.StructuralTokens && r.Compressor != "none" {
		t.Errorf("INV-C3 violado: %d > %d", r.Tokens, r.StructuralTokens)
	}
}

func TestEngineNoGain(t *testing.T) {
	// Prosa sin frases repetidas ni párrafos largos: el motor no gana nada.
	in := "Primera línea con contenido variado: alfa, beta, gama, delta, épsilon y dseta sin repetir nada.\n" +
		"Segunda línea distinta que habla de eta, zeta, iota, kappa, lambda, mu y nu con otras palabras.\n" +
		"Tercera línea que menciona ómicron, pi, ro, sigma, tau e ípsilon, también sin duplicados.\n" +
		"Cuarta y última línea sobre fi, ji, psi y omega, para superar el umbral mínimo de tokens.\n"
	r, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(in, maxOpts)
	if r.Compressor != "structural" || r.FallbackReason != "no_gain" {
		t.Errorf("sin ganancia debe entregar la estructural con no_gain: %q %q", r.Compressor, r.FallbackReason)
	}
}

// panicCompressor provoca un pánico dentro de compressBlock.
func TestEnginePanicRecovered(t *testing.T) {
	b := block{content: "x", typ: "json"}
	e := NewEngine(newMemStore(), nil, nil, "p")
	// JSON inválido para el escáner: fuerza un índice fuera de rango.
	b.content = "["
	out, n, _, err := e.compressBlock(b)
	if err == nil || n != 0 || out != b.content {
		t.Errorf("un pánico debe devolver el bloque literal y error: out=%q n=%d err=%v", out, n, err)
	}
}

func TestEngineStoreUnavailableAndTimeout(t *testing.T) {
	in := readCorpus(t, "gotest-fail.log")
	failing := newMemStore()
	failing.failPut = errors.New("disco lleno")
	if r, _ := NewEngine(failing, nil, nil, "p").Compress(in, maxOpts); r.FallbackReason != "store_unavailable" || len(r.Refs) != 0 {
		t.Errorf("almacén caído: %q refs=%v", r.FallbackReason, r.Refs)
	}
	slow := newMemStore()
	slow.delay = time.Second
	e := NewEngine(slow, nil, nil, "p")
	e.StoreTimeout = 20 * time.Millisecond
	start := time.Now()
	r, _ := e.Compress(in, maxOpts)
	if r.FallbackReason != "store_unavailable" || time.Since(start) > 500*time.Millisecond {
		t.Errorf("almacén lento: %q en %v", r.FallbackReason, time.Since(start))
	}
	if r, _ := NewEngine(nil, nil, nil, "p").Compress(in, maxOpts); r.FallbackReason != "store_unavailable" {
		t.Errorf("sin almacén: %q", r.FallbackReason)
	}
}

func TestEnginePrivate(t *testing.T) {
	in := strings.Repeat("token <private>ghp_FAKE0000000000000000000000000000000</private>\n", 200)
	store := newMemStore()
	r, _ := NewEngine(store, nil, nil, "p").Compress(in, maxOpts)
	if r.FallbackReason != "private" || len(r.Refs) != 0 || store.puts != 0 {
		t.Errorf("privado: fallback=%q refs=%v puts=%d", r.FallbackReason, r.Refs, store.puts)
	}
}

func TestEngineStatsBestEffort(t *testing.T) {
	in := readCorpus(t, "gotest-fail.log")
	stats := &memStats{fail: true}
	store := newMemStore()
	r, err := NewEngine(store, stats, nil, "p").Compress(in, maxOpts)
	if err != nil || len(r.Refs) != 1 {
		t.Fatalf("un fallo de estadísticas no debe afectar: err=%v refs=%v", err, r.Refs)
	}
	if len(stats.records) != 1 || stats.records[0].Omissions == 0 {
		t.Errorf("se esperaba un Record con omisiones: %+v", stats.records)
	}
	// La ref recupera el original exacto.
	if got, _, ok, _ := store.Get(nil, r.Refs[0]); !ok || got != in {
		t.Error("la ref debe recuperar el original byte a byte")
	}
	if r.Tokens > r.StructuralTokens {
		t.Errorf("INV-C3: %d > %d", r.Tokens, r.StructuralTokens)
	}
}

func TestReplaceBlockRefPreservesCollidingBlocks(t *testing.T) {
	short := "0123456789ab"
	long := "0123456789abcdef"
	first := "first ref=" + short + "\n"
	second := `[{"ref": "` + short + `", "⟦mem⟧": "uno"}, {"ref": "` + short + `", "⟦mem⟧": "dos"}, {"ref": "` + short + `", "literal": "` + short + `"}]`
	input := first + second
	p := pendingOriginal{ref: short, storedRef: long, start: len(first), end: len(input)}
	want := first + `[{"ref": "` + long + `", "⟦mem⟧": "uno"}, {"ref": "` + long + `", "⟦mem⟧": "dos"}, {"ref": "` + short + `", "literal": "` + short + `"}]`
	if got := replaceBlockRef(input, p); got != want {
		t.Fatalf("la expansión debe afectar todas las marcas del bloque colisionado y ninguna del anterior:\n got %q\nwant %q", got, want)
	}

	text := "literal ref=" + short + " ⟦mem⟧ omitido · ref=" + short + " literal ref=" + short + "\n"
	if got := replaceMarkerRefs(text, short, long); got != "literal ref="+short+" ⟦mem⟧ omitido · ref="+long+" literal ref="+short+"\n" {
		t.Fatalf("la expansión textual alteró contenido literal: %q", got)
	}
	quoted := `["⟦mem⟧ omitido · ref=` + short + `", "literal ref=` + short + `"]`
	quotedWant := `["⟦mem⟧ omitido · ref=` + long + `", "literal ref=` + short + `"]`
	if got := replaceMarkerRefs(quoted, short, long); got != quotedWant {
		t.Fatalf("marcador escalar JSON equivocado: got %q want %q", got, quotedWant)
	}
}

// contadorFijo devuelve siempre n tokens, sea cual sea el texto.
type contadorFijo int

func (c contadorFijo) Count(string) int { return int(c) }

// C-002 (acr_0814de3a) — el motor decide con el contador inyectado, el mismo
// de BuildContextPack, y no con su heurística interna.
func TestEngineDecidesWithInjectedCounter(t *testing.T) {
	in := readCorpus(t, "gotest-fail.log")
	if r, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(in, maxOpts); !r.Compressed || r.FallbackReason != "" {
		t.Fatalf("control: con la heurística interna el corpus se comprime: %+v", r.FallbackReason)
	}

	// Para este contador la salida no gana tokens: el motor tiene que degradar.
	e := NewEngine(newMemStore(), nil, nil, "p")
	e.Counter = contadorFijo(1000)
	r, _ := e.Compress(in, maxOpts)
	if r.FallbackReason != "no_gain" || r.RawTokens != 1000 || r.StructuralTokens != 1000 {
		t.Errorf("la ganancia debe medirse con el contador inyectado: %+v", r)
	}

	// Por debajo del umbral según el contador inyectado, sale intacto (FR-004).
	e.Counter = contadorFijo(1)
	if r, _ := e.Compress(in, maxOpts); r.Content != in || r.Compressor != "none" {
		t.Errorf("el umbral mínimo debe medirse con el contador inyectado: %q", r.Compressor)
	}
}
