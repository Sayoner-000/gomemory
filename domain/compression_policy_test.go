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
