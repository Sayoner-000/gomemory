package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mem/domain"
)

// HCD-067: las reglas de trabajo que gomemory siembra por defecto exigen el
// grafo antes de editar y antes de buscar, la edición nativa también de los
// archivos de texto y la recuperación de decisiones previas aunque el
// historial HCD salga vacío. Así lo recibe cualquier proyecto y runtime sin
// depender de una preferencia de cada persona.
func TestHCD067_ReglasDeTrabajoV4(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRootHCD065(t), "infrastructure", "templates", "agent-preamble.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Se comparan las frases con los espacios normalizados: el ajuste de línea
	// del Markdown no cambia lo que dicen las reglas.
	reglas := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{
		"<!-- gomemory-workrules-v4 -->",
		"**Antes de editar, el grafo**",
		"aunque sea una línea",
		"quién llama a lo que cambia",
		"antes de cualquier búsqueda de texto",
		"archivos de texto del proyecto",
		"`search_memories`",
		"`hcd/`",
		"no preguntes qué significa",
	} {
		if !strings.Contains(reglas, want) {
			t.Errorf("la plantilla de reglas no contiene %q", want)
		}
	}
}

// Las reglas v3 que un proyecto no editó se actualizan solas a la v4; las que
// el equipo editó se respetan. La v3 se toma del tag v2.59.0, la última
// release que la distribuyó.
func TestHCD067_V3IntactaSeActualiza(t *testing.T) {
	v3, err := exec.Command("git", "-C", repoRootHCD065(t), "show", "v2.59.0:infrastructure/templates/agent-preamble.md").Output()
	if err != nil {
		t.Skipf("sin el tag v2.59.0 no se puede recuperar la v3: %v", err)
	}
	rules, ok := domain.PinnedDocByAlias("rules")
	if !ok {
		t.Fatal("el catálogo no tiene el documento rules")
	}
	if !strings.Contains(string(v3), "gomemory-workrules-v3") {
		t.Fatalf("v2.59.0 no distribuía la v3")
	}
	if !rules.IsPreviousDefault(string(v3)) {
		t.Error("la v3 intacta debe reconocerse como plantilla anterior para que mem update la actualice")
	}
	if rules.IsPreviousDefault(string(v3) + "\n- regla propia del equipo\n") {
		t.Error("una v3 editada por el equipo no debe tratarse como plantilla anterior")
	}
	if rules.DefaultSHA256 == domain.ContentFingerprint(string(v3)) {
		t.Error("la huella vigente sigue siendo la de la v3")
	}
}
