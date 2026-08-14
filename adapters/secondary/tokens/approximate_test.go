package tokens

import "testing"

func TestApproximateTokenCounter_EmptyString(t *testing.T) {
	c := ApproximateTokenCounter{}
	if got := c.Count(""); got != 0 {
		t.Fatalf("Count(\"\") = %d, want 0", got)
	}
}

func TestApproximateTokenCounter_Deterministic(t *testing.T) {
	c := ApproximateTokenCounter{}
	text := "El servicio de auth usa Redis para guardar sesiones de refresh token."
	first := c.Count(text)
	second := c.Count(text)
	if first != second {
		t.Fatalf("Count no es determinista: %d != %d para el mismo input", first, second)
	}
	if first <= 0 {
		t.Fatalf("Count(text no vacío) = %d, want > 0", first)
	}
}

func TestApproximateTokenCounter_ProportionalToLength(t *testing.T) {
	c := ApproximateTokenCounter{}
	short := c.Count("hola")
	long := c.Count("hola hola hola hola hola hola hola hola hola hola")
	if long <= short {
		t.Fatalf("Count(texto largo)=%d debería ser mayor que Count(texto corto)=%d", long, short)
	}
}

func TestApproximateTokenCounter_TableDriven(t *testing.T) {
	c := ApproximateTokenCounter{}
	cases := []struct {
		name string
		in   string
		min  int
	}{
		{"una palabra", "hola", 1},
		{"frase corta", "hola mundo", 1},
		{"con puntuación", "¿Cómo estás? Bien, gracias.", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.Count(tc.in); got < tc.min {
				t.Errorf("Count(%q) = %d, want >= %d", tc.in, got, tc.min)
			}
		})
	}
}
