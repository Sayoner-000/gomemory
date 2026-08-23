package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Este archivo verificaba antes TestInstallPreservesContentAroundLegacyProtocolBlock:
// que actualizar el bloque de protocolo dentro de AGENTS.md/CLAUDE.md preservara
// el contenido propio de la persona alrededor.
//
// La feature 021 retiró ese comportamiento a propósito: `mem install` ya no
// escribe el bloque de protocolo en archivos del proyecto, porque el servidor
// MCP lo entrega verbatim en la respuesta initialize (cmd_mcp.go, ServerOptions.
// Instructions). El archivo era una SEGUNDA copia del mismo texto.
//
// composeAgentFile/protocolStart/protocolEnd siguen existiendo y con sus tests
// propios (cmd_install_protocol_test.go): las usa `setup-mcp --scope global`
// para el ámbito de USUARIO, que esta feature no toca.
//
// Lo que se verifica ahora es el comportamiento nuevo: retirada con respaldo.

func runInstallEn(t *testing.T, bin, target string) string {
	t.Helper()
	cmd := exec.Command(bin, "install", target)
	cmd.Dir = target
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mem install: %v\n%s", err, out)
	}
	return string(out)
}

// TestInstallEliminaArchivosDeAgenteYRespalda cubre FR-016/FR-017: retirar los
// archivos de instrucciones es una operación destructiva autorizada, y el
// respaldo previo es lo que la hace responsable.
func TestInstallEliminaArchivosDeAgenteYRespalda(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	const propio = "# Mis notas\n\nTexto propio que escribí yo y no debe evaporarse.\n"
	for _, nombre := range []string{"AGENTS.md", "CLAUDE.md"} {
		if err := os.WriteFile(filepath.Join(target, nombre), []byte(propio+nombre), 0644); err != nil {
			t.Fatalf("preparar %s: %v", nombre, err)
		}
	}

	out := runInstallEn(t, bin, target)

	for _, nombre := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(target, nombre)); err == nil {
			t.Errorf("%s sigue en la raíz tras instalar", nombre)
		}
		respaldo := filepath.Join(target, ".memory", "backups", "agent-files", nombre)
		datos, err := os.ReadFile(respaldo)
		if err != nil {
			t.Fatalf("falta el respaldo de %s en %s: %v", nombre, respaldo, err)
		}
		if !strings.Contains(string(datos), "Texto propio que escribí yo") {
			t.Errorf("el respaldo de %s no conserva el contenido original", nombre)
		}
	}

	if !strings.Contains(out, "backups") {
		t.Errorf("la instalación debe informar dónde quedaron los respaldos.\nSalida:\n%s", out)
	}
}

// TestInstallNoGeneraArtefactos cubre FR-011/FR-012/FR-013 y el escenario §2
// del quickstart: la razón de ser de la feature.
func TestInstallNoGeneraArtefactos(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	runInstallEn(t, bin, target)

	prohibidos := []string{
		"AGENTS.md", "CLAUDE.md", "CLAUDE.txt",
		"speckit-constitution-gen.md",
		".windsurf", ".cline",
	}
	for _, artefacto := range prohibidos {
		if _, err := os.Stat(filepath.Join(target, artefacto)); err == nil {
			t.Errorf("mem install creó %q en la raíz del proyecto", artefacto)
		}
	}
}

// TestInstallEsIdempotente: una segunda pasada no reintroduce nada ni vuelve a
// informar respaldos que ya no existen.
func TestInstallEsIdempotente(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	runInstallEn(t, bin, target)
	out := runInstallEn(t, bin, target)

	if strings.Contains(out, "respaldado") {
		t.Errorf("la segunda instalación informó respaldos sin haber nada que respaldar.\nSalida:\n%s", out)
	}
	for _, artefacto := range []string{"AGENTS.md", "CLAUDE.md", "speckit-constitution-gen.md"} {
		if _, err := os.Stat(filepath.Join(target, artefacto)); err == nil {
			t.Errorf("la segunda instalación reintrodujo %q", artefacto)
		}
	}
}

// TestInstallMensajeFinal cubre FR-014: el mensaje debe explicar dónde viven
// ahora las reglas, y no referirse a archivos que ya no se generan.
func TestInstallMensajeFinal(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	out := runInstallEn(t, bin, target)

	for _, esperado := range []string{"get_context()", "/constitution"} {
		if !strings.Contains(out, esperado) {
			t.Errorf("el mensaje final debe mencionar %q.\nSalida:\n%s", esperado, out)
		}
	}
	for _, obsoleto := range []string{"al leer AGENTS.md", "setup-mcp --scope global"} {
		if strings.Contains(out, obsoleto) {
			t.Errorf("el mensaje final sigue mencionando %q, que ya no aplica", obsoleto)
		}
	}
}
