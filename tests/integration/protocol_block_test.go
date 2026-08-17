package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallPreservesContentAroundLegacyProtocolBlock cubre FR-015 (feature
// 019): actualizar el bloque de protocolo a la versión vigente DEBE preservar
// íntegro el contenido propio de la persona, tanto el anterior como el
// POSTERIOR al bloque, y no debe dejar restos de la versión vieja. Antes de
// esta feature, composeAgentFile hacía out[:idx] + integration, descartando
// todo lo que viniera después del bloque legado (sin marcador de fin,
// cmd_install.go: protocolStart/composeAgentFile).
func TestInstallPreservesContentAroundLegacyProtocolBlock(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	const before = "# Mis notas\nTexto propio ANTES del bloque.\n"
	const legacyBlock = "<!-- gomemory-protocol-v6 -->\n" +
		"## Memoria Persistente (`mem`) — Protocolo Activo\n\n" +
		"contenido viejo del bloque, versión anterior a la vigente.\n\n" +
		"### Una subsección legada\ndetalle que debe desaparecer tras actualizar.\n"
	const after = "\n## Mis reglas personales\nTexto propio DESPUÉS del bloque. NO DEBE PERDERSE.\n"

	claudePath := filepath.Join(target, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(before+legacyBlock+after), 0644); err != nil {
		t.Fatalf("seed CLAUDE.md: %v", err)
	}

	runInstall := func() string {
		t.Helper()
		cmd := exec.Command(bin, "install", target)
		cmd.Dir = target
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("mem install: %v\n%s", err, out)
		}
		data, err := os.ReadFile(claudePath)
		if err != nil {
			t.Fatalf("read CLAUDE.md: %v", err)
		}
		return string(data)
	}

	got := runInstall()

	if !strings.Contains(got, "Texto propio ANTES del bloque.") {
		t.Error("el contenido ANTERIOR al bloque se perdió")
	}
	if !strings.Contains(got, "Texto propio DESPUÉS del bloque. NO DEBE PERDERSE.") {
		t.Error("el contenido POSTERIOR al bloque se perdió — este es el bug que esta feature corrige")
	}
	if !strings.Contains(got, "## Mis reglas personales") {
		t.Error("el encabezado propio posterior al bloque se perdió")
	}
	if strings.Contains(got, "gomemory-protocol-v6") {
		t.Error("no debe quedar ningún resto del marcador de la versión vieja")
	}
	if strings.Contains(got, "contenido viejo del bloque") {
		t.Error("no debe quedar contenido del bloque legado tras actualizar")
	}
	if strings.Count(got, "## Memoria Persistente") != 1 {
		t.Errorf("el bloque de protocolo debe aparecer exactamente una vez, apareció %d",
			strings.Count(got, "## Memoria Persistente"))
	}

	// Idempotencia: reinstalar sobre el resultado ya actualizado no debe
	// volver a tocar el archivo (mismo criterio que
	// TestWriteClaudeHooksIsIdempotent para los hooks).
	second := runInstall()
	if got != second {
		t.Error("reinstalar sobre un archivo ya actualizado no debe modificarlo (idempotencia)")
	}
}
