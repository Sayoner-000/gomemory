package native

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"mem/domain"
)

// T018 — SmartCrusher: extremos, errores y atípicos se conservan; el resto se
// resume en un marcador; la salida es JSON válido.
func TestCompressJSONArray(t *testing.T) {
	var items []string
	for i := 0; i < 200; i++ {
		lat := 20 + i%5
		extra := ""
		if i == 150 {
			lat = 5000 // atípico
		}
		if i == 40 || i == 90 || i == 170 {
			extra = `, "error": "timeout contra 10.0.0.9"`
		}
		items = append(items, fmt.Sprintf(`{"id": %d, "status": 200, "latency_ms": %d%s}`, i, lat, extra))
	}
	in := "[\n  " + strings.Join(items, ",\n  ") + "\n]"
	out, n := compressJSON(in, "", "abc123abc123", maxTh)
	if n == 0 {
		t.Fatal("se esperaba compresión")
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("salida JSON inválida:\n%s", out)
	}
	for _, must := range []string{`"id": 0,`, `"id": 199,`, `"id": 40,`, `"id": 90,`, `"id": 170,`, `"id": 150,`} {
		if !strings.Contains(out, must) {
			t.Errorf("falta el elemento %s", must)
		}
	}
	var arr []map[string]any
	_ = json.Unmarshal([]byte(out), &arr)
	markers := 0
	for _, e := range arr {
		if _, ok := e[domain.MarkerTag]; ok {
			markers++
			if e["ref"] != "abc123abc123" {
				t.Errorf("marcador sin ref: %v", e)
			}
		}
	}
	if markers == 0 {
		t.Error("falta el marcador de omisión")
	}
	if err := checkLiteral(in, out); err != nil {
		t.Error(err)
	}
}

func TestCompressJSONShortArrayIntact(t *testing.T) {
	in := `[{"a":1},{"a":2},{"a":3}]`
	if out, n := compressJSON(in, "", "r", maxTh); n != 0 || out != in {
		t.Errorf("un array corto debe quedar intacto: %s", out)
	}
}

func TestCompressJSONCorpusSaving(t *testing.T) {
	for _, name := range []string{"golist.json", "gotest.jsonl"} {
		in := readCorpus(t, name)
		_, lang := classify(in)
		out, _ := compressJSON(in, lang, "abc123abc123", maxTh)
		saving := 100 * (1 - float64(approxTokens(out))/float64(approxTokens(in)))
		if saving < 70 {
			t.Errorf("%s: ahorro %.1f%% < 70%%", name, saving)
		}
		if lang == "" && !json.Valid([]byte(out)) {
			t.Errorf("%s: salida JSON inválida", name)
		}
		if err := checkLiteral(in, out); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// C-005 (acr_961a1676) — un valor con caracteres de control en los campos
// resumidos no puede sacar la salida de JSON válido: las marcas se escriben
// con escapes JSON, no con los de Go (\a, \x01).
func TestCompressJSONMarkerWithControlCharsStaysValid(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 40; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id":%d,"k":"a\u0007b\u007f<c>"}`, i)
	}
	sb.WriteString("]")
	out, n := compressJSON(sb.String(), "json", "abcdefabcdef", maxTh)
	if n == 0 {
		t.Fatal("se esperaba compresión")
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("la salida comprimida debe seguir siendo JSON válido:\n%s", out)
	}
}
