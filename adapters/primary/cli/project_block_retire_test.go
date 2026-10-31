package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const bloqueCompleto = "<!-- gomemory-protocol-v10 -->\n## Memoria Persistente\nx\n<!-- gomemory-protocol-end -->\n"

func proyectoCon(t *testing.T, nombre, contenido string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, nombre)
	if err := os.WriteFile(path, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func TestLimpiarArchivos_SinMarcadoresNoToca(t *testing.T) {
	const propio = "# Reglas del equipo\n\nNada de gomemory.\n"
	dir, path := proyectoCon(t, "AGENTS.md", propio)
	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })
	if got, err := os.ReadFile(path); err != nil || string(got) != propio {
		t.Fatalf("archivo alterado: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".memory", "backups")); !os.IsNotExist(err) {
		t.Fatal("no debe crear copia de un archivo sin bloques")
	}
}

func TestLimpiarArchivos_RetiraBloqueConCopia(t *testing.T) {
	original := "antes\n\n" + bloqueCompleto + "\ndespués\n"
	dir, path := proyectoCon(t, "CLAUDE.md", original)
	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })
	if got, _ := os.ReadFile(path); string(got) != "antes\n\ndespués\n" {
		t.Fatalf("resultado: %q", got)
	}
	copia, err := os.ReadFile(filepath.Join(dir, ".memory", "backups", "agent-files", "CLAUDE.md"))
	if err != nil || string(copia) != original {
		t.Fatalf("copia = %q, %v", copia, err)
	}
}

func TestLimpiarArchivos_SinCierreNoTocaEInforma(t *testing.T) {
	original := "antes\n\n<!-- gomemory-protocol-v5 -->\ntexto\n"
	dir, path := proyectoCon(t, "AGENTS.md", original)
	out := captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("archivo alterado: %q", got)
	}
	if !strings.Contains(out, "a mano") {
		t.Fatalf("salida sin instrucción: %s", out)
	}
}

func TestLimpiarArchivos_SoloBloqueSeElimina(t *testing.T) {
	dir, path := proyectoCon(t, "AGENTS.md", bloqueCompleto)
	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("un archivo que era solo el bloque debe eliminarse")
	}
}

func TestRetiradaProyecto_ConservaTextoPosterior(t *testing.T) {
	dir, path := proyectoCon(t, "AGENTS.md", "antes\n\n"+bloqueCompleto+"\ndespués propio\n")
	captureStdout(t, func() { removeIntegrationBlocks(dir) })
	if got, _ := os.ReadFile(path); string(got) != "antes\n\ndespués propio\n" {
		t.Fatalf("resultado: %q", got)
	}
}

func TestRetiradaProyecto_SinCierreNoToca(t *testing.T) {
	original := "antes\n\n<!-- gomemory-protocol-v5 -->\ntexto propio\n"
	dir, path := proyectoCon(t, "AGENTS.md", original)
	out := captureStdout(t, func() { removeIntegrationBlocks(dir) })
	if got, _ := os.ReadFile(path); string(got) != original || !strings.Contains(out, "a mano") {
		t.Fatalf("archivo %q, salida %s", got, out)
	}
}

func TestRemoveProtocolBlockFile_SinCierreNoToca(t *testing.T) {
	original := "antes\n\n<!-- gomemory-protocol-v5 -->\ntexto propio\n"
	_, path := proyectoCon(t, "CLAUDE.md", original)
	changed, err := removeProtocolBlockFile(path)
	if changed || err == nil {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("archivo alterado: %q", got)
	}
}

func TestRemoveProtocolBlockFile_EncabezadoSinMarcadorNoSeToca(t *testing.T) {
	original := "# Mío\n\n## Memoria Persistente del equipo\n\nNotas.\n"
	_, path := proyectoCon(t, "CLAUDE.md", original)
	if changed, err := removeProtocolBlockFile(path); changed || err != nil {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("archivo alterado: %q", got)
	}
}

func TestRetiradaProyecto_RulesLegadosNoSeTocan(t *testing.T) {
	original := "propio\n\n" + bloqueCompleto
	dir, path := proyectoCon(t, ".cursorrules", original)
	captureStdout(t, func() { removeIntegrationBlocks(dir) })
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("archivo alterado: %q", got)
	}
}
