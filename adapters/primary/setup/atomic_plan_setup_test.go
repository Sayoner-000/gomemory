package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const metodoDePrueba = "# Descomposición Atómica\n\nContenido del método de prueba.\n"

// TestInstallAtomicPlanWrappers_GeneraAmbosAgentes cubre FR-028: los
// envoltorios nativos son una capa opcional, pero cuando se generan deben
// existir para los dos agentes con formato propio.
func TestInstallAtomicPlanWrappers_GeneraAmbosAgentes(t *testing.T) {
	root := t.TempDir()

	if err := InstallAtomicPlanWrappers(root, metodoDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanWrappers: %v", err)
	}

	claude := filepath.Join(root, ".claude", "skills", "atomic-decomposition", "SKILL.md")
	opencode := filepath.Join(root, ".opencode", "commands", "atomic-decomposition.md")

	for _, p := range []string{claude, opencode} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("falta el envoltorio %s: %v", p, err)
		}
	}
}

// TestInstallAtomicPlanWrappers_ContenidoEquivalente cubre la exigencia de
// FR-028 de que el contenido sea EQUIVALENTE al del método embebido: los
// envoltorios se generan desde la misma fuente, nunca se editan a mano. Es lo
// que evita que método y envoltorios diverjan (riesgo registrado en D6).
func TestInstallAtomicPlanWrappers_ContenidoEquivalente(t *testing.T) {
	root := t.TempDir()

	if err := InstallAtomicPlanWrappers(root, metodoDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanWrappers: %v", err)
	}

	for _, p := range []string{
		filepath.Join(root, ".claude", "skills", "atomic-decomposition", "SKILL.md"),
		filepath.Join(root, ".opencode", "commands", "atomic-decomposition.md"),
	} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("leer %s: %v", p, err)
		}
		if !strings.Contains(string(data), "Contenido del método de prueba.") {
			t.Errorf("%s no contiene el método embebido", p)
		}
	}
}

// TestInstallAtomicPlanWrappers_TienenFrontmatter verifica que cada envoltorio
// lleva los metadatos que su agente necesita para descubrirlo.
func TestInstallAtomicPlanWrappers_TienenFrontmatter(t *testing.T) {
	root := t.TempDir()

	if err := InstallAtomicPlanWrappers(root, metodoDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanWrappers: %v", err)
	}

	skill, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "atomic-decomposition", "SKILL.md"))
	if err != nil {
		t.Fatalf("leer SKILL.md: %v", err)
	}
	if !strings.HasPrefix(string(skill), "---\nname: atomic-decomposition\n") {
		t.Errorf("la habilidad de Claude Code necesita frontmatter con name; empieza por:\n%.80s", skill)
	}

	cmd, err := os.ReadFile(filepath.Join(root, ".opencode", "commands", "atomic-decomposition.md"))
	if err != nil {
		t.Fatalf("leer comando de opencode: %v", err)
	}
	if !strings.HasPrefix(string(cmd), "---\ndescription:") {
		t.Errorf("el comando de OpenCode necesita frontmatter con description; empieza por:\n%.80s", cmd)
	}
}

// TestInstallAtomicPlanWrappers_EsIdempotente cubre FR-029.
func TestInstallAtomicPlanWrappers_EsIdempotente(t *testing.T) {
	root := t.TempDir()
	skill := filepath.Join(root, ".claude", "skills", "atomic-decomposition", "SKILL.md")

	if err := InstallAtomicPlanWrappers(root, metodoDePrueba); err != nil {
		t.Fatalf("primera instalación: %v", err)
	}
	antes, err := os.Stat(skill)
	if err != nil {
		t.Fatalf("stat tras la primera instalación: %v", err)
	}

	if err := InstallAtomicPlanWrappers(root, metodoDePrueba); err != nil {
		t.Fatalf("segunda instalación: %v", err)
	}
	despues, err := os.Stat(skill)
	if err != nil {
		t.Fatalf("stat tras la segunda instalación: %v", err)
	}

	if !antes.ModTime().Equal(despues.ModTime()) {
		t.Error("la segunda instalación reescribió un archivo idéntico (debe ser idempotente)")
	}
}

// TestInstallAtomicPlanWrappers_ActualizaContenidoViejo cubre FR-030: si el
// método cambia, el envoltorio debe actualizarse sin dejar restos.
func TestInstallAtomicPlanWrappers_ActualizaContenidoViejo(t *testing.T) {
	root := t.TempDir()
	skill := filepath.Join(root, ".claude", "skills", "atomic-decomposition", "SKILL.md")

	if err := InstallAtomicPlanWrappers(root, "# Descomposición Atómica\n\nversión vieja\n"); err != nil {
		t.Fatalf("instalación previa: %v", err)
	}
	if err := InstallAtomicPlanWrappers(root, metodoDePrueba); err != nil {
		t.Fatalf("instalación nueva: %v", err)
	}

	data, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("leer SKILL.md: %v", err)
	}
	if strings.Contains(string(data), "versión vieja") {
		t.Error("quedaron restos de la versión anterior del método")
	}
	if !strings.Contains(string(data), "Contenido del método de prueba.") {
		t.Error("no se escribió la versión nueva del método")
	}
}

// TestInstallAtomicPlanWrappers_SinMetodoNoHaceNada degrada con gracia si la
// plantilla embebida no pudo cargarse: mejor no escribir nada que escribir un
// envoltorio vacío que el agente creería válido.
func TestInstallAtomicPlanWrappers_SinMetodoNoHaceNada(t *testing.T) {
	root := t.TempDir()

	if err := InstallAtomicPlanWrappers(root, "   "); err != nil {
		t.Fatalf("no debe fallar sin método: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "atomic-decomposition")); err == nil {
		t.Error("sin método no debe crearse el envoltorio")
	}
}
