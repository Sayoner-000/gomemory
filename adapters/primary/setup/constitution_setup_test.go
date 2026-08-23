package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallConstitutionWrappers_NoLlevanCopiaDelTexto es el test que importa.
//
// El paso que la feature 021 elimina copiaba las 635 líneas de la constitución a
// la raíz del proyecto, donde quedaban congeladas y divergían de la fuente en
// cuanto alguien editaba una de las dos. Repetir ese error dentro del envoltorio
// sería exactamente el mismo fallo con otro nombre.
func TestInstallConstitutionWrappers_NoLlevanCopiaDelTexto(t *testing.T) {
	root := t.TempDir()
	if err := InstallConstitutionWrappers(root); err != nil {
		t.Fatalf("instalar envoltorios: %v", err)
	}

	for _, rel := range []string{
		filepath.Join(".claude", "skills", "constitution", "SKILL.md"),
		filepath.Join(".opencode", "commands", "constitution.md"),
	} {
		datos, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("falta %s: %v", rel, err)
		}
		cuerpo := string(datos)

		// Marcas inconfundibles del documento real.
		for _, marca := range []string{
			"Constitución Genérica",
			"## 1. Stack por Lenguaje",
			"Contenerización",
		} {
			if strings.Contains(cuerpo, marca) {
				t.Errorf("%s incrusta el texto de la constitución (%q): debe resolverla en la invocación", rel, marca)
			}
		}

		// Y sí debe decir cómo obtenerla.
		if !strings.Contains(cuerpo, "mem constitution") {
			t.Errorf("%s debe indicar cómo recuperar la constitución vigente", rel)
		}
		if !strings.Contains(cuerpo, "docs import constitution") {
			t.Errorf("%s debe explicar cómo reemplazarla por la del equipo", rel)
		}
	}
}

func TestInstallConstitutionWrappers_TienenFrontmatter(t *testing.T) {
	root := t.TempDir()
	if err := InstallConstitutionWrappers(root); err != nil {
		t.Fatalf("instalar: %v", err)
	}

	skill, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "constitution", "SKILL.md"))
	if err != nil {
		t.Fatalf("leer skill: %v", err)
	}
	if !strings.HasPrefix(string(skill), "---\nname: constitution\n") {
		t.Errorf("la skill necesita frontmatter con name para que /constitution la descubra:\n%s", primerasLineas(string(skill), 4))
	}

	cmd, err := os.ReadFile(filepath.Join(root, ".opencode", "commands", "constitution.md"))
	if err != nil {
		t.Fatalf("leer comando: %v", err)
	}
	if !strings.HasPrefix(string(cmd), "---\ndescription:") {
		t.Errorf("el comando de opencode necesita frontmatter con description:\n%s", primerasLineas(string(cmd), 4))
	}
}

// Idempotencia: mismo criterio que InstallAtomicPlanWrappers — solo se reescribe
// si el contenido difiere, para no tocar mtime en cada instalación.
func TestInstallConstitutionWrappers_EsIdempotente(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, ".claude", "skills", "constitution", "SKILL.md")

	if err := InstallConstitutionWrappers(root); err != nil {
		t.Fatalf("primera: %v", err)
	}
	antes, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if err := InstallConstitutionWrappers(root); err != nil {
		t.Fatalf("segunda: %v", err)
	}
	despues, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !antes.ModTime().Equal(despues.ModTime()) {
		t.Error("el envoltorio se reescribió sin haber cambiado")
	}
}

// Un envoltorio editado a mano se regenera: la fuente de verdad es el binario,
// misma regla que los envoltorios del método de planificación.
func TestInstallConstitutionWrappers_RegeneraSiFueAlterado(t *testing.T) {
	root := t.TempDir()
	if err := InstallConstitutionWrappers(root); err != nil {
		t.Fatalf("instalar: %v", err)
	}
	dest := filepath.Join(root, ".opencode", "commands", "constitution.md")
	if err := os.WriteFile(dest, []byte("alterado a mano\n"), 0o644); err != nil {
		t.Fatalf("alterar: %v", err)
	}

	if err := InstallConstitutionWrappers(root); err != nil {
		t.Fatalf("reinstalar: %v", err)
	}
	datos, _ := os.ReadFile(dest)
	if strings.Contains(string(datos), "alterado a mano") {
		t.Error("el envoltorio debe regenerarse desde la fuente embebida")
	}
}

func primerasLineas(s string, n int) string {
	ls := strings.Split(s, "\n")
	if len(ls) > n {
		ls = ls[:n]
	}
	return strings.Join(ls, "\n")
}
