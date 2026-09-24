package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/domain"
)

// TestPrintSeedReport: lo que ve la persona durante `mem install` y
// `mem update` dice qué se actualizó solo y cómo adoptar la versión nueva
// cuando el equipo editó el documento.
func TestPrintSeedReport(t *testing.T) {
	var b bytes.Buffer
	printSeedReport(&b, seedReport{
		Upgraded:   []string{domain.TopicConstitution},
		Customized: []string{domain.TopicWorkRules},
	})
	out := b.String()
	for _, q := range []string{
		"constitution actualizada a la versión por defecto",
		"rules tiene ediciones del equipo y no se modificó",
		"mem docs export rules -o respaldo.md && mem docs reset rules",
	} {
		if !strings.Contains(out, q) {
			t.Errorf("falta %q en:\n%s", q, out)
		}
	}

	b.Reset()
	printSeedReport(&b, seedReport{})
	if !strings.Contains(b.String(), "al día") {
		t.Errorf("sin cambios debía decir que las semillas están al día: %q", b.String())
	}
}

// TestRefreshSpeckitConstitution: la copia de spec-kit que dejó
// `mem constitution --sync` es la que leen /speckit-plan y compañía. Si es
// intacta de una versión anterior se refresca; si el equipo la editó, no.
func TestRefreshSpeckitConstitution(t *testing.T) {
	doc := domain.PinnedDoc{Alias: "constitution", PreviousDefaultSHA256: []string{domain.ContentFingerprint("VIEJA")}}
	casos := []struct {
		nombre, previo, want string
		aviso                string
	}{
		{"intacta", "VIEJA\n", "NUEVA", "actualizada"},
		{"editada", "DEL EQUIPO", "DEL EQUIPO", "tiene ediciones"},
		{"al día", "NUEVA", "NUEVA", ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			root := t.TempDir()
			ruta := filepath.Join(root, ".specify", "memory", "constitution.md")
			if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ruta, []byte(c.previo), 0o644); err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			refreshSpeckitConstitution(&b, root, doc, "NUEVA")
			got, _ := os.ReadFile(ruta)
			if string(got) != c.want {
				t.Errorf("contenido = %q; want %q", got, c.want)
			}
			if c.aviso == "" && b.Len() != 0 || c.aviso != "" && !strings.Contains(b.String(), c.aviso) {
				t.Errorf("aviso = %q; want que contenga %q", b.String(), c.aviso)
			}
		})
	}

	// Sin spec-kit no se crea nada.
	root := t.TempDir()
	refreshSpeckitConstitution(&bytes.Buffer{}, root, doc, "NUEVA")
	if _, err := os.Stat(filepath.Join(root, ".specify")); !os.IsNotExist(err) {
		t.Error("en un proyecto sin spec-kit no debe crearse .specify/")
	}
}
