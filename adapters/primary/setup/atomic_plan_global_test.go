package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// composeDePrueba simula lo que inyecta el paquete cli: añade el bloque una vez
// y reporta "sin cambios" si ya está (misma semántica que composeAgentFile).
func composeDePrueba(existing string) (string, bool) {
	const bloque = "<!-- gomemory-protocol-v6 -->\nprotocolo con disparador de modo plan\n"
	if strings.Contains(existing, "gomemory-protocol-v6") {
		return existing, false
	}
	return strings.TrimRight(existing, "\n") + "\n" + bloque, true
}

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

	if _, err := InstallAtomicPlanGlobal(metodoDePrueba, composeDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanGlobal: %v", err)
	}

	esperados := []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".claude", "skills", "atomic-decomposition", "SKILL.md"),
		filepath.Join(home, ".config", "opencode", "AGENTS.md"),
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

	if _, err := InstallAtomicPlanGlobal(metodoDePrueba, composeDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanGlobal: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".config", "opencode")); err == nil {
		t.Error("no debe crearse la configuración de un agente que la persona no usa")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "CLAUDE.md")); err != nil {
		t.Errorf("sí debe escribirse para el agente presente: %v", err)
	}
}

// TestInstallAtomicPlanGlobal_PreservaContenidoDelUsuario es la garantía más
// importante de esta ruta: CLAUDE.md es el archivo de instrucciones personales
// de la persona para TODOS sus proyectos. Solo puede tocarse el bloque marcado
// de gomemory.
func TestInstallAtomicPlanGlobal_PreservaContenidoDelUsuario(t *testing.T) {
	home := homeFalso(t, ".claude")
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	previo := "# Mis instrucciones\n\nSiempre responde en español.\nNo uses emojis.\n"
	if err := os.WriteFile(claudeMD, []byte(previo), 0o644); err != nil {
		t.Fatalf("escribir contenido previo: %v", err)
	}

	if _, err := InstallAtomicPlanGlobal(metodoDePrueba, composeDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanGlobal: %v", err)
	}

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("leer CLAUDE.md: %v", err)
	}
	for _, linea := range []string{"# Mis instrucciones", "Siempre responde en español.", "No uses emojis."} {
		if !strings.Contains(string(data), linea) {
			t.Errorf("se perdió contenido personal del usuario: %q", linea)
		}
	}
	if !strings.Contains(string(data), "gomemory-protocol-v6") {
		t.Error("no se añadió el bloque de protocolo")
	}
}

// TestInstallAtomicPlanGlobal_EsIdempotente cubre FR-029 en ámbito global.
func TestInstallAtomicPlanGlobal_EsIdempotente(t *testing.T) {
	homeFalso(t, ".claude")

	primera, err := InstallAtomicPlanGlobal(metodoDePrueba, composeDePrueba)
	if err != nil {
		t.Fatalf("primera pasada: %v", err)
	}
	if len(primera) == 0 {
		t.Fatal("la primera pasada debía escribir algo")
	}

	segunda, err := InstallAtomicPlanGlobal(metodoDePrueba, composeDePrueba)
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	if len(segunda) != 0 {
		t.Errorf("la segunda pasada no debía escribir nada, escribió: %v", segunda)
	}
}

// TestInstallAtomicPlanGlobal_SinMetodoSoloEscribeElProtocolo: sin método
// embebido no debe quedar un envoltorio vacío, pero el disparador sí debe
// instalarse — es lo que activa la funcionalidad.
func TestInstallAtomicPlanGlobal_SinMetodoSoloEscribeElProtocolo(t *testing.T) {
	home := homeFalso(t, ".claude")

	if _, err := InstallAtomicPlanGlobal("  ", composeDePrueba); err != nil {
		t.Fatalf("InstallAtomicPlanGlobal: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".claude", "CLAUDE.md")); err != nil {
		t.Errorf("el bloque de protocolo debe escribirse igual: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "atomic-decomposition")); err == nil {
		t.Error("sin método no debe crearse el envoltorio")
	}
}
