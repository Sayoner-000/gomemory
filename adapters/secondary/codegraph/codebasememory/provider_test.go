package codebasememory

import (
	"context"
	"os"
	"testing"
)

// Fixtures capturados de la salida REAL del CLI del proveedor (stdout), no
// inventados: `codebase-memory-mcp cli list_projects` / `... get_architecture`.
const realRoot = "/Users/josegomezj/home/rcw/go_memory"

func TestParseProjectName_MatchesByRootPath(t *testing.T) {
	out, err := os.ReadFile("testdata/list_projects.json")
	if err != nil {
		t.Fatal(err)
	}
	name, ok := parseProjectName(out, realRoot)
	if !ok {
		t.Fatal("esperaba casar el proyecto por root_path")
	}
	if name != "Users-josegomezj-home-rcw-go_memory" {
		t.Fatalf("nombre inesperado: %q", name)
	}
}

func TestParseProjectName_UnknownRoot(t *testing.T) {
	out, _ := os.ReadFile("testdata/list_projects.json")
	if _, ok := parseProjectName(out, "/ruta/que/no/existe"); ok {
		t.Fatal("root desconocido debería devolver false")
	}
}

func TestParseProjectName_Garbage(t *testing.T) {
	if _, ok := parseProjectName([]byte("no es json"), realRoot); ok {
		t.Fatal("JSON inválido debería devolver false")
	}
}

func TestParseArchitecture_Condensa(t *testing.T) {
	out, err := os.ReadFile("testdata/get_architecture.json")
	if err != nil {
		t.Fatal(err)
	}
	arch, ok := parseArchitecture(out)
	if !ok {
		t.Fatal("parse falló sobre fixture real")
	}
	if arch.TotalNodes == 0 || arch.TotalEdges == 0 {
		t.Fatalf("totales vacíos: nodes=%d edges=%d", arch.TotalNodes, arch.TotalEdges)
	}
	if len(arch.Clusters) == 0 || len(arch.Clusters) > maxClusters {
		t.Fatalf("clusters=%d (esperaba 1..%d)", len(arch.Clusters), maxClusters)
	}
	if len(arch.Hotspots) == 0 || len(arch.Hotspots) > maxHotspots {
		t.Fatalf("hotspots=%d", len(arch.Hotspots))
	}
	if len(arch.Languages) == 0 {
		t.Fatal("esperaba al menos un lenguaje")
	}
	// Clusters ordenados desc por members.
	for i := 1; i < len(arch.Clusters); i++ {
		if arch.Clusters[i-1].Members < arch.Clusters[i].Members {
			t.Fatal("clusters no ordenados desc por members")
		}
	}
	// top_nodes acotado.
	for _, c := range arch.Clusters {
		if len(c.TopNodes) > maxTopNodes {
			t.Fatalf("top_nodes sin acotar: %d", len(c.TopNodes))
		}
	}
}

func TestParseArchitecture_Garbage(t *testing.T) {
	if _, ok := parseArchitecture([]byte("<html>nope</html>")); ok {
		t.Fatal("JSON inválido debería devolver false")
	}
}

func TestSnapshot_SinArchivo(t *testing.T) {
	p := New(realRoot, t.TempDir(), "")
	snap := p.Snapshot()
	if snap.Available {
		t.Fatal("sin snapshot en disco debería ser Available=false")
	}
	if !snap.Stale(snapshotTTL) {
		t.Fatal("snapshot vacío debería ser stale")
	}
}

// Refresh sin binario NO debe indexar ni lanzar: escribe un snapshot
// Available=false con CheckedAt seteado (degradación silenciosa).
func TestRefresh_SinBinario(t *testing.T) {
	dir := t.TempDir()
	p := &Provider{root: realRoot, memDir: dir, binPath: ""}
	p.Refresh(context.Background())
	snap := p.Snapshot()
	if snap.Available {
		t.Fatal("sin binario esperaba Available=false")
	}
	if snap.CheckedAt.IsZero() {
		t.Fatal("CheckedAt debería estar seteado tras Refresh")
	}
	if snap.Stale(snapshotTTL) {
		t.Fatal("snapshot recién refrescado no debería ser stale")
	}
}
