package setup

import (
	"os"
	"path/filepath"
	"regexp"
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

// T012 (feature 038, FR-006): instalación global repetida en el destino Codex
// con contenido ajeno preservado, HOME aislado y sin tocar el AGENTS.md real.
// El compositor de prueba preserva el contenido previo, como exige el contrato
// a composeAgentFile.
func TestInstallAtomicPlanGlobal_CodexPreservaAjenoYEsIdempotente(t *testing.T) {
	home := homeFalso(t, ".codex")
	destino := filepath.Join(home, ".codex", "AGENTS.md")
	ajeno := "# Notas propias\n\nPreferencias del equipo.\n"
	if err := os.WriteFile(destino, []byte(ajeno+"<!-- gomemory-protocol-v8 -->\n## Memoria Persistente\nprotocolo viejo\n## Sección propia posterior\nNota posterior.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	componer := func(existing string) (string, bool) {
		const bloque = "<!-- gomemory-protocol-v9 -->\nprotocolo vigente\n"
		if strings.Contains(existing, "gomemory-protocol-v9") {
			return existing, false
		}
		if idx := strings.Index(existing, "<!-- gomemory-protocol-v"); idx != -1 {
			antes := strings.TrimRight(existing[:idx], "\n")
			resto := existing[idx:]
			// Misma regla que protocolEnd (cli): delimitador final si existe;
			// si no, se saltan la línea del marcador y la del título propio y
			// se corta en el siguiente encabezado de nivel 2; si no, EOF.
			corte := len(existing)
			if fin := strings.Index(resto, "<!-- gomemory-protocol-end -->"); fin != -1 {
				corte = idx + fin + len("<!-- gomemory-protocol-end -->")
			} else if lineas := strings.SplitAfterN(resto, "\n", 3); len(lineas) == 3 {
				tras := len(resto) - len(lineas[2])
				if h2 := regexp.MustCompile(`(?m)^## `).FindStringIndex(lineas[2]); h2 != nil {
					corte = idx + tras + h2[0]
				}
			}
			return antes + "\n" + bloque + strings.TrimLeft(existing[corte:], "\n"), true
		}
		return strings.TrimRight(existing, "\n") + "\n" + bloque, true
	}

	if _, err := InstallAtomicPlanGlobal(metodoDePrueba, componer); err != nil {
		t.Fatalf("InstallAtomicPlanGlobal: %v", err)
	}
	una, err := os.ReadFile(destino)
	if err != nil {
		t.Fatalf("falta %s: %v", destino, err)
	}
	for _, want := range []string{"Preferencias del equipo.", "## Sección propia posterior\nNota posterior.", "gomemory-protocol-v9"} {
		if !strings.Contains(string(una), want) {
			t.Errorf("la instalación perdió %q", want)
		}
	}
	if strings.Contains(string(una), "protocolo viejo") || strings.Contains(string(una), "## Memoria Persistente") {
		t.Error("quedó contenido del protocolo anterior")
	}

	if _, err := InstallAtomicPlanGlobal(metodoDePrueba, componer); err != nil {
		t.Fatalf("segunda instalación: %v", err)
	}
	dos, _ := os.ReadFile(destino)
	if string(una) != string(dos) {
		t.Error("la segunda instalación alteró el contenido: no es idempotente")
	}
}
