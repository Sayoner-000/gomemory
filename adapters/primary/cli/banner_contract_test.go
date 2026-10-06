package cli

import "testing"

func TestBannerDoesNotDependOnSearchContent(t *testing.T) {
	for _, args := range [][]string{{"prueba"}, {"json"}, {"prueba", "json"}, {"-n", "5", "json"}, {"--", "--json"}, {"prueba", "--json"}} {
		if !commandHasBanner("search", args) {
			t.Errorf("consulta altera cabecera: %v", args)
		}
	}
	if commandHasBanner("usage", []string{"--session", "json", "--json"}) {
		t.Fatal("flag JSON real debe conservar stdout limpio")
	}
	if commandHasBanner("doctor", []string{"--json"}) {
		t.Fatal("doctor JSON tiene cabecera")
	}
	if !commandHasBanner("save", []string{"-t", "json", "contenido"}) {
		t.Fatal("título altera cabecera")
	}
}
