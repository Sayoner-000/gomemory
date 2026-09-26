package tui

import (
	"os"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"

	"mem/domain"
)

func TestSave_ValidaYGuardaMemoriaDesdeFormulario(t *testing.T) {
	m := modeloDocs(t)
	m.screen = screenSave
	m.saveTitle = textinput.New()
	m.saveType = textinput.New()
	m.saveContent = textinput.New()
	m.saveFilepath = textinput.New()
	m.saveContent.SetValue("   ")
	next, _ := m.updateSave(keyMsg("enter"))
	m = next.(model)
	if m.screen != screenSave || !strings.Contains(m.saveErr, "obligatorio") {
		t.Fatalf("contenido vacío aceptado: %+v", m)
	}
	m.saveTitle.SetValue("Decisión de prueba")
	m.saveType.SetValue("decision")
	m.saveContent.SetValue("Persistir contenido desde la TUI")
	m.saveFilepath.SetValue("docs/architecture.md")
	next, _ = m.updateSave(keyMsg("enter"))
	m = next.(model)
	if m.screen != screenList || !m.saved || len(m.memories) != 1 {
		t.Fatalf("la memoria no se guardó: pantalla=%d saved=%t memorias=%d error=%q", m.screen, m.saved, len(m.memories), m.saveErr)
	}
	if m.memories[0].Type != domain.Decision || m.memories[0].Filepath != "docs/architecture.md" {
		t.Fatalf("memoria alterada: %+v", m.memories[0])
	}
}

func TestData_ExportarEImportarBundleDesdeTUI(t *testing.T) {
	m := modeloDocs(t)
	if _, err := m.memRepo.Insert(&domain.Memory{Project: m.project, Type: domain.Learning, Title: "Dato exportado", Content: "Contenido"}); err != nil {
		t.Fatal(err)
	}
	path, memories, _, err := m.exportMemories()
	if err != nil || memories != 1 {
		t.Fatalf("exportar: ruta=%q memorias=%d error=%v", path, memories, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("bundle ausente: %v", err)
	}
	m.screen = screenImport
	m.importPath = textinput.New()
	next, _ := m.updateImport(keyMsg("enter"))
	m = next.(model)
	if !strings.Contains(m.importErr, "ruta") {
		t.Fatalf("ruta vacía aceptada: %q", m.importErr)
	}
	m.importPath.SetValue(path)
	next, _ = m.updateImport(keyMsg("enter"))
	m = next.(model)
	if m.screen != screenConfig || !strings.Contains(m.statusMsg, "Import:") || len(m.memories) != 1 {
		t.Fatalf("importación: pantalla=%d status=%q error=%q memorias=%d", m.screen, m.statusMsg, m.importErr, len(m.memories))
	}
}
