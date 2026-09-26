package domain

import (
	"errors"
	"testing"
)

// FR-014: proyectos → configuración global de agentes → almacén → binario.
func TestUninstallPlan_OrdenFR014(t *testing.T) {
	p := UninstallPlan{Items: []UninstallItem{
		{Category: CategoryBinary, Path: "/bin/mem"},
		{Category: CategoryMemory, Path: "/data"},
		{Category: CategoryAgentConfig, Path: "~/.claude.json"},
		{Category: CategoryProjectFiles, Path: "/p1/.memory"},
		{Category: CategoryAgentConfig, Path: "~/.codex/config.toml"},
		{Category: CategoryProjectFiles, Path: "/p2/.memory"},
	}}
	p.Sort()
	want := []string{"/p1/.memory", "/p2/.memory", "~/.claude.json", "~/.codex/config.toml", "/data", "/bin/mem"}
	for i, it := range p.Items {
		if it.Path != want[i] {
			t.Fatalf("posición %d = %s; orden esperado %v", i, it.Path, want)
		}
	}
}

// FR-015: un paso fallido queda en warn con su detalle y no detiene el resto.
func TestUninstallPlan_ExecuteContinuaTrasUnWarn(t *testing.T) {
	p := UninstallPlan{Items: []UninstallItem{
		{Category: CategoryProjectFiles, Path: "a"},
		{Category: CategoryAgentConfig, Path: "b", Manual: "quita b a mano"},
		{Category: CategoryBinary, Path: "c"},
	}}
	var visitados []string
	p.Execute(func(it *UninstallItem) error {
		visitados = append(visitados, it.Path)
		if it.Path == "b" {
			return errors.New("permiso denegado")
		}
		return nil
	})
	if len(visitados) != 3 {
		t.Fatalf("todos los pasos deben ejecutarse: %v", visitados)
	}
	got := []ItemResult{p.Items[0].Result, p.Items[1].Result, p.Items[2].Result}
	if got[0] != ResultOK || got[1] != ResultWarn || got[2] != ResultOK {
		t.Fatalf("resultados = %v", got)
	}
	if p.Items[1].Detail != "permiso denegado" || p.Items[1].Manual != "quita b a mano" {
		t.Errorf("el warn conserva el motivo y el comando manual: %+v", p.Items[1])
	}
	if p.Warnings() != 1 {
		t.Errorf("Warnings() = %d", p.Warnings())
	}
}

// Todo elemento nace pendiente; en --dry-run se queda así.
func TestUninstallPlan_NaceEnPending(t *testing.T) {
	it := NewUninstallItem(CategoryMemory, "/data/projects/x", KindDir)
	if it.Result != ResultPending {
		t.Fatalf("resultado inicial = %v", it.Result)
	}
}

func TestShouldSkipScanDir(t *testing.T) {
	for _, d := range []string{".git", "node_modules", "vendor", ".venv", "target", "dist", "build", "Library", ".cache", ".npm", ".cargo"} {
		if !ShouldSkipScanDir(d) {
			t.Errorf("%s debe omitirse en el escaneo", d)
		}
	}
	for _, d := range []string{"proyectos", "home", "gomemory", ".memory", "src"} {
		if ShouldSkipScanDir(d) {
			t.Errorf("%s no debe omitirse", d)
		}
	}
	if UninstallScanMaxDepth != 6 {
		t.Errorf("profundidad = %d; aclaración 2026-09-26: 6", UninstallScanMaxDepth)
	}
}
