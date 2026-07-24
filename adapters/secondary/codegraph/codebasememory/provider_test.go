package codebasememory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mem/domain"
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

// Historia 3 (feature 010): con múltiples proveedores candidatos, cada uno
// necesita SU PROPIO archivo de snapshot — si compartieran uno solo, el
// último en refrescar pisaría los datos buenos de otro proveedor disponible.
func TestSnapshotPath_DiffersPerBinOverride(t *testing.T) {
	dir := t.TempDir()
	a := New(realRoot, dir, "/ruta/a/proveedor-a")
	b := New(realRoot, dir, "/ruta/a/proveedor-b")
	if a.snapshotPath() == b.snapshotPath() {
		t.Fatalf("dos proveedores distintos no deberían compartir archivo de snapshot: %q", a.snapshotPath())
	}
}

// Retrocompatibilidad: el caso por defecto (sin binOverride, autodetección
// en PATH) debe seguir usando el nombre de archivo LEGADO — así una base
// existente de un solo proveedor no invalida su cache al actualizar.
func TestSnapshotPath_DefaultUsesLegacyFilename(t *testing.T) {
	dir := t.TempDir()
	p := New(realRoot, dir, "")
	want := filepath.Join(dir, snapshotFile)
	if p.snapshotPath() != want {
		t.Fatalf("snapshotPath() = %q, se esperaba el legado %q", p.snapshotPath(), want)
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

// ─── Historia 1 (feature 010): anotación de impacto por archivo ──────────
//
// get_architecture NO expone "file" por hotspot (verificado contra el CLI
// real: solo trae name/qualified_name/fan_in) — search_code sí lo expone,
// casando por qualified_name. Por eso el archivo se resuelve aparte, no
// leyendo un campo que la fixture real no tiene.

func TestHotspotQualifiedNames_MatchesRealFixture(t *testing.T) {
	out, err := os.ReadFile("testdata/get_architecture.json")
	if err != nil {
		t.Fatal(err)
	}
	qn := hotspotQualifiedNames(out)

	want := "Users-josegomezj-home-rcw-go_memory.adapters.secondary.persistence.db.FindRoot"
	if got := qn["FindRoot"]; got != want {
		t.Fatalf("qualified_name de FindRoot = %q, se esperaba %q", got, want)
	}
	if _, ok := qn["WriteFile"]; !ok {
		t.Fatal("esperaba que WriteFile apareciera en el mapa (es hotspot en la fixture real)")
	}
}

func TestHotspotQualifiedNames_Garbage(t *testing.T) {
	qn := hotspotQualifiedNames([]byte("no es json"))
	if len(qn) != 0 {
		t.Fatalf("JSON inválido debería devolver mapa vacío, hubo %d entradas", len(qn))
	}
}

// Sin binario, resolveHotspotFiles no debe tocar los hotspots ni lanzar:
// degradación silenciosa, igual que el resto del adaptador.
func TestResolveHotspotFiles_SinBinario(t *testing.T) {
	p := &Provider{root: realRoot, memDir: t.TempDir(), binPath: ""}
	hotspots := []domain.CodeHotspot{{Name: "FindRoot", FanIn: 10}}
	qualifiedNames := map[string]string{"FindRoot": "algo.db.FindRoot"}

	p.resolveHotspotFiles(context.Background(), "proj", hotspots, qualifiedNames)

	if hotspots[0].File != "" {
		t.Fatalf("sin binario, File debería quedar vacío, fue %q", hotspots[0].File)
	}
}

func writeTestSnapshot(t *testing.T, dir string, arch *domain.CodeArchitecture) {
	t.Helper()
	snap := domain.CodeProviderSnapshot{
		Provider:     ProviderName,
		RootPath:     realRoot,
		Available:    true,
		CheckedAt:    time.Now(),
		Architecture: arch,
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, snapshotFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImpactFor_MatchesByFile(t *testing.T) {
	dir := t.TempDir()
	writeTestSnapshot(t, dir, &domain.CodeArchitecture{
		Hotspots: []domain.CodeHotspot{
			{Name: "FindRoot", FanIn: 10, File: "adapters/secondary/persistence/db.go"},
		},
	})
	p := New(realRoot, dir, "")

	ann, ok := p.ImpactFor("adapters/secondary/persistence/db.go")
	if !ok {
		t.Fatal("esperaba match por archivo")
	}
	if !ann.Hotspot || ann.Symbol != "FindRoot" || ann.FanIn != 10 {
		t.Fatalf("anotación inesperada: %+v", ann)
	}
}

func TestImpactFor_NoMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestSnapshot(t, dir, &domain.CodeArchitecture{
		Hotspots: []domain.CodeHotspot{
			{Name: "FindRoot", FanIn: 10, File: "adapters/secondary/persistence/db.go"},
		},
	})
	p := New(realRoot, dir, "")

	if _, ok := p.ImpactFor("un/archivo/cualquiera.go"); ok {
		t.Fatal("no debería matchear un archivo que no está en los hotspots")
	}
}

func TestImpactFor_SinSnapshot(t *testing.T) {
	p := New(realRoot, t.TempDir(), "")
	if _, ok := p.ImpactFor("cualquier/archivo.go"); ok {
		t.Fatal("sin snapshot en disco, ImpactFor debería devolver false")
	}
}

// ─── Historia 2 (feature 010): documento único de ADR (manage_adr) ───────

func TestParseGetADRResponse_NoADR(t *testing.T) {
	out, err := os.ReadFile("testdata/manage_adr_get_no_adr.json")
	if err != nil {
		t.Fatal(err)
	}
	content, ok := parseGetADRResponse(out)
	if !ok {
		t.Fatal("una respuesta 'no_adr' es válida (documento vacío), no debería fallar el parseo")
	}
	if content != "" {
		t.Fatalf("esperaba content vacío, hubo %q", content)
	}
}

func TestParseGetADRResponse_WithContent(t *testing.T) {
	out := []byte(`{"content":"## PURPOSE\n\ntexto\n","status":"ok"}`)
	content, ok := parseGetADRResponse(out)
	if !ok || content != "## PURPOSE\n\ntexto\n" {
		t.Fatalf("content = %q, ok = %v", content, ok)
	}
}

func TestParseGetADRResponse_Garbage(t *testing.T) {
	if _, ok := parseGetADRResponse([]byte("no es json")); ok {
		t.Fatal("JSON inválido debería devolver false")
	}
}

func TestGetDocument_SinBinario(t *testing.T) {
	p := &Provider{root: realRoot, memDir: t.TempDir(), binPath: ""}
	if _, err := p.GetDocument(context.Background()); err == nil {
		t.Fatal("sin binario, GetDocument debería devolver error (proveedor no disponible)")
	}
}

func TestUpdateDocument_SinBinario(t *testing.T) {
	p := &Provider{root: realRoot, memDir: t.TempDir(), binPath: ""}
	if err := p.UpdateDocument(context.Background(), "## PURPOSE\n"); err == nil {
		t.Fatal("sin binario, UpdateDocument debería devolver error")
	}
}
