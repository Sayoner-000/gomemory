package domain

import (
	"strings"
	"testing"
)

// TestPinnedDocs_CatalogoBienFormado protege las invariantes del catálogo: son
// las que permiten que CLI y TUI lo recorran sin casos especiales (feature 021,
// contracts/pinned-docs.md §1). Un catálogo mal formado rompería la resolución
// por alias y el estado derivado en silencio, no con un error.
func TestPinnedDocs_CatalogoBienFormado(t *testing.T) {
	if len(PinnedDocs) == 0 {
		t.Fatal("el catálogo de documentos fijados no puede estar vacío")
	}

	alias := map[string]bool{}
	topics := map[string]bool{}

	for _, d := range PinnedDocs {
		if strings.TrimSpace(d.Alias) == "" {
			t.Errorf("documento con alias vacío: %+v", d)
		}
		if alias[d.Alias] {
			t.Errorf("alias duplicado en el catálogo: %q", d.Alias)
		}
		alias[d.Alias] = true

		if !strings.HasPrefix(d.TopicKey, "gomemory:") {
			t.Errorf("%s: la clave de tópico debe llevar el prefijo gomemory: (tiene %q)", d.Alias, d.TopicKey)
		}
		if topics[d.TopicKey] {
			t.Errorf("clave de tópico duplicada en el catálogo: %q", d.TopicKey)
		}
		topics[d.TopicKey] = true

		if strings.TrimSpace(d.Title) == "" {
			t.Errorf("%s: título vacío", d.Alias)
		}
		if strings.TrimSpace(d.Label) == "" {
			t.Errorf("%s: rótulo de TUI vacío", d.Alias)
		}
		if !strings.HasSuffix(d.Template, ".md") {
			t.Errorf("%s: nombre de plantilla inesperado %q", d.Alias, d.Template)
		}
		if ValidMemoryType(string(d.Type)) != d.Type {
			t.Errorf("%s: tipo de memoria inválido %q", d.Alias, d.Type)
		}
	}
}

// TestPinnedDocs_ClavesCanonicasPresentes fija el contrato de las dos semillas
// que la feature 021 siembra. Cambiar estas claves huerfanaría las semillas ya
// creadas en proyectos existentes (data-model.md §1), así que el test las clava.
func TestPinnedDocs_ClavesCanonicasPresentes(t *testing.T) {
	if TopicWorkRules != "gomemory:work-rules" {
		t.Errorf("TopicWorkRules cambió a %q: huerfanaría las semillas existentes", TopicWorkRules)
	}
	if TopicConstitution != "gomemory:constitution" {
		t.Errorf("TopicConstitution cambió a %q: huerfanaría las semillas existentes", TopicConstitution)
	}

	rules, ok := PinnedDocByAlias("rules")
	if !ok {
		t.Fatal("el alias 'rules' debe existir en el catálogo")
	}
	if rules.TopicKey != TopicWorkRules || rules.Type != Preference {
		t.Errorf("rules mal declarado: %+v", rules)
	}

	consti, ok := PinnedDocByAlias("constitution")
	if !ok {
		t.Fatal("el alias 'constitution' debe existir en el catálogo")
	}
	if consti.TopicKey != TopicConstitution || consti.Type != Architecture {
		t.Errorf("constitution mal declarado: %+v", consti)
	}
}

func TestPinnedDocByAlias_Desconocido(t *testing.T) {
	if _, ok := PinnedDocByAlias("no-existe"); ok {
		t.Error("un alias desconocido no debe resolver")
	}
}

func TestPinnedDocByTopicKey(t *testing.T) {
	d, ok := PinnedDocByTopicKey(TopicConstitution)
	if !ok || d.Alias != "constitution" {
		t.Errorf("PinnedDocByTopicKey(%q) = %+v, %v", TopicConstitution, d, ok)
	}
	if _, ok := PinnedDocByTopicKey("equipo:otro"); ok {
		t.Error("una clave fuera del catálogo no debe resolver")
	}
}

func TestPinnedDocAliases(t *testing.T) {
	got := PinnedDocAliases()
	if len(got) != len(PinnedDocs) {
		t.Fatalf("PinnedDocAliases devolvió %d alias, el catálogo tiene %d", len(got), len(PinnedDocs))
	}
}
