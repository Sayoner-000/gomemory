package native

import (
	"strings"
	"testing"

	"mem/domain"
)

// T016 — cada muestra del corpus se clasifica con el tipo esperado.
func TestRouterCorpus(t *testing.T) {
	for name, want := range loadExpectations(t).Samples {
		got, _ := classify(readCorpus(t, name))
		if string(got) != want.Type {
			t.Errorf("%s: tipo %s, quiero %s", name, got, want.Type)
		}
	}
}

func TestRouterFencedSegments(t *testing.T) {
	doc := "Texto de introducción.\n\n```go\npackage p\n\nfunc F() {}\n```\n\nMás texto.\n```json\n{\"a\": 1}\n```\n"
	blocks := segment(doc)
	var fenced []domain.ContentType
	for _, b := range blocks {
		if b.fenced {
			fenced = append(fenced, b.typ)
		}
	}
	if len(fenced) != 2 || fenced[0] != domain.ContentCode || fenced[1] != domain.ContentJSON {
		t.Fatalf("segmentos inesperados: %+v", blocks)
	}
	var rebuilt strings.Builder
	for _, b := range blocks {
		rebuilt.WriteString(b.open + b.content + b.close)
	}
	if rebuilt.String() != doc {
		t.Error("segmentar y reunir debe reproducir la entrada byte a byte")
	}
}

func TestRouterInvalidJSONNotJSON(t *testing.T) {
	for _, s := range []string{`{"a": 1, "b": [1, 2`, `[{"x":1},{"x":2}`} {
		if got, _ := classify(s); got == domain.ContentJSON {
			t.Errorf("JSON truncado clasificado como json: %q", s)
		}
	}
}

func TestRouterUnclosedFenceIsText(t *testing.T) {
	doc := "intro\n```go\nfunc sinCierre() {\n"
	for _, b := range segment(doc) {
		if b.fenced {
			t.Fatal("una cerca sin cerrar no debe inventar un bloque")
		}
	}
}
