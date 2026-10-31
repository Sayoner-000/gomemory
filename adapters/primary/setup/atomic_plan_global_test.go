package setup

import (
	"os"
	"path/filepath"
	"testing"
)

// homeFalso aísla la prueba del HOME real: escribir en el directorio personal de
// quien ejecuta los tests sería un efecto secundario inaceptable.
func homeFalso(t *testing.T, agentes ...string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows
	for _, a := range agentes {
		if err := os.MkdirAll(filepath.Join(home, a), 0o755); err != nil {
			t.Fatalf("crear %s: %v", a, err)
		}
	}
	return home
}

// TestInstallAtomicPlanGlobal_EscribeEnAmbosAgentes cubre FR-024: habilitar una
// vez en ámbito de usuario debe alcanzar a todos los proyectos.
func TestInstallAtomicPlanGlobal_EscribeEnAmbosAgentes(t *testing.T) {
	home := homeFalso(t, ".claude", filepath.Join(".config", "opencode"))

	if _, err := InstallAtomicPlanGlobal(metodoDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanGlobal: %v", err)
	}

	esperados := []string{
		filepath.Join(home, ".claude", "skills", "atomic-decomposition", "SKILL.md"),
		filepath.Join(home, ".config", "opencode", "commands", "atomic-decomposition.md"),
	}
	for _, p := range esperados {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("falta %s: %v", p, err)
		}
	}
}

// TestInstallAtomicPlanGlobal_NoCreaConfigDeAgentesNoUsados evita ensuciar el
// directorio personal: si la persona no usa OpenCode, no hay razón para crearle
// ~/.config/opencode.
func TestInstallAtomicPlanGlobal_NoCreaConfigDeAgentesNoUsados(t *testing.T) {
	home := homeFalso(t, ".claude")

	if _, err := InstallAtomicPlanGlobal(metodoDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanGlobal: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".config", "opencode")); err == nil {
		t.Error("no debe crearse la configuración de un agente que la persona no usa")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "atomic-decomposition", "SKILL.md")); err != nil {
		t.Errorf("sí debe escribirse para el agente presente: %v", err)
	}
}

// TestInstallAtomicPlanGlobal_EsIdempotente cubre FR-029 en ámbito global.
func TestInstallAtomicPlanGlobal_EsIdempotente(t *testing.T) {
	homeFalso(t, ".claude")

	primera, err := InstallAtomicPlanGlobal(metodoDePrueba)
	if err != nil {
		t.Fatalf("primera pasada: %v", err)
	}
	if len(primera) == 0 {
		t.Fatal("la primera pasada debía escribir algo")
	}

	segunda, err := InstallAtomicPlanGlobal(metodoDePrueba)
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	if len(segunda) != 0 {
		t.Errorf("la segunda pasada no debía escribir nada, escribió: %v", segunda)
	}
}
