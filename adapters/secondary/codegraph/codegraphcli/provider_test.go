package codegraphcli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"mem/domain"
)

func TestParseStatus_ExigeIndiceActualDelProyecto(t *testing.T) {
	root := "/tmp/project"
	valid := fmt.Sprintf(`{"initialized":true,"projectPath":%q,"nodeCount":8297,"edgeCount":22677,"languages":["go","typescript"],"index":{"state":"complete"},"pendingChanges":{"added":0,"modified":0,"removed":0}}`, root)
	arch, ok := parseStatus([]byte(valid), root)
	if !ok || arch.TotalNodes != 8297 || arch.TotalEdges != 22677 || len(arch.Hotspots) != 0 || len(arch.LanguageNames) != 2 {
		t.Fatalf("status válido = %#v, %v", arch, ok)
	}
	for name, raw := range map[string]string{
		"otro proyecto":   `{"initialized":true,"projectPath":"/tmp/other","nodeCount":2,"index":{"state":"complete"}}`,
		"sin inicializar": `{"initialized":false,"projectPath":"/tmp/project","index":{"state":"complete"}}`,
		"pendiente":       `{"initialized":true,"projectPath":"/tmp/project","index":{"state":"complete"},"pendingChanges":{"modified":1}}`,
		"incompleto":      `{"initialized":true,"projectPath":"/tmp/project","index":{"state":"building"}}`,
		"obsoleto":        `{"initialized":true,"projectPath":"/tmp/project","index":{"state":"complete","reindexRecommended":true}}`,
		"inválido":        `{`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := parseStatus([]byte(raw), root); ok {
				t.Fatal("un índice ausente o desactualizado no debe declararse disponible")
			}
		})
	}
}

func TestSnapshot_EsIndependienteYNoBloquea(t *testing.T) {
	root := t.TempDir()
	memDir := filepath.Join(root, ".memory")
	p := New(root, memDir, "codegraph")
	p.writeSnapshot(domain.CodeProviderSnapshot{
		Provider: ProviderName, RootPath: root, Project: root, ProjectArg: "projectPath",
		Available: true, Architecture: &domain.CodeArchitecture{TotalNodes: 9},
	})
	snap := p.Snapshot()
	if !snap.Available || snap.Architecture.TotalNodes != 9 || snap.ProjectArg != "projectPath" {
		t.Fatalf("snapshot = %#v", snap)
	}
	if info, err := os.Stat(p.snapshotPath()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos del snapshot: %v, %v", info, err)
	}
	other := New(root, memDir, "/usr/local/bin/codegraph")
	if other.Snapshot().Available {
		t.Fatal("dos identidades no deben compartir snapshot")
	}
}

func TestLiveCodeGraph_RefrescaIndiceInstalado(t *testing.T) {
	if os.Getenv("GOMEMORY_CODEGRAPH_LIVE") != "1" {
		t.Skip("smoke test opt-in del CodeGraph instalado")
	}
	if _, err := exec.LookPath("codegraph"); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	p := New(root, t.TempDir(), "codegraph")
	p.Refresh(context.Background())
	snap := p.Snapshot()
	if !snap.Available || snap.Architecture == nil || snap.Architecture.TotalNodes == 0 || snap.Project != root {
		t.Fatalf("CodeGraph instalado no produjo un snapshot válido: %#v", snap)
	}
}
