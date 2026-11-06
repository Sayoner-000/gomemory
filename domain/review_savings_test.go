package domain

import "testing"

// HCD-035: el ahorro de una revisión enlazada es lo que el revisor no necesita
// releer (alcance fuera del delta) menos lo que recibe de más (heredados).
func TestReviewSavings(t *testing.T) {
	files := map[string]int{"a.go": 100, "b.go": 300, "dir/c.go": 50}
	got := MeasureSinceSavings(files, []string{"./dir/c.go", "otro/fuera.go"}, 20)
	want := SinceSavings{Measurable: true, ScopeFiles: 3, DeltaFiles: 1, ScopeTokens: 450, DeltaTokens: 50, InheritedTokens: 20, AvoidedTokens: 380}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	// Si todo cambió y además hay heredados, el ahorro es negativo: se informa.
	if got := MeasureSinceSavings(map[string]int{"a.go": 10}, []string{"a.go"}, 5); got.AvoidedTokens != -5 {
		t.Fatalf("ahorro negativo: %+v", got)
	}
}
