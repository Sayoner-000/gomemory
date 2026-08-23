package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func proyectoLegado(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, n := range []string{"AGENTS.md", "CLAUDE.md", "CLAUDE.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("# "+n+"\n\nTexto propio que escribí yo.\n"), 0o644); err != nil {
			t.Fatalf("preparar %s: %v", n, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "speckit-constitution-gen.md"), []byte("constitución vieja\n"), 0o644); err != nil {
		t.Fatalf("preparar constitución: %v", err)
	}

	escribirJSON(t, dir, ".windsurf/mcp_config.json", map[string]any{
		"mcpServers": map[string]any{"gomemory": map[string]any{"command": "./mem"}},
	})
	escribirJSON(t, dir, ".cline/mcp_settings.json", map[string]any{
		"mcpServers": map[string]any{
			"gomemory": map[string]any{"command": "./mem"},
			"otro":     map[string]any{"command": "otra-cosa"},
		},
	})
	return dir
}

func escribirJSON(t *testing.T, dir, rel string, v any) {
	t.Helper()
	ruta := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	datos, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(ruta, datos, 0o644); err != nil {
		t.Fatalf("escribir %s: %v", rel, err)
	}
}

// FR-016/FR-017: retirar es destructivo y autorizado; el respaldo es lo que lo
// hace responsable.
func TestCleanup_RespaldaYRetiraLosArchivosDeAgente(t *testing.T) {
	dir := proyectoLegado(t)

	out := captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })

	for _, n := range []string{"AGENTS.md", "CLAUDE.md", "CLAUDE.txt"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			t.Errorf("%s sigue en la raíz", n)
		}
		datos, err := os.ReadFile(filepath.Join(dir, ".memory", "backups", "agent-files", n))
		if err != nil {
			t.Fatalf("falta el respaldo de %s: %v", n, err)
		}
		if !strings.Contains(string(datos), "Texto propio que escribí yo") {
			t.Errorf("el respaldo de %s no conserva el contenido", n)
		}
	}
	if !strings.Contains(out, "respaldado en") {
		t.Errorf("debe informarse la ruta de cada respaldo:\n%s", out)
	}
}

// FR-018: si el respaldo no se puede escribir, el original NO se borra. Es
// preferible dejar un archivo obsoleto que perder texto de alguien.
func TestCleanup_SinRespaldoNoBorra(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("contenido irreemplazable\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	// .memory existe como ARCHIVO: MkdirAll del directorio de respaldo fallará.
	if err := os.WriteFile(filepath.Join(dir, ".memory"), []byte("no soy un directorio"), 0o644); err != nil {
		t.Fatalf("preparar bloqueo: %v", err)
	}

	out := captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })

	datos, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal("CLAUDE.md se borró pese a que el respaldo falló: es exactamente lo que FR-018 prohíbe")
	}
	if string(datos) != "contenido irreemplazable\n" {
		t.Errorf("el contenido cambió: %q", datos)
	}
	if !strings.Contains(out, "se conserva el archivo") {
		t.Errorf("debe informarse por qué no se retiró:\n%s", out)
	}
}

// FR-019: la copia de la constitución se elimina sin respaldo — es copia
// literal de una plantilla embebida.
func TestCleanup_EliminaLaConstitucionGenerada(t *testing.T) {
	dir := proyectoLegado(t)

	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })

	if _, err := os.Stat(filepath.Join(dir, "speckit-constitution-gen.md")); err == nil {
		t.Error("la copia de la constitución sigue en la raíz")
	}
}

// FR-020: conservador con lo ajeno. Solo se retira la carpeta cuando el archivo
// era enteramente nuestro.
func TestCleanup_DesregistraSinDestruirLoAjeno(t *testing.T) {
	dir := proyectoLegado(t)

	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })

	if _, err := os.Stat(filepath.Join(dir, ".windsurf")); err == nil {
		t.Error(".windsurf solo tenía gomemory: debió eliminarse entera")
	}

	datos, err := os.ReadFile(filepath.Join(dir, ".cline", "mcp_settings.json"))
	if err != nil {
		t.Fatal(".cline tenía otro servidor: no debió eliminarse")
	}
	var cfg map[string]any
	if err := json.Unmarshal(datos, &cfg); err != nil {
		t.Fatalf("el JSON quedó corrupto: %v", err)
	}
	servidores := cfg["mcpServers"].(map[string]any)
	if _, tiene := servidores["gomemory"]; tiene {
		t.Error("la entrada de gomemory sigue registrada")
	}
	if _, tiene := servidores["otro"]; !tiene {
		t.Error("se destruyó la configuración de un servidor ajeno")
	}
}

// FR-021: un JSON que no se puede interpretar es de alguien más.
func TestCleanup_JSONInvalidoQuedaIntacto(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, ".cline", "mcp_settings.json")
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const basura = "esto no es json {{{\n"
	if err := os.WriteFile(ruta, []byte(basura), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	out := captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })

	datos, err := os.ReadFile(ruta)
	if err != nil || string(datos) != basura {
		t.Errorf("el archivo se modificó o desapareció: %q, %v", datos, err)
	}
	if !strings.Contains(out, "se deja intacto") {
		t.Errorf("debe informarse que no se tocó:\n%s", out)
	}
}

// FR-022: idempotente y silenciosa cuando no hay nada que limpiar.
func TestCleanup_SinArtefactosEsSilenciosa(t *testing.T) {
	dir := proyectoLegado(t)

	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })
	segunda := captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })

	if strings.TrimSpace(segunda) != "" {
		t.Errorf("la segunda pasada no debe imprimir nada:\n%s", segunda)
	}

	limpio := t.TempDir()
	if out := captureStdout(t, func() { cleanupLegacyArtifacts(limpio, ".memory") }); strings.TrimSpace(out) != "" {
		t.Errorf("sobre un proyecto limpio no debe imprimir nada:\n%s", out)
	}
}

// FR-023: los archivos de reglas que el instalador nunca creó son de la
// persona y no se tocan.
func TestCleanup_NoTocaArchivosQueNuncaGeneramos(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{".cursorrules", ".windsurfrules"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("reglas propias\n"), 0o644); err != nil {
			t.Fatalf("preparar %s: %v", n, err)
		}
	}

	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })

	for _, n := range []string{".cursorrules", ".windsurfrules"} {
		datos, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil || string(datos) != "reglas propias\n" {
			t.Errorf("%s fue modificado o eliminado: %q, %v", n, datos, err)
		}
	}
}
