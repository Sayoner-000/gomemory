package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
)

// T006 (feature 038, FR-009): fija la superficie real de recuperación propia.
// search_code entrega líneas de ubicación; get_symbol entrega firma, paquete,
// ubicación y vecinos, pero NO el cuerpo literal. Sin compresor configurado el
// resultado va íntegro y sin marcas de omisión. Candado de comportamiento:
// ningún cambio de esta feature debe presentar get_symbol como equivalente a
// un cuerpo.
func TestCodeToolsPolicy_SuperficieEstructuralSinCuerpos(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module demo\n\ngo 1.22\n")
	write("app/app.go", `package app

// Saludo arma un saludo.
func Saludo(nombre string) string { return prefijo() + nombre }

func prefijo() string { return "hola " }
`)

	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	deps := &Deps{Root: root, Project: "proj", CodeGraphRepo: persistence.NewCodeGraphRepository(db)}
	cs := connectMCPTestSession(t, deps, root, "proj")
	call := func(tool string, args map[string]any) string {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v res=%s", tool, err, toolText(res))
		}
		return toolText(res)
	}

	if got := call("index_project", map[string]any{"force": true}); !strings.Contains(got, "Índice actualizado") {
		t.Fatalf("index_project: %q", got)
	}
	search := call("search_code", map[string]any{"query": "Saludo"})
	if !strings.Contains(search, "Saludo") || !strings.Contains(search, "app/app.go") {
		t.Fatalf("search_code sin ubicación:\n%s", search)
	}
	if strings.Contains(search, "⟦mem⟧") {
		t.Fatalf("search_code sin compresor no debe llevar marcas de omisión:\n%s", search)
	}

	def := call("get_symbol", map[string]any{"name": "Saludo"})
	for _, want := range []string{"Saludo", "Paquete: app", "Ubicación: app/app.go"} {
		if !strings.Contains(def, want) {
			t.Fatalf("get_symbol sin %q:\n%s", want, def)
		}
	}
	if strings.Contains(def, `return prefijo()`) {
		t.Fatalf("get_symbol no entrega el cuerpo literal:\n%s", def)
	}
}
