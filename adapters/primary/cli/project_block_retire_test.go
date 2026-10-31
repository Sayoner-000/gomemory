package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/domain"
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

func TestCopiaProyecto_NoReemplazaLaAnterior(t *testing.T) {
	primero := "uno\n\n" + bloqueCompleto
	dir, path := proyectoCon(t, "AGENTS.md", primero)
	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })
	segundo := "dos\n\n" + bloqueCompleto
	if err := os.WriteFile(path, []byte(segundo), 0o644); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })
	copias, err := filepath.Glob(filepath.Join(dir, ".memory", "backups", "agent-files", "AGENTS.md*"))
	if err != nil || len(copias) != 2 {
		t.Fatalf("copias = %v, %v", copias, err)
	}
	contenidos := map[string]bool{}
	for _, c := range copias {
		data, _ := os.ReadFile(c)
		contenidos[string(data)] = true
	}
	if !contenidos[primero] || !contenidos[segundo] {
		t.Fatalf("copias sin los dos originales: %v", contenidos)
	}
}

func TestCopiaProyecto_UninstallNoCopia(t *testing.T) {
	dir, _ := proyectoCon(t, "AGENTS.md", "antes\n\n"+bloqueCompleto)
	captureStdout(t, func() { removeIntegrationBlocks(dir) })
	if _, err := os.Stat(filepath.Join(dir, ".memory", "backups")); !os.IsNotExist(err) {
		t.Fatalf("uninstall creó copia: %v", err)
	}
}

func TestUninstallProyecto_FalloDeRetirada(t *testing.T) {
	dir, _ := proyectoCon(t, "AGENTS.md", "antes\n\n<!-- gomemory-protocol-v5 -->\ntexto\n")
	var err error
	captureStdout(t, func() { err = removeProjectIntegration(dir, true) })
	if err == nil || !strings.Contains(err.Error(), "AGENTS.md") {
		t.Fatalf("err = %v", err)
	}
}

func TestRetirada_TituloEsTextoPropio(t *testing.T) {
	for _, titulo := range []string{"# Instrucciones para agentes AI\n", "# Instrucciones para Claude Code\n"} {
		dir, path := proyectoCon(t, "AGENTS.md", titulo+"\n"+bloqueCompleto)
		captureStdout(t, func() { removeIntegrationBlocks(dir) })
		if got, err := os.ReadFile(path); err != nil || strings.TrimSpace(string(got)) != strings.TrimSpace(titulo) {
			t.Fatalf("%q: %q, %v", titulo, got, err)
		}
	}
}

func TestRetirada_FalloDeLectura(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "AGENTS.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	var err error
	captureStdout(t, func() { err = removeIntegrationBlocks(dir) })
	if err == nil || !strings.Contains(err.Error(), "AGENTS.md") {
		t.Fatalf("err = %v", err)
	}
	if ausente := t.TempDir(); removeIntegrationBlocks(ausente) != nil {
		t.Fatal("un archivo inexistente no es un fallo")
	}
}

func TestGlobalRemover_BaselineSolo(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<!-- gomemory-universal-agent-instructions-v1 -->\nx\n<!-- gomemory-universal-agent-instructions-end -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, present, apply := globalRemover(home, domain.MatrixCell{Agent: "claude", Kind: domain.KindInstructions, Scope: domain.ScopeUser, Path: []string{".claude", "CLAUDE.md"}, Legacy: true})
	if apply == nil || !present() {
		t.Fatal("el baseline solo debe entrar en el plan")
	}
}
