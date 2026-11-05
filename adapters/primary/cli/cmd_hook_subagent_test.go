package cli

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mem/domain"
)

// HCD-026: el subagente recibía el bootstrap completo del agente principal
// (~5.900 caracteres, ~40 tools, protocolo de guardado y end_session). Estas
// pruebas fijan el aviso mínimo de solo lectura que lo sustituye.

func TestSubagentNotice_SinEscrituraNiSesion(t *testing.T) {
	for _, provider := range []bool{false, true} {
		notice := subagentNotice(provider)
		prohibidas := []string{"PRIMERA ACCIÓN", "OCTOPUS"}
		for _, tool := range []string{
			domain.ToolSaveMemory, domain.ToolForgetMemory, domain.ToolJudgeMemories,
			domain.ToolStartSession, domain.ToolSaveSessionSummary, domain.ToolIndexProject,
		} {
			prohibidas = append(prohibidas, "mcp__gomemory__"+tool)
		}
		prohibidas = append(prohibidas, domain.MCPPrefixed("mcp__gomemory__", domain.MCPReviewTools)...)
		prohibidas = append(prohibidas, domain.MCPPrefixed("mcp__gomemory__", domain.MCPContextPackTools)...)
		prohibidas = append(prohibidas, domain.MCPPrefixed("mcp__gomemory__", domain.MCPOctopusTools)...)
		for _, p := range prohibidas {
			if strings.Contains(notice, p) {
				t.Errorf("provider=%v: el aviso del subagente no debe contener %q:\n%s", provider, p, notice)
			}
		}
		// end_session solo puede aparecer para prohibirlo, nunca en el select.
		if strings.Contains(notice, "mcp__gomemory__"+domain.ToolEndSession) {
			t.Errorf("provider=%v: el aviso no debe precargar end_session:\n%s", provider, notice)
		}
	}
}

func TestSubagentNotice_PrecargaSoloLectura(t *testing.T) {
	notice := subagentNotice(true)
	for _, tool := range []string{
		domain.ToolSearchMemories, domain.ToolGetMemory,
		domain.ToolSearchCode, domain.ToolGetSymbol, domain.ToolListDependencies,
	} {
		if !strings.Contains(notice, "mcp__gomemory__"+tool) {
			t.Errorf("el aviso debe precargar %s:\n%s", tool, notice)
		}
	}
	for _, name := range domain.MCPPrefixed(domain.CodebaseMemoryMCPPrefix, domain.CodebaseMemoryMCPDiscoveryTools) {
		if !strings.Contains(notice, name) {
			t.Errorf("con proveedor activo el aviso debe precargar %s:\n%s", name, notice)
		}
	}
	for _, rt := range []string{"", "agy", "opencode"} {
		if n := subagentNoticeFor(rt, true); !strings.Contains(n, "grafo antes que grep") {
			t.Errorf("runtime %q: el aviso debe exigir el grafo antes que grep:\n%s", rt, n)
		}
	}
	if sin := subagentNotice(false); strings.Contains(sin, domain.CodebaseMemoryMCPPrefix) {
		t.Errorf("sin proveedor el aviso no debe nombrar codebase-memory-mcp:\n%s", sin)
	}
}

func TestSubagentNotice_Acotado(t *testing.T) {
	const tope = 1200
	if n := utf8.RuneCountInString(subagentNotice(true)); n > tope {
		t.Errorf("el aviso del subagente ocupa %d caracteres; el tope es %d", n, tope)
	}
}
