package domain

import "testing"

func TestParseCompressionLevel(t *testing.T) {
	cases := []struct {
		level  string
		legacy bool
		want   string
	}{
		{"", false, CompressionLevelStructural},
		{"", true, CompressionLevelNone},
		{"max", true, CompressionLevelMax}, // el ajuste nuevo manda sobre el heredado
		{"none", false, CompressionLevelNone},
		{"maxima", false, CompressionLevelStructural},
	}
	for _, c := range cases {
		if got := ParseCompressionLevel(c.level, c.legacy); got != c.want {
			t.Errorf("ParseCompressionLevel(%q,%v)=%q, quiero %q", c.level, c.legacy, got, c.want)
		}
	}
}

func TestThresholdsFor(t *testing.T) {
	prev := ThresholdsFor(AggressivenessMax)
	if !prev.Enabled || prev.JSONMinItems != 8 {
		t.Fatalf("escalón 3 inesperado: %+v", prev)
	}
	for a := Aggressiveness(2); a >= 1; a-- {
		cur := ThresholdsFor(a)
		// Bajar un escalón conserva más: todos los umbrales suben.
		if cur.JSONMinItems <= prev.JSONMinItems || cur.CodeMinBodyLines <= prev.CodeMinBodyLines ||
			cur.LogMinRun <= prev.LogMinRun || cur.ProseMaxSentences <= prev.ProseMaxSentences {
			t.Errorf("escalón %d no es más conservador que el anterior: %+v vs %+v", a, cur, prev)
		}
		prev = cur
	}
	if ThresholdsFor(AggressivenessOff).Enabled {
		t.Error("escalón 0 debe desactivar el motor")
	}
	if ThresholdsFor(9) != ThresholdsFor(3) {
		t.Error("fuera de rango por arriba debe equivaler a 3")
	}
}

// C-001 (acr_961a1676) — el tope configurado nunca queda por debajo de lo que
// una llamada puede necesitar guardar.
func TestEffectiveOriginalsMaxMB(t *testing.T) {
	cases := map[int]int{0: 0, -3: 0, 1: CompressionOriginalsMinMB, CompressionOriginalsMinMB: CompressionOriginalsMinMB, 512: 512}
	for in, want := range cases {
		if got := EffectiveOriginalsMaxMB(in); got != want {
			t.Errorf("EffectiveOriginalsMaxMB(%d) = %d, se esperaba %d", in, got, want)
		}
	}
	if got := EffectiveOriginalsMaxBytes(0); got != CompressionOriginalsMaxBytes {
		t.Errorf("sin ajuste debe aplicar el tope de fábrica: %d", got)
	}
	if got := EffectiveOriginalsMaxBytes(1); got < 2*CompressionMaxInputBytes {
		t.Errorf("el tope efectivo debe cubrir dos entradas máximas: %d", got)
	}
}
