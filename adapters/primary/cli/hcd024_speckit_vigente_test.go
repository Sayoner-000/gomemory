package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"mem/domain"
)

type topicsFijos map[string]string

func (f topicsFijos) ByTopicKey(_, key string) (*domain.Memory, error) {
	if c, ok := f[key]; ok {
		return &domain.Memory{Content: c}, nil
	}
	return nil, nil
}

// HCD-024: la copia de Spec Kit se compara con la constitución vigente del
// proyecto, no con la plantilla del binario.
func TestSpeckitConstitution_UsaLaVigente(t *testing.T) {
	doc, ok := domain.PinnedDocByAlias("constitution")
	if !ok {
		t.Fatal("falta el documento constitution en el catálogo")
	}
	personal := "# Constitución del equipo\n\n## 25. Excepciones del proyecto\n"
	topics := topicsFijos{doc.TopicKey: personal}
	vigente := speckitConstitutionSource(topics, "p", doc)
	if vigente != personal {
		t.Fatalf("la fuente debe ser la constitución vigente en memoria, no la plantilla")
	}

	root := t.TempDir()
	ruta := filepath.Join(root, ".specify", "memory", "constitution.md")
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(personal), 0o644); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	refreshSpeckitConstitution(&b, root, doc, vigente)
	if b.Len() != 0 {
		t.Errorf("sincronizada con la vigente no debe avisar: %q", b.String())
	}

	// Sin memoria, la vigente es la plantilla del binario.
	if got := speckitConstitutionSource(topicsFijos{}, "p", doc); got != embeddedTemplate(doc.Template) {
		t.Error("sin constitución en memoria, la vigente es la plantilla")
	}
}
